package customapps

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/alepito/deploy-cluster/pkg/k8s"
	"github.com/alepito/deploy-cluster/pkg/logger"
	"github.com/alepito/deploy-cluster/pkg/retry"
	"github.com/alepito/deploy-cluster/pkg/template"
	"gopkg.in/yaml.v3"
)

// execCommand is a package-level variable for creating exec.Cmd, replaceable in tests.
var execCommand = exec.Command

// Plugin implements the plugin.Plugin interface for custom apps.
type Plugin struct {
	Log     *logger.Logger
	Timeout time.Duration
}

// New creates a new custom apps plugin.
func New(log *logger.Logger, timeout time.Duration) *Plugin {
	return &Plugin{Log: log, Timeout: timeout}
}

// Name returns the plugin name.
func (p *Plugin) Name() string {
	return "custom-apps"
}

// Install installs all custom apps from the config.
func (p *Plugin) Install(cfg interface{}, kubecontext string, providerType string) error {
	apps, ok := cfg.([]template.CustomAppTemplate)
	if !ok {
		return fmt.Errorf("invalid config type for custom-apps plugin: expected []template.CustomAppTemplate")
	}

	for _, app := range apps {
		if err := p.installSingle(app, kubecontext); err != nil {
			return fmt.Errorf("failed to install custom app %q: %w", app.Name, err)
		}
	}
	return nil
}

// Uninstall removes all custom apps.
func (p *Plugin) Uninstall(cfg interface{}, kubecontext string) error {
	apps, ok := cfg.([]template.CustomAppTemplate)
	if !ok {
		return fmt.Errorf("invalid config type for custom-apps plugin")
	}

	for _, app := range apps {
		if err := p.uninstallSingle(app.Name, app.Namespace, kubecontext); err != nil {
			return fmt.Errorf("failed to uninstall custom app %q: %w", app.Name, err)
		}
	}
	return nil
}

// IsInstalled checks if any custom apps are installed.
// Returns true if at least one app is installed.
func (p *Plugin) IsInstalled(kubecontext string) (bool, error) {
	// List all helm releases across namespaces
	cmd := execCommand("helm", "list", "-A", "--kube-context", kubecontext, "-q")
	output, err := cmd.Output()
	if err != nil {
		return false, nil
	}
	return len(strings.TrimSpace(string(output))) > 0, nil
}

// Upgrade upgrades all custom apps (idempotent install).
func (p *Plugin) Upgrade(cfg interface{}, kubecontext string, providerType string) error {
	// For custom apps, upgrade is the same as install (helm upgrade --install)
	return p.Install(cfg, kubecontext, providerType)
}

// DryRun shows what would be installed.
func (p *Plugin) DryRun(cfg interface{}, kubecontext string, providerType string) error {
	apps, ok := cfg.([]template.CustomAppTemplate)
	if !ok {
		return fmt.Errorf("invalid config type for custom-apps plugin")
	}

	fmt.Printf("[custom-apps] Would install %d application(s):\n", len(apps))
	for _, app := range apps {
		ns := app.Namespace
		if ns == "" {
			ns = app.Name
		}
		fmt.Printf("  - %s (chart: %s, namespace: %s)\n", app.Name, app.ChartName, ns)
		if app.Version != "" {
			fmt.Printf("    Version: %s\n", app.Version)
		}
		if app.Ingress != nil && app.Ingress.Enabled {
			fmt.Printf("    Ingress: %s\n", app.Ingress.Host)
		}
	}
	return nil
}

// InstallAll installs all custom apps (backward compatibility).
func (p *Plugin) InstallAll(apps []template.CustomAppTemplate, kubecontext string) error {
	return p.Install(apps, kubecontext, "")
}

// installSingle installs a single custom app via Helm.
func (p *Plugin) installSingle(app template.CustomAppTemplate, kubecontext string) error {
	namespace := app.Namespace
	if namespace == "" {
		namespace = app.Name
	}

	p.Log.Info("Installing %s (%s)...\n", app.Name, app.ChartName)

	args := []string{
		"upgrade", "--install", app.Name, app.ChartName,
		"--repo", app.ChartRepo,
		"--namespace", namespace,
		"--create-namespace",
		"--kube-context", kubecontext,
		"--wait",
		"--timeout", p.Timeout.String(),
	}

	if app.Version != "" {
		args = append(args, "--version", app.Version)
	}

	// Handle values
	valuesFile, cleanup, err := p.resolveValues(app)
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup()
	}
	if valuesFile != "" {
		args = append(args, "--values", valuesFile)
	}

	err = retry.Run(3, 5*time.Second, p.Log.Warn, func() error {
		cmd := execCommand("helm", args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	})
	if err != nil {
		return fmt.Errorf("helm install failed: %w", err)
	}

	p.Log.Success("%s installed successfully\n", app.Name)

	// Configure ingress if enabled
	if app.Ingress != nil && app.Ingress.Enabled {
		if err := p.configureIngress(app, kubecontext); err != nil {
			return fmt.Errorf("failed to configure ingress for %s: %w", app.Name, err)
		}
	}

	return nil
}

// uninstallSingle removes a single custom app via Helm.
func (p *Plugin) uninstallSingle(name, namespace, kubecontext string) error {
	if namespace == "" {
		namespace = name
	}

	p.Log.Info("Uninstalling %s...\n", name)

	cmd := execCommand("helm", "uninstall", name,
		"--namespace", namespace,
		"--kube-context", kubecontext)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("helm uninstall failed: %w", err)
	}

	p.Log.Success("%s uninstalled\n", name)
	return nil
}

// IsAppInstalled checks if a specific release is installed.
func (p *Plugin) IsAppInstalled(name, namespace, kubecontext string) (bool, error) {
	if namespace == "" {
		namespace = name
	}
	cmd := execCommand("helm", "status", name,
		"--namespace", namespace, "--kube-context", kubecontext)
	if err := cmd.Run(); err != nil {
		return false, nil
	}
	return true, nil
}

// ListInstalled returns the names of all helm releases that match the given custom app names.
func (p *Plugin) ListInstalled(apps []template.CustomAppTemplate, kubecontext string) ([]string, error) {
	var installed []string
	for _, app := range apps {
		ns := app.Namespace
		if ns == "" {
			ns = app.Name
		}
		ok, err := p.IsAppInstalled(app.Name, ns, kubecontext)
		if err != nil {
			return nil, err
		}
		if ok {
			installed = append(installed, app.Name)
		}
	}
	return installed, nil
}

// resolveValues returns a path to a values file (either the user-provided one
// or a temp file with inline values marshalled to YAML). The cleanup function
// should be deferred to remove temp files.
func (p *Plugin) resolveValues(app template.CustomAppTemplate) (string, func(), error) {
	// ValuesFile takes precedence
	if app.ValuesFile != "" {
		path := app.ValuesFile
		if strings.HasPrefix(path, "~/") {
			home, _ := os.UserHomeDir()
			path = home + path[1:]
		}
		return path, nil, nil
	}

	// Inline values
	if len(app.Values) > 0 {
		data, err := yaml.Marshal(app.Values)
		if err != nil {
			return "", nil, fmt.Errorf("failed to marshal values: %w", err)
		}

		tmpFile, err := os.CreateTemp("", "deploy-cluster-values-*.yaml")
		if err != nil {
			return "", nil, fmt.Errorf("failed to create temp values file: %w", err)
		}

		if _, err := tmpFile.Write(data); err != nil {
			tmpFile.Close()
			os.Remove(tmpFile.Name())
			return "", nil, fmt.Errorf("failed to write temp values file: %w", err)
		}
		tmpFile.Close()

		cleanup := func() {
			os.Remove(tmpFile.Name())
		}
		return tmpFile.Name(), cleanup, nil
	}

	return "", nil, nil
}

func (p *Plugin) configureIngress(app template.CustomAppTemplate, kubecontext string) error {
	ing := app.Ingress
	ns := app.Namespace
	if ns == "" {
		ns = app.Name
	}

	serviceName := ing.ServiceName
	if serviceName == "" {
		serviceName = app.Name
	}

	p.Log.Info("Configuring ingress for %s (%s)...\n", app.Name, ing.Host)

	manifest := k8s.IngressManifest(k8s.IngressConfig{
		Name:        app.Name + "-ingress",
		Namespace:   ns,
		Host:        ing.Host,
		ServiceName: serviceName,
		ServicePort: ing.ServicePort,
	})

	err := retry.Run(3, 5*time.Second, p.Log.Warn, func() error {
		cmd := execCommand("kubectl", "--context", kubecontext, "apply", "-f", "-")
		cmd.Stdin = strings.NewReader(manifest)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	})
	if err != nil {
		return fmt.Errorf("failed to apply ingress: %w", err)
	}

	p.Log.Success("%s available at: http://%s\n", app.Name, ing.Host)
	return nil
}
