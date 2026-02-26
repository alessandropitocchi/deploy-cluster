package main

import (
	"fmt"

	"github.com/alessandropitocchi/deploy-cluster/pkg/template"
	"github.com/spf13/cobra"
)

var (
	runTemplateFile string
	runEnvFile      string
	runFailFast     bool
)

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run cluster deployment from template",
	Long: `Deploy a Kubernetes cluster and install configured plugins from a template file.

This command handles both:
- Creating new clusters (kind, k3d)
- Deploying to existing clusters (EKS, GKE, AKS, or existing kind/k3d)

For 'existing' provider, it skips cluster creation and only installs plugins.`,
	Example: `  # Deploy from template.yaml
  klastr run

  # Deploy with specific template
  klastr run --template production.yaml

  # Deploy with environment variables
  klastr run --template template.yaml --env secrets.env`,
	RunE: func(cmd *cobra.Command, args []string) error {
		log := newLogger("")

		// Load .env file
		if err := template.LoadEnvFile(runEnvFile); err != nil {
			return fmt.Errorf("failed to load env file: %w", err)
		}

		// Load config
		log.Info("Loading template from %s...\n", runTemplateFile)
		cfg, err := template.Load(runTemplateFile)
		if err != nil {
			return fmt.Errorf("failed to load template: %w", err)
		}

		log.Info("\n")
		log.Info("Cluster: %s\n", cfg.Name)
		log.Info("Provider: %s\n", cfg.Provider.Type)
		if cfg.Provider.Type != "existing" {
			log.Info("Control planes: %d\n", cfg.Cluster.ControlPlanes)
			log.Info("Workers: %d\n", cfg.Cluster.Workers)
			if cfg.Cluster.Version != "" {
				log.Info("Kubernetes version: %s\n", cfg.Cluster.Version)
			}
		} else {
			if cfg.Provider.Kubeconfig != "" {
				log.Info("Kubeconfig: %s\n", cfg.Provider.Kubeconfig)
			}
			if cfg.Provider.Context != "" {
				log.Info("Context: %s\n", cfg.Provider.Context)
			}
		}
		log.Info("\n")

		// Get provider
		provider, err := getProviderFromTemplate(cfg)
		if err != nil {
			return err
		}

		// For existing provider, just validate connection
		// For kind/k3d, create the cluster
		if cfg.Provider.Type == "existing" {
			log.Info("Using existing cluster...\n")
			if err := provider.Create(cfg); err != nil {
				return fmt.Errorf("failed to connect to existing cluster: %w", err)
			}
			log.Success("Connected to existing cluster\n")
		} else {
			log.Info("Creating cluster...\n")
			if err := provider.Create(cfg); err != nil {
				return fmt.Errorf("failed to create cluster: %w", err)
			}
			log.Success("Cluster created\n")
		}

		// Determine kubecontext based on provider
		kubecontext := provider.KubeContext(cfg.Name)

		// Install plugins with result tracking
		results := installPlugins(cfg, kubecontext, runFailFast)

		// Print summary and final info
		if len(results) > 0 {
			printPluginSummary(results, log)
		}

		log.Info("\nTo use the cluster:\n")
		log.Info("  kubectl cluster-info --context %s\n", kubecontext)

		if hasErrors(results) {
			return fmt.Errorf("some plugins failed to install, see summary above")
		}

		return nil
	},
}

func init() {
	runCmd.Flags().StringVarP(&runTemplateFile, "template", "t", "template.yaml", "cluster template file")
	runCmd.Flags().StringVarP(&runEnvFile, "env", "e", ".env", "environment file for secrets")
	runCmd.Flags().BoolVar(&runFailFast, "fail-fast", false, "stop at first plugin failure")
	rootCmd.AddCommand(runCmd)
}
