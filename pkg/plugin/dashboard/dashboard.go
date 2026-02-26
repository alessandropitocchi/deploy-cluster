package dashboard

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
	defaultHeadlampChart    = "https://kubernetes-sigs.github.io/headlamp/"
	defaultHeadlampVersion  = "0.40.0"
	defaultHeadlampChartRef = "headlamp/headlamp"
	releaseName             = "headlamp"
	namespace               = "headlamp"
)

// Plugin implements the plugin.Plugin interface for dashboard.
type Plugin struct {
	Log     *logger.Logger
	Timeout time.Duration
}

// New creates a new dashboard plugin.
func New(log *logger.Logger, timeout time.Duration) *Plugin {
	return &Plugin{Log: log, Timeout: timeout}
}

// Name returns the plugin name.
func (p *Plugin) Name() string {
	return "dashboard"
}

// Install installs the dashboard plugin.
func (p *Plugin) Install(cfg interface{}, kubecontext string, providerType string) error {
	dashCfg, ok := cfg.(*template.DashboardTemplate)
	if !ok {
		return fmt.Errorf("invalid config type for dashboard plugin: expected *template.DashboardTemplate")
	}

	switch dashCfg.Type {
	case "headlamp":
		return p.installHeadlamp(dashCfg, kubecontext)
	default:
		return fmt.Errorf("unsupported dashboard type: %s (supported: headlamp)", dashCfg.Type)
	}
}

// Uninstall removes the dashboard plugin.
func (p *Plugin) Uninstall(cfg interface{}, kubecontext string) error {
	dashCfg, ok := cfg.(*template.DashboardTemplate)
	if !ok {
		return fmt.Errorf("invalid config type for dashboard plugin")
	}

	switch dashCfg.Type {
	case "headlamp":
		return p.uninstallHeadlamp(kubecontext)
	default:
		return fmt.Errorf("unsupported dashboard type: %s", dashCfg.Type)
	}
}

// IsInstalled checks if dashboard is installed.
func (p *Plugin) IsInstalled(kubecontext string) (bool, error) {
	cmd := execCommand("helm", "status", releaseName,
		"--namespace", namespace, "--kube-context", kubecontext)
	if err := cmd.Run(); err != nil {
		return false, nil
	}
	return true, nil
}

// Upgrade re-applies the dashboard configuration (idempotent).
func (p *Plugin) Upgrade(cfg interface{}, kubecontext string, providerType string) error {
	// For dashboard, upgrade is the same as install (idempotent)
	return p.Install(cfg, kubecontext, providerType)
}

// DryRun shows what would be installed.
func (p *Plugin) DryRun(cfg interface{}, kubecontext string, providerType string) error {
	dashCfg, ok := cfg.(*template.DashboardTemplate)
	if !ok {
		return fmt.Errorf("invalid config type for dashboard plugin")
	}

	version := p.chartVersion(dashCfg)
	fmt.Printf("[dashboard] Would install: %s (chart version: %s)\n", dashCfg.Type, version)

	installed, err := p.IsInstalled(kubecontext)
	if err != nil {
		return err
	}
	if installed {
		fmt.Println("  Status: already installed (would upgrade)")
	} else {
		fmt.Println("  Status: not installed")
	}
	fmt.Printf("  Chart: %s\n", defaultHeadlampChartRef)
	fmt.Printf("  Repository: %s\n", defaultHeadlampChart)
	fmt.Printf("  Namespace: %s\n", namespace)

	if dashCfg.Ingress != nil && dashCfg.Ingress.Enabled {
		fmt.Printf("  Ingress: enabled (host: %s)\n", dashCfg.Ingress.Host)
	}
	return nil
}

func (p *Plugin) chartVersion(cfg *template.DashboardTemplate) string {
	if cfg.Version != "" {
		return cfg.Version
	}
	return defaultHeadlampVersion
}

func (p *Plugin) installHeadlamp(cfg *template.DashboardTemplate, kubecontext string) error {
	version := p.chartVersion(cfg)
	p.Log.Info("Installing Headlamp %s via Helm...\n", version)

	args := []string{
		"upgrade", "--install", releaseName, defaultHeadlampChartRef,
		"--version", version,
		"--namespace", namespace,
		"--create-namespace",
		"--kube-context", kubecontext,
		"--wait",
		"--timeout", p.Timeout.String(),
	}

	err := retry.Run(3, 5*time.Second, p.Log.Warn, func() error {
		_ = execCommand("helm", "repo", "add", releaseName, defaultHeadlampChart).Run()
		_ = execCommand("helm", "repo", "update").Run()
		cmd := execCommand("helm", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	})
	if err != nil {
		return fmt.Errorf("failed to install Headlamp: %w", err)
	}

	// Create ClusterRoleBinding for the headlamp service account
	p.Log.Debug("Creating ClusterRoleBinding for Headlamp...\n")
	crbManifest := fmt.Sprintf(`apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: headlamp-admin
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: cluster-admin
subjects:
  - kind: ServiceAccount
    name: headlamp
    namespace: %s`, namespace)

	crbCmd := execCommand("kubectl", "--context", kubecontext, "apply", "-f", "-")
	crbCmd.Stdin = strings.NewReader(crbManifest)
	crbCmd.Stdout = os.Stdout
	crbCmd.Stderr = os.Stderr
	if err := crbCmd.Run(); err != nil {
		p.Log.Warn("Warning: failed to create ClusterRoleBinding: %v\n", err)
	}

	p.Log.Success("Headlamp installed successfully\n")

	// Configure ingress if enabled
	if cfg.Ingress != nil && cfg.Ingress.Enabled {
		if err := p.configureIngress(cfg.Ingress, kubecontext); err != nil {
			return fmt.Errorf("failed to configure Headlamp ingress: %w", err)
		}
	} else {
		p.Log.Info("\nTo access Headlamp:\n")
		p.Log.Info("  kubectl port-forward svc/headlamp -n %s 4466:80\n", namespace)
		p.Log.Info("  Open: http://localhost:4466\n")
	}

	return nil
}

func (p *Plugin) configureIngress(cfg *template.DashboardIngressTemplate, kubecontext string) error {
	p.Log.Info("Configuring ingress for Headlamp...\n")

	manifest := k8s.IngressManifest(k8s.IngressConfig{
		Name:        "headlamp-ingress",
		Namespace:   namespace,
		Host:        cfg.Host,
		ServiceName: "headlamp",
		ServicePort: 80,
	})

	err := retry.Run(3, 5*time.Second, p.Log.Warn, func() error {
		cmd := execCommand("kubectl", "--context", kubecontext, "apply", "-f", "-")
		cmd.Stdin = strings.NewReader(manifest)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	})
	if err != nil {
		return fmt.Errorf("failed to apply headlamp ingress: %w", err)
	}

	p.Log.Success("Headlamp available at: http://%s\n", cfg.Host)
	return nil
}

func (p *Plugin) uninstallHeadlamp(kubecontext string) error {
	p.Log.Info("Uninstalling Headlamp...\n")

	cmd := execCommand("helm", "uninstall", releaseName,
		"--namespace", namespace,
		"--kube-context", kubecontext)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to uninstall Headlamp: %w", err)
	}

	// Clean up ClusterRoleBinding
	crbCmd := execCommand("kubectl", "--context", kubecontext,
		"delete", "clusterrolebinding", "headlamp-admin", "--ignore-not-found=true")
	crbCmd.Stdout = os.Stdout
	crbCmd.Stderr = os.Stderr
	_ = crbCmd.Run()

	p.Log.Success("Headlamp uninstalled\n")
	return nil
}
