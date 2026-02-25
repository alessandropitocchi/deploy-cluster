package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/alepito/deploy-cluster/pkg/logger"
	"github.com/alepito/deploy-cluster/pkg/plugin"
	"github.com/alepito/deploy-cluster/pkg/plugin/argocd"
	"github.com/alepito/deploy-cluster/pkg/plugin/certmanager"
	"github.com/alepito/deploy-cluster/pkg/plugin/customapps"
	"github.com/alepito/deploy-cluster/pkg/plugin/dashboard"
	"github.com/alepito/deploy-cluster/pkg/plugin/externaldns"
	"github.com/alepito/deploy-cluster/pkg/plugin/ingress"
	"github.com/alepito/deploy-cluster/pkg/plugin/monitoring"
	"github.com/alepito/deploy-cluster/pkg/plugin/storage"
	"github.com/alepito/deploy-cluster/pkg/template"
)

// createRegistry creates and configures the plugin registry with all available plugins.
func createRegistry(log *logger.Logger, timeout time.Duration) *plugin.Registry {
	registry := plugin.NewRegistry()

	registry.Register(storage.New(log.WithPrefix("[storage]"), timeout))
	registry.Register(ingress.New(log.WithPrefix("[ingress]"), timeout))
	registry.Register(certmanager.New(log.WithPrefix("[cert-manager]"), timeout))
	registry.Register(externaldns.New(log.WithPrefix("[external-dns]"), timeout))
	registry.Register(monitoring.New(log.WithPrefix("[monitoring]"), timeout))
	registry.Register(dashboard.New(log.WithPrefix("[dashboard]"), timeout))
	registry.Register(customapps.New(log.WithPrefix("[custom-apps]"), timeout))
	registry.Register(argocd.New(log.WithPrefix("[argocd]"), timeout))

	return registry
}

// installPlugins installs all enabled plugins using the unified plugin system.
func installPlugins(cfg *template.Template, kubecontext string, failFast bool) []plugin.InstallResult {
	log := newLogger("")
	timeout := time.Duration(globalTimeout) * time.Second

	// Create registry and manager
	registry := createRegistry(log, timeout)
	manager := plugin.NewManager(registry, log,
		plugin.WithFailFast(failFast),
	)

	// Get enabled plugins from template
	enabledPlugins := plugin.GetEnabledPlugins(cfg)

	// Install options
	opts := plugin.InstallConfig{
		Kubecontext:  kubecontext,
		ProviderType: cfg.Provider.Type,
	}

	// Install all plugins
	results := manager.InstallAll(enabledPlugins, opts)

	// Print summary
	if len(results) > 0 {
		printPluginSummary(results, log)
	}

	return results
}

// upgradePlugins upgrades all enabled plugins using the unified plugin system.
func upgradePlugins(cfg *template.Template, kubecontext string, failFast bool) []plugin.InstallResult {
	log := newLogger("")
	timeout := time.Duration(globalTimeout) * time.Second

	// Create registry and manager
	registry := createRegistry(log, timeout)
	manager := plugin.NewManager(registry, log,
		plugin.WithFailFast(failFast),
	)

	// Get enabled plugins from template
	enabledPlugins := plugin.GetEnabledPlugins(cfg)

	// Upgrade options
	opts := plugin.InstallConfig{
		Kubecontext:  kubecontext,
		ProviderType: cfg.Provider.Type,
	}

	// Upgrade all plugins
	results := manager.UpgradeAll(enabledPlugins, opts)

	// Check for disabled but installed ArgoCD
	if cfg.Plugins.ArgoCD != nil && !cfg.Plugins.ArgoCD.Enabled {
		checkDisabledArgoCD(cfg, kubecontext, log)
	}

	// Print summary
	if len(results) > 0 {
		printPluginSummary(results, log)
	}

	return results
}

// dryRunPlugins performs a dry run of all enabled plugins.
func dryRunPlugins(cfg *template.Template, kubecontext string) []plugin.InstallResult {
	log := newLogger("")
	timeout := time.Duration(globalTimeout) * time.Second

	// Create registry and manager
	registry := createRegistry(log, timeout)
	manager := plugin.NewManager(registry, log)

	// Get enabled plugins from template
	enabledPlugins := plugin.GetEnabledPlugins(cfg)

	// Dry run options
	opts := plugin.InstallConfig{
		Kubecontext:  kubecontext,
		ProviderType: cfg.Provider.Type,
	}

	// Dry run all plugins
	return manager.DryRun(enabledPlugins, opts)
}

// checkDisabledArgoCD warns if ArgoCD is installed but disabled in template.
func checkDisabledArgoCD(cfg *template.Template, kubecontext string, log *logger.Logger) {
	argoPlugin := argocd.New(log, time.Duration(globalTimeout)*time.Second)
	namespace := cfg.Plugins.ArgoCD.Namespace
	if namespace == "" {
		namespace = "argocd"
	}
	installed, err := argoPlugin.IsInstalledInNamespace(kubecontext, namespace)
	if err == nil && installed {
		log.Warn("[argocd] WARNING: ArgoCD is installed but disabled in template. It will NOT be automatically uninstalled.\n")
		log.Warn("[argocd] To uninstall manually: kubectl delete namespace %s --context %s\n", namespace, kubecontext)
	}
}

// printPluginSummary prints a summary of plugin installation results.
func printPluginSummary(results []plugin.InstallResult, log interface {
	Info(string, ...any)
	Success(string, ...any)
	Error(string, ...any)
}) {
	fmt.Println()
	fmt.Println(strings.Repeat("-", 40))
	fmt.Println("Plugin Installation Summary:")
	fmt.Println(strings.Repeat("-", 40))

	successful := 0
	skipped := 0
	failed := 0

	for _, r := range results {
		if r.Err != nil {
			fmt.Printf("  ✗ %s: %v\n", r.Name, r.Err)
			failed++
		} else if r.Skipped {
			fmt.Printf("  ⊘ %s (skipped - already installed)\n", r.Name)
			skipped++
		} else {
			fmt.Printf("  ✓ %s\n", r.Name)
			successful++
		}
	}

	fmt.Println(strings.Repeat("-", 40))
	fmt.Printf("Total: %d | Successful: %d | Skipped: %d | Failed: %d\n",
		len(results), successful, skipped, failed)
	fmt.Println(strings.Repeat("-", 40))
}

// hasErrors checks if any result has an error.
func hasErrors(results []plugin.InstallResult) bool {
	for _, r := range results {
		if r.Err != nil {
			return true
		}
	}
	return false
}
