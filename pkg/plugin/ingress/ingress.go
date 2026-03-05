package ingress

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/alessandropitocchi/deploy-cluster/pkg/k8s"
	"github.com/alessandropitocchi/deploy-cluster/pkg/logger"
	"github.com/alessandropitocchi/deploy-cluster/pkg/retry"
	"github.com/alessandropitocchi/deploy-cluster/pkg/template"
)

// execCommand is a package-level variable for creating exec.Cmd, replaceable in tests.
var execCommand = exec.Command

const (
	// Traefik Helm chart
	traefikRepoURL      = "https://traefik.github.io/charts"
	traefikChartName    = "traefik"
	traefikNamespace    = "traefik"
	traefikReleaseName  = "traefik"

	// NGINX Gateway Fabric OCI chart
	nginxGatewayOCIChart    = "oci://ghcr.io/nginx/charts/nginx-gateway-fabric"
	nginxGatewayNamespace   = "nginx-gateway"
	nginxGatewayReleaseName = "ngf"
	nginxGatewayVersion     = "v2.4.2"
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
	case "traefik":
		return p.installTraefik(ingressCfg, kubecontext, providerType)
	case "nginx-gateway-fabric":
		return p.installNginxGatewayFabric(ingressCfg, kubecontext)
	default:
		return fmt.Errorf("unsupported ingress type: %s (supported: traefik, nginx-gateway-fabric)", ingressCfg.Type)
	}
}

// Uninstall removes the ingress plugin.
func (p *Plugin) Uninstall(cfg interface{}, kubecontext string) error {
	ingressCfg, ok := cfg.(*template.IngressTemplate)
	if !ok {
		return fmt.Errorf("invalid config type for ingress plugin")
	}

	switch ingressCfg.Type {
	case "traefik":
		return p.uninstallTraefik(kubecontext)
	case "nginx-gateway-fabric":
		return p.uninstallNginxGatewayFabric(kubecontext)
	default:
		return fmt.Errorf("unsupported ingress type: %s", ingressCfg.Type)
	}
}

// IsInstalled checks if the ingress plugin is installed.
func (p *Plugin) IsInstalled(kubecontext string) (bool, error) {
	// Check Traefik
	cmd := execCommand("kubectl", "--context", kubecontext,
		"get", "deployment", "traefik", "-n", traefikNamespace)
	if err := cmd.Run(); err == nil {
		return true, nil
	}

	// Check NGINX Gateway Fabric
	cmd = execCommand("kubectl", "--context", kubecontext,
		"get", "deployment", nginxGatewayReleaseName+"-nginx-gateway-fabric", "-n", nginxGatewayNamespace)
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
		switch ingressCfg.Type {
		case "traefik":
			fmt.Printf("  Helm Chart: %s/%s\n", traefikRepoURL, traefikChartName)
			fmt.Printf("  Namespace: %s\n", traefikNamespace)
		case "nginx-gateway-fabric":
			fmt.Printf("  OCI Chart: %s\n", nginxGatewayOCIChart)
			fmt.Printf("  Namespace: %s\n", nginxGatewayNamespace)
		}
		if ingressCfg.Version != "" {
			fmt.Printf("  Version: %s\n", ingressCfg.Version)
		}
	}
	return nil
}

// installTraefik installs Traefik with Gateway API support
func (p *Plugin) installTraefik(cfg *template.IngressTemplate, kubecontext string, providerType string) error {
	p.Log.Info("Installing Traefik ingress controller with Gateway API support...\n")

	// Add Traefik Helm repo
	p.Log.Info("Adding Traefik Helm repository...\n")
	cmd := execCommand("helm", "repo", "add", "traefik", traefikRepoURL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		p.Log.Warn("Failed to add Traefik repo (may already exist), continuing...\n")
	}

	// Update repos
	cmd = execCommand("helm", "repo", "update")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to update Helm repos: %w", err)
	}

	// Create namespace
	p.Log.Info("Creating Traefik namespace...\n")
	cmd = execCommand("kubectl", "--context", kubecontext,
		"create", "namespace", traefikNamespace, "--dry-run=client", "-o", "yaml")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to generate namespace manifest: %w", err)
	}

	applyCmd := execCommand("kubectl", "--context", kubecontext, "apply", "-f", "-")
	applyCmd.Stdin = bytes.NewReader(output)
	applyCmd.Stdout = os.Stdout
	applyCmd.Stderr = os.Stderr
	// Ignore error if namespace already exists
	_ = applyCmd.Run()

	// Install Traefik with Gateway API enabled
	p.Log.Info("Installing Traefik Helm chart...\n")
	err = retry.Run(3, 5*time.Second, p.Log.Warn, func() error {
		args := []string{
			"upgrade", "--install", traefikReleaseName, traefikChartName + "/" + traefikChartName,
			"--namespace", traefikNamespace,
			"--kube-context", kubecontext,
			"--set", "experimental.kubernetesGateway.enabled=true",
			"--set", "providers.kubernetesIngress.enabled=true",
			"--set", "providers.kubernetesGateway.enabled=true",
			"--set", "ports.web.exposedPort=80",
			"--set", "ports.websecure.exposedPort=443",
		}

		// Add version if specified
		if cfg.Version != "" {
			args = append(args, "--version", cfg.Version)
		}

		// Add values from file if specified
		if cfg.ValuesFile != "" {
			args = append(args, "--values", cfg.ValuesFile)
		}

		// Add inline values
		for key, value := range cfg.Values {
			args = append(args, "--set", fmt.Sprintf("%s=%v", key, value))
		}

		args = append(args, "--wait", "--timeout", p.Timeout.String())

		cmd := execCommand("helm", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	})
	if err != nil {
		return fmt.Errorf("failed to install Traefik: %w", err)
	}

	p.Log.Success("Traefik ingress controller installed successfully\n")
	p.Log.Info("Ingress class: traefik\n")
	p.Log.Info("Gateway class: traefik\n")

	// Create shared Gateway for HTTPRoutes
	if err := p.createSharedGateway(kubecontext, "traefik"); err != nil {
		p.Log.Warn("Failed to create shared Gateway: %v\n", err)
	}

	return nil
}

// createSharedGateway creates a shared Gateway resource for HTTPRoutes
func (p *Plugin) createSharedGateway(kubecontext string, ingressType string) error {
	p.Log.Info("Creating shared Gateway for HTTPRoutes...\n")

	gatewayClass := k8s.GetGatewayClassName(ingressType)
	var namespace string
	if ingressType == "traefik" {
		namespace = traefikNamespace
	} else {
		namespace = nginxGatewayNamespace
	}

	// Determine port based on ingress type
	var port int32 = 8000 // Traefik default
	if ingressType == "nginx-gateway-fabric" {
		port = 80
	}

	manifest := k8s.GatewayManifest(k8s.GatewayConfig{
		Name:             "shared-gateway",
		Namespace:        namespace,
		GatewayClassName: gatewayClass,
		Hosts:            []string{}, // Empty = accept all hosts
		Port:             port,
		AllowAllRoutes:   true, // Allow routes from all namespaces
	})

	cmd := execCommand("kubectl", "--context", kubecontext, "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(manifest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to apply Gateway: %w", err)
	}

	p.Log.Success("Shared Gateway created in namespace %s\n", namespace)
	return nil
}

func (p *Plugin) uninstallTraefik(kubecontext string) error {
	p.Log.Info("Uninstalling Traefik ingress controller...\n")

	cmd := execCommand("helm", "uninstall", traefikReleaseName,
		"--namespace", traefikNamespace,
		"--kube-context", kubecontext)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to uninstall Traefik: %w", err)
	}

	// Delete namespace
	cmd = execCommand("kubectl", "--context", kubecontext,
		"delete", "namespace", traefikNamespace, "--ignore-not-found=true")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()

	p.Log.Success("Traefik ingress controller uninstalled\n")
	return nil
}

// installNginxGatewayFabric installs NGINX Gateway Fabric (F5) with Gateway API support via Helm OCI
func (p *Plugin) installNginxGatewayFabric(cfg *template.IngressTemplate, kubecontext string) error {
	p.Log.Info("Installing NGINX Gateway Fabric (Gateway API)...\n")

	// Install Gateway API CRDs first
	p.Log.Info("Installing Gateway API CRDs...\n")
	crdsCmd := execCommand("sh", "-c",
		fmt.Sprintf("kubectl kustomize 'https://github.com/nginx/nginx-gateway-fabric/config/crd/gateway-api/standard?ref=%s' | kubectl --context %s apply -f -",
			nginxGatewayVersion, kubecontext))
	crdsCmd.Stdout = os.Stdout
	crdsCmd.Stderr = os.Stderr
	if err := crdsCmd.Run(); err != nil {
		return fmt.Errorf("failed to install Gateway API CRDs: %w", err)
	}

	// Install NGINX Gateway Fabric via Helm OCI
	p.Log.Info("Installing NGINX Gateway Fabric Helm chart...\n")
	err := retry.Run(3, 5*time.Second, p.Log.Warn, func() error {
		args := []string{
			"upgrade", "--install", nginxGatewayReleaseName,
			nginxGatewayOCIChart,
			"--namespace", nginxGatewayNamespace,
			"--kube-context", kubecontext,
			"--create-namespace",
		}

		// Add version if specified
		if cfg.Version != "" {
			args = append(args, "--version", cfg.Version)
		}

		// Add values from file if specified
		if cfg.ValuesFile != "" {
			args = append(args, "--values", cfg.ValuesFile)
		}

		// Add inline values
		for key, value := range cfg.Values {
			args = append(args, "--set", fmt.Sprintf("%s=%v", key, value))
		}

		args = append(args, "--wait", "--timeout", p.Timeout.String())

		cmd := execCommand("helm", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	})
	if err != nil {
		return fmt.Errorf("failed to install NGINX Gateway Fabric: %w", err)
	}

	p.Log.Success("NGINX Gateway Fabric installed successfully\n")
	p.Log.Info("Gateway class: nginx\n")

	// Create shared Gateway for HTTPRoutes
	if err := p.createSharedGateway(kubecontext, "nginx-gateway-fabric"); err != nil {
		p.Log.Warn("Failed to create shared Gateway: %v\n", err)
	}

	return nil
}

func (p *Plugin) uninstallNginxGatewayFabric(kubecontext string) error {
	p.Log.Info("Uninstalling NGINX Gateway Fabric...\n")

	cmd := execCommand("helm", "uninstall", nginxGatewayReleaseName,
		"--namespace", nginxGatewayNamespace,
		"--kube-context", kubecontext)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to uninstall NGINX Gateway Fabric: %w", err)
	}

	// Delete namespace
	cmd = execCommand("kubectl", "--context", kubecontext,
		"delete", "namespace", nginxGatewayNamespace, "--ignore-not-found=true")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	_ = cmd.Run()

	p.Log.Success("NGINX Gateway Fabric uninstalled\n")
	return nil
}
