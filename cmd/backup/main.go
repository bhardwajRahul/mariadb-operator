package backup

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-logr/logr"
	mariadbv1alpha1 "github.com/mariadb-operator/mariadb-operator/v25/api/v1alpha1"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/backup"
	"github.com/mariadb-operator/mariadb-operator/v25/pkg/log"
	"github.com/spf13/cobra"
	ctrl "sigs.k8s.io/controller-runtime"
)

var (
	logger = ctrl.Log

	path              string
	targetFilePath    string
	backupContentType string
	cleanupTargetFile bool

	s3           bool
	s3Bucket     string
	s3Endpoint   string
	s3Region     string
	s3TLS        bool
	s3CACertPath string
	s3Prefix     string

	maxRetention time.Duration

	compression string
)

func init() {
	RootCmd.PersistentFlags().StringVar(&path, "path", "/backup", "Directory path where the backup files are located."+
		"When S3 is enabled, it is used as staging area and the source of truth of backups remains in S3.")
	RootCmd.PersistentFlags().StringVar(&targetFilePath, "target-file-path", "/backup/0-backup-target.txt",
		"Path to a file that contains the name of the backup target file.")
	if err := RootCmd.MarkPersistentFlagRequired("path"); err != nil {
		fmt.Printf("error marking 'path' flag as required: %v", err)
		os.Exit(1)
	}
	if err := RootCmd.MarkPersistentFlagRequired("target-file-path"); err != nil {
		fmt.Printf("error marking 'target-file-path' flag as required: %v", err)
		os.Exit(1)
	}
	RootCmd.PersistentFlags().StringVar(&backupContentType, "backup-content-type", string(mariadbv1alpha1.BackupContentTypeLogical),
		"Backup type: Logical (mariadb-dump) or Physical (mariadb-backup).")
	RootCmd.PersistentFlags().BoolVar(&cleanupTargetFile, "cleanup-target-file", false,
		"Whether to clean up the target file after S3 backups are completed."+
			"This option should be used exclusively with external backups, such as S3.")

	RootCmd.PersistentFlags().BoolVar(&s3, "s3", false, "Enable S3 backup storage.")
	RootCmd.PersistentFlags().StringVar(&s3Bucket, "s3-bucket", "backups", "Name of the bucket to store backups.")
	RootCmd.PersistentFlags().StringVar(&s3Endpoint, "s3-endpoint", "s3.amazonaws.com", "S3 API endpoint without scheme.")
	RootCmd.PersistentFlags().StringVar(&s3Region, "s3-region", "us-east-1", "S3 region name to use.")
	RootCmd.PersistentFlags().BoolVar(&s3TLS, "s3-tls", false, "Enable S3 TLS connections.")
	RootCmd.PersistentFlags().StringVar(&s3CACertPath, "s3-ca-cert-path", "", "Path to the CA to be trusted when connecting to S3.")
	RootCmd.PersistentFlags().StringVar(&s3Prefix, "s3-prefix", "", "S3 bucket prefix name to use.")

	RootCmd.PersistentFlags().StringVar(&compression, "compression", string(mariadbv1alpha1.CompressNone),
		"Compression algorithm: none, gzip or bzip2.")

	RootCmd.Flags().DurationVar(&maxRetention, "max-retention", 30*24*time.Hour,
		"Defines the retention policy for backups. Older backups will be deleted.")

	RootCmd.AddCommand(restoreCommand)
}

var RootCmd = &cobra.Command{
	Use:   "backup",
	Short: "Backup.",
	Long:  `Manages backup files, stores them in multiple storage types and implements retention policy.`,
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		if err := log.SetupLoggerWithCommand(cmd); err != nil {
			fmt.Printf("error setting up logger: %v\n", err)
			os.Exit(1)
		}
		logger.Info("starting backup")

		ctx, cancel := newContext()
		defer cancel()

		backupProcessor, err := getBackupProcessor()
		if err != nil {
			logger.Error(err, "error getting backup processor")
			os.Exit(1)
		}
		backupStorage, err := getBackupStorage(backupProcessor)
		if err != nil {
			logger.Error(err, "error getting backup storage")
			os.Exit(1)
		}
		backupCompressor, err := getBackupCompressor(backupProcessor)
		if err != nil {
			logger.Error(err, "error getting backup compressor")
			os.Exit(1)
		}

		logger.Info("reading target file", "file", targetFilePath)
		backupTargetFile, err := readTargetFile()
		if err != nil {
			logger.Error(err, "error reading target file", "file", targetFilePath)
			os.Exit(1)
		}
		logger.Info("obtained target backup", "file", backupTargetFile)

		if err := backupCompressor.Compress(backupTargetFile); err != nil {
			logger.Error(err, "error compressing backup", "file", backupTargetFile)
			os.Exit(1)
		}

		logger.Info("pushing target backup", "file", backupTargetFile, "prefix", s3Prefix)
		if err := backupStorage.Push(ctx, backupTargetFile); err != nil {
			logger.Error(err, "error pushing target backup", "file", backupTargetFile, "prefix", s3Prefix)
			os.Exit(1)
		}

		logger.Info("cleaning up old backups")
		backupNames, err := backupStorage.List(ctx)
		if err != nil {
			logger.Error(err, "error listing backup files")
			os.Exit(1)
		}
		oldBackups := backupProcessor.GetOldBackupFiles(backupNames, maxRetention, logger.WithName("backup-cleanup"))
		logger.Info("old backups to delete", "backups", len(oldBackups))
		for _, backup := range oldBackups {
			logger.V(1).Info("deleting old backup", "backup", backup)
			if err := backupStorage.Delete(ctx, backup); err != nil {
				logger.Error(err, "error removing old backup", "backup", backup)
			}
		}

		if err := cleanupFile(backupTargetFile, logger.WithName("cleanup")); err != nil && os.IsNotExist(err) {
			logger.Error(err, "error cleaning up target file", "file", backupTargetFile)
			os.Exit(1)
		}
	},
}

func newContext() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), []os.Signal{
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGKILL,
		syscall.SIGHUP,
		syscall.SIGQUIT}...,
	)
}

func getBackupProcessor() (backup.BackupProcessor, error) {
	backType := mariadbv1alpha1.BackupContentType(backupContentType)
	if err := backType.Validate(); err != nil {
		return nil, fmt.Errorf("backup type not supported: %v", err)
	}
	switch backType {
	case mariadbv1alpha1.BackupContentTypeLogical:
		logger.Info("configuring logical backup processor")
		return backup.NewLogicalBackupProcessor(), nil
	case mariadbv1alpha1.BackupContentTypePhysical:
		logger.Info("configuring physical backup processor")
		return backup.NewPhysicalBackupProcessor(), nil
	default:
		return nil, fmt.Errorf("unsupported backup type: %v", backType)
	}
}

func getBackupStorage(processor backup.BackupProcessor) (backup.BackupStorage, error) {
	if s3 {
		logger.Info("configuring S3 backup storage")
		return backup.NewS3BackupStorage(
			path,
			s3Bucket,
			s3Endpoint,
			processor,
			logger.WithName("s3-storage"),
			backup.WithTLS(s3TLS),
			backup.WithCACertPath(s3CACertPath),
			backup.WithRegion(s3Region),
			backup.WithPrefix(s3Prefix),
		)
	}
	logger.Info("configuring filesystem backup storage")
	return backup.NewFileSystemBackupStorage(path, processor, logger.WithName("file-system-storage")), nil
}

func getBackupCompressor(processor backup.BackupProcessor) (backup.BackupCompressor, error) {
	calg := mariadbv1alpha1.CompressAlgorithm(compression)
	if err := calg.Validate(); err != nil {
		return nil, fmt.Errorf("compression algorithm not supported: %v", err)
	}
	return getBackupCompressorWithAlgorithm(calg, processor)
}

func getBackupCompressorWithAlgorithm(calg mariadbv1alpha1.CompressAlgorithm,
	processor backup.BackupProcessor) (backup.BackupCompressor, error) {
	switch calg {
	case mariadbv1alpha1.CompressNone:
		return backup.NewNopCompressor(path, processor, logger.WithName("nop-compressor")), nil
	case mariadbv1alpha1.CompressGzip:
		return backup.NewGzipBackupCompressor(path, processor, logger.WithName("gzip-compressor")), nil
	case mariadbv1alpha1.CompressBzip2:
		return backup.NewBzip2BackupCompressor(path, processor, logger.WithName("bzip2-compressor")), nil
	default:
		return nil, fmt.Errorf("unsupported compression algorithm: %v", calg)
	}
}

func readTargetFile() (string, error) {
	bytes, err := os.ReadFile(targetFilePath)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func cleanupFile(fileName string, logger logr.Logger) error {
	if !s3 || !cleanupTargetFile {
		return nil
	}
	filePath := backup.GetFilePath(path, fileName)
	logger.Info("cleaning up target file", "file", filePath)

	return os.Remove(filePath)
}
