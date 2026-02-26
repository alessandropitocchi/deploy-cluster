// Package plugin provides a unified interface for cluster plugins.
package plugin

import (
	"fmt"

	"github.com/alessandropitocchi/deploy-cluster/pkg/logger"
	"github.com/alessandropitocchi/deploy-cluster/pkg/template"
)

// Plugin is the unified interface that all plugins must implement.
type Plugin interface {
	// Name returns the plugin identifier (e.g., "storage", "ingress")
	Name() string

	// Install installs the plugin on the cluster.
	// cfg is the plugin-specific configuration from the template.
	Install(cfg interface{}, kubecontext string, providerType string) error

	// IsInstalled checks if the plugin is already installed.
	IsInstalled(kubecontext string) (bool, error)

	// Upgrade updates the plugin to match the desired configuration.
	// For most plugins, this is a re-install (idempotent).
	Upgrade(cfg interface{}, kubecontext string, providerType string) error

	// Uninstall removes the plugin from the cluster.
	Uninstall(cfg interface{}, kubecontext string) error

	// DryRun shows what would change without applying anything.
	DryRun(cfg interface{}, kubecontext string, providerType string) error
}

// BasePlugin contains common fields for all plugins.
type BasePlugin struct {
	Log     *logger.Logger
	Timeout int // seconds
}

// InstallResult tracks the outcome of a plugin installation.
type InstallResult struct {
	Name    string
	Skipped bool
	Err     error
}

// Registry manages all available plugins.
type Registry struct {
	plugins map[string]Plugin
}

// NewRegistry creates a new plugin registry.
func NewRegistry() *Registry {
	return &Registry{
		plugins: make(map[string]Plugin),
	}
}

// Register adds a plugin to the registry.
func (r *Registry) Register(p Plugin) {
	r.plugins[p.Name()] = p
}

// Get retrieves a plugin by name.
func (r *Registry) Get(name string) (Plugin, bool) {
	p, ok := r.plugins[name]
	return p, ok
}

// All returns all registered plugins.
func (r *Registry) All() map[string]Plugin {
	return r.plugins
}

// InstallOrder defines the default installation order for plugins.
// Plugins not in this list are installed after these in alphabetical order.
var InstallOrder = []string{
	"storage",
	"ingress",
	"cert-manager",
	"external-dns",
	"istio",
	"monitoring",
	"dashboard",
	"custom-apps",
	"argocd",
}

// GetInstallOrder returns the installation priority for a plugin name.
// Lower number = higher priority (install first).
func GetInstallOrder(name string) int {
	for i, n := range InstallOrder {
		if n == name {
			return i
		}
	}
	// Not in list: return high number (install last)
	return len(InstallOrder) + 100
}

// PluginConfigExtractor extracts plugin config from the template.
type PluginConfigExtractor func(t *template.Template) (name string, cfg interface{}, enabled bool)

// Extractors maps plugin names to their config extractors.
var Extractors = []PluginConfigExtractor{
	func(t *template.Template) (string, interface{}, bool) {
		if t.Plugins.Storage != nil {
			return "storage", t.Plugins.Storage, t.Plugins.Storage.Enabled
		}
		return "storage", nil, false
	},
	func(t *template.Template) (string, interface{}, bool) {
		if t.Plugins.Ingress != nil {
			return "ingress", t.Plugins.Ingress, t.Plugins.Ingress.Enabled
		}
		return "ingress", nil, false
	},
	func(t *template.Template) (string, interface{}, bool) {
		if t.Plugins.CertManager != nil {
			return "cert-manager", t.Plugins.CertManager, t.Plugins.CertManager.Enabled
		}
		return "cert-manager", nil, false
	},
	func(t *template.Template) (string, interface{}, bool) {
		if t.Plugins.ExternalDNS != nil {
			return "external-dns", t.Plugins.ExternalDNS, t.Plugins.ExternalDNS.Enabled
		}
		return "external-dns", nil, false
	},
	func(t *template.Template) (string, interface{}, bool) {
		if t.Plugins.Istio != nil {
			return "istio", t.Plugins.Istio, t.Plugins.Istio.Enabled
		}
		return "istio", nil, false
	},
	func(t *template.Template) (string, interface{}, bool) {
		if t.Plugins.Monitoring != nil {
			return "monitoring", t.Plugins.Monitoring, t.Plugins.Monitoring.Enabled
		}
		return "monitoring", nil, false
	},
	func(t *template.Template) (string, interface{}, bool) {
		if t.Plugins.Dashboard != nil {
			return "dashboard", t.Plugins.Dashboard, t.Plugins.Dashboard.Enabled
		}
		return "dashboard", nil, false
	},
	func(t *template.Template) (string, interface{}, bool) {
		// Custom apps are always "enabled" if the list is non-empty
		if len(t.Plugins.CustomApps) > 0 {
			return "custom-apps", t.Plugins.CustomApps, true
		}
		return "custom-apps", nil, false
	},
	func(t *template.Template) (string, interface{}, bool) {
		if t.Plugins.ArgoCD != nil {
			return "argocd", t.Plugins.ArgoCD, t.Plugins.ArgoCD.Enabled
		}
		return "argocd", nil, false
	},
}

// GetEnabledPlugins returns all enabled plugins from a template.
func GetEnabledPlugins(t *template.Template) []struct {
	Name    string
	Config  interface{}
	Enabled bool
} {
	var result []struct {
		Name    string
		Config  interface{}
		Enabled bool
	}
	for _, extractor := range Extractors {
		name, cfg, enabled := extractor(t)
		result = append(result, struct {
			Name    string
			Config  interface{}
			Enabled bool
		}{name, cfg, enabled})
	}
	return result
}

// NotEnabledError is returned when a plugin is not enabled in the template.
type NotEnabledError struct {
	PluginName string
}

func (e *NotEnabledError) Error() string {
	return fmt.Sprintf("plugin %s is not enabled", e.PluginName)
}

// IsNotEnabled checks if an error is a NotEnabledError.
func IsNotEnabled(err error) bool {
	_, ok := err.(*NotEnabledError)
	return ok
}
