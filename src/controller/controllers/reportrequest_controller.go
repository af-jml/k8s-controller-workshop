// Package controllers contains the ReportRequest reconciler — the heart of the workshop.
//
// The reconcile loop embodies the core Kubernetes idea: continuously drive the actual
// state of the world toward the desired state declared in the ReportRequest spec.
package controllers

import (
	"context"
	"fmt"
	"os"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	reportsv1alpha1 "github.com/workshop/controller/api/v1alpha1"
)

// ReportRequestReconciler reconciles ReportRequest objects.
type ReportRequestReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// WorkerImage is the image used for the per-request Job.
	WorkerImage string
	// Environment forwarded to the worker so it can reach mock-ai and MinIO.
	MockAIURL      string
	MinioEndpoint  string
	MinioBucket    string
	MinioSecret    string // name of the Secret holding MinIO credentials
	ProcessingSecs string
}

// Reconcile is called whenever a ReportRequest (or a Job it owns) changes.
func (r *ReportRequestReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	// 1. Fetch the ReportRequest. If it's gone, there's nothing to do.
	var rr reportsv1alpha1.ReportRequest
	if err := r.Get(ctx, req.NamespacedName, &rr); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// 2. Terminal phases need no further work.
	if rr.Status.Phase == reportsv1alpha1.PhaseCompleted || rr.Status.Phase == reportsv1alpha1.PhaseFailed {
		return ctrl.Result{}, nil
	}

	// 3. Has the worker Job been created yet?
	jobName := rr.Status.JobName
	if jobName == "" {
		jobName = fmt.Sprintf("report-%s", rr.Name)
	}
	var job batchv1.Job
	jobErr := r.Get(ctx, types.NamespacedName{Namespace: rr.Namespace, Name: jobName}, &job)

	switch {
	case apierrors.IsNotFound(jobErr):
		// No Job yet: create one and move to Processing.
		objectKey := fmt.Sprintf("%s-%d.pdf", rr.Name, rr.Generation)
		newJob := r.buildJob(&rr, jobName, objectKey)
		if err := ctrl.SetControllerReference(&rr, newJob, r.Scheme); err != nil {
			return ctrl.Result{}, err
		}
		if err := r.Create(ctx, newJob); err != nil {
			if apierrors.IsAlreadyExists(err) {
				return ctrl.Result{Requeue: true}, nil
			}
			return ctrl.Result{}, err
		}
		l.Info("created worker Job", "job", jobName, "objectKey", objectKey)

		now := metav1.Now()
		rr.Status.Phase = reportsv1alpha1.PhaseProcessing
		rr.Status.Message = "Worker Job created; generating report"
		rr.Status.JobName = jobName
		rr.Status.PDFObjectKey = objectKey
		rr.Status.StartTime = &now
		rr.Status.ObservedGeneration = rr.Generation
		return ctrl.Result{}, r.Status().Update(ctx, &rr)

	case jobErr != nil:
		return ctrl.Result{}, jobErr
	}

	// 4. Job exists: inspect its status.
	switch {
	case job.Status.Succeeded > 0:
		now := metav1.Now()
		rr.Status.Phase = reportsv1alpha1.PhaseCompleted
		rr.Status.Message = "Report generated successfully"
		rr.Status.CompletionTime = &now
		l.Info("report completed", "name", rr.Name)
		return ctrl.Result{}, r.Status().Update(ctx, &rr)

	case job.Status.Failed > 0:
		now := metav1.Now()
		rr.Status.Phase = reportsv1alpha1.PhaseFailed
		rr.Status.Message = "Worker Job failed; see Job logs"
		rr.Status.CompletionTime = &now
		l.Info("report failed", "name", rr.Name)
		return ctrl.Result{}, r.Status().Update(ctx, &rr)

	default:
		// Still running: ensure phase reflects Processing and wait for the next event.
		if rr.Status.Phase != reportsv1alpha1.PhaseProcessing {
			rr.Status.Phase = reportsv1alpha1.PhaseProcessing
			rr.Status.Message = "Worker Job is running"
			return ctrl.Result{}, r.Status().Update(ctx, &rr)
		}
		return ctrl.Result{}, nil
	}
}

// buildJob constructs the worker Job for a ReportRequest.
func (r *ReportRequestReconciler) buildJob(rr *reportsv1alpha1.ReportRequest, jobName, objectKey string) *batchv1.Job {
	backoffLimit := int32(2)
	ttl := int32(3600) // clean the Job up an hour after it finishes

	return &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: rr.Namespace,
			Labels: map[string]string{
				"app":                         "report-worker",
				"reports.workshop.io/request": rr.Name,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttl,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "report-worker"},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:            "worker",
							Image:           r.WorkerImage,
							ImagePullPolicy: corev1.PullIfNotPresent,
							Env: []corev1.EnvVar{
								{Name: "REPORT_TITLE", Value: rr.Spec.Title},
								{Name: "REPORT_DATASET", Value: rr.Spec.Dataset},
								{Name: "REPORT_INSTRUCTIONS", Value: rr.Spec.Instructions},
								{Name: "OBJECT_KEY", Value: objectKey},
								{Name: "MOCK_AI_URL", Value: r.MockAIURL},
								{Name: "MINIO_ENDPOINT", Value: r.MinioEndpoint},
								{Name: "MINIO_BUCKET", Value: r.MinioBucket},
								{Name: "PROCESSING_DELAY_SECONDS", Value: r.ProcessingSecs},
								{
									Name: "MINIO_ACCESS_KEY",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: r.MinioSecret},
											Key:                  "accesskey",
										},
									},
								},
								{
									Name: "MINIO_SECRET_KEY",
									ValueFrom: &corev1.EnvVarSource{
										SecretKeyRef: &corev1.SecretKeySelector{
											LocalObjectReference: corev1.LocalObjectReference{Name: r.MinioSecret},
											Key:                  "secretkey",
										},
									},
								},
							},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("50m"),
									corev1.ResourceMemory: resource.MustParse("64Mi"),
								},
							},
						},
					},
				},
			},
		},
	}
}

// SetupWithManager wires the reconciler to watch ReportRequests and the Jobs they own.
func (r *ReportRequestReconciler) SetupWithManager(mgr ctrl.Manager) error {
	r.Scheme = mgr.GetScheme()
	return ctrl.NewControllerManagedBy(mgr).
		For(&reportsv1alpha1.ReportRequest{}).
		Owns(&batchv1.Job{}).
		Complete(r)
}

// envOrDefault is a tiny helper used by main to read configuration.
func EnvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
