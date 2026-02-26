package certmanager

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

const defaultVersion = "v1.16.3"

// Plugin implements the plugin.Plugin interface for cert-manager.
type Plugin struct {
	Log     *logger.Logger
	Timeout time.Duration
}

// New creates a new cert-manager plugin.
func New(log *logger.Logger, timeout time.Duration) *Plugin {
	return &Plugin{Log: log, Timeout: timeout}
}

// Name returns the plugin name.
func (p *Plugin) Name() string {
	return "cert-manager"
}

// Install installs the cert-manager plugin.
func (p *Plugin) Install(cfg interface{}, kubecontext string, providerType string) error {
	certCfg, ok := cfg.(*template.CertManagerTemplate)
	if !ok {
		return fmt.Errorf("invalid config type for cert-manager plugin: expected *template.CertManagerTemplate")
	}

	version := certCfg.Version
	if version == "" {
		version = defaultVersion
	}

	p.Log.Info("Installing cert-manager %s...\n", version)

	url := p.manifestURL(version)
	err := retry.Run(3, 5*time.Second, p.Log.Warn, func() error {
		cmd := execCommand("kubectl", "--context", kubecontext, "apply", "-f", url)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	})
	if err != nil {
		return fmt.Errorf("failed to apply cert-manager manifest: %w", err)
	}

	// Wait for webhook to be ready (critical for cert-manager to work)
	p.Log.Info("Waiting for cert-manager-webhook to be ready...\n")
	waitCmd := execCommand("kubectl", "--context", kubecontext,
		"rollout", "status", "deployment/cert-manager-webhook",
		"-n", "cert-manager", "--timeout", p.Timeout.String())
	waitCmd.Stdout = os.Stdout
	waitCmd.Stderr = os.Stderr
	if err := waitCmd.Run(); err != nil {
		return fmt.Errorf("cert-manager-webhook not ready: %w", err)
	}

	// Also wait for the main controller
	p.Log.Info("Waiting for cert-manager controller to be ready...\n")
	waitCtrl := execCommand("kubectl", "--context", kubecontext,
		"rollout", "status", "deployment/cert-manager",
		"-n", "cert-manager", "--timeout", p.Timeout.String())
	waitCtrl.Stdout = os.Stdout
	waitCtrl.Stderr = os.Stderr
	if err := waitCtrl.Run(); err != nil {
		return fmt.Errorf("cert-manager controller not ready: %w", err)
	}

	p.Log.Success("cert-manager %s installed successfully\n", version)
	p.Log.Info("You can now create Issuer/ClusterIssuer and Certificate resources\n")
	return nil
}

// Uninstall removes the cert-manager plugin.
func (p *Plugin) Uninstall(cfg interface{}, kubecontext string) error {
	certCfg, ok := cfg.(*template.CertManagerTemplate)
	if !ok {
		return fmt.Errorf("invalid config type for cert-manager plugin")
	}

	version := certCfg.Version
	if version == "" {
		version = defaultVersion
	}

	p.Log.Info("Uninstalling cert-manager...\n")

	url := p.manifestURL(version)
	cmd := execCommand("kubectl", "--context", kubecontext, "delete", "-f", url)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to delete cert-manager: %w", err)
	}

	p.Log.Success("cert-manager uninstalled\n")
	return nil
}

// IsInstalled checks if cert-manager is installed.
func (p *Plugin) IsInstalled(kubecontext string) (bool, error) {
	cmd := execCommand("kubectl", "--context", kubecontext,
		"get", "deployment", "cert-manager", "-n", "cert-manager")
	if err := cmd.Run(); err != nil {
		return false, nil
	}
	return true, nil
}

// Upgrade re-applies the cert-manager configuration (idempotent).
func (p *Plugin) Upgrade(cfg interface{}, kubecontext string, providerType string) error {
	// For cert-manager, upgrade is the same as install (idempotent)
	return p.Install(cfg, kubecontext, providerType)
}

// DryRun shows what would be installed.
func (p *Plugin) DryRun(cfg interface{}, kubecontext string, providerType string) error {
	certCfg, ok := cfg.(*template.CertManagerTemplate)
	if !ok {
		return fmt.Errorf("invalid config type for cert-manager plugin")
	}

	version := certCfg.Version
	if version == "" {
		version = defaultVersion
	}

	fmt.Printf("[cert-manager] Would install version: %s\n", version)

	installed, err := p.IsInstalled(kubecontext)
	if err != nil {
		return err
	}
	if installed {
		fmt.Println("  Status: already installed (would re-apply)")
	} else {
		fmt.Println("  Status: not installed")
	}
	fmt.Printf("  Manifest: %s\n", p.manifestURL(version))
	return nil
}

func (p *Plugin) manifestURL(version string) string {
	return fmt.Sprintf("https://github.com/cert-manager/cert-manager/releases/download/%s/cert-manager.yaml", version)
}
