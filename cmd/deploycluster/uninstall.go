package main

import (
	"fmt"

	"github.com/alepito/deploy-cluster/pkg/plugin/argocd"
	"github.com/alepito/deploy-cluster/pkg/plugin/certmanager"
	"github.com/alepito/deploy-cluster/pkg/plugin/customapps"
	"github.com/alepito/deploy-cluster/pkg/plugin/dashboard"
	"github.com/alepito/deploy-cluster/pkg/plugin/ingress"
	"github.com/alepito/deploy-cluster/pkg/plugin/monitoring"
	"github.com/alepito/deploy-cluster/pkg/plugin/storage"
	"github.com/alepito/deploy-cluster/pkg/template"
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

		log.Info("Uninstalling plugins from cluster '%s'...\n\n", cfg.Name)

		results := uninstallPlugins(cfg, kubecontext, uninstallFailFast)

		if len(results) > 0 {
			printSummary(results, log)
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
func uninstallPlugins(cfg *template.Template, kubecontext string, failFast bool) []pluginResult {
	var results []pluginResult

	// Reverse order: ArgoCD → customApps → dashboard → monitoring → cert-manager → ingress → storage

	if cfg.Plugins.ArgoCD != nil && cfg.Plugins.ArgoCD.Enabled {
		pluginLog := newLogger("[argocd]")
		argoPlugin := argocd.New(pluginLog, globalTimeout)
		namespace := cfg.Plugins.ArgoCD.Namespace
		if namespace == "" {
			namespace = "argocd"
		}
		err := argoPlugin.Uninstall(kubecontext, namespace)
		results = append(results, pluginResult{Name: "argocd", Err: err})
		if err != nil && failFast {
			return results
		}
	}

	if len(cfg.Plugins.CustomApps) > 0 {
		pluginLog := newLogger("[customApps]")
		customPlugin := customapps.New(pluginLog, globalTimeout)
		var firstErr error
		for _, app := range cfg.Plugins.CustomApps {
			ns := app.Namespace
			if ns == "" {
				ns = app.Name
			}
			if err := customPlugin.Uninstall(app.Name, ns, kubecontext); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		results = append(results, pluginResult{Name: "customApps", Err: firstErr})
		if firstErr != nil && failFast {
			return results
		}
	}

	if cfg.Plugins.Dashboard != nil && cfg.Plugins.Dashboard.Enabled {
		pluginLog := newLogger("[dashboard]")
		dashPlugin := dashboard.New(pluginLog, globalTimeout)
		err := dashPlugin.Uninstall(cfg.Plugins.Dashboard, kubecontext)
		results = append(results, pluginResult{Name: "dashboard", Err: err})
		if err != nil && failFast {
			return results
		}
	}

	if cfg.Plugins.Monitoring != nil && cfg.Plugins.Monitoring.Enabled {
		pluginLog := newLogger("[monitoring]")
		monPlugin := monitoring.New(pluginLog, globalTimeout)
		err := monPlugin.Uninstall(cfg.Plugins.Monitoring, kubecontext)
		results = append(results, pluginResult{Name: "monitoring", Err: err})
		if err != nil && failFast {
			return results
		}
	}

	if cfg.Plugins.CertManager != nil && cfg.Plugins.CertManager.Enabled {
		pluginLog := newLogger("[cert-manager]")
		cmPlugin := certmanager.New(pluginLog, globalTimeout)
		err := cmPlugin.Uninstall(cfg.Plugins.CertManager, kubecontext)
		results = append(results, pluginResult{Name: "cert-manager", Err: err})
		if err != nil && failFast {
			return results
		}
	}

	if cfg.Plugins.Ingress != nil && cfg.Plugins.Ingress.Enabled {
		pluginLog := newLogger("[ingress]")
		ingressPlugin := ingress.New(pluginLog, globalTimeout)
		err := ingressPlugin.Uninstall(cfg.Plugins.Ingress, kubecontext)
		results = append(results, pluginResult{Name: "ingress", Err: err})
		if err != nil && failFast {
			return results
		}
	}

	if cfg.Plugins.Storage != nil && cfg.Plugins.Storage.Enabled {
		pluginLog := newLogger("[storage]")
		storagePlugin := storage.New(pluginLog, globalTimeout)
		err := storagePlugin.Uninstall(cfg.Plugins.Storage, kubecontext)
		results = append(results, pluginResult{Name: "storage", Err: err})
	}

	return results
}

func init() {
	uninstallCmd.Flags().StringVarP(&uninstallTemplateFile, "template", "t", "template.yaml", "cluster template file")
	uninstallCmd.Flags().StringVarP(&uninstallEnvFile, "env", "e", ".env", "environment file for secrets")
	uninstallCmd.Flags().BoolVar(&uninstallFailFast, "fail-fast", false, "stop at first plugin failure")
	rootCmd.AddCommand(uninstallCmd)
}
