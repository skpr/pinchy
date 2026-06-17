package main

import (
	"flag"
	"os"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	"github.com/skpr/pinchy/apis/pinchy/v1beta1"
	"github.com/skpr/pinchy/controllers/environment"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		panic(err)
	}

	if err := corev1.AddToScheme(scheme); err != nil {
		panic(err)
	}

	if err := batchv1.AddToScheme(scheme); err != nil {
		panic(err)
	}

	if err := v1beta1.AddToScheme(scheme); err != nil {
		panic(err)
	}
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	flag.StringVar(&metricsAddr, "metrics-addr", "0", "The address the metric endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "enable-leader-election", false,
		"Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	flag.Parse()

	ctrl.SetLogger(zap.New(func(o *zap.Options) {
		o.Development = true
	}))

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress: metricsAddr,
		},
		LeaderElection: enableLeaderElection,
		WebhookServer: webhook.NewServer(webhook.Options{
			Port: 9443,
		}),
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err = (&environment.Reconciler{
		Client:   mgr.GetClient(),
		Log:      ctrl.Log.WithName(environment.ControllerName),
		Recorder: mgr.GetEventRecorderFor(environment.ControllerName),
		Scheme:   mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "name", environment.ControllerName)
		os.Exit(1)
	}

	setupLog.Info("starting manager")

	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
