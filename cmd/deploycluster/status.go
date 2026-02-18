package main

import (
	"fmt"

	"github.com/alepito/deploy-cluster/pkg/config"
	"github.com/alepito/deploy-cluster/pkg/plugin/argocd"
	"github.com/alepito/deploy-cluster/pkg/plugin/certmanager"
	"github.com/alepito/deploy-cluster/pkg/plugin/dashboard"
	"github.com/alepito/deploy-cluster/pkg/plugin/ingress"
	"github.com/alepito/deploy-cluster/pkg/plugin/monitoring"
	"github.com/alepito/deploy-cluster/pkg/plugin/storage"
	"github.com/spf13/cobra"
)

var (
	statusConfigFile string
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the current status of a cluster",
	Long:  `Show the current status of a cluster, including existence and installed plugins.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(statusConfigFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Check cluster existence
		provider, err := getProvider(cfg.Provider.Type)
		if err != nil {
			return err
		}

		exists, err := provider.Exists(cfg.Name)
		if err != nil {
			return fmt.Errorf("failed to check cluster: %w", err)
		}

		fmt.Printf("Cluster: %s\n", cfg.Name)
		fmt.Printf("Provider: %s\n", cfg.Provider.Type)
		if exists {
			fmt.Printf("Status: running\n")
		} else {
			fmt.Printf("Status: not found\n")
			return nil
		}

		kubecontext := provider.KubeContext(cfg.Name)

		// Storage status
		storagePlugin := storage.New()
		storagePlugin.Verbose = false
		storageInstalled, err := storagePlugin.IsInstalled(kubecontext)
		if err != nil {
			fmt.Printf("\nStorage: error checking (%v)\n", err)
		} else if storageInstalled {
			fmt.Printf("\nStorage: installed (local-path-provisioner)\n")
		} else {
			fmt.Printf("\nStorage: not installed\n")
		}

		// Ingress status
		ingressPlugin := ingress.New()
		ingressPlugin.Verbose = false
		ingressInstalled, err := ingressPlugin.IsInstalled(kubecontext)
		if err != nil {
			fmt.Printf("\nIngress: error checking (%v)\n", err)
		} else if ingressInstalled {
			fmt.Printf("\nIngress: installed (nginx)\n")
		} else {
			fmt.Printf("\nIngress: not installed\n")
		}

		// Cert-manager status
		cmPlugin := certmanager.New()
		cmPlugin.Verbose = false
		cmInstalled, err := cmPlugin.IsInstalled(kubecontext)
		if err != nil {
			fmt.Printf("\nCert-manager: error checking (%v)\n", err)
		} else if cmInstalled {
			fmt.Printf("\nCert-manager: installed\n")
		} else {
			fmt.Printf("\nCert-manager: not installed\n")
		}

		// Monitoring status
		monPlugin := monitoring.New()
		monPlugin.Verbose = false
		monInstalled, err := monPlugin.IsInstalled(kubecontext)
		if err != nil {
			fmt.Printf("\nMonitoring: error checking (%v)\n", err)
		} else if monInstalled {
			fmt.Printf("\nMonitoring: installed (prometheus)\n")
		} else {
			fmt.Printf("\nMonitoring: not installed\n")
		}

		// Dashboard status
		dashPlugin := dashboard.New()
		dashPlugin.Verbose = false
		dashInstalled, err := dashPlugin.IsInstalled(kubecontext)
		if err != nil {
			fmt.Printf("\nDashboard: error checking (%v)\n", err)
		} else if dashInstalled {
			fmt.Printf("\nDashboard: installed (headlamp)\n")
		} else {
			fmt.Printf("\nDashboard: not installed\n")
		}

		// ArgoCD status
		argoPlugin := argocd.New()
		argoPlugin.Verbose = false

		namespace := "argocd"
		if cfg.Plugins.ArgoCD != nil && cfg.Plugins.ArgoCD.Namespace != "" {
			namespace = cfg.Plugins.ArgoCD.Namespace
		}

		installed, err := argoPlugin.IsInstalled(kubecontext, namespace)
		if err != nil {
			fmt.Printf("\nArgoCD: error checking (%v)\n", err)
			return nil
		}

		if !installed {
			fmt.Printf("\nArgoCD: not installed\n")
			return nil
		}

		fmt.Printf("\nArgoCD: installed (namespace: %s)\n", namespace)

		repos, err := argoPlugin.ListCurrentRepos(kubecontext, namespace)
		if err != nil {
			fmt.Printf("  Repos: error listing (%v)\n", err)
		} else if len(repos) == 0 {
			fmt.Printf("  Repos: none\n")
		} else {
			fmt.Printf("  Repos (%d):\n", len(repos))
			for _, name := range repos {
				fmt.Printf("    - %s\n", name)
			}
		}

		apps, err := argoPlugin.ListCurrentApps(kubecontext, namespace)
		if err != nil {
			fmt.Printf("  Apps: error listing (%v)\n", err)
		} else if len(apps) == 0 {
			fmt.Printf("  Apps: none\n")
		} else {
			fmt.Printf("  Apps (%d):\n", len(apps))
			for _, name := range apps {
				fmt.Printf("    - %s\n", name)
			}
		}

		return nil
	},
}

func init() {
	statusCmd.Flags().StringVarP(&statusConfigFile, "config", "c", "cluster.yaml", "cluster configuration file")
	rootCmd.AddCommand(statusCmd)
}
