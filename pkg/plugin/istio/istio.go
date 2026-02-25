// Package istio provides the Istio service mesh plugin.
package istio

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/alepito/deploy-cluster/pkg/logger"
	"github.com/alepito/deploy-cluster/pkg/retry"
	"github.com/alepito/deploy-cluster/pkg/template"
)

// execCommand is a package-level variable for creating exec.Cmd, replaceable in tests.
var execCommand = exec.Command

const (
	defaultVersion   = "1.24.0"
	defaultNamespace = "istio-system"
)

// Plugin implements the plugin.Plugin interface for Istio.
type Plugin struct {
	Log     *logger.Logger
	Timeout time.Duration
}

// New creates a new Istio plugin.
func New(log *logger.Logger, timeout time.Duration) *Plugin {
	return &Plugin{Log: log, Timeout: timeout}
}

// Name returns the plugin name.
func (p *Plugin) Name() string {
	return "istio"
}

// Install installs the Istio plugin.
func (p *Plugin) Install(cfg interface{}, kubecontext string, providerType string) error {
	istioCfg, ok := cfg.(*template.IstioTemplate)
	if !ok {
		return fmt.Errorf("invalid config type for istio plugin: expected *template.IstioTemplate")
	}

	version := istioCfg.Version
	if version == "" {
		version = defaultVersion
	}

	profile := istioCfg.Profile
	if profile == "" {
		profile = "default"
	}

	p.Log.Info("Installing Istio %s (profile: %s)...\n", version, profile)

	// Download istioctl if not present
	istioctlPath, err := p.ensureIstioctl(version)
	if err != nil {
		return fmt.Errorf("failed to setup istioctl: %w", err)
	}

	// Create namespace
	p.Log.Debug("Creating namespace '%s'...\n", defaultNamespace)
	if err := p.runKubectl(kubecontext, "create", "namespace", defaultNamespace, "--dry-run=client", "-o", "yaml", "|", "kubectl", "apply", "-f", "-"); err != nil {
		// Ignore if already exists
		p.Log.Debug("Namespace may already exist, continuing...\n")
	}

	// Install Istio using istioctl
	p.Log.Info("Installing Istio control plane...\n")
	installArgs := []string{
		"install",
		"--set", fmt.Sprintf("profile=%s", profile),
		"--set", fmt.Sprintf("values.global.istioNamespace=%s", defaultNamespace),
		"--kube-context", kubecontext,
		"--skip-confirmation",
	}

	// Add revision for canary upgrades if specified
	if istioCfg.Revision != "" {
		installArgs = append(installArgs, "--revision", istioCfg.Revision)
	}

	// Add custom values if specified
	for key, value := range istioCfg.Values {
		installArgs = append(installArgs, "--set", fmt.Sprintf("%s=%v", key, value))
	}

	err = retry.Run(3, 5*time.Second, p.Log.Warn, func() error {
		cmd := execCommand(istioctlPath, installArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	})
	if err != nil {
		return fmt.Errorf("failed to install Istio: %w", err)
	}

	// Wait for istiod to be ready
	p.Log.Info("Waiting for Istio control plane to be ready...\n")
	if err := p.waitForDeployment(kubecontext, defaultNamespace, "istiod", p.Timeout); err != nil {
		return fmt.Errorf("istiod not ready: %w", err)
	}

	// Install ingress gateway if enabled
	if istioCfg.IngressGateway {
		p.Log.Info("Installing Istio ingress gateway...\n")
		if err := p.installIngressGateway(kubecontext, istioctlPath, istioCfg); err != nil {
			return fmt.Errorf("failed to install ingress gateway: %w", err)
		}
	}

	// Install egress gateway if enabled
	if istioCfg.EgressGateway {
		p.Log.Info("Installing Istio egress gateway...\n")
		if err := p.installEgressGateway(kubecontext, istioctlPath, istioCfg); err != nil {
			return fmt.Errorf("failed to install egress gateway: %w", err)
		}
	}

	p.Log.Success("Istio %s installed successfully\n", version)
	p.Log.Info("Profile: %s\n", profile)
	if istioCfg.IngressGateway {
		p.Log.Info("Ingress Gateway: enabled\n")
	}
	if istioCfg.EgressGateway {
		p.Log.Info("Egress Gateway: enabled\n")
	}
	p.Log.Info("\nTo enable sidecar injection on a namespace:\n")
	p.Log.Info("  kubectl label namespace <namespace> istio-injection=enabled --context %s\n", kubecontext)

	return nil
}

// Uninstall removes the Istio plugin.
func (p *Plugin) Uninstall(cfg interface{}, kubecontext string) error {
	istioCfg, ok := cfg.(*template.IstioTemplate)
	if !ok {
		return fmt.Errorf("invalid config type for istio plugin")
	}

	version := istioCfg.Version
	if version == "" {
		version = defaultVersion
	}

	p.Log.Info("Uninstalling Istio...\n")

	// Download istioctl if not present
	istioctlPath, err := p.ensureIstioctl(version)
	if err != nil {
		return fmt.Errorf("failed to setup istioctl: %w", err)
	}

	// Uninstall Istio
	cmd := execCommand(istioctlPath, "uninstall", "--purge", "-y", "--kube-context", kubecontext)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to uninstall Istio: %w", err)
	}

	// Delete namespace
	p.Log.Debug("Deleting namespace '%s'...\n", defaultNamespace)
	_ = p.runKubectl(kubecontext, "delete", "namespace", defaultNamespace, "--ignore-not-found=true")

	p.Log.Success("Istio uninstalled\n")
	return nil
}

// IsInstalled checks if Istio is installed.
func (p *Plugin) IsInstalled(kubecontext string) (bool, error) {
	cmd := execCommand("kubectl", "--context", kubecontext,
		"get", "deployment", "istiod", "-n", defaultNamespace)
	if err := cmd.Run(); err != nil {
		return false, nil
	}
	return true, nil
}

// Upgrade re-applies the Istio configuration.
func (p *Plugin) Upgrade(cfg interface{}, kubecontext string, providerType string) error {
	// For Istio, upgrade is similar to install (istioctl handles upgrades)
	return p.Install(cfg, kubecontext, providerType)
}

// DryRun shows what would be installed.
func (p *Plugin) DryRun(cfg interface{}, kubecontext string, providerType string) error {
	istioCfg, ok := cfg.(*template.IstioTemplate)
	if !ok {
		return fmt.Errorf("invalid config type for istio plugin")
	}

	version := istioCfg.Version
	if version == "" {
		version = defaultVersion
	}

	profile := istioCfg.Profile
	if profile == "" {
		profile = "default"
	}

	fmt.Printf("[istio] Would install version: %s\n", version)

	installed, err := p.IsInstalled(kubecontext)
	if err != nil {
		return err
	}
	if installed {
		fmt.Println("  Status: already installed (would upgrade)")
	} else {
		fmt.Println("  Status: not installed")
	}
	fmt.Printf("  Profile: %s\n", profile)
	fmt.Printf("  Namespace: %s\n", defaultNamespace)
	if istioCfg.IngressGateway {
		fmt.Println("  Ingress Gateway: enabled")
	}
	if istioCfg.EgressGateway {
		fmt.Println("  Egress Gateway: enabled")
	}

	return nil
}

// ensureIstioctl downloads and returns the path to istioctl.
func (p *Plugin) ensureIstioctl(version string) (string, error) {
	// Check if istioctl is already in PATH
	if path, err := exec.LookPath("istioctl"); err == nil {
		p.Log.Debug("Using istioctl from PATH: %s\n", path)
		return path, nil
	}

	// Download istioctl to a temporary location
	p.Log.Debug("Downloading istioctl %s...\n", version)

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	istioctlDir := fmt.Sprintf("%s/.deploy-cluster/istio/%s", home, version)
	istioctlPath := fmt.Sprintf("%s/bin/istioctl", istioctlDir)

	// Check if already downloaded
	if _, err := os.Stat(istioctlPath); err == nil {
		p.Log.Debug("Using cached istioctl: %s\n", istioctlPath)
		return istioctlPath, nil
	}

	// Create directory
	if err := os.MkdirAll(fmt.Sprintf("%s/bin", istioctlDir), 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Download using curl
	downloadURL := fmt.Sprintf("https://github.com/istio/istio/releases/download/%s/istioctl-%s-linux-amd64.tar.gz", version, version)
	if os.Getenv("ISTIO_DOWNLOAD_URL") != "" {
		downloadURL = os.Getenv("ISTIO_DOWNLOAD_URL")
	}

	p.Log.Debug("Downloading from: %s\n", downloadURL)

	// Download and extract
	curlCmd := exec.Command("curl", "-L", "-s", downloadURL, "-o", "/tmp/istioctl.tar.gz")
	if err := curlCmd.Run(); err != nil {
		return "", fmt.Errorf("failed to download istioctl: %w", err)
	}

	tarCmd := exec.Command("tar", "-xzf", "/tmp/istioctl.tar.gz", "-C", fmt.Sprintf("%s/bin", istioctlDir), "istioctl")
	if err := tarCmd.Run(); err != nil {
		return "", fmt.Errorf("failed to extract istioctl: %w", err)
	}

	// Make executable
	if err := os.Chmod(istioctlPath, 0755); err != nil {
		return "", fmt.Errorf("failed to make istioctl executable: %w", err)
	}

	p.Log.Debug("istioctl downloaded to: %s\n", istioctlPath)
	return istioctlPath, nil
}

func (p *Plugin) installIngressGateway(kubecontext, istioctlPath string, cfg *template.IstioTemplate) error {
	args := []string{
		"install",
		"--set", "profile=empty",
		"--set", "components.ingressGateways[0].name=istio-ingressgateway",
		"--set", "components.ingressGateways[0].enabled=true",
		"--kube-context", kubecontext,
		"--skip-confirmation",
	}

	if cfg.Revision != "" {
		args = append(args, "--revision", cfg.Revision)
	}

	return retry.Run(3, 5*time.Second, p.Log.Warn, func() error {
		cmd := execCommand(istioctlPath, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	})
}

func (p *Plugin) installEgressGateway(kubecontext, istioctlPath string, cfg *template.IstioTemplate) error {
	args := []string{
		"install",
		"--set", "profile=empty",
		"--set", "components.egressGateways[0].name=istio-egressgateway",
		"--set", "components.egressGateways[0].enabled=true",
		"--kube-context", kubecontext,
		"--skip-confirmation",
	}

	if cfg.Revision != "" {
		args = append(args, "--revision", cfg.Revision)
	}

	return retry.Run(3, 5*time.Second, p.Log.Warn, func() error {
		cmd := execCommand(istioctlPath, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	})
}

func (p *Plugin) waitForDeployment(kubecontext, namespace, name string, timeout time.Duration) error {
	cmd := execCommand("kubectl", "--context", kubecontext, "rollout", "status", "deployment/"+name, "-n", namespace, "--timeout", timeout.String())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (p *Plugin) runKubectl(kubecontext string, args ...string) error {
	fullArgs := append([]string{"--context", kubecontext}, args...)
	cmd := execCommand("kubectl", fullArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
