package main

import (
	"fmt"

	"github.com/alepito/deploy-cluster/pkg/template"
	"github.com/spf13/cobra"
)

var (
	upgradeTemplateFile string
	upgradeEnvFile      string
	upgradeDryRun       bool
	upgradeFailFast     bool
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade an existing cluster applying template changes",
	Long: `Upgrade an existing Kubernetes cluster by applying only the differences
compared to the current state. The cluster is not recreated, but plugins
(ArgoCD repos/apps) are updated: additions, modifications and removals.

Use --dry-run to preview changes without applying them.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		log := newLogger("")

		// Load .env file
		if err := template.LoadEnvFile(upgradeEnvFile); err != nil {
			return fmt.Errorf("failed to load env file: %w", err)
		}

		// Load config
		log.Info("Loading template from %s...\n", upgradeTemplateFile)
		cfg, err := template.Load(upgradeTemplateFile)
		if err != nil {
			return fmt.Errorf("failed to load template: %w", err)
		}

		// Get provider and check cluster exists
		provider, err := getProvider(cfg.Provider.Type)
		if err != nil {
			return err
		}

		exists, err := provider.Exists(cfg.Name)
		if err != nil {
			return fmt.Errorf("failed to check cluster existence: %w", err)
		}
		if !exists {
			return fmt.Errorf("cluster '%s' does not exist. Use 'create' to create it first", cfg.Name)
		}

		kubecontext := provider.KubeContext(cfg.Name)

		if upgradeDryRun {
			return runUpgradeDryRun(cfg, kubecontext)
		}

		log.Info("Upgrading cluster '%s'...\n\n", cfg.Name)

		results := upgradePlugins(cfg, kubecontext, upgradeFailFast)

		// Print summary
		if len(results) > 0 {
			printPluginSummary(results, log)
		}

		if hasErrors(results) {
			return fmt.Errorf("some plugins failed to upgrade, see summary above")
		}

		log.Success("\nUpgrade completed for cluster '%s'.\n", cfg.Name)
		return nil
	},
}

func runUpgradeDryRun(cfg *template.Template, kubecontext string) error {
	fmt.Printf("Dry-run for cluster '%s':\n", cfg.Name)

	results := dryRunPlugins(cfg, kubecontext)

	fmt.Println("\nNo changes applied (dry-run).")

	if hasErrors(results) {
		return fmt.Errorf("some plugins failed dry-run")
	}

	return nil
}

func init() {
	upgradeCmd.Flags().StringVarP(&upgradeTemplateFile, "template", "t", "template.yaml", "cluster template file")
	upgradeCmd.Flags().StringVarP(&upgradeEnvFile, "env", "e", ".env", "environment file for secrets")
	upgradeCmd.Flags().BoolVar(&upgradeDryRun, "dry-run", false, "preview changes without applying them")
	upgradeCmd.Flags().BoolVar(&upgradeFailFast, "fail-fast", false, "stop at first plugin failure")
	rootCmd.AddCommand(upgradeCmd)
}
