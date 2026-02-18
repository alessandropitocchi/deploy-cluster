package customapps

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/alepito/deploy-cluster/pkg/config"
	"gopkg.in/yaml.v3"
)

type Plugin struct {
	Verbose bool
}

func New() *Plugin {
	return &Plugin{Verbose: true}
}

func (p *Plugin) Name() string {
	return "customApps"
}

func (p *Plugin) log(format string, args ...any) {
	if p.Verbose {
		fmt.Printf(format, args...)
	}
}

// InstallAll installs all custom apps from the config.
func (p *Plugin) InstallAll(apps []config.CustomAppConfig, kubecontext string) error {
	for _, app := range apps {
		if err := p.Install(app, kubecontext); err != nil {
			return fmt.Errorf("failed to install custom app %q: %w", app.Name, err)
		}
	}
	return nil
}

// Install installs a single custom app via Helm.
func (p *Plugin) Install(app config.CustomAppConfig, kubecontext string) error {
	namespace := app.Namespace
	if namespace == "" {
		namespace = app.Name
	}

	p.log("[customApps] Installing %s (%s)...\n", app.Name, app.Chart)

	args := []string{
		"upgrade", "--install", app.Name, app.Chart,
		"--namespace", namespace,
		"--create-namespace",
		"--kube-context", kubecontext,
		"--wait",
		"--timeout", (5 * time.Minute).String(),
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

	cmd := exec.Command("helm", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("helm install failed: %w", err)
	}

	p.log("[customApps] ✓ %s installed successfully\n", app.Name)

	// Configure ingress if enabled
	if app.Ingress != nil && app.Ingress.Enabled {
		if err := p.configureIngress(app, kubecontext); err != nil {
			return fmt.Errorf("failed to configure ingress for %s: %w", app.Name, err)
		}
	}

	return nil
}

// Uninstall removes a single custom app via Helm.
func (p *Plugin) Uninstall(name, namespace, kubecontext string) error {
	if namespace == "" {
		namespace = name
	}

	p.log("[customApps] Uninstalling %s...\n", name)

	cmd := exec.Command("helm", "uninstall", name,
		"--namespace", namespace,
		"--kube-context", kubecontext)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("helm uninstall failed: %w", err)
	}

	p.log("[customApps] ✓ %s uninstalled\n", name)
	return nil
}

// IsInstalled checks if a specific release is installed.
func (p *Plugin) IsInstalled(name, namespace, kubecontext string) (bool, error) {
	if namespace == "" {
		namespace = name
	}
	cmd := exec.Command("helm", "status", name,
		"--namespace", namespace, "--kube-context", kubecontext)
	if err := cmd.Run(); err != nil {
		return false, nil
	}
	return true, nil
}

// ListInstalled returns the names of all helm releases that match the given custom app names.
func (p *Plugin) ListInstalled(apps []config.CustomAppConfig, kubecontext string) ([]string, error) {
	var installed []string
	for _, app := range apps {
		ns := app.Namespace
		if ns == "" {
			ns = app.Name
		}
		ok, err := p.IsInstalled(app.Name, ns, kubecontext)
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
func (p *Plugin) resolveValues(app config.CustomAppConfig) (string, func(), error) {
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

func (p *Plugin) configureIngress(app config.CustomAppConfig, kubecontext string) error {
	ing := app.Ingress
	namespace := app.Namespace
	if namespace == "" {
		namespace = app.Name
	}

	serviceName := ing.ServiceName
	if serviceName == "" {
		serviceName = app.Name
	}
	servicePort := ing.ServicePort
	if servicePort == 0 {
		servicePort = 80
	}

	p.log("[customApps] Configuring ingress for %s (%s)...\n", app.Name, ing.Host)

	manifest := fmt.Sprintf(`apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: %s-ingress
  namespace: %s
  annotations:
    nginx.ingress.kubernetes.io/backend-protocol: "HTTP"
    nginx.ingress.kubernetes.io/ssl-redirect: "false"
spec:
  ingressClassName: nginx
  rules:
    - host: %s
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: %s
                port:
                  number: %d`, app.Name, namespace, ing.Host, serviceName, servicePort)

	cmd := exec.Command("kubectl", "--context", kubecontext, "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(manifest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to apply ingress: %w", err)
	}

	p.log("[customApps] ✓ %s available at: http://%s\n", app.Name, ing.Host)
	return nil
}
