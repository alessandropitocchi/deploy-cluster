package main

import (
	"fmt"
	"strings"

	"github.com/alepito/deploy-cluster/pkg/s3"
	"github.com/alepito/deploy-cluster/pkg/snapshot"
	"github.com/alepito/deploy-cluster/pkg/template"
	"github.com/spf13/cobra"
)

var (
	snapshotTemplateFile   string
	snapshotNamespaces     string
	snapshotDryRun         bool
	snapshotEnvFile        string
	snapshotExcludeSecrets bool
	snapshotS3             bool
	snapshotS3Bucket       string
	snapshotS3Prefix       string
	snapshotS3Region       string
	snapshotS3Endpoint     string
)

var snapshotCmd = &cobra.Command{
	Use:   "snapshot",
	Short: "Manage cluster snapshots",
	Long:  `Save, restore, list, and delete Kubernetes cluster snapshots.`,
}

var snapshotSaveCmd = &cobra.Command{
	Use:   "save <name>",
	Short: "Save a snapshot of cluster resources",
	Long: `Export all Kubernetes resources from the cluster to a local snapshot.
The snapshot can later be restored to the same or a different cluster.

Note: Snapshots may contain Kubernetes Secrets in plain text.
Use --exclude-secrets to omit Secrets from the snapshot.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		log := newLogger("[snapshot]")
		name := args[0]

		// Load .env file
		if err := template.LoadEnvFile(snapshotEnvFile); err != nil {
			return fmt.Errorf("failed to load env file: %w", err)
		}

		// Load template
		cfg, err := template.Load(snapshotTemplateFile)
		if err != nil {
			return fmt.Errorf("failed to load template: %w", err)
		}

		// Get provider
		provider, err := getProviderFromTemplate(cfg)
		if err != nil {
			return err
		}

		// Check cluster exists
		exists, err := provider.Exists(cfg.Name)
		if err != nil {
			return fmt.Errorf("failed to check cluster: %w", err)
		}
		if !exists {
			return fmt.Errorf("cluster %q does not exist", cfg.Name)
		}

		kubecontext := provider.KubeContext(cfg.Name)

		// Parse namespaces flag
		var namespaces []string
		if snapshotNamespaces != "" {
			namespaces = strings.Split(snapshotNamespaces, ",")
		}

		// Save locally or to S3
		// Check if S3 is enabled via flag or template config
		useS3 := snapshotS3 || (cfg.Snapshot != nil && cfg.Snapshot.Enabled)
		
		if useS3 {
			s3Client, err := getS3Client(cfg)
			if err != nil {
				return err
			}
			return snapshot.SaveToS3(name, kubecontext, cfg.Name, cfg.Provider.Type, snapshotTemplateFile, namespaces, snapshotExcludeSecrets, s3Client, log)
		}

		return snapshot.Save(name, kubecontext, cfg.Name, cfg.Provider.Type, snapshotTemplateFile, namespaces, snapshotExcludeSecrets, log)
	},
}

var snapshotRestoreCmd = &cobra.Command{
	Use:   "restore <name>",
	Short: "Restore a snapshot to the cluster",
	Long: `Apply resources from a saved snapshot to the cluster.
Use --dry-run to preview what would be applied without making changes.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		log := newLogger("[snapshot]")
		name := args[0]

		// Load .env file
		if err := template.LoadEnvFile(snapshotEnvFile); err != nil {
			return fmt.Errorf("failed to load env file: %w", err)
		}

		// Load template
		cfg, err := template.Load(snapshotTemplateFile)
		if err != nil {
			return fmt.Errorf("failed to load template: %w", err)
		}

		// Get provider
		provider, err := getProviderFromTemplate(cfg)
		if err != nil {
			return err
		}

		// Check cluster exists
		exists, err := provider.Exists(cfg.Name)
		if err != nil {
			return fmt.Errorf("failed to check cluster: %w", err)
		}
		if !exists {
			return fmt.Errorf("cluster %q does not exist", cfg.Name)
		}

		kubecontext := provider.KubeContext(cfg.Name)

		// Restore from local or S3
		// Check if S3 is enabled via flag or template config
		useS3 := snapshotS3 || (cfg.Snapshot != nil && cfg.Snapshot.Enabled)
		
		if useS3 {
			s3Client, err := getS3Client(cfg)
			if err != nil {
				return err
			}
			return snapshot.RestoreFromS3(name, kubecontext, snapshotDryRun, s3Client, log)
		}

		return snapshot.Restore(name, kubecontext, snapshotDryRun, log)
	},
}

var snapshotListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all snapshots",
	Long:  `Display all saved snapshots with their metadata.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Check if we should use S3
		useS3 := snapshotS3
		var cfg *template.Template
		
		// If --s3 not set explicitly, try to load template and check config
		if !useS3 {
			if loadedCfg, err := template.Load(snapshotTemplateFile); err == nil && loadedCfg.Snapshot != nil && loadedCfg.Snapshot.Enabled {
				useS3 = true
				cfg = loadedCfg
			}
		}
		
		// List from S3 or local
		if useS3 {
			s3Client, err := getS3Client(cfg)
			if err != nil {
				return err
			}

			snapshots, err := snapshot.ListS3(s3Client)
			if err != nil {
				return fmt.Errorf("failed to list S3 snapshots: %w", err)
			}

			if len(snapshots) == 0 {
				fmt.Println("No S3 snapshots found")
				return nil
			}

			fmt.Println("S3 SNAPSHOTS")
			fmt.Println("─────────────────────────────────────────────────────────────")
			for _, name := range snapshots {
				fmt.Printf("• %s\n", name)
			}
			return nil
		}

		snapshots, err := snapshot.List()
		if err != nil {
			return fmt.Errorf("failed to list snapshots: %w", err)
		}

		if len(snapshots) == 0 {
			fmt.Println("No snapshots found")
			return nil
		}

		fmt.Println("SNAPSHOTS")
		fmt.Println("─────────────────────────────────────────────────────────────")
		for _, s := range snapshots {
			fmt.Printf("• %s\n", s.Name)
			fmt.Printf("  Cluster: %s (%s)\n", s.ClusterName, s.Provider)
			fmt.Printf("  Resources: %d\n", s.ResourceCount)
			fmt.Printf("  Created: %s\n", s.CreatedAt.Format("2006-01-02 15:04:05"))
			if len(s.Namespaces) > 0 {
				fmt.Printf("  Namespaces: %s\n", strings.Join(s.Namespaces, ", "))
			}
			fmt.Println()
		}

		return nil
	},
}

var snapshotDeleteCmd = &cobra.Command{
	Use:   "delete <name>",
	Short: "Delete a snapshot",
	Long:  `Permanently delete a saved snapshot from disk or S3.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		log := newLogger("[snapshot]")

		// Check if we should use S3
		useS3 := snapshotS3
		var cfg *template.Template
		
		// If --s3 not set explicitly, try to load template and check config
		if !useS3 {
			if loadedCfg, err := template.Load(snapshotTemplateFile); err == nil && loadedCfg.Snapshot != nil && loadedCfg.Snapshot.Enabled {
				useS3 = true
				cfg = loadedCfg
			}
		}

		// Delete from S3 or local
		if useS3 {
			s3Client, err := getS3Client(cfg)
			if err != nil {
				return err
			}
			if err := snapshot.DeleteS3(name, s3Client); err != nil {
				return fmt.Errorf("failed to delete S3 snapshot: %w", err)
			}
			log.Success("Snapshot %q deleted from S3\n", name)
			return nil
		}

		if err := snapshot.Delete(name); err != nil {
			return fmt.Errorf("failed to delete snapshot: %w", err)
		}

		log.Success("Snapshot %q deleted\n", name)
		return nil
	},
}

var snapshotDiffCmd = &cobra.Command{
	Use:   "diff <name>",
	Short: "Compare a snapshot against the live cluster",
	Long: `Compare resources in a saved snapshot with the current state of the cluster.
Shows which resources would need to be restored and which already exist.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		log := newLogger("[snapshot]")
		name := args[0]

		// Load .env file
		if err := template.LoadEnvFile(snapshotEnvFile); err != nil {
			return fmt.Errorf("failed to load env file: %w", err)
		}

		// Load template
		cfg, err := template.Load(snapshotTemplateFile)
		if err != nil {
			return fmt.Errorf("failed to load template: %w", err)
		}

		// Get provider
		provider, err := getProviderFromTemplate(cfg)
		if err != nil {
			return err
		}

		// Check cluster exists
		exists, err := provider.Exists(cfg.Name)
		if err != nil {
			return fmt.Errorf("failed to check cluster: %w", err)
		}
		if !exists {
			return fmt.Errorf("cluster %q does not exist", cfg.Name)
		}

		kubecontext := provider.KubeContext(cfg.Name)

		log.Info("Comparing snapshot %q vs cluster...\n", name)
		result, err := snapshot.DiffSnapshot(name, snapshot.DiffOptions{
			Kubecontext: kubecontext,
			Log:         log,
		})
		if err != nil {
			return fmt.Errorf("failed to diff snapshot: %w", err)
		}

		total := len(result.ToRestore) + len(result.Existing)
		fmt.Printf("\nSnapshot %q vs cluster (%d resources)\n\n", name, total)

		if len(result.ToRestore) > 0 {
			fmt.Println("  Would restore (not in cluster):")
			for _, e := range result.ToRestore {
				fmt.Printf("    + %s\n", e.String())
			}
			fmt.Println()
		}

		if len(result.Existing) > 0 {
			fmt.Println("  Already in cluster:")
			for _, e := range result.Existing {
				fmt.Printf("    = %s\n", e.String())
			}
			fmt.Println()
		}

		fmt.Printf("  Summary: %d to restore, %d already present\n", len(result.ToRestore), len(result.Existing))
		return nil
	},
}

// getS3Client creates an S3 client from flags, template config, or environment variables.
func getS3Client(cfg *template.Template) (*s3.Client, error) {
	// Priority: flags > template config > env vars
	s3Cfg := s3.Config{
		Bucket:    snapshotS3Bucket,
		Prefix:    snapshotS3Prefix,
		Region:    snapshotS3Region,
		Endpoint:  snapshotS3Endpoint,
		AccessKey: "", // Will be loaded from AWS credentials chain
		SecretKey: "", // Will be loaded from AWS credentials chain
	}

	// If flags not set, check template config
	if s3Cfg.Bucket == "" && cfg != nil && cfg.Snapshot != nil && cfg.Snapshot.Enabled {
		s3Cfg.Bucket = cfg.Snapshot.Bucket
		s3Cfg.Prefix = cfg.Snapshot.Prefix
		s3Cfg.Region = cfg.Snapshot.Region
		s3Cfg.Endpoint = cfg.Snapshot.Endpoint
	}

	// If still not set, use env vars
	if s3Cfg.Bucket == "" {
		envCfg := s3.ConfigFromEnv()
		s3Cfg = envCfg
	}

	if s3Cfg.Bucket == "" {
		return nil, fmt.Errorf("S3 bucket not configured. Set in template, use --s3-bucket flag, or set DEPLOY_CLUSTER_S3_BUCKET env var")
	}

	return s3.NewClient(s3Cfg)
}

func init() {
	// Save command flags
	snapshotSaveCmd.Flags().StringVarP(&snapshotTemplateFile, "template", "t", "template.yaml", "cluster template file")
	snapshotSaveCmd.Flags().StringVar(&snapshotNamespaces, "namespace", "", "comma-separated list of namespaces to snapshot (default: all non-system)")
	snapshotSaveCmd.Flags().StringVarP(&snapshotEnvFile, "env", "e", ".env", "environment file for secrets")
	snapshotSaveCmd.Flags().BoolVar(&snapshotExcludeSecrets, "exclude-secrets", false, "exclude Kubernetes Secrets from the snapshot")
	// S3 flags
	snapshotSaveCmd.Flags().BoolVar(&snapshotS3, "s3", false, "upload snapshot to S3")
	snapshotSaveCmd.Flags().StringVar(&snapshotS3Bucket, "s3-bucket", "", "S3 bucket name (default: DEPLOY_CLUSTER_S3_BUCKET env var)")
	snapshotSaveCmd.Flags().StringVar(&snapshotS3Prefix, "s3-prefix", "", "S3 key prefix (default: DEPLOY_CLUSTER_S3_PREFIX env var)")
	snapshotSaveCmd.Flags().StringVar(&snapshotS3Region, "s3-region", "", "AWS region (default: AWS_REGION env var)")
	snapshotSaveCmd.Flags().StringVar(&snapshotS3Endpoint, "s3-endpoint", "", "S3 endpoint URL for S3-compatible services (default: DEPLOY_CLUSTER_S3_ENDPOINT env var)")

	// Restore command flags
	snapshotRestoreCmd.Flags().StringVarP(&snapshotTemplateFile, "template", "t", "template.yaml", "cluster template file")
	snapshotRestoreCmd.Flags().BoolVar(&snapshotDryRun, "dry-run", false, "preview restore without applying")
	snapshotRestoreCmd.Flags().StringVarP(&snapshotEnvFile, "env", "e", ".env", "environment file for secrets")
	snapshotRestoreCmd.Flags().BoolVar(&snapshotS3, "s3", false, "restore snapshot from S3")
	snapshotRestoreCmd.Flags().StringVar(&snapshotS3Bucket, "s3-bucket", "", "S3 bucket name")
	snapshotRestoreCmd.Flags().StringVar(&snapshotS3Prefix, "s3-prefix", "", "S3 key prefix")
	snapshotRestoreCmd.Flags().StringVar(&snapshotS3Region, "s3-region", "", "AWS region")
	snapshotRestoreCmd.Flags().StringVar(&snapshotS3Endpoint, "s3-endpoint", "", "S3 endpoint URL")

	// List command flags
	snapshotListCmd.Flags().StringVarP(&snapshotTemplateFile, "template", "t", "template.yaml", "cluster template file (for S3 config)")
	snapshotListCmd.Flags().BoolVar(&snapshotS3, "s3", false, "list snapshots in S3")
	snapshotListCmd.Flags().StringVar(&snapshotS3Bucket, "s3-bucket", "", "S3 bucket name")
	snapshotListCmd.Flags().StringVar(&snapshotS3Prefix, "s3-prefix", "", "S3 key prefix")
	snapshotListCmd.Flags().StringVar(&snapshotS3Region, "s3-region", "", "AWS region")
	snapshotListCmd.Flags().StringVar(&snapshotS3Endpoint, "s3-endpoint", "", "S3 endpoint URL")

	// Delete command flags
	snapshotDeleteCmd.Flags().StringVarP(&snapshotTemplateFile, "template", "t", "template.yaml", "cluster template file (for S3 config)")
	snapshotDeleteCmd.Flags().BoolVar(&snapshotS3, "s3", false, "delete snapshot from S3")
	snapshotDeleteCmd.Flags().StringVar(&snapshotS3Bucket, "s3-bucket", "", "S3 bucket name")
	snapshotDeleteCmd.Flags().StringVar(&snapshotS3Prefix, "s3-prefix", "", "S3 key prefix")
	snapshotDeleteCmd.Flags().StringVar(&snapshotS3Region, "s3-region", "", "AWS region")
	snapshotDeleteCmd.Flags().StringVar(&snapshotS3Endpoint, "s3-endpoint", "", "S3 endpoint URL")

	snapshotDiffCmd.Flags().StringVarP(&snapshotTemplateFile, "template", "t", "template.yaml", "cluster template file")
	snapshotDiffCmd.Flags().StringVarP(&snapshotEnvFile, "env", "e", ".env", "environment file for secrets")

	snapshotCmd.AddCommand(snapshotSaveCmd)
	snapshotCmd.AddCommand(snapshotRestoreCmd)
	snapshotCmd.AddCommand(snapshotListCmd)
	snapshotCmd.AddCommand(snapshotDeleteCmd)
	snapshotCmd.AddCommand(snapshotDiffCmd)
	rootCmd.AddCommand(snapshotCmd)
}
