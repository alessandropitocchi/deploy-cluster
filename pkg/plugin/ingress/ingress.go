package ingress

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/alepito/deploy-cluster/pkg/template"
	"github.com/alepito/deploy-cluster/pkg/logger"
	"github.com/alepito/deploy-cluster/pkg/retry"
)

// execCommand is a package-level variable for creating exec.Cmd, replaceable in tests.
var execCommand = exec.Command

const (
	nginxManifestKindURL  = "https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.12.0/deploy/static/provider/kind/deploy.yaml"
	nginxManifestCloudURL = "https://raw.githubusercontent.com/kubernetes/ingress-nginx/controller-v1.12.0/deploy/static/provider/cloud/deploy.yaml"
)

type Plugin struct {
	Log     *logger.Logger
	Timeout time.Duration
}

func New(log *logger.Logger, timeout time.Duration) *Plugin {
	return &Plugin{Log: log, Timeout: timeout}
}

func (p *Plugin) Name() string {
	return "ingress"
}

func (p *Plugin) Install(cfg *template.IngressTemplate, kubecontext string, providerType string) error {
	switch cfg.Type {
	case "nginx":
		return p.installNginx(kubecontext, providerType)
	case "traefik":
		return p.installTraefik(kubecontext)
	default:
		return fmt.Errorf("unsupported ingress type: %s (supported: nginx, traefik)", cfg.Type)
	}
}

func (p *Plugin) Uninstall(cfg *template.IngressTemplate, kubecontext string) error {
	switch cfg.Type {
	case "nginx":
		return p.uninstallNginx(kubecontext)
	case "traefik":
		return p.uninstallTraefik(kubecontext)
	default:
		return fmt.Errorf("unsupported ingress type: %s", cfg.Type)
	}
}

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
