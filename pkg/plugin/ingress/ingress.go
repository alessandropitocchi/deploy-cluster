package ingress

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/alessandropitocchi/deploy-cluster/pkg/logger"
	"github.com/alessandropitocchi/deploy-cluster/pkg/retry"
	"github.com/alessandropitocchi/deploy-cluster/pkg/template"
)

// execCommand is a package-level variable for creating exec.Cmd, replaceable in tests.
var execCommand = exec.Command

const (
	nginxManifestKindURL  = "https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.12.0/deploy/static/provider/kind/deploy.yaml"
	nginxManifestCloudURL = "https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.12.0/deploy/static/provider/cloud/deploy.yaml"
)

// Plugin implements the plugin.Plugin interface for ingress.
type Plugin struct {
	Log     *logger.Logger
	Timeout time.Duration
}

// New creates a new ingress plugin.
func New(log *logger.Logger, timeout time.Duration) *Plugin {
	return &Plugin{Log: log, Timeout: timeout}
}

// Name returns the plugin name.
func (p *Plugin) Name() string {
	return "ingress"
}

// Install installs the ingress plugin.
func (p *Plugin) Install(cfg interface{}, kubecontext string, providerType string) error {
	ingressCfg, ok := cfg.(*template.IngressTemplate)
	if !ok {
		return fmt.Errorf("invalid config type for ingress plugin: expected *template.IngressTemplate")
	}

	switch ingressCfg.Type {
	case "nginx":
		return p.installNginx(kubecontext, providerType)
	case "traefik":
		return p.installTraefik(kubecontext)
	default:
		return fmt.Errorf("unsupported ingress type: %s (supported: nginx, traefik)", ingressCfg.Type)
	}
}

// Uninstall removes the ingress plugin.
func (p *Plugin) Uninstall(cfg interface{}, kubecontext string) error {
	ingressCfg, ok := cfg.(*template.IngressTemplate)
	if !ok {
		return fmt.Errorf("invalid config type for ingress plugin")
	}

	switch ingressCfg.Type {
	case "nginx":
		return p.uninstallNginx(kubecontext)
	case "traefik":
		return p.uninstallTraefik(kubecontext)
	default:
		return fmt.Errorf("unsupported ingress type: %s", ingressCfg.Type)
	}
}

// IsInstalled checks if the ingress plugin is installed.
func (p *Plugin) IsInstalled(kubecontext string) (bool, error) {
	// Check nginx
	cmd := execCommand("kubectl", "--context", kubecontext,
		"get", "deployment", "ingress-nginx-controller", "-n", "ingress-nginx")
	if err := cmd.Run(); err == nil {
		return true, nil
	}

	// Check traefik (in kube-system, as deployed by k3d)
	cmd = execCommand("kubectl", "--context", kubecontext,
		"get", "deployment", "traefik", "-n", "kube-system")
	if err := cmd.Run(); err == nil {
		return true, nil
	}

	return false, nil
}

// Upgrade re-applies the ingress configuration (idempotent).
func (p *Plugin) Upgrade(cfg interface{}, kubecontext string, providerType string) error {
	// For ingress, upgrade is the same as install (idempotent)
	return p.Install(cfg, kubecontext, providerType)
}

// DryRun shows what would be installed.
func (p *Plugin) DryRun(cfg interface{}, kubecontext string, providerType string) error {
	ingressCfg, ok := cfg.(*template.IngressTemplate)
	if !ok {
		return fmt.Errorf("invalid config type for ingress plugin")
	}

	fmt.Printf("[ingress] Would install: %s\n", ingressCfg.Type)

	installed, err := p.IsInstalled(kubecontext)
	if err != nil {
		return err
	}
	if installed {
		fmt.Println("  Status: already installed (would skip)")
	} else {
		fmt.Println("  Status: not installed")
		if ingressCfg.Type == "nginx" {
			url := p.nginxManifestURL(providerType)
			fmt.Printf("  Manifest: %s\n", url)
		}
	}
	return nil
}

func (p *Plugin) nginxManifestURL(providerType string) string {
	if providerType == "kind" {
		return nginxManifestKindURL
	}
	return nginxManifestCloudURL
}

func (p *Plugin) installNginx(kubecontext string, providerType string) error {
	manifestURL := p.nginxManifestURL(providerType)
	p.Log.Info("Installing nginx ingress controller...\n")

	err := retry.Run(3, 5*time.Second, p.Log.Warn, func() error {
		cmd := execCommand("kubectl", "--context", kubecontext,
			"apply", "-f", manifestURL)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	})
	if err != nil {
		return fmt.Errorf("failed to apply nginx ingress manifest: %w", err)
	}

	p.Log.Info("Waiting for nginx ingress controller to be ready...\n")
	waitCmd := execCommand("kubectl", "--context", kubecontext,
		"rollout", "status", "deployment/ingress-nginx-controller",
		"-n", "ingress-nginx", "--timeout", p.Timeout.String())
	waitCmd.Stdout = os.Stdout
	waitCmd.Stderr = os.Stderr
	if err := waitCmd.Run(); err != nil {
		return fmt.Errorf("nginx ingress controller not ready: %w", err)
	}

	p.Log.Success("nginx ingress controller installed successfully\n")
	p.Log.Info("Ingress class: nginx\n")
	return nil
}

func (p *Plugin) uninstallNginx(kubecontext string) error {
	p.Log.Info("Uninstalling nginx ingress controller...\n")

	// Try kind URL first, then cloud URL for cleanup
	cmd := execCommand("kubectl", "--context", kubecontext,
		"delete", "-f", nginxManifestKindURL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		// Try cloud URL
		cmd = execCommand("kubectl", "--context", kubecontext,
			"delete", "-f", nginxManifestCloudURL)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to delete nginx ingress: %w", err)
		}
	}

	p.Log.Success("nginx ingress controller uninstalled\n")
	return nil
}

func (p *Plugin) installTraefik(kubecontext string) error {
	// On k3d, Traefik is already installed by the provider.
	// Just verify it's ready.
	p.Log.Info("Verifying Traefik ingress controller readiness...\n")

	waitCmd := execCommand("kubectl", "--context", kubecontext,
		"rollout", "status", "deployment/traefik",
		"-n", "kube-system", "--timeout", p.Timeout.String())
	waitCmd.Stdout = os.Stdout
	waitCmd.Stderr = os.Stderr
	if err := waitCmd.Run(); err != nil {
		return fmt.Errorf("traefik ingress controller not ready: %w", err)
	}

	p.Log.Success("Traefik ingress controller is ready\n")
	p.Log.Info("Ingress class: traefik\n")
	return nil
}

func (p *Plugin) uninstallTraefik(kubecontext string) error {
	p.Log.Info("Uninstalling Traefik ingress controller...\n")

	cmd := execCommand("kubectl", "--context", kubecontext,
		"delete", "deployment", "traefik", "-n", "kube-system")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to delete traefik ingress: %w", err)
	}

	p.Log.Success("Traefik ingress controller uninstalled\n")
	return nil
}
