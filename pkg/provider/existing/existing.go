// Package existing provides a provider for connecting to existing Kubernetes clusters.
package existing

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/alepito/deploy-cluster/pkg/template"
)

// Provider connects to an existing Kubernetes cluster.
type Provider struct {
	kubeconfigPath string
	context        string
}

// New creates a new existing cluster provider.
func New(kubeconfigPath, context string) *Provider {
	return &Provider{
		kubeconfigPath: kubeconfigPath,
		context:        context,
	}
}

// Name returns the provider name.
func (p *Provider) Name() string {
	return "existing"
}

// Create validates the connection to the existing cluster.
// For existing clusters, this just verifies connectivity rather than creating anything.
func (p *Provider) Create(cfg *template.Template) error {
	// Verify kubeconfig exists
	if p.kubeconfigPath == "" {
		p.kubeconfigPath = os.Getenv("KUBECONFIG")
		if p.kubeconfigPath == "" {
			p.kubeconfigPath = os.ExpandEnv("$HOME/.kube/config")
		}
	}

	if _, err := os.Stat(p.kubeconfigPath); err != nil {
		return fmt.Errorf("kubeconfig not found at %s: %w", p.kubeconfigPath, err)
	}

	// Verify cluster connectivity
	if err := p.verifyConnectivity(); err != nil {
		return fmt.Errorf("cannot connect to existing cluster: %w", err)
	}

	return nil
}

// Delete is a no-op for existing clusters.
// We don't delete clusters we didn't create.
func (p *Provider) Delete(name string) error {
	return fmt.Errorf("cannot delete existing cluster '%s': use 'destroy' only for clusters created by deploy-cluster", name)
}

// GetKubeconfig returns the path to the kubeconfig file.
func (p *Provider) GetKubeconfig(name string) (string, error) {
	return p.kubeconfigPath, nil
}

// Exists checks if the cluster is accessible.
func (p *Provider) Exists(name string) (bool, error) {
	if err := p.verifyConnectivity(); err != nil {
		return false, nil
	}
	return true, nil
}

// KubeContext returns the Kubernetes context to use.
func (p *Provider) KubeContext(name string) string {
	if p.context != "" {
		return p.context
	}
	// Try to get current context from kubectl
	ctx, _ := p.getCurrentContext()
	if ctx != "" {
		return ctx
	}
	return name
}

// verifyConnectivity checks if we can connect to the cluster.
func (p *Provider) verifyConnectivity() error {
	args := []string{"cluster-info"}
	
	if p.kubeconfigPath != "" {
		args = append(args, "--kubeconfig", p.kubeconfigPath)
	}
	if p.context != "" {
		args = append(args, "--context", p.context)
	}

	cmd := exec.Command("kubectl", args...)
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

// getCurrentContext returns the current kubectl context.
func (p *Provider) getCurrentContext() (string, error) {
	args := []string{"config", "current-context"}
	
	if p.kubeconfigPath != "" {
		args = append(args, "--kubeconfig", p.kubeconfigPath)
	}

	cmd := exec.Command("kubectl", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	
	return strings.TrimSpace(string(out)), nil
}

// GetKubeconfigPath returns the kubeconfig path.
func (p *Provider) GetKubeconfigPath() string {
	return p.kubeconfigPath
}

// GetContext returns the configured context.
func (p *Provider) GetContext() string {
	return p.context
}
