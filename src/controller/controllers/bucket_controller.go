// Package controllers — the Bucket reconciler.
//
// This is the workshop's "Kubernetes as a cloud API" example. Unlike the ReportRequest
// controller (which creates a Kubernetes Job and lets that do the work), this controller
// provisions a resource in an *external* system — MinIO — directly from its reconcile loop:
//
//   - ensure the bucket exists,
//   - apply the requested access policy (private / public-read),
//   - apply a storage quota (via MinIO's admin API),
//   - and, because owner references cannot garbage-collect things outside Kubernetes, use a
//     finalizer to optionally delete the real bucket when the resource is removed.
//
// The result: `kubectl apply -f bucket.yaml` makes a real bucket appear, the same way a
// cloud provider's operator makes a real S3 bucket appear. The cluster *is* the API.
package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	madmin "github.com/minio/madmin-go/v3"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	storagev1alpha1 "github.com/workshop/controller/api/storage/v1alpha1"
)

// bucketFinalizer is added to every Bucket so the controller gets a chance to clean up the
// real MinIO bucket before the Kubernetes object is removed.
const bucketFinalizer = "storage.workshop.io/finalizer"

// BucketReconciler reconciles Bucket objects against a MinIO instance.
type BucketReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// MinIO connection details. The controller needs real credentials here (unlike the
	// ReportRequest controller, which only passes a Secret *name* down to the worker).
	MinioEndpoint  string
	MinioAccessKey string
	MinioSecretKey string
	MinioUseSSL    bool
}

// Reconcile drives the actual MinIO state toward the Bucket spec.
func (r *BucketReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	var b storagev1alpha1.Bucket
	if err := r.Get(ctx, req.NamespacedName, &b); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Apply defaults for the fields we read below (the CRD also defaults these server-side,
	// but defaulting here keeps the controller correct on its own).
	policy := b.Spec.AccessPolicy
	if policy == "" {
		policy = storagev1alpha1.AccessPrivate
	}
	delPolicy := b.Spec.DeletionPolicy
	if delPolicy == "" {
		delPolicy = storagev1alpha1.DeletionRetain
	}

	// --- Deletion path -------------------------------------------------------------------
	// A non-zero DeletionTimestamp means someone ran `kubectl delete`. Kubernetes will not
	// actually remove the object while our finalizer is present, so we do cleanup first.
	if !b.DeletionTimestamp.IsZero() {
		if !containsString(b.Finalizers, bucketFinalizer) {
			return ctrl.Result{}, nil
		}
		if delPolicy == storagev1alpha1.DeletionDelete {
			l.Info("deletionPolicy=Delete: removing bucket in MinIO", "bucket", b.Spec.BucketName)
			if err := r.deleteBucket(ctx, b.Spec.BucketName); err != nil {
				l.Error(err, "failed to delete bucket in MinIO", "bucket", b.Spec.BucketName)
				r.setFailed(ctx, &b, fmt.Sprintf("delete failed: %v", err))
				return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
			}
		}
		b.Finalizers = removeString(b.Finalizers, bucketFinalizer)
		if err := r.Update(ctx, &b); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{}, nil
	}

	// --- Ensure finalizer ----------------------------------------------------------------
	// Add it *before* creating anything external, so a delete that races provisioning still
	// triggers cleanup. The Update re-queues us, so we return here.
	if !containsString(b.Finalizers, bucketFinalizer) {
		b.Finalizers = append(b.Finalizers, bucketFinalizer)
		return ctrl.Result{}, r.Update(ctx, &b)
	}

	// --- Provision path ------------------------------------------------------------------
	quotaBytes := int64(0)
	if b.Spec.Quota != "" {
		q, err := resource.ParseQuantity(b.Spec.Quota)
		if err != nil {
			r.setFailed(ctx, &b, fmt.Sprintf("invalid quota %q: %v", b.Spec.Quota, err))
			return ctrl.Result{}, nil // bad input; requeueing won't help until spec changes
		}
		quotaBytes = q.Value()
	}

	if err := r.provision(ctx, b.Spec.BucketName, policy, quotaBytes); err != nil {
		l.Error(err, "provisioning failed", "bucket", b.Spec.BucketName)
		r.setFailed(ctx, &b, err.Error())
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	now := metav1.Now()
	b.Status.Phase = storagev1alpha1.PhaseReady
	b.Status.Message = "Bucket provisioned in MinIO"
	b.Status.AppliedPolicy = policy
	b.Status.QuotaBytes = quotaBytes
	b.Status.Endpoint = fmt.Sprintf("%s/%s", r.MinioEndpoint, b.Spec.BucketName)
	b.Status.ObservedGeneration = b.Generation
	if b.Status.ReadyTime == nil {
		b.Status.ReadyTime = &now
	}
	l.Info("bucket ready", "bucket", b.Spec.BucketName, "policy", policy, "quotaBytes", quotaBytes)
	return ctrl.Result{}, r.Status().Update(ctx, &b)
}

// provision makes the desired state real in MinIO: bucket, policy, quota.
func (r *BucketReconciler) provision(ctx context.Context, bucket string, policy storagev1alpha1.AccessPolicy, quotaBytes int64) error {
	cl, err := r.s3Client()
	if err != nil {
		return fmt.Errorf("minio client: %w", err)
	}

	exists, err := cl.BucketExists(ctx, bucket)
	if err != nil {
		return fmt.Errorf("checking bucket: %w", err)
	}
	if !exists {
		if err := cl.MakeBucket(ctx, bucket, minio.MakeBucketOptions{}); err != nil {
			return fmt.Errorf("creating bucket: %w", err)
		}
	}

	// Access policy. SetBucketPolicy with an empty string removes any anonymous policy.
	switch policy {
	case storagev1alpha1.AccessPublicRead:
		if err := cl.SetBucketPolicy(ctx, bucket, publicReadPolicy(bucket)); err != nil {
			return fmt.Errorf("setting public-read policy: %w", err)
		}
	default:
		if err := cl.SetBucketPolicy(ctx, bucket, ""); err != nil {
			return fmt.Errorf("clearing policy: %w", err)
		}
	}

	// Quota is not part of the S3 API — it's a MinIO admin feature, so it needs the admin
	// client (the same thing `mc admin bucket quota` uses).
	adm, err := r.adminClient()
	if err != nil {
		return fmt.Errorf("minio admin client: %w", err)
	}
	quota := &madmin.BucketQuota{}
	if quotaBytes > 0 {
		// Set both Quota (deprecated) and Size for compatibility across MinIO versions.
		quota = &madmin.BucketQuota{Type: madmin.HardQuota, Quota: uint64(quotaBytes), Size: uint64(quotaBytes)}
	}
	if err := adm.SetBucketQuota(ctx, bucket, quota); err != nil {
		return fmt.Errorf("setting quota: %w", err)
	}
	return nil
}

// deleteBucket empties and removes a bucket. RemoveBucket requires an empty bucket, so we
// delete all objects first.
func (r *BucketReconciler) deleteBucket(ctx context.Context, bucket string) error {
	cl, err := r.s3Client()
	if err != nil {
		return fmt.Errorf("minio client: %w", err)
	}
	exists, err := cl.BucketExists(ctx, bucket)
	if err != nil {
		return fmt.Errorf("checking bucket: %w", err)
	}
	if !exists {
		return nil
	}
	objectsCh := cl.ListObjects(ctx, bucket, minio.ListObjectsOptions{Recursive: true})
	for removeErr := range cl.RemoveObjects(ctx, bucket, objectsCh, minio.RemoveObjectsOptions{}) {
		if removeErr.Err != nil {
			return fmt.Errorf("removing object %s: %w", removeErr.ObjectName, removeErr.Err)
		}
	}
	if err := cl.RemoveBucket(ctx, bucket); err != nil {
		return fmt.Errorf("removing bucket: %w", err)
	}
	return nil
}

// setFailed records a Failed phase on the resource and logs any status-update error.
func (r *BucketReconciler) setFailed(ctx context.Context, b *storagev1alpha1.Bucket, msg string) {
	b.Status.Phase = storagev1alpha1.PhaseFailed
	b.Status.Message = msg
	b.Status.ObservedGeneration = b.Generation
	if err := r.Status().Update(ctx, b); err != nil {
		log.FromContext(ctx).Error(err, "failed to update Bucket status")
	}
}

func (r *BucketReconciler) s3Client() (*minio.Client, error) {
	return minio.New(r.MinioEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(r.MinioAccessKey, r.MinioSecretKey, ""),
		Secure: r.MinioUseSSL,
	})
}

func (r *BucketReconciler) adminClient() (*madmin.AdminClient, error) {
	return madmin.New(r.MinioEndpoint, r.MinioAccessKey, r.MinioSecretKey, r.MinioUseSSL)
}

// publicReadPolicy returns the standard S3 bucket policy granting anonymous GetObject.
func publicReadPolicy(bucket string) string {
	policy := map[string]any{
		"Version": "2012-10-17",
		"Statement": []map[string]any{
			{
				"Effect":    "Allow",
				"Principal": map[string]any{"AWS": []string{"*"}},
				"Action":    []string{"s3:GetObject"},
				"Resource":  []string{fmt.Sprintf("arn:aws:s3:::%s/*", bucket)},
			},
		},
	}
	out, _ := json.Marshal(policy)
	return string(out)
}

// SetupWithManager wires the reconciler to watch Bucket objects.
func (r *BucketReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Scheme = mgr.GetScheme()
	return ctrl.NewControllerManagedBy(mgr).
		For(&storagev1alpha1.Bucket{}).
		Complete(r)
}

// containsString / removeString are tiny finalizer helpers. (controller-runtime ships
// controllerutil.{Contains,Add,Remove}Finalizer, but a finalizer is just a string in a
// slice — doing it by hand keeps this workshop self-contained and dependency-light.)
func containsString(slice []string, s string) bool {
	for _, item := range slice {
		if item == s {
			return true
		}
	}
	return false
}

func removeString(slice []string, s string) []string {
	out := slice[:0]
	for _, item := range slice {
		if item != s {
			out = append(out, item)
		}
	}
	return out
}
