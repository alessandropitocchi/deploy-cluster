package main

import (
	"fmt"

	"github.com/alepito/deploy-cluster/pkg/config"
	"github.com/alepito/deploy-cluster/pkg/plugin/argocd"
	"github.com/alepito/deploy-cluster/pkg/plugin/storage"
	"github.com/spf13/cobra"
)

var (
	upgradeConfigFile string
	upgradeEnvFile    string
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade an existing cluster applying configuration changes",
	Long: `Upgrade an existing Kubernetes cluster by applying only the differences
compared to the current state. The cluster is not recreated, but plugins
(ArgoCD repos/apps) are updated: additions, modifications and removals.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load .env file
		if err := config.LoadEnvFile(upgradeEnvFile); err != nil {
			return fmt.Errorf("failed to load env file: %w", err)
		}

		// Load config
		fmt.Printf("Loading configuration from %s...\n", upgradeConfigFile)
		cfg, err := config.Load(upgradeConfigFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
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

		fmt.Printf("Upgrading cluster '%s'...\n\n", cfg.Name)

		// Determine kubecontext based on provider
		kubecontext := fmt.Sprintf("kind-%s", cfg.Name)

		// Upgrade storage plugin
		if cfg.Plugins.Storage != nil && cfg.Plugins.Storage.Enabled {
			storagePlugin := storage.New()
			installed, err := storagePlugin.IsInstalled(kubecontext)
			if err != nil {
				return fmt.Errorf("failed to check storage status: %w", err)
			}
			if !installed {
				fmt.Println("[storage] Storage not installed, performing installation...")
				if err := storagePlugin.Install(cfg.Plugins.Storage, kubecontext); err != nil {
					return fmt.Errorf("failed to install storage: %w", err)
				}
			} else {
				fmt.Println("[storage] Storage already installed, re-applying...")
				if err := storagePlugin.Install(cfg.Plugins.Storage, kubecontext); err != nil {
					return fmt.Errorf("failed to upgrade storage: %w", err)
				}
			}
		}

		// Upgrade ArgoCD plugin
		argoPlugin := argocd.New()

		if cfg.Plugins.ArgoCD != nil && cfg.Plugins.ArgoCD.Enabled {
			namespace := cfg.Plugins.ArgoCD.Namespace
			if namespace == "" {
				namespace = "argocd"
			}

			installed, err := argoPlugin.IsInstalled(kubecontext, namespace)
			if err != nil {
				return fmt.Errorf("failed to check ArgoCD status: %w", err)
			}

			if !installed {
				// Not installed yet, do a full install
				fmt.Println("[argocd] ArgoCD not installed, performing full installation...")
				if err := argoPlugin.Install(cfg.Plugins.ArgoCD, kubecontext); err != nil {
					return fmt.Errorf("failed to install ArgoCD: %w", err)
				}
			} else {
				// Already installed, perform upgrade (diff-based)
				if err := argoPlugin.Upgrade(cfg.Plugins.ArgoCD, kubecontext); err != nil {
					return fmt.Errorf("failed to upgrade ArgoCD: %w", err)
				}
			}
		} else if cfg.Plugins.ArgoCD != nil && !cfg.Plugins.ArgoCD.Enabled {
			namespace := cfg.Plugins.ArgoCD.Namespace
			if namespace == "" {
				namespace = "argocd"
			}
			installed, err := argoPlugin.IsInstalled(kubecontext, namespace)
			if err == nil && installed {
				fmt.Println("[argocd] WARNING: ArgoCD is installed but disabled in config. It will NOT be automatically uninstalled.")
				fmt.Printf("[argocd] To uninstall manually: kubectl delete namespace %s --context %s\n", namespace, kubecontext)
			}
		}

		fmt.Printf("\nUpgrade completed for cluster '%s'.\n", cfg.Name)
		return nil
	},
}

func init() {
	upgradeCmd.Flags().StringVarP(&upgradeConfigFile, "config", "c", "cluster.yaml", "cluster configuration file")
	upgradeCmd.Flags().StringVarP(&upgradeEnvFile, "env", "e", ".env", "environment file for secrets")
	rootCmd.AddCommand(upgradeCmd)
}
