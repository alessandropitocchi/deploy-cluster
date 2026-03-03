package kind

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/alessandropitocchi/deploy-cluster/pkg/template"
	"gopkg.in/yaml.v3"
)

// KindConfig represents kind cluster configuration
type KindConfig struct {
	Kind       string     `yaml:"kind"`
	APIVersion string     `yaml:"apiVersion"`
	Name       string     `yaml:"name,omitempty"`
	Nodes      []KindNode `yaml:"nodes"`
}

type KindNode struct {
	Role                 string              `yaml:"role"`
	Image                string              `yaml:"image,omitempty"`
	KubeadmConfigPatches []string            `yaml:"kubeadmConfigPatches,omitempty"`
	ExtraPortMappings    []KindPortMapping   `yaml:"extraPortMappings,omitempty"`
	Labels               map[string]string   `yaml:"labels,omitempty"`
}

type KindPortMapping struct {
	ContainerPort int    `yaml:"containerPort"`
	HostPort      int    `yaml:"hostPort"`
	Protocol      string `yaml:"protocol,omitempty"`
	ListenAddress string `yaml:"listenAddress,omitempty"`
}

// Provider implements the Provider interface for kind
type Provider struct {
	Verbose bool
}

func New() *Provider {
	return &Provider{Verbose: true}
}

func (p *Provider) Name() string {
	return "kind"
}

func (p *Provider) log(format string, args ...interface{}) {
	if p.Verbose {
		fmt.Printf(format, args...)
	}
}

func (p *Provider) Create(cfg *template.Template) error {
	// Check if kind is installed
	p.log("[kind] Checking if kind is installed...\n")
	if _, err := exec.LookPath("kind"); err != nil {
		return fmt.Errorf("kind not found in PATH: %w", err)
	}
	p.log("[kind] ✓ kind found\n")

	// Check if cluster already exists
	p.log("[kind] Checking if cluster '%s' already exists...\n", cfg.Name)
	exists, err := p.Exists(cfg.Name)
	if err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("cluster %s already exists", cfg.Name)
	}
	p.log("[kind] ✓ Cluster name is available\n")

	// Generate kind config
	p.log("[kind] Generating kind configuration...\n")
	kindCfg := p.generateKindConfig(cfg)
	kindYAML, err := yaml.Marshal(kindCfg)
	if err != nil {
		return fmt.Errorf("failed to generate kind config: %w", err)
	}

	p.log("[kind] Generated configuration:\n")
	p.log("---\n%s---\n", string(kindYAML))

	// Create cluster with streaming output
	p.log("[kind] Creating cluster (this may take a few minutes)...\n\n")

	cmd := exec.Command("kind", "create", "cluster", "--name", cfg.Name, "--config", "-")
	cmd.Stdin = bytes.NewReader(kindYAML)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create cluster: %w", err)
	}

	p.log("\n[kind] ✓ Cluster created successfully\n")
	return nil
}

func (p *Provider) Delete(name string) error {
	p.log("[kind] Deleting cluster '%s'...\n\n", name)

	cmd := exec.Command("kind", "delete", "cluster", "--name", name)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to delete cluster: %w", err)
	}

	p.log("\n[kind] ✓ Cluster deleted successfully\n")
	return nil
}

func (p *Provider) GetKubeconfig(name string) (string, error) {
	cmd := exec.Command("kind", "get", "kubeconfig", "--name", name)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to get kubeconfig: %w", err)
	}
	return string(output), nil
}

func (p *Provider) Exists(name string) (bool, error) {
	cmd := exec.Command("kind", "get", "clusters")
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("failed to list clusters: %w", err)
	}

	clusters := strings.Split(strings.TrimSpace(string(output)), "\n")
	for _, c := range clusters {
		if c == name {
			return true, nil
		}
	}
	return false, nil
}

func (p *Provider) KubeContext(name string) string {
	return fmt.Sprintf("kind-%s", name)
}

func (p *Provider) generateKindConfig(cfg *template.Template) *KindConfig {
	kindCfg := &KindConfig{
		Kind:       "Cluster",
		APIVersion: "kind.x-k8s.io/v1alpha4",
		Nodes:      []KindNode{},
	}

	// Determine node image based on version
	var image string
	if cfg.Cluster.Version != "" {
		image = fmt.Sprintf("kindest/node:%s", cfg.Cluster.Version)
	}

	// Check if ingress is enabled
	ingressEnabled := cfg.Plugins.Ingress != nil && cfg.Plugins.Ingress.Enabled
	ingressType := ""
	if ingressEnabled {
		ingressType = cfg.Plugins.Ingress.Type
	}

	// Add control plane nodes
	for i := 0; i < cfg.Cluster.ControlPlanes; i++ {
		node := KindNode{Role: "control-plane"}
		if image != "" {
			node.Image = image
		}
		// First control-plane gets ingress config (label + port mappings)
		if i == 0 && ingressEnabled {
			// Port mappings are needed for all ingress types
			node.ExtraPortMappings = []KindPortMapping{
				{ContainerPort: 80, HostPort: 80, Protocol: "TCP"},
				{ContainerPort: 443, HostPort: 443, Protocol: "TCP"},
			}
			// Legacy label only needed for old nginx ingress controller
			// New controllers (traefik, nginx-gateway-fabric) don't need this
			if ingressType == "" {
				// Default behavior for backward compatibility - no label needed
				_ = ingressType
			}
		}
		// Add custom port mappings to first control-plane
		if i == 0 && len(cfg.Cluster.ExtraPortMappings) > 0 {
			for _, pm := range cfg.Cluster.ExtraPortMappings {
				protocol := pm.Protocol
				if protocol == "" {
					protocol = "TCP"
				}
				kindPM := KindPortMapping{
					ContainerPort: pm.ContainerPort,
					HostPort:      pm.HostPort,
					Protocol:      protocol,
				}
				if pm.ListenAddress != "" {
					kindPM.ListenAddress = pm.ListenAddress
				}
				node.ExtraPortMappings = append(node.ExtraPortMappings, kindPM)
			}
		}
		kindCfg.Nodes = append(kindCfg.Nodes, node)
	}

	// Add worker nodes
	for i := 0; i < cfg.Cluster.Workers; i++ {
		node := KindNode{Role: "worker"}
		if image != "" {
			node.Image = image
		}
		kindCfg.Nodes = append(kindCfg.Nodes, node)
	}

	return kindCfg
}
