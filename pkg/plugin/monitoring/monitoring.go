package monitoring

import (
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
	defaultChartRef     = "oci://ghcr.io/prometheus-community/charts/kube-prometheus-stack"
	defaultChartVersion = "72.6.2"
	releaseName         = "kube-prometheus-stack"
	namespace           = "monitoring"
)

// Plugin implements the plugin.Plugin interface for monitoring.
type Plugin struct {
	Log     *logger.Logger
	Timeout time.Duration
}

// New creates a new monitoring plugin.
func New(log *logger.Logger, timeout time.Duration) *Plugin {
	return &Plugin{Log: log, Timeout: timeout}
}

// Name returns the plugin name.
func (p *Plugin) Name() string {
	return "monitoring"
}

// Install installs the monitoring plugin.
func (p *Plugin) Install(cfg interface{}, kubecontext string, providerType string) error {
	monCfg, ok := cfg.(*template.MonitoringTemplate)
	if !ok {
		return fmt.Errorf("invalid config type for monitoring plugin: expected *template.MonitoringTemplate")
	}

	switch monCfg.Type {
	case "prometheus":
		return p.installPrometheus(monCfg, kubecontext)
	default:
		return fmt.Errorf("unsupported monitoring type: %s (supported: prometheus)", monCfg.Type)
	}
}

// Uninstall removes the monitoring plugin.
func (p *Plugin) Uninstall(cfg interface{}, kubecontext string) error {
	monCfg, ok := cfg.(*template.MonitoringTemplate)
	if !ok {
		return fmt.Errorf("invalid config type for monitoring plugin")
	}

	switch monCfg.Type {
	case "prometheus":
		return p.uninstallPrometheus(kubecontext)
	default:
		return fmt.Errorf("unsupported monitoring type: %s", monCfg.Type)
	}
}

// IsInstalled checks if monitoring is installed.
func (p *Plugin) IsInstalled(kubecontext string) (bool, error) {
	cmd := execCommand("helm", "status", releaseName,
		"--namespace", namespace, "--kube-context", kubecontext)
	if err := cmd.Run(); err != nil {
		return false, nil
	}
	return true, nil
}

// Upgrade re-applies the monitoring configuration (idempotent).
func (p *Plugin) Upgrade(cfg interface{}, kubecontext string, providerType string) error {
	// For monitoring, upgrade is the same as install (idempotent)
	return p.Install(cfg, kubecontext, providerType)
}

// DryRun shows what would be installed.
func (p *Plugin) DryRun(cfg interface{}, kubecontext string, providerType string) error {
	monCfg, ok := cfg.(*template.MonitoringTemplate)
	if !ok {
		return fmt.Errorf("invalid config type for monitoring plugin")
	}

	version := p.chartVersion(monCfg)
	fmt.Printf("[monitoring] Would install: %s (chart version: %s)\n", monCfg.Type, version)

	installed, err := p.IsInstalled(kubecontext)
	if err != nil {
		return err
	}
	if installed {
		fmt.Println("  Status: already installed (would upgrade)")
	} else {
		fmt.Println("  Status: not installed")
	}
	fmt.Printf("  Chart: %s\n", defaultChartRef)
	fmt.Printf("  Namespace: %s\n", namespace)

	if monCfg.Ingress != nil && monCfg.Ingress.Enabled {
		fmt.Printf("  Ingress: enabled (host: %s)\n", monCfg.Ingress.Host)
	}
	return nil
}

func (p *Plugin) chartVersion(cfg *template.MonitoringTemplate) string {
	if cfg.Version != "" {
		return cfg.Version
	}
	return defaultChartVersion
}

func (p *Plugin) installPrometheus(cfg *template.MonitoringTemplate, kubecontext string) error {
	version := p.chartVersion(cfg)
	p.Log.Info("Installing kube-prometheus-stack %s via Helm...\n", version)

	args := []string{
		"upgrade", "--install", releaseName, defaultChartRef,
		"--version", version,
		"--namespace", namespace,
		"--create-namespace",
		"--kube-context", kubecontext,
		"--wait",
		"--timeout", p.Timeout.String(),
	}

	// Add values from file if specified
	if cfg.ValuesFile != "" {
		args = append(args, "--values", cfg.ValuesFile)
	}

	// Add inline values
	for key, value := range cfg.Values {
		args = append(args, "--set", fmt.Sprintf("%s=%v", key, value))
	}

	err := retry.Run(3, 5*time.Second, p.Log.Warn, func() error {
		cmd := execCommand("helm", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	})
	if err != nil {
		return fmt.Errorf("failed to install kube-prometheus-stack: %w", err)
	}

	p.Log.Success("kube-prometheus-stack installed successfully\n")

	// Configure ingress for Grafana if enabled
	if cfg.Ingress != nil && cfg.Ingress.Enabled {
		if err := p.configureGrafanaIngress(cfg.Ingress, kubecontext); err != nil {
			return fmt.Errorf("failed to configure Grafana ingress: %w", err)
		}
	} else {
		p.Log.Info("\nTo access Grafana:\n")
		p.Log.Info("  kubectl port-forward svc/kube-prometheus-stack-grafana -n %s 3000:80\n", namespace)
		p.Log.Info("  Open: http://localhost:3000 (admin/prom-operator)\n")
	}

	p.Log.Info("\nTo access Prometheus:\n")
	p.Log.Info("  kubectl port-forward svc/kube-prometheus-stack-prometheus -n %s 9090:9090\n", namespace)
	p.Log.Info("  Open: http://localhost:9090\n")
	return nil
}

func (p *Plugin) configureGrafanaIngress(cfg *template.MonitoringIngressTemplate, kubecontext string) error {
	p.Log.Info("Configuring HTTPRoute for Grafana...\n")

	// Try to detect which ingress controller is installed by checking namespaces
	gatewayNS := p.detectIngressNamespace(kubecontext)
	if gatewayNS == "" {
		p.Log.Warn("Could not detect ingress controller, defaulting to traefik namespace\n")
		gatewayNS = "traefik"
	}

	manifest := k8s.HTTPRouteManifest(k8s.HTTPRouteConfig{
		Name:             "grafana",
		Namespace:        namespace,
		Host:             cfg.Host,
		GatewayName:      "shared-gateway",
		GatewayNamespace: gatewayNS,
		ServiceName:      "kube-prometheus-stack-grafana",
		ServicePort:      80,
	})

	err := retry.Run(3, 5*time.Second, p.Log.Warn, func() error {
		cmd := execCommand("kubectl", "--context", kubecontext, "apply", "-f", "-")
		cmd.Stdin = strings.NewReader(manifest)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	})
	if err != nil {
		return fmt.Errorf("failed to apply grafana HTTPRoute: %w", err)
	}

	p.Log.Success("Grafana available at: http://%s (admin/prom-operator)\n", cfg.Host)
	return nil
}

// detectIngressNamespace tries to detect which ingress controller namespace exists
func (p *Plugin) detectIngressNamespace(kubecontext string) string {
	// Check traefik namespace
	cmd := execCommand("kubectl", "--context", kubecontext, "get", "namespace", "traefik")
	if err := cmd.Run(); err == nil {
		return "traefik"
	}

	// Check nginx-gateway namespace
	cmd = execCommand("kubectl", "--context", kubecontext, "get", "namespace", "nginx-gateway")
	if err := cmd.Run(); err == nil {
		return "nginx-gateway"
	}

	return ""
}

func (p *Plugin) uninstallPrometheus(kubecontext string) error {
	p.Log.Info("Uninstalling kube-prometheus-stack...\n")

	cmd := execCommand("helm", "uninstall", releaseName,
		"--namespace", namespace,
		"--kube-context", kubecontext)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to uninstall kube-prometheus-stack: %w", err)
	}

	// Clean up CRDs (helm doesn't remove CRDs by default)
	p.Log.Info("Cleaning up Prometheus CRDs...\n")
	crds := []string{
		"alertmanagerconfigs.monitoring.coreos.com",
		"alertmanagers.monitoring.coreos.com",
		"podmonitors.monitoring.coreos.com",
		"probes.monitoring.coreos.com",
		"prometheusagents.monitoring.coreos.com",
		"prometheuses.monitoring.coreos.com",
		"prometheusrules.monitoring.coreos.com",
		"scrapeconfigs.monitoring.coreos.com",
		"servicemonitors.monitoring.coreos.com",
		"thanosrulers.monitoring.coreos.com",
	}
	for _, crd := range crds {
		crdCmd := execCommand("kubectl", "--context", kubecontext,
			"delete", "crd", crd, "--ignore-not-found=true")
		crdCmd.Stdout = os.Stdout
		crdCmd.Stderr = os.Stderr
		_ = crdCmd.Run()
	}

	p.Log.Success("kube-prometheus-stack uninstalled\n")
	return nil
}
