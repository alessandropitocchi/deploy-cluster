package template

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/alessandropitocchi/deploy-cluster/pkg/templating"
	"gopkg.in/yaml.v3"
)

var (
	validProviders       = []string{"kind", "k3d", "existing"}
	validStorageTypes    = []string{"local-path"}
	validIngressTypes    = []string{"nginx", "traefik"}
	validMonitoringTypes = []string{"prometheus"}
	validDashboardTypes  = []string{"headlamp"}
	validExternalDNSProviders = []string{"cloudflare", "route53", "google", "azure", "digitalocean"}
	validIstioProfiles   = []string{"default", "demo", "minimal", "empty", "preview", "ambient"}
)

type Template struct {
	Name     string           `yaml:"name"`
	Provider ProviderTemplate `yaml:"provider"`
	Cluster  ClusterTemplate  `yaml:"cluster"`
	Plugins  PluginsTemplate  `yaml:"plugins,omitempty"`
	Snapshot *SnapshotConfig  `yaml:"snapshot,omitempty"` // Optional S3 snapshot configuration
}

type ProviderTemplate struct {
	Type       string `yaml:"type"`                 // kind, k3d, existing
	Kubeconfig string `yaml:"kubeconfig,omitempty"` // Path to kubeconfig (for existing)
	Context    string `yaml:"context,omitempty"`    // Kubectl context (for existing)
}

type ClusterTemplate struct {
	ControlPlanes int    `yaml:"controlPlanes"`
	Workers       int    `yaml:"workers"`
	Version       string `yaml:"version,omitempty"` // Kubernetes version
}

// SnapshotConfig holds S3 snapshot configuration.
type SnapshotConfig struct {
	Enabled  bool   `yaml:"enabled"`            // Enable S3 snapshot storage
	Bucket   string `yaml:"bucket"`             // S3 bucket name
	Prefix   string `yaml:"prefix,omitempty"`   // Optional: key prefix
	Region   string `yaml:"region,omitempty"`   // Optional: AWS region (defaults to AWS_REGION env var)
	Endpoint string `yaml:"endpoint,omitempty"` // Optional: custom S3 endpoint for S3-compatible services
}

type PluginsTemplate struct {
	Storage     *StorageTemplate     `yaml:"storage,omitempty"`
	Ingress     *IngressTemplate     `yaml:"ingress,omitempty"`
	CertManager *CertManagerTemplate `yaml:"certManager,omitempty"`
	ExternalDNS *ExternalDNSTemplate `yaml:"externalDNS,omitempty"`
	Istio       *IstioTemplate       `yaml:"istio,omitempty"`
	Monitoring  *MonitoringTemplate  `yaml:"monitoring,omitempty"`
	Dashboard   *DashboardTemplate   `yaml:"dashboard,omitempty"`
	CustomApps  []CustomAppTemplate  `yaml:"customApps,omitempty"`
	ArgoCD      *ArgoCDTemplate      `yaml:"argocd,omitempty"`
}

type ArgoCDTemplate struct {
	Enabled   bool                   `yaml:"enabled"`
	Namespace string                 `yaml:"namespace,omitempty"` // ArgoCD installation namespace
	Version   string                 `yaml:"version,omitempty"`   // ArgoCD version
	Ingress   *ArgoCDIngressTemplate `yaml:"ingress,omitempty"`   // Ingress for ArgoCD UI
	Repos     []ArgoCDRepoTemplate   `yaml:"repos,omitempty"`     // Repositories to add
	Apps      []ArgoCDAppTemplate    `yaml:"apps,omitempty"`      // Applications to create
}

type ArgoCDIngressTemplate struct {
	Enabled bool   `yaml:"enabled"`
	Host    string `yaml:"host"`          // Hostname (e.g. argocd.localhost)
	TLS     bool   `yaml:"tls,omitempty"` // Enable TLS (requires cert-manager)
}

type ArgoCDAppTemplate struct {
	Name           string                 `yaml:"name"`                     // Application name
	Namespace      string                 `yaml:"namespace,omitempty"`      // Destination namespace
	Project        string                 `yaml:"project,omitempty"`        // ArgoCD project (default: default)
	RepoURL        string                 `yaml:"repoURL"`                  // Chart repo URL or Git repo URL
	Chart          string                 `yaml:"chart,omitempty"`          // Helm chart name (for helm repos)
	Path           string                 `yaml:"path,omitempty"`           // Path in Git repo (for git sources)
	TargetRevision string                 `yaml:"targetRevision,omitempty"` // Chart version or branch/tag
	Values         map[string]interface{} `yaml:"values,omitempty"`         // Inline Helm values
	ValuesFile     string                 `yaml:"valuesFile,omitempty"`     // Path to external values file
	AutoSync       *bool                  `yaml:"autoSync,omitempty"`       // Enable auto sync (default: true)
}

type ArgoCDRepoTemplate struct {
	Name       string `yaml:"name,omitempty"`       // Repository name (optional)
	URL        string `yaml:"url"`                  // Repository URL
	Type       string `yaml:"type,omitempty"`       // git or helm (default: git)
	Insecure   *bool  `yaml:"insecure,omitempty"`   // Skip TLS verification (auto-detected for non-HTTPS)
	Username   string `yaml:"username,omitempty"`   // For private repos (HTTPS)
	Password   string `yaml:"password,omitempty"`   // For private repos (HTTPS)
	SSHKeyEnv  string `yaml:"sshKeyEnv,omitempty"`  // Env var containing SSH private key
	SSHKeyFile string `yaml:"sshKeyFile,omitempty"` // Path to SSH private key file
}

type StorageTemplate struct {
	Enabled bool   `yaml:"enabled"`
	Type    string `yaml:"type"` // local-path, openebs
}

type IngressTemplate struct {
	Enabled bool   `yaml:"enabled"`
	Type    string `yaml:"type"` // nginx, traefik
}

type CertManagerTemplate struct {
	Enabled bool   `yaml:"enabled"`
	Version string `yaml:"version,omitempty"` // cert-manager version (default: v1.16.3)
}

type ExternalDNSTemplate struct {
	Enabled     bool   `yaml:"enabled"`
	Version     string `yaml:"version,omitempty"`     // external-dns version (default: 1.15.0)
	Provider    string `yaml:"provider"`              // DNS provider: cloudflare, route53, google, azure
	Zone        string `yaml:"zone,omitempty"`        // DNS zone (e.g., example.com)
	Credentials map[string]string `yaml:"credentials,omitempty"` // Provider-specific credentials
	Source      string `yaml:"source,omitempty"`      // Source: ingress (default), service, both
}

type IstioTemplate struct {
	Enabled         bool                   `yaml:"enabled"`
	Version         string                 `yaml:"version,omitempty"`         // Istio version (default: 1.24.0)
	Profile         string                 `yaml:"profile,omitempty"`         // Profile: default, demo, minimal, empty (default: default)
	Revision        string                 `yaml:"revision,omitempty"`        // Revision for canary upgrades
	IngressGateway  bool                   `yaml:"ingressGateway,omitempty"`  // Enable ingress gateway
	EgressGateway   bool                   `yaml:"egressGateway,omitempty"`   // Enable egress gateway
	Values          map[string]interface{} `yaml:"values,omitempty"`          // Additional Helm values
}

type MonitoringTemplate struct {
	Enabled bool                       `yaml:"enabled"`
	Type    string                     `yaml:"type"`              // prometheus
	Version string                     `yaml:"version,omitempty"` // chart version (default: 72.6.2)
	Ingress *MonitoringIngressTemplate `yaml:"ingress,omitempty"` // Ingress for Grafana UI
}

type MonitoringIngressTemplate struct {
	Enabled bool   `yaml:"enabled"`
	Host    string `yaml:"host"` // Hostname for Grafana (e.g. grafana.localhost)
}

type DashboardTemplate struct {
	Enabled bool                      `yaml:"enabled"`
	Type    string                    `yaml:"type"`              // headlamp
	Version string                    `yaml:"version,omitempty"` // chart version
	Ingress *DashboardIngressTemplate `yaml:"ingress,omitempty"` // Ingress for dashboard UI
}

type DashboardIngressTemplate struct {
	Enabled bool   `yaml:"enabled"`
	Host    string `yaml:"host"` // Hostname (e.g. headlamp.localhost)
}

type CustomAppTemplate struct {
	Name       string                    `yaml:"name"`                 // Release name
	ChartName  string                    `yaml:"chartName"`            // Helm chart name
	ChartRepo  string                    `yaml:"chartRepo"`            // Optional chart repository URL (for non-OCI charts)
	Version    string                    `yaml:"version,omitempty"`    // Chart version
	Namespace  string                    `yaml:"namespace,omitempty"`  // Target namespace (default: name)
	Values     map[string]interface{}    `yaml:"values,omitempty"`     // Inline Helm values
	ValuesFile string                    `yaml:"valuesFile,omitempty"` // Path to external values file
	Ingress    *CustomAppIngressTemplate `yaml:"ingress,omitempty"`    // Optional ingress
}

type CustomAppIngressTemplate struct {
	Enabled     bool   `yaml:"enabled"`
	Host        string `yaml:"host"`                  // Hostname
	ServiceName string `yaml:"serviceName,omitempty"` // Backend service name (default: release name)
	ServicePort int    `yaml:"servicePort,omitempty"` // Backend service port (default: 80)
}

// DefaultTemplate returns a starter template
func DefaultTemplate() *Template {
	return &Template{
		Name: "my-cluster",
		Provider: ProviderTemplate{
			Type: "kind",
		},
		Cluster: ClusterTemplate{
			ControlPlanes: 1,
			Workers:       2,
			Version:       "v1.31.0",
		},
		Plugins: PluginsTemplate{
			Storage: &StorageTemplate{
				Enabled: false,
				Type:    "local-path",
			},
			Ingress: &IngressTemplate{
				Enabled: false,
				Type:    "nginx",
			},
			CertManager: &CertManagerTemplate{
				Enabled: false,
				Version: "v1.16.3",
			},
			Monitoring: &MonitoringTemplate{
				Enabled: false,
				Type:    "prometheus",
			},
			ArgoCD: &ArgoCDTemplate{
				Enabled:   false,
				Namespace: "argocd",
				Version:   "stable",
				Repos: []ArgoCDRepoTemplate{
					{
						Name: "my-gitops-repo",
						URL:  "https://github.com/user/gitops-repo.git",
						Type: "git",
					},
				},
				Apps: []ArgoCDAppTemplate{
					{
						Name:           "nginx",
						Namespace:      "demo-app",
						RepoURL:        "https://charts.bitnami.com/bitnami",
						Chart:          "nginx",
						TargetRevision: "18.2.4",
						Values: map[string]interface{}{
							"replicaCount": 2,
							"service": map[string]interface{}{
								"type": "ClusterIP",
							},
						},
					},
				},
			},
		},
	}
}

// Validate checks the template for errors and returns all found issues.
func (t *Template) Validate() error {
	var errs []string

	if t.Name == "" {
		errs = append(errs, "name is required")
	}

	if t.Provider.Type == "" {
		errs = append(errs, "provider.type is required")
	} else {
		valid := false
		for _, p := range validProviders {
			if t.Provider.Type == p {
				valid = true
				break
			}
		}
		if !valid {
			errs = append(errs, fmt.Sprintf("provider.type %q is not supported (valid: %s)", t.Provider.Type, strings.Join(validProviders, ", ")))
		}
	}

	// Cluster config is optional for existing clusters
	if t.Provider.Type != "existing" {
		if t.Cluster.ControlPlanes < 1 {
			errs = append(errs, "cluster.controlPlanes must be at least 1")
		}

		if t.Cluster.Workers < 0 {
			errs = append(errs, "cluster.workers cannot be negative")
		}
	}

	if stor := t.Plugins.Storage; stor != nil && stor.Enabled {
		if stor.Type == "" {
			errs = append(errs, "plugins.storage.type is required")
		} else {
			valid := false
			for _, tp := range validStorageTypes {
				if stor.Type == tp {
					valid = true
					break
				}
			}
			if !valid {
				errs = append(errs, fmt.Sprintf("plugins.storage.type %q is not supported (valid: %s)", stor.Type, strings.Join(validStorageTypes, ", ")))
			}
		}
	}

	if ing := t.Plugins.Ingress; ing != nil && ing.Enabled {
		if ing.Type == "" {
			errs = append(errs, "plugins.ingress.type is required")
		} else {
			valid := false
			for _, tp := range validIngressTypes {
				if ing.Type == tp {
					valid = true
					break
				}
			}
			if !valid {
				errs = append(errs, fmt.Sprintf("plugins.ingress.type %q is not supported (valid: %s)", ing.Type, strings.Join(validIngressTypes, ", ")))
			}
		}
	}

	if extdns := t.Plugins.ExternalDNS; extdns != nil && extdns.Enabled {
		if extdns.Provider == "" {
			errs = append(errs, "plugins.externalDNS.provider is required")
		} else {
			valid := false
			for _, p := range validExternalDNSProviders {
				if extdns.Provider == p {
					valid = true
					break
				}
			}
			if !valid {
				errs = append(errs, fmt.Sprintf("plugins.externalDNS.provider %q is not supported (valid: %s)", extdns.Provider, strings.Join(validExternalDNSProviders, ", ")))
			}
		}
		// Validate source if provided
		if extdns.Source != "" {
			validSources := []string{"ingress", "service", "both"}
			valid := false
			for _, s := range validSources {
				if extdns.Source == s {
					valid = true
					break
				}
			}
			if !valid {
				errs = append(errs, fmt.Sprintf("plugins.externalDNS.source %q is not supported (valid: ingress, service, both)", extdns.Source))
			}
		}
	}

	if istio := t.Plugins.Istio; istio != nil && istio.Enabled {
		if istio.Profile != "" {
			valid := false
			for _, p := range validIstioProfiles {
				if istio.Profile == p {
					valid = true
					break
				}
			}
			if !valid {
				errs = append(errs, fmt.Sprintf("plugins.istio.profile %q is not supported (valid: %s)", istio.Profile, strings.Join(validIstioProfiles, ", ")))
			}
		}
	}

	if mon := t.Plugins.Monitoring; mon != nil && mon.Enabled {
		if mon.Type == "" {
			errs = append(errs, "plugins.monitoring.type is required")
		} else {
			valid := false
			for _, tp := range validMonitoringTypes {
				if mon.Type == tp {
					valid = true
					break
				}
			}
			if !valid {
				errs = append(errs, fmt.Sprintf("plugins.monitoring.type %q is not supported (valid: %s)", mon.Type, strings.Join(validMonitoringTypes, ", ")))
			}
		}
		if ing := mon.Ingress; ing != nil && ing.Enabled {
			if ing.Host == "" {
				errs = append(errs, "plugins.monitoring.ingress.host is required when ingress is enabled")
			}
		}
	}

	if dash := t.Plugins.Dashboard; dash != nil && dash.Enabled {
		if dash.Type == "" {
			errs = append(errs, "plugins.dashboard.type is required")
		} else {
			valid := false
			for _, tp := range validDashboardTypes {
				if dash.Type == tp {
					valid = true
					break
				}
			}
			if !valid {
				errs = append(errs, fmt.Sprintf("plugins.dashboard.type %q is not supported (valid: %s)", dash.Type, strings.Join(validDashboardTypes, ", ")))
			}
		}
		if ing := dash.Ingress; ing != nil && ing.Enabled {
			if ing.Host == "" {
				errs = append(errs, "plugins.dashboard.ingress.host is required when ingress is enabled")
			}
		}
	}

	for i, app := range t.Plugins.CustomApps {
		if app.Name == "" {
			errs = append(errs, fmt.Sprintf("plugins.customApps[%d]: name is required", i))
		}
		if app.ChartName == "" {
			errs = append(errs, fmt.Sprintf("plugins.customApps[%d]: chart is required", i))
		}
		if ing := app.Ingress; ing != nil && ing.Enabled {
			if ing.Host == "" {
				errs = append(errs, fmt.Sprintf("plugins.customApps[%d].ingress: host is required when ingress is enabled", i))
			}
		}
	}

	if argo := t.Plugins.ArgoCD; argo != nil && argo.Enabled {
		if ing := argo.Ingress; ing != nil && ing.Enabled {
			if ing.Host == "" {
				errs = append(errs, "plugins.argocd.ingress.host is required when ingress is enabled")
			}
		}
		for i, repo := range argo.Repos {
			if repo.URL == "" {
				errs = append(errs, fmt.Sprintf("plugins.argocd.repos[%d]: url is required", i))
			}
		}
		for i, app := range argo.Apps {
			if app.Name == "" {
				errs = append(errs, fmt.Sprintf("plugins.argocd.apps[%d]: name is required", i))
			}
			if app.RepoURL == "" {
				errs = append(errs, fmt.Sprintf("plugins.argocd.apps[%d]: repoURL is required", i))
			}
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("template validation failed:\n  - %s", strings.Join(errs, "\n  - "))
	}
	return nil
}

// Save writes the template to a YAML file
func (t *Template) Save(path string) error {
	data, err := yaml.Marshal(t)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// Load reads a template from a YAML file
func Load(path string) (*Template, error) {
	return LoadWithEnv(path, nil)
}

// LoadWithEnv reads a template from a YAML file with optional env files for templating.
func LoadWithEnv(path string, envFiles []string) (*Template, error) {
	// Check if file contains template expressions
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	// Process templating if needed
	content := string(data)
	if containsGoTemplate(content) {
		processed, err := templating.ProcessTemplateFile(path, envFiles)
		if err != nil {
			return nil, fmt.Errorf("template processing failed: %w", err)
		}
		content = processed
	}

	var tmpl Template
	if err := yaml.Unmarshal([]byte(content), &tmpl); err != nil {
		return nil, err
	}
	if err := tmpl.Validate(); err != nil {
		return nil, err
	}
	return &tmpl, nil
}

// containsGoTemplate checks if content contains Go template expressions.
func containsGoTemplate(content string) bool {
	re := regexp.MustCompile(`\{\{.*\}\}`)
	return re.MatchString(content)
}
