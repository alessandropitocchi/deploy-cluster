package main

import (
	"fmt"
	"strings"

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

		return snapshot.Restore(name, kubecontext, snapshotDryRun, log)
	},
}

var snapshotListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all snapshots",
	Long:  `Display all saved snapshots with their metadata.`,
	RunE: func(cmd *cobra.Command, args []string) error {
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
	Long:  `Permanently delete a saved snapshot from disk.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		log := newLogger("[snapshot]")

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

func init() {
	snapshotSaveCmd.Flags().StringVarP(&snapshotTemplateFile, "template", "t", "template.yaml", "cluster template file")
	snapshotSaveCmd.Flags().StringVar(&snapshotNamespaces, "namespace", "", "comma-separated list of namespaces to snapshot (default: all non-system)")
	snapshotSaveCmd.Flags().StringVarP(&snapshotEnvFile, "env", "e", ".env", "environment file for secrets")
	snapshotSaveCmd.Flags().BoolVar(&snapshotExcludeSecrets, "exclude-secrets", false, "exclude Kubernetes Secrets from the snapshot")

	snapshotRestoreCmd.Flags().StringVarP(&snapshotTemplateFile, "template", "t", "template.yaml", "cluster template file")
	snapshotRestoreCmd.Flags().BoolVar(&snapshotDryRun, "dry-run", false, "preview restore without applying")
	snapshotRestoreCmd.Flags().StringVarP(&snapshotEnvFile, "env", "e", ".env", "environment file for secrets")

	snapshotDiffCmd.Flags().StringVarP(&snapshotTemplateFile, "template", "t", "template.yaml", "cluster template file")
	snapshotDiffCmd.Flags().StringVarP(&snapshotEnvFile, "env", "e", ".env", "environment file for secrets")

	snapshotCmd.AddCommand(snapshotSaveCmd)
	snapshotCmd.AddCommand(snapshotRestoreCmd)
	snapshotCmd.AddCommand(snapshotListCmd)
	snapshotCmd.AddCommand(snapshotDeleteCmd)
	snapshotCmd.AddCommand(snapshotDiffCmd)
	rootCmd.AddCommand(snapshotCmd)
}
