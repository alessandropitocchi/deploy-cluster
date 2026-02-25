package main

import (
	"fmt"
	"os"

	"github.com/alepito/deploy-cluster/pkg/drift"
	"github.com/alepito/deploy-cluster/pkg/logger"
	"github.com/alepito/deploy-cluster/pkg/template"
	"github.com/spf13/cobra"
)

var (
	driftTemplateFile string
	driftEnvFile      string
	driftExitError    bool
)

var driftCmd = &cobra.Command{
	Use:   "drift",
	Short: "Detect drift between cluster and template",
	Long: `Detect drift between the actual cluster state and the desired state defined in the template.

Drift detection identifies:
  - Missing resources: In template but not in cluster
  - Orphan resources: In cluster but not in template  
  - Modified resources: Different configuration between cluster and template

Use this command to verify GitOps compliance and detect manual changes.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		log := newLogger("")

		// Load .env file
		if err := template.LoadEnvFile(driftEnvFile); err != nil {
			return fmt.Errorf("failed to load env file: %w", err)
		}

		// Load config
		log.Info("Loading template from %s...\n", driftTemplateFile)
		cfg, err := template.Load(driftTemplateFile)
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
			return fmt.Errorf("cluster '%s' does not exist", cfg.Name)
		}

		kubecontext := provider.KubeContext(cfg.Name)

		// Run drift detection
		detector := drift.NewDetector(logger.New("[drift]", logger.LevelQuiet))
		result, err := detector.Detect(cfg, kubecontext)
		if err != nil {
			return fmt.Errorf("drift detection failed: %w", err)
		}

		// Display results
		fmt.Println(drift.FormatResult(result))

		// Exit with error if drift detected and --exit-error flag is set
		if result.HasDrift && driftExitError {
			os.Exit(1)
		}

		return nil
	},
}

func init() {
	driftCmd.Flags().StringVarP(&driftTemplateFile, "template", "t", "template.yaml", "cluster template file")
	driftCmd.Flags().StringVarP(&driftEnvFile, "env", "e", ".env", "environment file for secrets")
	driftCmd.Flags().BoolVar(&driftExitError, "exit-error", false, "exit with error code if drift detected")
	rootCmd.AddCommand(driftCmd)
}
