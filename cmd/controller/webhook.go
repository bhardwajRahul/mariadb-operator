package main

import (
	"context"
	"crypto/x509"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	webhookv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/internal/webhook/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/log"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/pki"
	"github.com/spf13/cobra"
	ctrl "sigs.k8s.io/controller-runtime"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var (
	caCertPath   string
	certDir      string
	dnsName      string
	port         int
	validateCert bool
)

func init() {
	webhookCmd.Flags().StringVar(&caCertPath, "ca-cert-path", filepath.Join("/tmp/k8s-webhook-server/certificate-authority", pki.TLSCertKey),
		"Path containing the CA TLS certificate for the webhook server.")
	webhookCmd.Flags().StringVar(&certDir, "cert-dir", "/tmp/k8s-webhook-server/serving-certs",
		"Directory containing the TLS certificate for the webhook server. 'tls.crt' and 'tls.key' must be present in this directory.")
	webhookCmd.Flags().StringVar(&dnsName, "dns-name", "mariadb-operator-webhook.default.svc",
		"TLS certificate DNS name.")
	webhookCmd.Flags().IntVar(&port, "port", 9443, "Port to be used by the webhook server.")
	webhookCmd.Flags().BoolVar(&validateCert, "validate-cert", true,
		"Validate certificate as a requirement for the webhook server to be healthy.")
}

var webhookCmd = &cobra.Command{
	Use:   "webhook",
	Short: "MariaDB operator webhook server.",
	Long:  `Provides validation and inmutability checks for MariaDB resources.`,
	Run: func(cmd *cobra.Command, args []string) {
		log.SetupLogger(logLevel, logTimeEncoder, logDev)

		err := waitForCerts(dnsName, time.Now(), 3*time.Minute)
		if err != nil {
			setupLog.Error(err, "Unable to validate certificates")
			os.Exit(1)
		}

		mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
			Scheme: scheme,
			Metrics: metricsserver.Options{
				BindAddress: metricsAddr,
			},
			HealthProbeBindAddress: healthAddr,
			WebhookServer: webhook.NewServer(webhook.Options{
				CertDir: certDir,
				Port:    port,
			}),
		})
		if err != nil {
			setupLog.Error(err, "Unable to start manager")
			os.Exit(1)
		}

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

		if err := mgr.AddReadyzCheck("certs", func(_ *http.Request) error {
			return checkCerts(dnsName, time.Now())
		}); err != nil {
			setupLog.Error(err, "Unable to add readyz check")
			os.Exit(1)
		}

		setupLog.Info("Starting manager")
		if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
			setupLog.Error(err, "Error running manager")
			os.Exit(1)
		}
	},
}

func waitForCerts(dnsName string, at time.Time, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	for {
		setupLog.Info("Validating certs")
		if err := checkCerts(dnsName, at); err != nil {
			setupLog.V(1).Info("Invalid certs. Retrying...", "error", err)
			<-time.After(time.Second * 5)
			continue
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return nil
	}
}

func checkCerts(dnsName string, at time.Time) error {
	if !validateCert {
		setupLog.V(1).Info("Omitting certificate validation. Set --validate-cert to enable it.")
		return nil
	}
	caCert, err := readCert(caCertPath)
	if err != nil {
		setupLog.V(1).Info("Error reading CA KeyPair", "error", err)
		return err
	}
	certKeyPair, err := readKeyPair(certDir)
	if err != nil {
		setupLog.V(1).Info("Error reading certificate KeyPair", "error", err)
		return err
	}
	valid, err := pki.ValidateCert(caCert, certKeyPair, dnsName, at)
	if !valid || err != nil {
		err := fmt.Errorf("Certificate is not valid for %s", dnsName) //nolint:staticcheck
		setupLog.V(1).Info("Error validating certificate", "error", err)
		return err
	}
	return nil
}

func readCert(certPath string) ([]*x509.Certificate, error) {
	if _, err := os.Stat(certPath); err != nil {
		return nil, err
	}
	certBytes, err := os.ReadFile(certPath)
	if err != nil {
		return nil, err
	}
	return pki.ParseCertificates(certBytes)
}

func readKeyPair(dir string) (*pki.KeyPair, error) {
	certFile := filepath.Join(dir, pki.TLSCertKey)
	if _, err := os.Stat(certFile); err != nil {
		return nil, err
	}
	keyFile := filepath.Join(dir, pki.TLSKeyKey)
	if _, err := os.Stat(certFile); err != nil {
		return nil, err
	}
	certBytes, err := os.ReadFile(certFile)
	if err != nil {
		return nil, err
	}
	keyBytes, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, err
	}
	return pki.NewKeyPair(
		certBytes,
		keyBytes,
		pki.WithSupportedPrivateKeys(
			pki.PrivateKeyTypeECDSA,
			pki.PrivateKeyTypeRSA, // backwards compatibility with webhook certs from previous versions
		),
	)
}
