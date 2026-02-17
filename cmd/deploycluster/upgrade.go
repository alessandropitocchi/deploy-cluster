package main

import (
	"fmt"

	"github.com/alepito/deploy-cluster/pkg/config"
	"github.com/alepito/deploy-cluster/pkg/plugin/argocd"
	"github.com/alepito/deploy-cluster/pkg/plugin/ingress"
	"github.com/alepito/deploy-cluster/pkg/plugin/storage"
	"github.com/spf13/cobra"
)

var (
	upgradeConfigFile string
	upgradeEnvFile    string
	upgradeDryRun     bool
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade an existing cluster applying configuration changes",
	Long: `Upgrade an existing Kubernetes cluster by applying only the differences
compared to the current state. The cluster is not recreated, but plugins
(ArgoCD repos/apps) are updated: additions, modifications and removals.

Use --dry-run to preview changes without applying them.`,
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

		kubecontext := provider.KubeContext(cfg.Name)

		if upgradeDryRun {
			return runUpgradeDryRun(cfg, kubecontext)
		}

		fmt.Printf("Upgrading cluster '%s'...\n\n", cfg.Name)

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

		// Upgrade ingress plugin
		if cfg.Plugins.Ingress != nil && cfg.Plugins.Ingress.Enabled {
			ingressPlugin := ingress.New()
			installed, err := ingressPlugin.IsInstalled(kubecontext)
			if err != nil {
				return fmt.Errorf("failed to check ingress status: %w", err)
			}
			if !installed {
				fmt.Println("[ingress] Ingress not installed, performing installation...")
				if err := ingressPlugin.Install(cfg.Plugins.Ingress, kubecontext); err != nil {
					return fmt.Errorf("failed to install ingress: %w", err)
				}
			} else {
				fmt.Println("[ingress] Ingress already installed, re-applying...")
				if err := ingressPlugin.Install(cfg.Plugins.Ingress, kubecontext); err != nil {
					return fmt.Errorf("failed to upgrade ingress: %w", err)
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
				fmt.Println("[argocd] ArgoCD not installed, performing full installation...")
				if err := argoPlugin.Install(cfg.Plugins.ArgoCD, kubecontext); err != nil {
					return fmt.Errorf("failed to install ArgoCD: %w", err)
				}
			} else {
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

func runUpgradeDryRun(cfg *config.Config, kubecontext string) error {
	fmt.Printf("Dry-run for cluster '%s':\n", cfg.Name)

	// Storage
	if cfg.Plugins.Storage != nil && cfg.Plugins.Storage.Enabled {
		storagePlugin := storage.New()
		storagePlugin.Verbose = false
		installed, err := storagePlugin.IsInstalled(kubecontext)
		if err != nil {
			return fmt.Errorf("failed to check storage status: %w", err)
		}
		if installed {
			fmt.Printf("\n[storage] %s: installed (re-apply)\n", cfg.Plugins.Storage.Type)
		} else {
			fmt.Printf("\n[storage] %s: not installed (will install)\n", cfg.Plugins.Storage.Type)
		}
	}

	// Ingress
	if cfg.Plugins.Ingress != nil && cfg.Plugins.Ingress.Enabled {
		ingressPlugin := ingress.New()
		ingressPlugin.Verbose = false
		installed, err := ingressPlugin.IsInstalled(kubecontext)
		if err != nil {
			return fmt.Errorf("failed to check ingress status: %w", err)
		}
		if installed {
			fmt.Printf("\n[ingress] %s: installed (re-apply)\n", cfg.Plugins.Ingress.Type)
		} else {
			fmt.Printf("\n[ingress] %s: not installed (will install)\n", cfg.Plugins.Ingress.Type)
		}
	}

	// ArgoCD
	if cfg.Plugins.ArgoCD != nil && cfg.Plugins.ArgoCD.Enabled {
		argoPlugin := argocd.New()
		argoPlugin.Verbose = false

		namespace := cfg.Plugins.ArgoCD.Namespace
		if namespace == "" {
			namespace = "argocd"
		}

		installed, err := argoPlugin.IsInstalled(kubecontext, namespace)
		if err != nil {
			return fmt.Errorf("failed to check ArgoCD status: %w", err)
		}

		if !installed {
			fmt.Printf("\n[argocd] Not installed (will perform full install)\n")
		} else {
			if err := argoPlugin.DryRun(cfg.Plugins.ArgoCD, kubecontext); err != nil {
				return fmt.Errorf("failed to dry-run ArgoCD: %w", err)
			}
		}
	}

	fmt.Println("\nNo changes applied (dry-run).")
	return nil
}

func init() {
	upgradeCmd.Flags().StringVarP(&upgradeConfigFile, "config", "c", "cluster.yaml", "cluster configuration file")
	upgradeCmd.Flags().StringVarP(&upgradeEnvFile, "env", "e", ".env", "environment file for secrets")
	upgradeCmd.Flags().BoolVar(&upgradeDryRun, "dry-run", false, "preview changes without applying them")
	rootCmd.AddCommand(upgradeCmd)
}
