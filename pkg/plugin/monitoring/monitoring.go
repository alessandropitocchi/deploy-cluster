package monitoring

import (
	"fmt"
	"os"
	"os/exec"
	"time"

	"github.com/alepito/deploy-cluster/pkg/config"
)

const (
	defaultChartRef     = "oci://ghcr.io/prometheus-community/charts/kube-prometheus-stack"
	defaultChartVersion = "72.6.2"
	releaseName         = "kube-prometheus-stack"
	namespace           = "monitoring"
)

type Plugin struct {
	Verbose bool
}

func New() *Plugin {
	return &Plugin{Verbose: true}
}

func (p *Plugin) Name() string {
	return "monitoring"
}

func (p *Plugin) log(format string, args ...any) {
	if p.Verbose {
		fmt.Printf(format, args...)
	}
}

func (p *Plugin) Install(cfg *config.MonitoringConfig, kubecontext string) error {
	switch cfg.Type {
	case "prometheus":
		return p.installPrometheus(cfg, kubecontext)
	default:
		return fmt.Errorf("unsupported monitoring type: %s (supported: prometheus)", cfg.Type)
	}
}

func (p *Plugin) Uninstall(cfg *config.MonitoringConfig, kubecontext string) error {
	switch cfg.Type {
	case "prometheus":
		return p.uninstallPrometheus(kubecontext)
	default:
		return fmt.Errorf("unsupported monitoring type: %s", cfg.Type)
	}
}

func (p *Plugin) IsInstalled(kubecontext string) (bool, error) {
	cmd := exec.Command("helm", "status", releaseName,
		"--namespace", namespace, "--kube-context", kubecontext)
	if err := cmd.Run(); err != nil {
		return false, nil
	}
	return true, nil
}

func (p *Plugin) chartVersion(cfg *config.MonitoringConfig) string {
	if cfg.Version != "" {
		return cfg.Version
	}
	return defaultChartVersion
}

func (p *Plugin) installPrometheus(cfg *config.MonitoringConfig, kubecontext string) error {
	version := p.chartVersion(cfg)
	p.log("[monitoring] Installing kube-prometheus-stack %s via Helm...\n", version)

	args := []string{
		"upgrade", "--install", releaseName, defaultChartRef,
		"--version", version,
		"--namespace", namespace,
		"--create-namespace",
		"--kube-context", kubecontext,
		"--wait",
		"--timeout", (5 * time.Minute).String(),
	}

	cmd := exec.Command("helm", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install kube-prometheus-stack: %w", err)
	}

	p.log("[monitoring] ✓ kube-prometheus-stack installed successfully\n")
	p.log("\n[monitoring] To access Grafana:\n")
	p.log("  kubectl port-forward svc/kube-prometheus-stack-grafana -n %s 3000:80\n", namespace)
	p.log("  Open: http://localhost:3000 (admin/prom-operator)\n")
	p.log("\n[monitoring] To access Prometheus:\n")
	p.log("  kubectl port-forward svc/kube-prometheus-stack-prometheus -n %s 9090:9090\n", namespace)
	p.log("  Open: http://localhost:9090\n")
	return nil
}

func (p *Plugin) uninstallPrometheus(kubecontext string) error {
	p.log("[monitoring] Uninstalling kube-prometheus-stack...\n")

	cmd := exec.Command("helm", "uninstall", releaseName,
		"--namespace", namespace,
		"--kube-context", kubecontext)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to uninstall kube-prometheus-stack: %w", err)
	}

	// Clean up CRDs (helm doesn't remove CRDs by default)
	p.log("[monitoring] Cleaning up Prometheus CRDs...\n")
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
		crdCmd := exec.Command("kubectl", "--context", kubecontext,
			"delete", "crd", crd, "--ignore-not-found=true")
		crdCmd.Stdout = os.Stdout
		crdCmd.Stderr = os.Stderr
		_ = crdCmd.Run()
	}

	p.log("[monitoring] ✓ kube-prometheus-stack uninstalled\n")
	return nil
}
