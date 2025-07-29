package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/certwatcher"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/metrics/filters"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	crdv1 "pelotech/ot-sync-operator/api/v1"
	"pelotech/ot-sync-operator/internal/controller"
	datasyncservice "pelotech/ot-sync-operator/internal/datasync-service"
	dynamicconfigservice "pelotech/ot-sync-operator/internal/dynamic-config-service"
	errorservice "pelotech/ot-sync-operator/internal/error-service"
	generalutils "pelotech/ot-sync-operator/internal/general-utils"
	kubectlclient "pelotech/ot-sync-operator/internal/kubectl-client"
	resourcemanager "pelotech/ot-sync-operator/internal/resource-manager"

	snapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	cdiv1beta1 "kubevirt.io/containerized-data-importer-api/pkg/apis/core/v1beta1"
	// +kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(crdv1.AddToScheme(scheme))
	utilruntime.Must(cdiv1beta1.AddToScheme(scheme))
	utilruntime.Must(snapshotv1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme
}

const (
	metricsAddrDesc = "The address the metrics endpoint binds to. " +
		"Use :8443 for HTTPS or :8080 for HTTP," +
		" or leave as 0 to disable the metrics service."

	enableLEDesc = "Enable leader election for controller manager." +
		" Enabling this will ensure there is only one active controller manager."

	secureMetricsDesc = "If set, the metrics endpoint is served securely via HTTPS." +
		" Use --metrics-secure=false to use HTTP instead."

	operatorConfigMapNameDesc = "This configmap contains values used in the controller logic." +
		" It allows for configuration of behavior"

	probeAddrDesc           = "The address the probe endpoint binds to."
	webhookCertPathDesc     = "The directory that contains the webhook certificate."
	webhookCertNameDesc     = "The name of the webhook certificate file."
	webhookCertKeyDesc      = "The name of the webhook key file."
	metricsCertPathDesc     = "The directory that contains the metrics server certificate."
	metricsCertNameDesc     = "The name of the metrics server certificate file."
	metricsCertKeyDesc      = "The name of the metrics server key file."
	enableHTTP2Desc         = "If set, HTTP/2 will be enabled for the metrics and webhook servers"
	runningInClusterDesc    = "Whether or not we running inside the cluster."
	certConfigMapNameDesc   = "The name of the configmap where we store our Cert info for s3 auth."
	authSecretNameDesc      = "The name of the secret required for s3 auth."
	operatorNamespaceDesc   = "The namespace our operator is deployed to."
	maxSyncRestartCountDesc = "The maximum number of restarts we allow before we cancel a sync"
	maxSyncConcurrencyDesc  = "The maximum number of active syncs we allow at once"
	syncBackoffDurationDesc = "The amount of time in seconds we will wait to backoff if there has been an issue"
)

// nolint:gocyclo
func main() {
	var metricsAddr string
	var metricsCertPath, metricsCertName, metricsCertKey string
	var webhookCertPath, webhookCertName, webhookCertKey string
	var enableLeaderElection bool
	var probeAddr string
	var secureMetrics bool
	var enableHTTP2 bool
	var tlsOpts []func(*tls.Config)
	var runningInCluster bool
	var operatorConfigMapName string
	var certConfigMapName string
	var authSecretName string
	var operatorNamespace string
	var maxSyncRestartCount int
	var maxSyncConcurrency int
	var syncBackoffDurationSecondsCount int

	flag.StringVar(&metricsAddr, "metrics-bind-address", "0", metricsAddrDesc)
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", probeAddrDesc)
	flag.BoolVar(&enableLeaderElection, "leader-elect", false, enableLEDesc)
	flag.BoolVar(&secureMetrics, "metrics-secure", true, secureMetricsDesc)
	flag.StringVar(&webhookCertPath, "webhook-cert-path", "", webhookCertPathDesc)
	flag.StringVar(&webhookCertName, "webhook-cert-name", "tls.crt", webhookCertNameDesc)
	flag.StringVar(&webhookCertKey, "webhook-cert-key", "tls.key", webhookCertKeyDesc)
	flag.StringVar(&metricsCertPath, "metrics-cert-path", "", metricsCertPathDesc)
	flag.StringVar(&metricsCertName, "metrics-cert-name", "tls.crt", metricsCertNameDesc)
	flag.StringVar(&metricsCertKey, "metrics-cert-key", "tls.key", metricsCertKeyDesc)
	flag.BoolVar(&enableHTTP2, "enable-http2", false, enableHTTP2Desc)
	flag.BoolVar(&runningInCluster, "running-in-cluster", false, runningInClusterDesc)
	flag.StringVar(&operatorConfigMapName, "operator-configmap", "datasync-operator-config", operatorConfigMapNameDesc)
	flag.StringVar(&certConfigMapName, "cert-configmap-name", "lab-vm-images-registry-cert", certConfigMapNameDesc)
	flag.StringVar(&authSecretName, "auth-secret-name", "lab-vm-images-cache-s3-creds", authSecretNameDesc)
	flag.StringVar(&operatorNamespace, "operator-namespace", "default", operatorNamespaceDesc)
	flag.IntVar(&maxSyncRestartCount, "max-sync-restart", 2, maxSyncRestartCountDesc)
	flag.IntVar(&maxSyncConcurrency, "max-sync-concurrency", 2, maxSyncConcurrencyDesc)
	flag.IntVar(&syncBackoffDurationSecondsCount, "error-backoff-duration", 60, syncBackoffDurationDesc)

	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	// Create watchers for metrics and webhooks certificates
	var metricsCertWatcher, webhookCertWatcher *certwatcher.CertWatcher

	// Initial webhook TLS options
	webhookTLSOpts := tlsOpts

	if len(webhookCertPath) > 0 {
		setupLog.Info("Initializing webhook certificate watcher using provided certificates",
			"webhook-cert-path", webhookCertPath, "webhook-cert-name", webhookCertName, "webhook-cert-key", webhookCertKey)

		var err error
		webhookCertWatcher, err = certwatcher.New(
			filepath.Join(webhookCertPath, webhookCertName),
			filepath.Join(webhookCertPath, webhookCertKey),
		)
		if err != nil {
			setupLog.Error(err, "Failed to initialize webhook certificate watcher")
			os.Exit(1)
		}

		webhookTLSOpts = append(webhookTLSOpts, func(config *tls.Config) {
			config.GetCertificate = webhookCertWatcher.GetCertificate
		})
	}

	webhookServer := webhook.NewServer(webhook.Options{
		TLSOpts: webhookTLSOpts,
	})

	// Metrics endpoint is enabled in 'config/default/kustomization.yaml'. The Metrics options configure the server.
	// More info:
	// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/metrics/server
	// - https://book.kubebuilder.io/reference/metrics.html
	metricsServerOptions := metricsserver.Options{
		BindAddress:   metricsAddr,
		SecureServing: secureMetrics,
		TLSOpts:       tlsOpts,
	}

	if secureMetrics {
		// FilterProvider is used to protect the metrics endpoint with authn/authz.
		// These configurations ensure that only authorized users and service accounts
		// can access the metrics endpoint. The RBAC are configured in 'config/rbac/kustomization.yaml'. More info:
		// https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.21.0/pkg/metrics/filters#WithAuthenticationAndAuthorization
		metricsServerOptions.FilterProvider = filters.WithAuthenticationAndAuthorization
	}

	// If the certificate is not specified, controller-runtime will automatically
	// generate self-signed certificates for the metrics server. While convenient for development and testing,
	// this setup is not recommended for production.
	//
	// TODO(user): If you enable certManager, uncomment the following lines:
	// - [METRICS-WITH-CERTS] at config/default/kustomization.yaml to generate and use certificates
	// managed by cert-manager for the metrics server.
	// - [PROMETHEUS-WITH-CERTS] at config/prometheus/kustomization.yaml for TLS certification.
	if len(metricsCertPath) > 0 {
		setupLog.Info("Initializing metrics certificate watcher using provided certificates",
			"metrics-cert-path", metricsCertPath, "metrics-cert-name", metricsCertName, "metrics-cert-key", metricsCertKey)

		var err error
		metricsCertWatcher, err = certwatcher.New(
			filepath.Join(metricsCertPath, metricsCertName),
			filepath.Join(metricsCertPath, metricsCertKey),
		)
		if err != nil {
			setupLog.Error(err, "to initialize metrics certificate watcher", "error", err)
			os.Exit(1)
		}

		metricsServerOptions.TLSOpts = append(metricsServerOptions.TLSOpts, func(config *tls.Config) {
			config.GetCertificate = metricsCertWatcher.GetCertificate
		})
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Scheme:                 scheme,
		Metrics:                metricsServerOptions,
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "1f5c5280.pelotech.ot",
		Cache: cache.Options{
			DefaultNamespaces: map[string]cache.Config{
				operatorNamespace: {},
			},
		},
		// LeaderElectionReleaseOnCancel defines if the leader should step down voluntarily
		// when the Manager ends. This requires the binary to immediately end when the
		// Manager is stopped, otherwise, this setting is unsafe. Setting this significantly
		// speeds up voluntary leader transitions as the new leader don't have to wait
		// LeaseDuration time first.
		//
		// In the default scaffold provided, the program ends immediately after
		// the manager stops, so would be fine to enable this option. However,
		// if you are doing or is intended to do any operation such as perform cleanups
		// after the manager stops then its usage might be unsafe.
		// LeaderElectionReleaseOnCancel: true,
	})
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	// Standup our client we will use to deploy resources inside the controller

	// Check for S3 secrets and config map to allow for pulling from either s3 or a registry
	// We also do this in an init container on the pod when deployed.
	kubeConfig, err := kubectlclient.LoadKubectlConfig(runningInCluster)

	if err != nil {
		setupLog.Error(err, "unable to load kubeconfig file")
	}

	tmpClient, err := client.New(kubeConfig, client.Options{})

	_, err = generalutils.GetSecret(context.Background(), tmpClient, authSecretName, operatorNamespace)

	if err != nil {
		errMsg := fmt.Sprintf("secret by the name of %s in namespace %s not found", authSecretName, operatorNamespace)
		setupLog.Error(err, errMsg)
		os.Exit(1)
	}

	_, err = generalutils.GetConfigMap(context.Background(), tmpClient, certConfigMapName, operatorNamespace)

	if err != nil {
		errMsg := fmt.Sprintf("configmap by name of %s in namespace %s not found", authSecretName, operatorNamespace)
		setupLog.Error(err, errMsg)
		os.Exit(1)
	}

	defaultControllerConfig := dynamicconfigservice.OperatorConfig{
		Concurrency:          maxSyncConcurrency,
		RetryLimit:           maxSyncRestartCount,
		RetryBackoffDuration: time.Duration(syncBackoffDurationSecondsCount) * time.Second,
	}

	rm := &resourcemanager.DataSyncResourceManager{}

	recorder := mgr.GetEventRecorderFor("datasync-controller")
	errorHandler := &errorservice.ErrorHandler{
		Client:          mgr.GetClient(),
		Recorder:        recorder,
		ResourceManager: rm,
	}

	dataSyncService := &datasyncservice.DataSyncService{
		Client:          mgr.GetClient(),
		Recorder:        recorder,
		ResourceManager: rm,
		ErrorHandler:    errorHandler,
	}

	dynamicConfigService := &dynamicconfigservice.DynamicConfigService{
		Client:        mgr.GetClient(),
		ConfigMapName: "Configmap-name",
		DefaultConfig: defaultControllerConfig,
	}

	if err := (&controller.DataSyncReconciler{
		Client:               mgr.GetClient(),
		Scheme:               mgr.GetScheme(),
		Recorder:             recorder,
		DataSyncService:      *dataSyncService,
		DynamicConfigService: *dynamicConfigService,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "DataSync")
		os.Exit(1)
	}
	// +kubebuilder:scaffold:builder

	if metricsCertWatcher != nil {
		setupLog.Info("Adding metrics certificate watcher to manager")
		if err := mgr.Add(metricsCertWatcher); err != nil {
			setupLog.Error(err, "unable to add metrics certificate watcher to manager")
			os.Exit(1)
		}
	}

	if webhookCertWatcher != nil {
		setupLog.Info("Adding webhook certificate watcher to manager")
		if err := mgr.Add(webhookCertWatcher); err != nil {
			setupLog.Error(err, "unable to add webhook certificate watcher to manager")
			os.Exit(1)
		}
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
