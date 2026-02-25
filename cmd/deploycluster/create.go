package main

import (
	"fmt"

	"github.com/alepito/deploy-cluster/pkg/template"
	"github.com/spf13/cobra"
)

var (
	createTemplateFile string
	createEnvFile      string
	createFailFast     bool
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new cluster from template",
	Long:  `Create a new Kubernetes cluster based on the provided template file.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		log := newLogger("")

		// Load .env file
		if err := template.LoadEnvFile(createEnvFile); err != nil {
			return fmt.Errorf("failed to load env file: %w", err)
		}

		// Load config
		log.Info("Loading template from %s...\n", createTemplateFile)
		cfg, err := template.Load(createTemplateFile)
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

		// Create cluster
		if err := provider.Create(cfg); err != nil {
			return err
		}

		// Determine kubecontext based on provider
		kubecontext := provider.KubeContext(cfg.Name)

		// Install plugins with result tracking
		results := installPlugins(cfg, kubecontext, createFailFast)

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
	createCmd.Flags().StringVarP(&createTemplateFile, "template", "t", "template.yaml", "cluster template file")
	createCmd.Flags().StringVarP(&createEnvFile, "env", "e", ".env", "environment file for secrets")
	createCmd.Flags().BoolVar(&createFailFast, "fail-fast", false, "stop at first plugin failure")
	rootCmd.AddCommand(createCmd)
}
