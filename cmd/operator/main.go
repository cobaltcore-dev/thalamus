// Copyright SAP SE
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"flag"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	inferencev1 "sigs.k8s.io/gateway-api-inference-extension/api/v1"
	gatewayv1 "sigs.k8s.io/gateway-api/apis/v1"

	agentgatewayv1alpha1 "github.com/agentgateway/agentgateway/controller/api/v1alpha1/agentgateway"

	"github.com/cobaltcore-dev/thalamus/api/v1alpha1"
	"github.com/cobaltcore-dev/thalamus/internal/operator"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1alpha1.AddToScheme(scheme))
	utilruntime.Must(gatewayv1.Install(scheme))
	utilruntime.Must(inferencev1.Install(scheme))
	utilruntime.Must(agentgatewayv1alpha1.Install(scheme))
}

func main() {
	var metricsAddr string
	var probeAddr string
	var watchNamespace string
	var gatewayName string
	var gatewayListener string
	var routerServiceAccountName string
	var leaderElection bool
	var leaderElectionID string
	var leaderElectionNamespace string

	flag.StringVar(&metricsAddr, "metrics-bind-address", "0", "The address the metrics endpoint binds to. Use :8080 for HTTP or 0 to disable.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.StringVar(&watchNamespace, "namespace", "", "Namespace to watch. Must match the namespace the operator is deployed in.")
	flag.StringVar(&gatewayName, "gateway-name", "", "Name of the gateway the operator attaches its routes to. Required.")
	flag.StringVar(&gatewayListener, "gateway-listener", "", "Name of the gateway listener the operator attaches its routes to. Required.")
	flag.StringVar(&routerServiceAccountName, "router-service-account-name", "thalamus-router", "Name of the ServiceAccount the router pods run as.")
	flag.BoolVar(&leaderElection, "leader-election", true, "Enable leader election for the controller manager.")
	flag.StringVar(&leaderElectionID, "leader-election-id", "thalamus-operator", "Name of the Lease resource used for leader election.")
	flag.StringVar(&leaderElectionNamespace, "leader-election-namespace", "", "Namespace of the Lease resource. Defaults to --namespace, then POD_NAMESPACE.")
	// TODO: set to false before GA
	opts := zap.Options{Development: true}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	if gatewayName == "" {
		setupLog.Error(nil, "--gateway-name must be set")
		os.Exit(1)
	}
	if gatewayListener == "" {
		setupLog.Error(nil, "--gateway-listener must be set")
		os.Exit(1)
	}
	if routerServiceAccountName == "" {
		setupLog.Error(nil, "--router-service-account-name must be set")
		os.Exit(1)
	}

	if leaderElectionNamespace == "" {
		leaderElectionNamespace = watchNamespace
	}
	if leaderElectionNamespace == "" {
		leaderElectionNamespace = os.Getenv("POD_NAMESPACE")
	}
	if leaderElection && leaderElectionNamespace == "" {
		setupLog.Error(nil, "--leader-election-namespace (or --namespace / POD_NAMESPACE env) must be set")
		os.Exit(1)
	}

	cacheOpts := cache.Options{}
	if watchNamespace != "" {
		cacheOpts.DefaultNamespaces = map[string]cache.Config{watchNamespace: {}}
	}

	mgrOpts := ctrl.Options{
		Scheme:                        scheme,
		Metrics:                       metricsserver.Options{BindAddress: metricsAddr},
		HealthProbeBindAddress:        probeAddr,
		Cache:                         cacheOpts,
		LeaderElection:                leaderElection,
		LeaderElectionID:              leaderElectionID,
		LeaderElectionNamespace:       leaderElectionNamespace,
		LeaderElectionReleaseOnCancel: true,
	}
	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), mgrOpts)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	if err := (&operator.ModelReconciler{
		Client:                   mgr.GetClient(),
		Scheme:                   mgr.GetScheme(),
		GatewayName:              gatewayName,
		ListenerName:             gatewayListener,
		RouterServiceAccountName: routerServiceAccountName,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "model")
		os.Exit(1)
	}

	if err := (&operator.ModelListReconciler{
		Client:       mgr.GetClient(),
		Scheme:       mgr.GetScheme(),
		GatewayName:  gatewayName,
		ListenerName: gatewayListener,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "model-list")
		os.Exit(1)
	}

	if err := (&operator.BodyRoutingReconciler{
		Client:       mgr.GetClient(),
		Scheme:       mgr.GetScheme(),
		GatewayName:  gatewayName,
		ListenerName: gatewayListener,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "body-routing")
		os.Exit(1)
	}

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
