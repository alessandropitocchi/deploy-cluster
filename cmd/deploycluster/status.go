package main

import (
	"fmt"

	"github.com/alessandropitocchi/deploy-cluster/pkg/logger"
	"github.com/alessandropitocchi/deploy-cluster/pkg/plugin/argocd"
	"github.com/alessandropitocchi/deploy-cluster/pkg/plugin/certmanager"
	"github.com/alessandropitocchi/deploy-cluster/pkg/plugin/customapps"
	"github.com/alessandropitocchi/deploy-cluster/pkg/plugin/dashboard"
	"github.com/alessandropitocchi/deploy-cluster/pkg/plugin/ingress"
	"github.com/alessandropitocchi/deploy-cluster/pkg/plugin/monitoring"
	"github.com/alessandropitocchi/deploy-cluster/pkg/plugin/storage"
	"github.com/alessandropitocchi/deploy-cluster/pkg/template"
	"github.com/spf13/cobra"
)

var (
	statusTemplateFile string
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the current status of a cluster",
	Long:  `Show the current status of a cluster, including existence and installed plugins.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := template.Load(statusTemplateFile)
		if err != nil {
			return fmt.Errorf("failed to load template: %w", err)
		}

		// Check cluster existence
		provider, err := getProviderFromTemplate(cfg)
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

		// Use quiet logger for status checks — plugins should not print during status
		quietLog := logger.New("", logger.LevelQuiet)

		// Storage status
		storagePlugin := storage.New(quietLog, globalTimeout)
		storageInstalled, err := storagePlugin.IsInstalled(kubecontext)
		if err != nil {
			fmt.Printf("\nStorage: error checking (%v)\n", err)
		} else if storageInstalled {
			fmt.Printf("\nStorage: installed (local-path-provisioner)\n")
		} else {
			fmt.Printf("\nStorage: not installed\n")
		}

		// Ingress status
		ingressPlugin := ingress.New(quietLog, globalTimeout)
		ingressInstalled, err := ingressPlugin.IsInstalled(kubecontext)
		if err != nil {
			fmt.Printf("\nIngress: error checking (%v)\n", err)
		} else if ingressInstalled {
			fmt.Printf("\nIngress: installed (nginx)\n")
		} else {
			fmt.Printf("\nIngress: not installed\n")
		}

		// Cert-manager status
		cmPlugin := certmanager.New(quietLog, globalTimeout)
		cmInstalled, err := cmPlugin.IsInstalled(kubecontext)
		if err != nil {
			fmt.Printf("\nCert-manager: error checking (%v)\n", err)
		} else if cmInstalled {
			fmt.Printf("\nCert-manager: installed\n")
		} else {
			fmt.Printf("\nCert-manager: not installed\n")
		}

		// Monitoring status
		monPlugin := monitoring.New(quietLog, globalTimeout)
		monInstalled, err := monPlugin.IsInstalled(kubecontext)
		if err != nil {
			fmt.Printf("\nMonitoring: error checking (%v)\n", err)
		} else if monInstalled {
			fmt.Printf("\nMonitoring: installed (prometheus)\n")
		} else {
			fmt.Printf("\nMonitoring: not installed\n")
		}

		// Dashboard status
		dashPlugin := dashboard.New(quietLog, globalTimeout)
		dashInstalled, err := dashPlugin.IsInstalled(kubecontext)
		if err != nil {
			fmt.Printf("\nDashboard: error checking (%v)\n", err)
		} else if dashInstalled {
			fmt.Printf("\nDashboard: installed (headlamp)\n")
		} else {
			fmt.Printf("\nDashboard: not installed\n")
		}

		// Custom Apps status
		if len(cfg.Plugins.CustomApps) > 0 {
			customPlugin := customapps.New(quietLog, globalTimeout)
			installed, _ := customPlugin.ListInstalled(cfg.Plugins.CustomApps, kubecontext)
			installedSet := make(map[string]bool)
			for _, name := range installed {
				installedSet[name] = true
			}
			fmt.Printf("\nCustom Apps (%d configured):\n", len(cfg.Plugins.CustomApps))
			for _, app := range cfg.Plugins.CustomApps {
				if installedSet[app.Name] {
					fmt.Printf("  - %s: installed\n", app.Name)
				} else {
					fmt.Printf("  - %s: not installed\n", app.Name)
				}
			}
		}

		// ArgoCD status
		argoPlugin := argocd.New(quietLog, globalTimeout)

		namespace := "argocd"
		if cfg.Plugins.ArgoCD != nil && cfg.Plugins.ArgoCD.Namespace != "" {
			namespace = cfg.Plugins.ArgoCD.Namespace
		}

		installed, err := argoPlugin.IsInstalledInNamespace(kubecontext, namespace)
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
	statusCmd.Flags().StringVarP(&statusTemplateFile, "template", "t", "template.yaml", "cluster template file")
	rootCmd.AddCommand(statusCmd)
}
