package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/alessandropitocchi/deploy-cluster/pkg/plugin"
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
	uninstallTemplateFile string
	uninstallEnvFile      string
	uninstallFailFast     bool
)

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Uninstall plugins from a cluster",
	Long: `Uninstall all enabled plugins from an existing cluster in reverse order.
The cluster itself is NOT destroyed — only the plugins are removed.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		log := newLogger("")

		// Load .env file
		if err := template.LoadEnvFile(uninstallEnvFile); err != nil {
			return fmt.Errorf("failed to load env file: %w", err)
		}

		// Load config
		log.Info("Loading template from %s...\n", uninstallTemplateFile)
		cfg, err := template.Load(uninstallTemplateFile)
		if err != nil {
			return fmt.Errorf("failed to load template: %w", err)
		}

		// Get provider and check cluster exists
		provider, err := getProviderFromTemplate(cfg)
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

		log.Info("Uninstalling plugins from cluster '%s'...\n\n", cfg.Name)

		results := uninstallPlugins(cfg, kubecontext, uninstallFailFast)

		if len(results) > 0 {
			printUninstallSummary(results, log)
		} else {
			log.Info("No plugins to uninstall.\n")
		}

		if hasErrors(results) {
			return fmt.Errorf("some plugins failed to uninstall, see summary above")
		}

		log.Success("\nPlugins uninstalled from cluster '%s'.\n", cfg.Name)
		return nil
	},
}

// uninstallPlugins removes enabled plugins in reverse installation order.
func uninstallPlugins(cfg *template.Template, kubecontext string, failFast bool) []plugin.InstallResult {
	timeout := time.Duration(globalTimeout) * time.Second

	var results []plugin.InstallResult

	// Reverse order: ArgoCD → custom-apps → dashboard → monitoring → cert-manager → ingress → storage

	if cfg.Plugins.ArgoCD != nil && cfg.Plugins.ArgoCD.Enabled {
		pluginLog := newLogger("[argocd]")
		argoPlugin := argocd.New(pluginLog, timeout)
		err := argoPlugin.Uninstall(cfg.Plugins.ArgoCD, kubecontext)
		results = append(results, plugin.InstallResult{Name: "argocd", Err: err})
		if err != nil && failFast {
			return results
		}
	}

	if len(cfg.Plugins.CustomApps) > 0 {
		pluginLog := newLogger("[custom-apps]")
		customPlugin := customapps.New(pluginLog, timeout)
		err := customPlugin.Uninstall(cfg.Plugins.CustomApps, kubecontext)
		results = append(results, plugin.InstallResult{Name: "custom-apps", Err: err})
		if err != nil && failFast {
			return results
		}
	}

	if cfg.Plugins.Dashboard != nil && cfg.Plugins.Dashboard.Enabled {
		pluginLog := newLogger("[dashboard]")
		dashPlugin := dashboard.New(pluginLog, timeout)
		err := dashPlugin.Uninstall(cfg.Plugins.Dashboard, kubecontext)
		results = append(results, plugin.InstallResult{Name: "dashboard", Err: err})
		if err != nil && failFast {
			return results
		}
	}

	if cfg.Plugins.Monitoring != nil && cfg.Plugins.Monitoring.Enabled {
		pluginLog := newLogger("[monitoring]")
		monPlugin := monitoring.New(pluginLog, timeout)
		err := monPlugin.Uninstall(cfg.Plugins.Monitoring, kubecontext)
		results = append(results, plugin.InstallResult{Name: "monitoring", Err: err})
		if err != nil && failFast {
			return results
		}
	}

	if cfg.Plugins.CertManager != nil && cfg.Plugins.CertManager.Enabled {
		pluginLog := newLogger("[cert-manager]")
		cmPlugin := certmanager.New(pluginLog, timeout)
		err := cmPlugin.Uninstall(cfg.Plugins.CertManager, kubecontext)
		results = append(results, plugin.InstallResult{Name: "cert-manager", Err: err})
		if err != nil && failFast {
			return results
		}
	}

	if cfg.Plugins.Ingress != nil && cfg.Plugins.Ingress.Enabled {
		pluginLog := newLogger("[ingress]")
		ingressPlugin := ingress.New(pluginLog, timeout)
		err := ingressPlugin.Uninstall(cfg.Plugins.Ingress, kubecontext)
		results = append(results, plugin.InstallResult{Name: "ingress", Err: err})
		if err != nil && failFast {
			return results
		}
	}

	if cfg.Plugins.Storage != nil && cfg.Plugins.Storage.Enabled {
		pluginLog := newLogger("[storage]")
		storagePlugin := storage.New(pluginLog, timeout)
		err := storagePlugin.Uninstall(cfg.Plugins.Storage, kubecontext)
		results = append(results, plugin.InstallResult{Name: "storage", Err: err})
	}

	return results
}

// printUninstallSummary prints a summary of plugin uninstallation results.
func printUninstallSummary(results []plugin.InstallResult, log interface {
	Info(string, ...any)
	Success(string, ...any)
	Error(string, ...any)
}) {
	fmt.Println()
	fmt.Println(strings.Repeat("-", 40))
	fmt.Println("Plugin Uninstallation Summary:")
	fmt.Println(strings.Repeat("-", 40))

	successful := 0
	failed := 0

	for _, r := range results {
		if r.Err != nil {
			fmt.Printf("  ✗ %s: %v\n", r.Name, r.Err)
			failed++
		} else {
			fmt.Printf("  ✓ %s\n", r.Name)
			successful++
		}
	}

	fmt.Println(strings.Repeat("-", 40))
	fmt.Printf("Total: %d | Successful: %d | Failed: %d\n", len(results), successful, failed)
	fmt.Println(strings.Repeat("-", 40))
}

func init() {
	uninstallCmd.Flags().StringVarP(&uninstallTemplateFile, "template", "t", "template.yaml", "cluster template file")
	uninstallCmd.Flags().StringVarP(&uninstallEnvFile, "env", "e", ".env", "environment file for secrets")
	uninstallCmd.Flags().BoolVar(&uninstallFailFast, "fail-fast", false, "stop at first plugin failure")
	rootCmd.AddCommand(uninstallCmd)
}
