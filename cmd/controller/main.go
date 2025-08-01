package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	certmanagerv1 "github.com/cert-manager/cert-manager/pkg/apis/certmanager/v1"
	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v8/apis/volumesnapshot/v1"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	agentcmd "github.com/mariadb-operator/mariadb-operator/v25/cmd/agent"
	backupcmd "github.com/mariadb-operator/mariadb-operator/v25/cmd/backup"
	initcmd "github.com/mariadb-operator/mariadb-operator/v25/cmd/init"
	"github.com/mariadb-operator/mariadb-operator/v25/internal/controller"
	webhookv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/internal/webhook/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/backup"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/builder"
	condition "github.com/mariadb-operator/mariadb-operator/v25/pkg/condition"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/auth"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/batch"
	certctrl "github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/certificate"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/configmap"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/deployment"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/endpoints"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/galera"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/maxscale"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/pvc"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/rbac"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/replication"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/secret"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/service"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/servicemonitor"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/sql"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/controller/statefulset"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/discovery"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/environment"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/log"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/metadata"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/refresolver"
	monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/config"
	ctrlcontroller "sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var (
	scheme      = runtime.NewScheme()
	setupLog    = ctrl.Log.WithName("setup")
	metricsAddr string
	healthAddr  string

	leaderElect bool

	logLevel       string
	logTimeEncoder string
	logDev         bool
	logMaxScale    bool
	logSql         bool

	kubeApiQps   float32
	kubeApiBurst int

	maxConcurrentReconciles         int
	mariadbMaxConcurrentReconciles  int
	maxscaleMaxConcurrentReconciles int

	requeueConnection time.Duration
	requeueSql        time.Duration
	requeueSqlJob     time.Duration
	requeueMaxScale   time.Duration

	requeueSqlMaxOffset time.Duration

	syncPeriod time.Duration

	webhookEnabled bool
	webhookPort    int
	webhookCertDir string

	pprofEnabled bool
	pprofAddr    string

	featureMaxScaleSuspend bool
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(mariadbv1alpha1.AddToScheme(scheme))
	utilruntime.Must(monitoringv1.AddToScheme(scheme))
	utilruntime.Must(certmanagerv1.AddToScheme(scheme))
	utilruntime.Must(volumesnapshotv1.AddToScheme(scheme))

	rootCmd.PersistentFlags().StringVar(&metricsAddr, "metrics-addr", ":8080", "The address the metric endpoint binds to.")
	rootCmd.PersistentFlags().StringVar(&healthAddr, "health-addr", ":8081", "The address the probe endpoint binds to.")

	rootCmd.PersistentFlags().BoolVar(&leaderElect, "leader-elect", false, "Enable leader election for controller manager.")

	rootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "info", "Log level to use, one of: "+
		"debug, info, warn, error, dpanic, panic, fatal.")
	rootCmd.PersistentFlags().StringVar(&logTimeEncoder, "log-time-encoder", "epoch", "Log time encoder to use, one of: "+
		"epoch, millis, nano, iso8601, rfc3339 or rfc3339nano")
	rootCmd.PersistentFlags().BoolVar(&logDev, "log-dev", false, "Enable development logs.")
	rootCmd.Flags().BoolVar(&logMaxScale, "log-maxscale", false, "Enable MaxScale API request logs.")
	rootCmd.Flags().BoolVar(&logSql, "log-sql", false, "Enable SQL resource logs.")

	rootCmd.Flags().Float32Var(&kubeApiQps, "kube-api-qps", 20.0,
		"QPS limit for requests to Kubernetes API server (set to `-1` to disable client-side ratelimit).")
	rootCmd.Flags().IntVar(&kubeApiBurst, "kube-api-burst", 30, "Burst limit for requests to Kubernetes API server.")

	rootCmd.Flags().IntVar(&maxConcurrentReconciles, "max-concurrent-reconciles", 1,
		"Global maximum number of concurrent reconciles per resource.")
	rootCmd.Flags().IntVar(&mariadbMaxConcurrentReconciles, "mariadb-max-concurrent-reconciles", 10,
		"Maximum number of concurrent reconciles per MariaDB.")
	rootCmd.Flags().IntVar(&maxscaleMaxConcurrentReconciles, "maxscale-max-concurrent-reconciles", 10,
		"Maximum number of concurrent reconciles per MaxScale.")

	rootCmd.Flags().DurationVar(&requeueConnection, "requeue-connection", 1*time.Hour, "The interval at which Connections are requeued.")
	rootCmd.Flags().DurationVar(&requeueSql, "requeue-sql", 10*time.Hour, "The interval at which SQL objects are requeued.")
	rootCmd.Flags().DurationVar(&requeueSqlMaxOffset, "requeue-sql-max-offset", 1*time.Hour,
		"Maximum offset added to the interval at which SQL objects are requeued.")
	rootCmd.Flags().DurationVar(&requeueSqlJob, "requeue-sqljob", 30*time.Second, "The interval at which SqlJobs are requeued.")
	rootCmd.Flags().DurationVar(&requeueMaxScale, "requeue-maxscale", 1*time.Hour, "The interval at which MaxScales are requeued.")

	rootCmd.Flags().DurationVar(&syncPeriod, "sync-period", 10*time.Hour, "The interval at which watched resources are reconciled.")

	rootCmd.Flags().BoolVar(&webhookEnabled, "webhook", false, "Enable the webhook server.")
	rootCmd.Flags().IntVar(&webhookPort, "webhook-port", 9443, "Port to be used by the webhook server."+
		"This only applies if the webhook server is enabled.")
	rootCmd.Flags().StringVar(&webhookCertDir, "webhook-cert-dir", "/tmp/k8s-webhook-server/serving-certs",
		"Directory containing the TLS certificate for the webhook server. 'tls.crt' and 'tls.key' must be present in this directory."+
			"This only applies if the webhook server is enabled.")

	rootCmd.Flags().BoolVar(&pprofEnabled, "pprof", false, "Enable the pprof HTTP server.")
	rootCmd.Flags().StringVar(&pprofAddr, "pprof-addr", ":6060", "The address the pprof endpoint binds to.")

	rootCmd.Flags().BoolVar(&featureMaxScaleSuspend, "feature-maxscale-suspend", false, "Feature flag to enable MaxScale resource suspension.")
}

var rootCmd = &cobra.Command{
	Use:   "mariadb-operator",
	Short: "MariaDB operator.",
	Long:  `Run and operate MariaDB in a cloud native way.`,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		log.SetupLogger(logLevel, logTimeEncoder, logDev)

		ctx, cancel := signal.NotifyContext(context.Background(), []os.Signal{
			syscall.SIGINT,
			syscall.SIGTERM,
			syscall.SIGKILL,
			syscall.SIGHUP,
			syscall.SIGQUIT}...,
		)
		defer cancel()

		restConfig, err := ctrl.GetConfig()
		if err != nil {
			setupLog.Error(err, "Unable to get config")
			os.Exit(1)
		}
		restConfig.QPS = kubeApiQps
		restConfig.Burst = kubeApiBurst

		env, err := environment.GetOperatorEnv(ctx)
		if err != nil {
			setupLog.Error(err, "Error getting environment")
			os.Exit(1)
		}

		mgrOpts := ctrl.Options{
			Scheme: scheme,
			Metrics: metricsserver.Options{
				BindAddress: metricsAddr,
			},
			HealthProbeBindAddress: healthAddr,
			LeaderElection:         leaderElect,
			LeaderElectionID:       "mariadb-operator.mariadb.com",
			Controller: config.Controller{
				MaxConcurrentReconciles: maxConcurrentReconciles,
			},
			Cache: cache.Options{
				SyncPeriod: &syncPeriod,
			},
		}
		if webhookEnabled {
			setupLog.Info("Enabling webhook")
			mgrOpts.WebhookServer = webhook.NewServer(webhook.Options{
				CertDir: webhookCertDir,
				Port:    webhookPort,
			})
		}
		if pprofEnabled {
			setupLog.Info("Enabling pprof")
			mgrOpts.PprofBindAddress = pprofAddr
		}

		if env.WatchNamespace != "" {
			namespaces, err := env.WatchNamespaces()
			if err != nil {
				setupLog.Error(err, "Error getting namespaces to watch")
				os.Exit(1)
			}
			setupLog.Info("Watching namespaces", "namespaces", namespaces)
			mgrOpts.Cache.DefaultNamespaces = make(map[string]cache.Config, len(namespaces))
			for _, ns := range namespaces {
				mgrOpts.Cache.DefaultNamespaces[ns] = cache.Config{}
			}
		} else {
			setupLog.Info("Watching all namespaces")
		}
		mgr, err := ctrl.NewManager(restConfig, mgrOpts)
		if err != nil {
			setupLog.Error(err, "Unable to start manager")
			os.Exit(1)
		}

		client := mgr.GetClient()
		scheme := mgr.GetScheme()
		galeraRecorder := mgr.GetEventRecorderFor("galera")
		replRecorder := mgr.GetEventRecorderFor("replication")

		kubeClientset, err := kubernetes.NewForConfig(restConfig)
		if err != nil {
			setupLog.Error(err, "Error getting Kubernetes clientset")
			os.Exit(1)
		}

		discovery, err := discovery.NewDiscovery()
		if err != nil {
			setupLog.Error(err, "Error creating discovery")
			os.Exit(1)
		}
		if err := discovery.LogInfo(setupLog); err != nil {
			setupLog.Error(err, "Error discovering")
			os.Exit(1)
		}
		builder := builder.NewBuilder(scheme, env, discovery)
		refResolver := refresolver.New(client)

		conditionReady := condition.NewReady()
		conditionComplete := condition.NewComplete(client)

		backupProcessor := backup.NewPhysicalBackupProcessor(
			backup.WithPhysicalBackupValidationFn(mariadbv1alpha1.IsValidPhysicalBackup),
			backup.WithPhysicalBackupParseDateFn(mariadbv1alpha1.ParsePhysicalBackupTime),
		)

		secretReconciler, err := secret.NewSecretReconciler(client, builder)
		if err != nil {
			setupLog.Error(err, "Error creating Secret reconciler")
			os.Exit(1)
		}
		configMapReconciler := configmap.NewConfigMapReconciler(client, builder)
		statefulSetReconciler := statefulset.NewStatefulSetReconciler(client)
		serviceReconciler := service.NewServiceReconciler(client)
		endpointsReconciler := endpoints.NewEndpointsReconciler(client, builder)
		batchReconciler := batch.NewBatchReconciler(client, builder)
		rbacReconciler := rbac.NewRBACReconiler(client, builder)
		authReconciler := auth.NewAuthReconciler(client, builder)
		deployReconciler := deployment.NewDeploymentReconciler(client)
		pvcReconciler := pvc.NewPVCReconciler(client)
		svcMonitorReconciler := servicemonitor.NewServiceMonitorReconciler(client)
		certReconciler := certctrl.NewCertReconciler(client, scheme, mgr.GetEventRecorderFor("cert"), discovery, builder)

		mxsReconciler := maxscale.NewMaxScaleReconciler(client, builder, env)
		replConfig := replication.NewReplicationConfig(client, builder, secretReconciler, env)
		replicationReconciler, err := replication.NewReplicationReconciler(
			client,
			replRecorder,
			builder,
			replConfig,
			replication.WithRefResolver(refResolver),
			replication.WithSecretReconciler(secretReconciler),
			replication.WithServiceReconciler(serviceReconciler),
		)
		if err != nil {
			setupLog.Error(err, "Error creating Replication reconciler")
			os.Exit(1)
		}
		galeraReconciler := galera.NewGaleraReconciler(
			client,
			kubeClientset,
			galeraRecorder,
			env,
			builder,
			galera.WithRefResolver(refResolver),
			galera.WithConfigMapReconciler(configMapReconciler),
			galera.WithServiceReconciler(serviceReconciler),
		)

		podReplicationController := controller.NewPodController(
			"pod-replication",
			client,
			refResolver,
			controller.NewPodReplicationController(
				client,
				replRecorder,
				builder,
				refResolver,
				replConfig,
			),
			[]string{
				metadata.MariadbAnnotation,
				metadata.ReplicationAnnotation,
			},
		)
		podGaleraController := controller.NewPodController(
			"pod-galera",
			client,
			refResolver,
			controller.NewPodGaleraController(client, galeraRecorder),
			[]string{
				metadata.MariadbAnnotation,
				metadata.GaleraAnnotation,
			},
		)

		if err = (&controller.MariaDBReconciler{
			Client:   client,
			Scheme:   scheme,
			Recorder: mgr.GetEventRecorderFor("mariadb"),

			Environment:     env,
			Builder:         builder,
			RefResolver:     refResolver,
			ConditionReady:  conditionReady,
			Discovery:       discovery,
			BackupProcessor: backupProcessor,

			ConfigMapReconciler:      configMapReconciler,
			SecretReconciler:         secretReconciler,
			StatefulSetReconciler:    statefulSetReconciler,
			ServiceReconciler:        serviceReconciler,
			EndpointsReconciler:      endpointsReconciler,
			RBACReconciler:           rbacReconciler,
			AuthReconciler:           authReconciler,
			DeploymentReconciler:     deployReconciler,
			PVCReconciler:            pvcReconciler,
			ServiceMonitorReconciler: svcMonitorReconciler,
			CertReconciler:           certReconciler,

			MaxScaleReconciler:    mxsReconciler,
			ReplicationReconciler: replicationReconciler,
			GaleraReconciler:      galeraReconciler,
		}).SetupWithManager(ctx, mgr, env, ctrlcontroller.Options{MaxConcurrentReconciles: mariadbMaxConcurrentReconciles}); err != nil {
			setupLog.Error(err, "Unable to create controller", "controller", "MariaDB")
			os.Exit(1)
		}
		if err = (&controller.MaxScaleReconciler{
			Client:      client,
			Scheme:      scheme,
			Recorder:    mgr.GetEventRecorderFor("maxscale"),
			RefResolver: refResolver,

			Builder:        builder,
			ConditionReady: conditionReady,
			Environment:    env,
			Discovery:      discovery,

			SecretReconciler:         secretReconciler,
			RBACReconciler:           rbacReconciler,
			AuthReconciler:           authReconciler,
			StatefulSetReconciler:    statefulSetReconciler,
			ServiceReconciler:        serviceReconciler,
			DeploymentReconciler:     deployReconciler,
			ServiceMonitorReconciler: svcMonitorReconciler,
			CertReconciler:           certReconciler,

			SuspendEnabled: featureMaxScaleSuspend,

			RequeueInterval: requeueMaxScale,
			LogMaxScale:     logMaxScale,
		}).SetupWithManager(ctx, mgr, ctrlcontroller.Options{MaxConcurrentReconciles: maxscaleMaxConcurrentReconciles}); err != nil {
			setupLog.Error(err, "Unable to create controller", "controller", "MaxScale")
			os.Exit(1)
		}
		if err = (&controller.BackupReconciler{
			Client:            client,
			Scheme:            scheme,
			Builder:           builder,
			RefResolver:       refResolver,
			ConditionComplete: conditionComplete,
			RBACReconciler:    rbacReconciler,
			BatchReconciler:   batchReconciler,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "Unable to create controller", "controller", "Backup")
			os.Exit(1)
		}
		if err = (&controller.PhysicalBackupReconciler{
			Client:            client,
			Scheme:            scheme,
			Recorder:          mgr.GetEventRecorderFor("physicalbackup"),
			Builder:           builder,
			Discovery:         discovery,
			RefResolver:       refResolver,
			ConditionComplete: conditionComplete,
			RBACReconciler:    rbacReconciler,
			PVCReconciler:     pvcReconciler,
			BackupProcessor:   backupProcessor,
		}).SetupWithManager(ctx, mgr); err != nil {
			setupLog.Error(err, "Unable to create controller", "controller", "PhysicalBackup")
			os.Exit(1)
		}
		if err = (&controller.RestoreReconciler{
			Client:            client,
			Scheme:            scheme,
			Builder:           builder,
			RefResolver:       refResolver,
			ConditionComplete: conditionComplete,
			RBACReconciler:    rbacReconciler,
			BatchReconciler:   batchReconciler,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "Unable to create controller", "controller", "restore")
			os.Exit(1)
		}

		sqlOpts := []sql.SqlOpt{
			sql.WithRequeueInterval(requeueSql),
			sql.WithRequeueMaxOffset(requeueSqlMaxOffset),
			sql.WithLogSql(logSql),
		}
		if err = controller.NewUserReconciler(client, refResolver, conditionReady, sqlOpts...).SetupWithManager(ctx, mgr); err != nil {
			setupLog.Error(err, "Unable to create controller", "controller", "User")
			os.Exit(1)
		}
		if err = controller.NewGrantReconciler(client, refResolver, conditionReady, sqlOpts...).SetupWithManager(ctx, mgr); err != nil {
			setupLog.Error(err, "Unable to create controller", "controller", "Grant")
			os.Exit(1)
		}
		if err = controller.NewDatabaseReconciler(client, refResolver, conditionReady, sqlOpts...).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "Unable to create controller", "controller", "Database")
			os.Exit(1)
		}

		if err = (&controller.ConnectionReconciler{
			Client:           client,
			Scheme:           scheme,
			SecretReconciler: secretReconciler,
			RefResolver:      refResolver,
			ConditionReady:   conditionReady,
			RequeueInterval:  requeueConnection,
		}).SetupWithManager(ctx, mgr); err != nil {
			setupLog.Error(err, "Unable to create controller", "controller", "Connection")
			os.Exit(1)
		}
		if err = (&controller.SqlJobReconciler{
			Client:              client,
			Scheme:              scheme,
			Builder:             builder,
			RefResolver:         refResolver,
			ConfigMapReconciler: configMapReconciler,
			ConditionComplete:   conditionComplete,
			RBACReconciler:      rbacReconciler,
			RequeueInterval:     requeueSqlJob,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "Unable to create controller", "controller", "SqlJob")
			os.Exit(1)
		}
		if err = podReplicationController.SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "Unable to create controller", "controller", "PodReplication")
			os.Exit(1)
		}
		if err := podGaleraController.SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "Unable to create controller", "controller", "PodGalera")
			os.Exit(1)
		}
		if err = (&controller.StatefulSetGaleraReconciler{
			Client:      client,
			RefResolver: refResolver,
			Recorder:    galeraRecorder,
		}).SetupWithManager(mgr); err != nil {
			setupLog.Error(err, "Unable to create controller", "controller", "StatefulSetGalera")
			os.Exit(1)
		}

		if webhookEnabled {
			if err = webhookv1alpha1.SetupMariaDBWebhookWithManager(mgr); err != nil {
				setupLog.Error(err, "Unable to create webhook", "webhook", "MariaDB")
				os.Exit(1)
			}
			if err = webhookv1alpha1.SetupMaxScaleWebhookWithManager(mgr); err != nil {
				setupLog.Error(err, "Unable to create webhook", "webhook", "MaxScale")
				os.Exit(1)
			}
			if err = webhookv1alpha1.SetupBackupWebhookWithManager(mgr); err != nil {
				setupLog.Error(err, "Unable to create webhook", "webhook", "Backup")
				os.Exit(1)
			}
			if err = webhookv1alpha1.SetupPhysicalBackupWebhookWithManager(mgr); err != nil {
				setupLog.Error(err, "Unable to create webhook", "webhook", "PhysicalBackup")
				os.Exit(1)
			}
			if err = webhookv1alpha1.SetupRestoreWebhookWithManager(mgr); err != nil {
				setupLog.Error(err, "Unable to create webhook", "webhook", "Restore")
				os.Exit(1)
			}
			if err = webhookv1alpha1.SetupUserWebhookWithManager(mgr); err != nil {
				setupLog.Error(err, "Unable to create webhook", "webhook", "User")
				os.Exit(1)
			}
			if err = webhookv1alpha1.SetupGrantWebhookWithManager(mgr); err != nil {
				setupLog.Error(err, "Unable to create webhook", "webhook", "Grant")
				os.Exit(1)
			}
			if err = webhookv1alpha1.SetupDatabaseWebhookWithManager(mgr); err != nil {
				setupLog.Error(err, "Unable to create webhook", "webhook", "Database")
				os.Exit(1)
			}
			if err = webhookv1alpha1.SetupConnectionWebhookWithManager(mgr); err != nil {
				setupLog.Error(err, "Unable to create webhook", "webhook", "Connection")
				os.Exit(1)
			}
			if err = webhookv1alpha1.SetupSqlJobWebhookWithManager(mgr); err != nil {
				setupLog.Error(err, "Unable to create webhook", "webhook", "SqlJob")
				os.Exit(1)
			}

			if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
				setupLog.Error(err, "Unable to set up health check")
				os.Exit(1)
			}
			if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
				setupLog.Error(err, "Unable to set up ready check")
				os.Exit(1)
			}
		}

		setupLog.Info("Starting manager")
		if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
			setupLog.Error(err, "Error running manager")
			os.Exit(1)
		}
	},
}

func main() {
	rootCmd.AddCommand(certControllerCmd)
	rootCmd.AddCommand(webhookCmd)
	rootCmd.AddCommand(backupcmd.RootCmd)
	rootCmd.AddCommand(initcmd.RootCmd)
	rootCmd.AddCommand(agentcmd.RootCmd)

	cobra.CheckErr(rootCmd.Execute())
}
