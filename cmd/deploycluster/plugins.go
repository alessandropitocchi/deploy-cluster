package main

import (
	"fmt"
	"strings"

	"github.com/alepito/deploy-cluster/pkg/plugin/argocd"
	"github.com/alepito/deploy-cluster/pkg/plugin/certmanager"
	"github.com/alepito/deploy-cluster/pkg/plugin/customapps"
	"github.com/alepito/deploy-cluster/pkg/plugin/dashboard"
	"github.com/alepito/deploy-cluster/pkg/plugin/ingress"
	"github.com/alepito/deploy-cluster/pkg/plugin/monitoring"
	"github.com/alepito/deploy-cluster/pkg/plugin/storage"
	"github.com/alepito/deploy-cluster/pkg/template"
)

// pluginResult tracks the outcome of a plugin installation.
type pluginResult struct {
	Name string
	Err  error
}

// installPlugins runs all enabled plugins in order (for create).
func installPlugins(cfg *template.Template, kubecontext string, failFast bool) []pluginResult {
	log := newLogger("")
	var results []pluginResult

	if cfg.Plugins.Storage != nil && cfg.Plugins.Storage.Enabled {
		log.Info("\n")
		pluginLog := newLogger("[storage]")
		storagePlugin := storage.New(pluginLog, globalTimeout)
		err := storagePlugin.Install(cfg.Plugins.Storage, kubecontext)
		results = append(results, pluginResult{Name: "storage", Err: err})
		if err != nil && failFast {
			return results
		}
	}

	if cfg.Plugins.Ingress != nil && cfg.Plugins.Ingress.Enabled {
		log.Info("\n")
		pluginLog := newLogger("[ingress]")
		ingressPlugin := ingress.New(pluginLog, globalTimeout)
		err := ingressPlugin.Install(cfg.Plugins.Ingress, kubecontext, cfg.Provider.Type)
		results = append(results, pluginResult{Name: "ingress", Err: err})
		if err != nil && failFast {
			return results
		}
	}

	if cfg.Plugins.CertManager != nil && cfg.Plugins.CertManager.Enabled {
		log.Info("\n")
		pluginLog := newLogger("[cert-manager]")
		cmPlugin := certmanager.New(pluginLog, globalTimeout)
		err := cmPlugin.Install(cfg.Plugins.CertManager, kubecontext)
		results = append(results, pluginResult{Name: "cert-manager", Err: err})
		if err != nil && failFast {
			return results
		}
	}

	if cfg.Plugins.Monitoring != nil && cfg.Plugins.Monitoring.Enabled {
		log.Info("\n")
		pluginLog := newLogger("[monitoring]")
		monPlugin := monitoring.New(pluginLog, globalTimeout)
		err := monPlugin.Install(cfg.Plugins.Monitoring, kubecontext)
		results = append(results, pluginResult{Name: "monitoring", Err: err})
		if err != nil && failFast {
			return results
		}
	}

	if cfg.Plugins.Dashboard != nil && cfg.Plugins.Dashboard.Enabled {
		log.Info("\n")
		pluginLog := newLogger("[dashboard]")
		dashPlugin := dashboard.New(pluginLog, globalTimeout)
		err := dashPlugin.Install(cfg.Plugins.Dashboard, kubecontext)
		results = append(results, pluginResult{Name: "dashboard", Err: err})
		if err != nil && failFast {
			return results
		}
	}

	if len(cfg.Plugins.CustomApps) > 0 {
		log.Info("\n")
		pluginLog := newLogger("[customApps]")
		customPlugin := customapps.New(pluginLog, globalTimeout)
		err := customPlugin.InstallAll(cfg.Plugins.CustomApps, kubecontext)
		results = append(results, pluginResult{Name: "customApps", Err: err})
		if err != nil && failFast {
			return results
		}
	}

	if cfg.Plugins.ArgoCD != nil && cfg.Plugins.ArgoCD.Enabled {
		log.Info("\n")
		pluginLog := newLogger("[argocd]")
		argoPlugin := argocd.New(pluginLog, globalTimeout)
		err := argoPlugin.Install(cfg.Plugins.ArgoCD, kubecontext)
		results = append(results, pluginResult{Name: "argocd", Err: err})
	}

	return results
}

// upgradePlugins runs all enabled plugins in order (for upgrade).
// Checks IsInstalled first and logs accordingly.
// Uses argoPlugin.Upgrade() when ArgoCD is already installed.
func upgradePlugins(cfg *template.Template, kubecontext string, failFast bool) []pluginResult {
	log := newLogger("")
	var results []pluginResult

	if cfg.Plugins.Storage != nil && cfg.Plugins.Storage.Enabled {
		pluginLog := newLogger("[storage]")
		storagePlugin := storage.New(pluginLog, globalTimeout)
		installed, checkErr := storagePlugin.IsInstalled(kubecontext)
		if checkErr != nil {
			results = append(results, pluginResult{Name: "storage", Err: fmt.Errorf("failed to check status: %w", checkErr)})
		} else {
			if !installed {
				pluginLog.Info("Storage not installed, performing installation...\n")
			} else {
				pluginLog.Info("Storage already installed, re-applying...\n")
			}
			err := storagePlugin.Install(cfg.Plugins.Storage, kubecontext)
			results = append(results, pluginResult{Name: "storage", Err: err})
		}
		if hasErrors(results) && failFast {
			return results
		}
	}

	if cfg.Plugins.Ingress != nil && cfg.Plugins.Ingress.Enabled {
		pluginLog := newLogger("[ingress]")
		ingressPlugin := ingress.New(pluginLog, globalTimeout)
		installed, checkErr := ingressPlugin.IsInstalled(kubecontext)
		if checkErr != nil {
			results = append(results, pluginResult{Name: "ingress", Err: fmt.Errorf("failed to check status: %w", checkErr)})
		} else {
			if !installed {
				pluginLog.Info("Ingress not installed, performing installation...\n")
			} else {
				pluginLog.Info("Ingress already installed, re-applying...\n")
			}
			err := ingressPlugin.Install(cfg.Plugins.Ingress, kubecontext, cfg.Provider.Type)
			results = append(results, pluginResult{Name: "ingress", Err: err})
		}
		if hasErrors(results) && failFast {
			return results
		}
	}

	if cfg.Plugins.CertManager != nil && cfg.Plugins.CertManager.Enabled {
		pluginLog := newLogger("[cert-manager]")
		cmPlugin := certmanager.New(pluginLog, globalTimeout)
		installed, checkErr := cmPlugin.IsInstalled(kubecontext)
		if checkErr != nil {
			results = append(results, pluginResult{Name: "cert-manager", Err: fmt.Errorf("failed to check status: %w", checkErr)})
		} else {
			if !installed {
				pluginLog.Info("Not installed, performing installation...\n")
			} else {
				pluginLog.Info("Already installed, re-applying...\n")
			}
			err := cmPlugin.Install(cfg.Plugins.CertManager, kubecontext)
			results = append(results, pluginResult{Name: "cert-manager", Err: err})
		}
		if hasErrors(results) && failFast {
			return results
		}
	}

	if cfg.Plugins.Monitoring != nil && cfg.Plugins.Monitoring.Enabled {
		pluginLog := newLogger("[monitoring]")
		monPlugin := monitoring.New(pluginLog, globalTimeout)
		installed, checkErr := monPlugin.IsInstalled(kubecontext)
		if checkErr != nil {
			results = append(results, pluginResult{Name: "monitoring", Err: fmt.Errorf("failed to check status: %w", checkErr)})
		} else {
			if !installed {
				pluginLog.Info("Not installed, performing installation...\n")
			} else {
				pluginLog.Info("Already installed, re-applying...\n")
			}
			err := monPlugin.Install(cfg.Plugins.Monitoring, kubecontext)
			results = append(results, pluginResult{Name: "monitoring", Err: err})
		}
		if hasErrors(results) && failFast {
			return results
		}
	}

	if cfg.Plugins.Dashboard != nil && cfg.Plugins.Dashboard.Enabled {
		pluginLog := newLogger("[dashboard]")
		dashPlugin := dashboard.New(pluginLog, globalTimeout)
		installed, checkErr := dashPlugin.IsInstalled(kubecontext)
		if checkErr != nil {
			results = append(results, pluginResult{Name: "dashboard", Err: fmt.Errorf("failed to check status: %w", checkErr)})
		} else {
			if !installed {
				pluginLog.Info("Dashboard not installed, performing installation...\n")
			} else {
				pluginLog.Info("Dashboard already installed, re-applying...\n")
			}
			err := dashPlugin.Install(cfg.Plugins.Dashboard, kubecontext)
			results = append(results, pluginResult{Name: "dashboard", Err: err})
		}
		if hasErrors(results) && failFast {
			return results
		}
	}

	if len(cfg.Plugins.CustomApps) > 0 {
		pluginLog := newLogger("[customApps]")
		pluginLog.Info("Upgrading custom apps...\n")
		customPlugin := customapps.New(pluginLog, globalTimeout)
		err := customPlugin.InstallAll(cfg.Plugins.CustomApps, kubecontext)
		results = append(results, pluginResult{Name: "customApps", Err: err})
		if err != nil && failFast {
			return results
		}
	}

	if cfg.Plugins.ArgoCD != nil && cfg.Plugins.ArgoCD.Enabled {
		pluginLog := newLogger("[argocd]")
		argoPlugin := argocd.New(pluginLog, globalTimeout)

		namespace := cfg.Plugins.ArgoCD.Namespace
		if namespace == "" {
			namespace = "argocd"
		}

		installed, checkErr := argoPlugin.IsInstalled(kubecontext, namespace)
		if checkErr != nil {
			results = append(results, pluginResult{Name: "argocd", Err: fmt.Errorf("failed to check status: %w", checkErr)})
		} else if !installed {
			pluginLog.Info("ArgoCD not installed, performing full installation...\n")
			err := argoPlugin.Install(cfg.Plugins.ArgoCD, kubecontext)
			results = append(results, pluginResult{Name: "argocd", Err: err})
		} else {
			err := argoPlugin.Upgrade(cfg.Plugins.ArgoCD, kubecontext)
			results = append(results, pluginResult{Name: "argocd", Err: err})
		}
		if hasErrors(results) && failFast {
			return results
		}
	} else if cfg.Plugins.ArgoCD != nil && !cfg.Plugins.ArgoCD.Enabled {
		pluginLog := newLogger("[argocd]")
		argoPlugin := argocd.New(pluginLog, globalTimeout)
		namespace := cfg.Plugins.ArgoCD.Namespace
		if namespace == "" {
			namespace = "argocd"
		}
		installed, err := argoPlugin.IsInstalled(kubecontext, namespace)
		if err == nil && installed {
			log.Warn("[argocd] WARNING: ArgoCD is installed but disabled in template. It will NOT be automatically uninstalled.\n")
			log.Warn("[argocd] To uninstall manually: kubectl delete namespace %s --context %s\n", namespace, kubecontext)
		}
	}

	return results
}

func printSummary(results []pluginResult, log interface{ Info(string, ...any) }) {
	fmt.Println()
	fmt.Println(strings.Repeat("-", 40))
	fmt.Println("Plugin Installation Summary:")
	fmt.Println(strings.Repeat("-", 40))
	for _, r := range results {
		if r.Err == nil {
			fmt.Printf("  ✓ %s\n", r.Name)
		} else {
			fmt.Printf("  ✗ %s: %v\n", r.Name, r.Err)
		}
	}
	fmt.Println(strings.Repeat("-", 40))
}

func hasErrors(results []pluginResult) bool {
	for _, r := range results {
		if r.Err != nil {
			return true
		}
	}
	return false
}
