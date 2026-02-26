package k3d

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/alessandropitocchi/deploy-cluster/pkg/template"
	"gopkg.in/yaml.v3"
)

// K3dConfig represents k3d SimpleConfig (k3d.io/v1alpha5)
type K3dConfig struct {
	APIVersion string          `yaml:"apiVersion"`
	Kind       string          `yaml:"kind"`
	Metadata   K3dMetadata     `yaml:"metadata"`
	Servers    int             `yaml:"servers"`
	Agents     int             `yaml:"agents"`
	Image      string          `yaml:"image,omitempty"`
	Ports      []K3dPortConfig `yaml:"ports,omitempty"`
	Options    *K3dOptions     `yaml:"options,omitempty"`
}

type K3dMetadata struct {
	Name string `yaml:"name"`
}

type K3dPortConfig struct {
	Port        string   `yaml:"port"`
	NodeFilters []string `yaml:"nodeFilters"`
}

type K3dOptions struct {
	K3s *K3sOptions `yaml:"k3s,omitempty"`
}

type K3sOptions struct {
	ExtraArgs []K3sExtraArg `yaml:"extraArgs,omitempty"`
}

type K3sExtraArg struct {
	Arg         string   `yaml:"arg"`
	NodeFilters []string `yaml:"nodeFilters"`
}

// k3dClusterInfo is used for JSON parsing of `k3d cluster list -o json`
type k3dClusterInfo struct {
	Name string `json:"name"`
}

// Provider implements the Provider interface for k3d
type Provider struct {
	Verbose bool
}

func New() *Provider {
	return &Provider{Verbose: true}
}

func (p *Provider) Name() string {
	return "k3d"
}

func (p *Provider) log(format string, args ...interface{}) {
	if p.Verbose {
		fmt.Printf(format, args...)
	}
}

func (p *Provider) KubeContext(name string) string {
	return fmt.Sprintf("k3d-%s", name)
}

func (p *Provider) Create(cfg *template.Template) error {
	// Check if k3d is installed
	p.log("[k3d] Checking if k3d is installed...\n")
	if _, err := exec.LookPath("k3d"); err != nil {
		return fmt.Errorf("k3d not found in PATH: %w", err)
	}
	p.log("[k3d] k3d found\n")

	// Check if cluster already exists
	p.log("[k3d] Checking if cluster '%s' already exists...\n", cfg.Name)
	exists, err := p.Exists(cfg.Name)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("cluster %s already exists", cfg.Name)
	}
	p.log("[k3d] Cluster name is available\n")

	// Generate k3d config
	p.log("[k3d] Generating k3d configuration...\n")
	k3dCfg := p.generateK3dConfig(cfg)
	k3dYAML, err := yaml.Marshal(k3dCfg)
	if err != nil {
		return fmt.Errorf("failed to generate k3d config: %w", err)
	}

	p.log("[k3d] Generated configuration:\n")
	p.log("---\n%s---\n", string(k3dYAML))

	// Write config to temp file (k3d doesn't support stdin)
	tmpFile, err := os.CreateTemp("", "k3d-config-*.yaml")
	if err != nil {
		return fmt.Errorf("failed to create temp config file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(k3dYAML); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write temp config file: %w", err)
	}
	tmpFile.Close()

	// Create cluster
	p.log("[k3d] Creating cluster (this may take a few minutes)...\n\n")

	cmd := exec.Command("k3d", "cluster", "create", cfg.Name, "--config", tmpFile.Name())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create cluster: %w", err)
	}

	p.log("\n[k3d] Cluster created successfully\n")
	return nil
}

func (p *Provider) Delete(name string) error {
	p.log("[k3d] Deleting cluster '%s'...\n\n", name)

	cmd := exec.Command("k3d", "cluster", "delete", name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to delete cluster: %w", err)
	}

	p.log("\n[k3d] Cluster deleted successfully\n")
	return nil
}

func (p *Provider) GetKubeconfig(name string) (string, error) {
	cmd := exec.Command("k3d", "kubeconfig", "get", name)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get kubeconfig: %w", err)
	}
	return string(output), nil
}

func (p *Provider) Exists(name string) (bool, error) {
	cmd := exec.Command("k3d", "cluster", "list", "-o", "json")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to list clusters: %w", err)
	}

	outputStr := strings.TrimSpace(string(output))
	if outputStr == "" || outputStr == "null" {
		return false, nil
	}

	var clusters []k3dClusterInfo
	if err := json.Unmarshal(output, &clusters); err != nil {
		return false, fmt.Errorf("failed to parse cluster list: %w", err)
	}

	for _, c := range clusters {
		if c.Name == name {
			return true, nil
		}
	}
	return false, nil
}

func (p *Provider) generateK3dConfig(cfg *template.Template) *K3dConfig {
	k3dCfg := &K3dConfig{
		APIVersion: "k3d.io/v1alpha5",
		Kind:       "Simple",
		Metadata:   K3dMetadata{Name: cfg.Name},
		Servers:    cfg.Cluster.ControlPlanes,
		Agents:     cfg.Cluster.Workers,
	}

	// Set image based on version
	if cfg.Cluster.Version != "" {
		k3dCfg.Image = fmt.Sprintf("rancher/k3s:%s-k3s1", cfg.Cluster.Version)
	}

	// Check if ingress is enabled
	ingressEnabled := cfg.Plugins.Ingress != nil && cfg.Plugins.Ingress.Enabled

	if ingressEnabled {
		// Port mappings go on loadbalancer (not on nodes like kind)
		k3dCfg.Ports = []K3dPortConfig{
			{Port: "80:80", NodeFilters: []string{"loadbalancer"}},
			{Port: "443:443", NodeFilters: []string{"loadbalancer"}},
		}

		// Disable Traefik if user chose nginx (k3d ships Traefik by default)
		ingressType := cfg.Plugins.Ingress.Type
		if ingressType == "nginx" {
			k3dCfg.Options = &K3dOptions{
				K3s: &K3sOptions{
					ExtraArgs: []K3sExtraArg{
						{
							Arg:         "--disable=traefik",
							NodeFilters: []string{"server:*"},
						},
					},
				},
			}
		}
		// If type == "traefik", don't disable it — it's built-in
	}

	return k3dCfg
}
