// Command controller runs the ReportRequest controller.
//
// It starts a controller-runtime manager, registers our API types, and wires up the
// ReportRequestReconciler. Configuration (worker image, MinIO/mock-ai coordinates) comes
// from environment variables set on the controller Deployment.
package main

import (
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	reportsv1alpha1 "github.com/workshop/controller/api/v1alpha1"
	"github.com/workshop/controller/controllers"
)

var scheme = runtime.NewScheme()

func init() {
	// Register built-in types (Job, Pod, Secret, ...) and our custom types.
	_ = clientgoscheme.AddToScheme(scheme)
	_ = reportsv1alpha1.AddToScheme(scheme)
}

func main() {
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))
	setupLog := ctrl.Log.WithName("setup")

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsserver.Options{BindAddress: ":8081"},
		HealthProbeBindAddress: ":8082",
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	reconciler := &controllers.ReportRequestReconciler{
		Client:         mgr.GetClient(),
		WorkerImage:    controllers.EnvOrDefault("WORKER_IMAGE", "ghcr.io/your-org/k8s-controller-workshop/worker:latest"),
		MockAIURL:      controllers.EnvOrDefault("MOCK_AI_URL", "http://mock-ai.report-queue.svc.cluster.local:8080"),
		MinioEndpoint:  controllers.EnvOrDefault("MINIO_ENDPOINT", "minio.report-queue.svc.cluster.local:9000"),
		MinioBucket:    controllers.EnvOrDefault("MINIO_BUCKET", "reports"),
		MinioSecret:    controllers.EnvOrDefault("MINIO_SECRET", "minio-credentials"),
		ProcessingSecs: controllers.EnvOrDefault("PROCESSING_DELAY_SECONDS", "5"),
	}
	if err := reconciler.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ReportRequest")
		os.Exit(1)
	}

	_ = mgr.AddHealthzCheck("healthz", healthz.Ping)
	_ = mgr.AddReadyzCheck("readyz", healthz.Ping)

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
