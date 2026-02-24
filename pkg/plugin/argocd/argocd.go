package argocd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	texttemplate "text/template"
	"time"

	"github.com/alepito/deploy-cluster/pkg/k8s"
	"github.com/alepito/deploy-cluster/pkg/logger"
	"github.com/alepito/deploy-cluster/pkg/retry"
	"github.com/alepito/deploy-cluster/pkg/template"
	"gopkg.in/yaml.v3"
)

// execCommand is a package-level variable for creating exec.Cmd, replaceable in tests.
var execCommand = exec.Command

const repoSecretTemplate = `apiVersion: v1
kind: Secret
metadata:
  name: repo-{{ .Name }}
  namespace: {{ .Namespace }}
  labels:
    argocd.argoproj.io/secret-type: repository
stringData:
  type: {{ .Type }}
  url: {{ .URL }}
{{- if .Username }}
  username: {{ .Username }}
{{- end }}
{{- if .Password }}
  password: {{ .Password }}
{{- end }}
{{- if .Insecure }}
  insecure: "true"
{{- end }}
{{- if .SSHPrivateKey }}
  sshPrivateKey: |
{{ .SSHPrivateKey | indent 4 }}
{{- end }}
`

// Plugin implements the plugin.Plugin interface for ArgoCD.
type Plugin struct {
	Log     *logger.Logger
	Timeout time.Duration
}

// New creates a new ArgoCD plugin.
func New(log *logger.Logger, timeout time.Duration) *Plugin {
	return &Plugin{Log: log, Timeout: timeout}
}

// Name returns the plugin name.
func (p *Plugin) Name() string {
	return "argocd"
}

// Install installs the ArgoCD plugin.
func (p *Plugin) Install(cfg interface{}, kubecontext string, providerType string) error {
	argoCfg, ok := cfg.(*template.ArgoCDTemplate)
	if !ok {
		return fmt.Errorf("invalid config type for argocd plugin: expected *template.ArgoCDTemplate")
	}

	// Set defaults
	namespace := argoCfg.Namespace
	if namespace == "" {
		namespace = "argocd"
	}

	version := argoCfg.Version
	if version == "" {
		version = "stable"
	}

	p.Log.Info("Installing ArgoCD...\n")
	p.Log.Info("Namespace: %s\n", namespace)
	p.Log.Info("Version: %s\n", version)

	// Create namespace
	p.Log.Debug("Creating namespace '%s'...\n", namespace)
	if err := p.runKubectl(kubecontext, "create", "namespace", namespace); err != nil {
		// Ignore if already exists
		if !strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("failed to create namespace: %w", err)
		}
		p.Log.Debug("Namespace already exists, continuing...\n")
	}

	// Install ArgoCD
	manifestURL := fmt.Sprintf("https://raw.githubusercontent.com/argoproj/argo-cd/%s/manifests/install.yaml", version)
	p.Log.Info("Applying manifest from %s...\n", manifestURL)

	if err := p.runKubectlApply(kubecontext, namespace, manifestURL); err != nil {
		return fmt.Errorf("failed to install ArgoCD: %w", err)
	}

	// Wait for ArgoCD to be ready
	p.Log.Info("Waiting for ArgoCD server to be ready...\n")
	if err := p.waitForDeployment(kubecontext, namespace, "argocd-server", p.Timeout); err != nil {
		return fmt.Errorf("ArgoCD server not ready: %w", err)
	}
	p.Log.Success("ArgoCD server is ready\n")

	// Configure ingress if enabled
	if argoCfg.Ingress != nil && argoCfg.Ingress.Enabled {
		if err := p.configureIngress(argoCfg.Ingress, kubecontext, namespace); err != nil {
			return fmt.Errorf("failed to configure ArgoCD ingress: %w", err)
		}
	}

	// Add repositories
	if len(argoCfg.Repos) > 0 {
		p.Log.Info("Adding repositories...\n")
		for _, repo := range argoCfg.Repos {
			if err := p.addRepository(repo, kubecontext, namespace); err != nil {
				return fmt.Errorf("failed to add repository %s: %w", repo.URL, err)
			}
		}
		p.Log.Success("Repositories added\n")
	}

	// Create applications
	if len(argoCfg.Apps) > 0 {
		p.Log.Info("Creating applications...\n")
		for _, app := range argoCfg.Apps {
			if err := p.createApplication(app, kubecontext, namespace); err != nil {
				return fmt.Errorf("failed to create application %s: %w", app.Name, err)
			}
		}
		p.Log.Success("Applications created\n")
	}

	// Print access info
	p.Log.Success("\nArgoCD installed successfully!\n")
	if argoCfg.Ingress != nil && argoCfg.Ingress.Enabled {
		p.Log.Info("\nArgoCD UI available at: http://%s\n", argoCfg.Ingress.Host)
	} else {
		p.Log.Info("\nTo access ArgoCD UI:\n")
		p.Log.Info("  kubectl port-forward svc/argocd-server -n %s 8080:443\n", namespace)
		p.Log.Info("  Open: https://localhost:8080\n")
	}
	p.Log.Info("\nGet admin password:\n")
	p.Log.Info("  kubectl -n %s get secret argocd-initial-admin-secret -o jsonpath=\"{.data.password}\" | base64 -d\n", namespace)

	if len(argoCfg.Repos) > 0 {
		p.Log.Info("\nConfigured repositories:\n")
		for _, repo := range argoCfg.Repos {
			p.Log.Info("  - %s (%s)\n", repo.URL, repo.Name)
		}
	}

	if len(argoCfg.Apps) > 0 {
		p.Log.Info("\nConfigured applications:\n")
		for _, app := range argoCfg.Apps {
			if app.Chart != "" {
				p.Log.Info("  - %s (chart: %s@%s -> %s)\n", app.Name, app.Chart, app.TargetRevision, app.Namespace)
			} else {
				p.Log.Info("  - %s (path: %s@%s -> %s)\n", app.Name, app.Path, app.TargetRevision, app.Namespace)
			}
		}
	}

	return nil
}

// Uninstall removes the ArgoCD plugin.
func (p *Plugin) Uninstall(cfg interface{}, kubecontext string) error {
	argoCfg, ok := cfg.(*template.ArgoCDTemplate)
	if !ok {
		return fmt.Errorf("invalid config type for argocd plugin")
	}

	namespace := argoCfg.Namespace
	if namespace == "" {
		namespace = "argocd"
	}

	p.Log.Info("Uninstalling ArgoCD from namespace '%s'...\n", namespace)

	if err := p.runKubectl(kubecontext, "delete", "namespace", namespace); err != nil {
		return fmt.Errorf("failed to delete namespace: %w", err)
	}

	p.Log.Success("ArgoCD uninstalled\n")
	return nil
}

// IsInstalled checks if ArgoCD is installed.
func (p *Plugin) IsInstalled(kubecontext string) (bool, error) {
	// Use default namespace for check
	return p.IsInstalledInNamespace(kubecontext, "argocd")
}

// IsInstalledInNamespace checks if ArgoCD is installed in a specific namespace.
func (p *Plugin) IsInstalledInNamespace(kubecontext, namespace string) (bool, error) {
	if namespace == "" {
		namespace = "argocd"
	}

	cmd := execCommand("kubectl", "--context", kubecontext, "get", "deployment", "argocd-server", "-n", namespace)
	if err := cmd.Run(); err != nil {
		return false, nil
	}
	return true, nil
}

// Upgrade upgrades ArgoCD with diff-based repos/apps management.
func (p *Plugin) Upgrade(cfg interface{}, kubecontext string, providerType string) error {
	argoCfg, ok := cfg.(*template.ArgoCDTemplate)
	if !ok {
		return fmt.Errorf("invalid config type for argocd plugin")
	}

	namespace := argoCfg.Namespace
	if namespace == "" {
		namespace = "argocd"
	}

	version := argoCfg.Version
	if version == "" {
		version = "stable"
	}

	p.Log.Info("Upgrading ArgoCD...\n")

	// Re-apply ArgoCD manifest (idempotent, updates version if changed)
	manifestURL := fmt.Sprintf("https://raw.githubusercontent.com/argoproj/argo-cd/%s/manifests/install.yaml", version)
	p.Log.Info("Applying manifest from %s...\n", manifestURL)
	if err := p.runKubectlApply(kubecontext, namespace, manifestURL); err != nil {
		return fmt.Errorf("failed to apply ArgoCD manifest: %w", err)
	}

	p.Log.Info("Waiting for ArgoCD server to be ready...\n")
	if err := p.waitForDeployment(kubecontext, namespace, "argocd-server", p.Timeout); err != nil {
		return fmt.Errorf("ArgoCD server not ready: %w", err)
	}
	p.Log.Success("ArgoCD server is ready\n")

	// Configure ingress if enabled
	if argoCfg.Ingress != nil && argoCfg.Ingress.Enabled {
		if err := p.configureIngress(argoCfg.Ingress, kubecontext, namespace); err != nil {
			return fmt.Errorf("failed to configure ArgoCD ingress: %w", err)
		}
	}

	// --- Repos diff ---
	desiredRepos := make(map[string]template.ArgoCDRepoTemplate)
	for _, repo := range argoCfg.Repos {
		name := p.repoName(repo)
		desiredRepos[name] = repo
	}

	currentRepos, err := p.ListCurrentRepos(kubecontext, namespace)
	if err != nil {
		return fmt.Errorf("failed to list current repos: %w", err)
	}

	// Add/update desired repos (kubectl apply is idempotent)
	for _, repo := range argoCfg.Repos {
		if err := p.addRepository(repo, kubecontext, namespace); err != nil {
			return fmt.Errorf("failed to add/update repository %s: %w", repo.URL, err)
		}
	}
	p.Log.Success("Repositories applied (%d)\n", len(argoCfg.Repos))

	// Remove repos that are no longer in config
	removedRepos := 0
	for _, currentName := range currentRepos {
		if _, desired := desiredRepos[currentName]; !desired {
			p.Log.Info("Removing repository '%s'...\n", currentName)
			if err := p.deleteRepo(currentName, kubecontext, namespace); err != nil {
				return fmt.Errorf("failed to delete repository %s: %w", currentName, err)
			}
			removedRepos++
		}
	}
	if removedRepos > 0 {
		p.Log.Success("Removed %d repository(ies)\n", removedRepos)
	}

	// --- Apps diff ---
	desiredApps := make(map[string]template.ArgoCDAppTemplate)
	for _, app := range argoCfg.Apps {
		desiredApps[app.Name] = app
	}

	currentApps, err := p.ListCurrentApps(kubecontext, namespace)
	if err != nil {
		return fmt.Errorf("failed to list current apps: %w", err)
	}

	// Add/update desired apps (kubectl apply is idempotent)
	for _, app := range argoCfg.Apps {
		if err := p.createApplication(app, kubecontext, namespace); err != nil {
			return fmt.Errorf("failed to add/update application %s: %w", app.Name, err)
		}
	}
	p.Log.Success("Applications applied (%d)\n", len(argoCfg.Apps))

	// Remove apps that are no longer in config
	removedApps := 0
	for _, currentName := range currentApps {
		if _, desired := desiredApps[currentName]; !desired {
			p.Log.Info("Removing application '%s'...\n", currentName)
			if err := p.deleteApp(currentName, kubecontext, namespace); err != nil {
				return fmt.Errorf("failed to delete application %s: %w", currentName, err)
			}
			removedApps++
		}
	}
	if removedApps > 0 {
		p.Log.Success("Removed %d application(s)\n", removedApps)
	}

	p.Log.Success("\nArgoCD upgrade completed\n")
	return nil
}

// DryRun shows what would change without applying anything.
func (p *Plugin) DryRun(cfg interface{}, kubecontext string, providerType string) error {
	argoCfg, ok := cfg.(*template.ArgoCDTemplate)
	if !ok {
		return fmt.Errorf("invalid config type for argocd plugin")
	}

	namespace := argoCfg.Namespace
	if namespace == "" {
		namespace = "argocd"
	}

	version := argoCfg.Version
	if version == "" {
		version = "stable"
	}

	fmt.Printf("[argocd] Dry-run: version %s, namespace %s\n", version, namespace)

	// --- Repos diff ---
	desiredRepos := make(map[string]bool)
	for _, repo := range argoCfg.Repos {
		desiredRepos[p.repoName(repo)] = true
	}

	currentRepos, err := p.ListCurrentRepos(kubecontext, namespace)
	if err != nil {
		return fmt.Errorf("failed to list current repos: %w", err)
	}
	currentRepoSet := make(map[string]bool)
	for _, name := range currentRepos {
		currentRepoSet[name] = true
	}

	fmt.Println("\n  Repositories:")
	changes := false
	for _, repo := range argoCfg.Repos {
		name := p.repoName(repo)
		if currentRepoSet[name] {
			fmt.Printf("    ~ %s (update)\n", name)
		} else {
			fmt.Printf("    + %s (add)\n", name)
		}
		changes = true
	}
	for _, name := range currentRepos {
		if !desiredRepos[name] {
			fmt.Printf("    - %s (remove)\n", name)
			changes = true
		}
	}
	if !changes {
		fmt.Println("    (no changes)")
	}

	// --- Apps diff ---
	desiredApps := make(map[string]bool)
	for _, app := range argoCfg.Apps {
		desiredApps[app.Name] = true
	}

	currentApps, err := p.ListCurrentApps(kubecontext, namespace)
	if err != nil {
		return fmt.Errorf("failed to list current apps: %w", err)
	}
	currentAppSet := make(map[string]bool)
	for _, name := range currentApps {
		currentAppSet[name] = true
	}

	fmt.Println("\n  Applications:")
	changes = false
	for _, app := range argoCfg.Apps {
		if currentAppSet[app.Name] {
			fmt.Printf("    ~ %s (update)\n", app.Name)
		} else {
			fmt.Printf("    + %s (add)\n", app.Name)
		}
		changes = true
	}
	for _, name := range currentApps {
		if !desiredApps[name] {
			fmt.Printf("    - %s (remove)\n", name)
			changes = true
		}
	}
	if !changes {
		fmt.Println("    (no changes)")
	}

	return nil
}

func (p *Plugin) addRepository(repo template.ArgoCDRepoTemplate, kubecontext string, namespace string) error {
	// Set defaults
	name := p.repoName(repo)

	repoType := repo.Type
	if repoType == "" {
		repoType = "git"
	}

	// Get SSH key if configured
	var sshPrivateKey string
	if repo.SSHKeyEnv != "" {
		sshPrivateKey = os.Getenv(repo.SSHKeyEnv)
		if sshPrivateKey == "" {
			return fmt.Errorf("environment variable %s is not set", repo.SSHKeyEnv)
		}
		p.Log.Debug("Adding repository: %s (using SSH key from $%s)\n", repo.URL, repo.SSHKeyEnv)
	} else if repo.SSHKeyFile != "" {
		keyPath := repo.SSHKeyFile
		if strings.HasPrefix(keyPath, "~/") {
			home, _ := os.UserHomeDir()
			keyPath = home + keyPath[1:]
		}
		keyData, err := os.ReadFile(keyPath)
		if err != nil {
			return fmt.Errorf("failed to read SSH key file %s: %w", repo.SSHKeyFile, err)
		}
		sshPrivateKey = string(keyData)
		p.Log.Debug("Adding repository: %s (using SSH key from %s)\n", repo.URL, repo.SSHKeyFile)
	} else {
		p.Log.Debug("Adding repository: %s\n", repo.URL)
	}

	// Template functions
	funcMap := texttemplate.FuncMap{
		"indent": func(spaces int, s string) string {
			pad := strings.Repeat(" ", spaces)
			lines := strings.Split(s, "\n")
			for i, line := range lines {
				if line != "" {
					lines[i] = pad + line
				}
			}
			return strings.Join(lines, "\n")
		},
	}

	// Generate Secret manifest
	tmpl, err := texttemplate.New("repo").Funcs(funcMap).Parse(repoSecretTemplate)
	if err != nil {
		return err
	}

	// Determine insecure flag: explicit config or auto-detect for non-HTTPS
	insecure := false
	if repo.Insecure != nil {
		insecure = *repo.Insecure
	} else if !strings.HasPrefix(repo.URL, "https://") {
		insecure = true
	}
	if insecure {
		p.Log.Debug("Using insecure mode (skip TLS verification)\n")
	}

	data := struct {
		Name          string
		Namespace     string
		Type          string
		URL           string
		Insecure      bool
		Username      string
		Password      string
		SSHPrivateKey string
	}{
		Name:          name,
		Namespace:     namespace,
		Type:          repoType,
		URL:           repo.URL,
		Insecure:      insecure,
		Username:      repo.Username,
		Password:      repo.Password,
		SSHPrivateKey: sshPrivateKey,
	}

	var manifest bytes.Buffer
	if err := tmpl.Execute(&manifest, data); err != nil {
		return err
	}

	// Apply manifest with retry
	manifestStr := manifest.String()
	return retry.Run(3, 5*time.Second, p.Log.Warn, func() error {
		cmd := execCommand("kubectl", "--context", kubecontext, "apply", "-f", "-")
		cmd.Stdin = strings.NewReader(manifestStr)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	})
}

func (p *Plugin) createApplication(app template.ArgoCDAppTemplate, kubecontext string, argoNamespace string) error {
	// Set defaults
	project := app.Project
	if project == "" {
		project = "default"
	}
	destNamespace := app.Namespace
	if destNamespace == "" {
		destNamespace = "default"
	}
	targetRevision := app.TargetRevision
	if targetRevision == "" {
		targetRevision = "HEAD"
	}
	autoSync := true
	if app.AutoSync != nil {
		autoSync = *app.AutoSync
	}

	// Build source section
	var sourceYAML string
	if app.Chart != "" {
		// Helm chart source
		sourceYAML = fmt.Sprintf("    repoURL: %s\n    chart: %s\n    targetRevision: %s\n", app.RepoURL, app.Chart, targetRevision)
	} else {
		// Git repo source
		path := app.Path
		if path == "" {
			path = "."
		}
		sourceYAML = fmt.Sprintf("    repoURL: %s\n    targetRevision: %s\n    path: %s\n", app.RepoURL, targetRevision, path)
	}

	// Build helm values
	var valuesYAML string
	if len(app.Values) > 0 {
		valuesBytes, err := yaml.Marshal(app.Values)
		if err != nil {
			return fmt.Errorf("failed to marshal values: %w", err)
		}
		valuesYAML = string(valuesBytes)
	} else if app.ValuesFile != "" {
		filePath := app.ValuesFile
		if strings.HasPrefix(filePath, "~/") {
			home, _ := os.UserHomeDir()
			filePath = home + filePath[1:]
		}
		data, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("failed to read values file %s: %w", app.ValuesFile, err)
		}
		valuesYAML = string(data)
	}

	// Build helm section if values exist
	helmSection := ""
	if valuesYAML != "" {
		// Indent values for the manifest
		indentedValues := ""
		for _, line := range strings.Split(strings.TrimSpace(valuesYAML), "\n") {
			indentedValues += "          " + line + "\n"
		}
		helmSection = fmt.Sprintf("    helm:\n      values: |\n%s", indentedValues)
	}

	// Build sync policy
	syncPolicy := ""
	if autoSync {
		syncPolicy = `  syncPolicy:
    automated:
      prune: true
      selfHeal: true
    syncOptions:
      - CreateNamespace=true
`
	}

	// Build full manifest
	manifest := fmt.Sprintf(`apiVersion: argoproj.io/v1alpha1
kind: Application
metadata:
  name: %s
  namespace: %s
spec:
  project: %s
  source:
%s%s  destination:
    server: https://kubernetes.default.svc
    namespace: %s
%s`, app.Name, argoNamespace, project, sourceYAML, helmSection, destNamespace, syncPolicy)

	p.Log.Debug("Creating application '%s'...\n", app.Name)
	p.Log.Debug("Application manifest:\n---\n%s---\n", manifest)

	// Apply manifest with retry
	return retry.Run(3, 5*time.Second, p.Log.Warn, func() error {
		cmd := execCommand("kubectl", "--context", kubecontext, "apply", "-f", "-")
		cmd.Stdin = strings.NewReader(manifest)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	})
}

// repoName returns the secret name for a repo config (without the "repo-" prefix).
func (p *Plugin) repoName(repo template.ArgoCDRepoTemplate) string {
	name := repo.Name
	if name == "" {
		name = strings.ReplaceAll(repo.URL, "https://", "")
		name = strings.ReplaceAll(name, "http://", "")
		name = strings.ReplaceAll(name, "git@", "")
		name = strings.ReplaceAll(name, ":", "-")
		name = strings.ReplaceAll(name, "/", "-")
		name = strings.ReplaceAll(name, ".", "-")
		name = strings.TrimSuffix(name, "-git")
	}
	return name
}

// ListCurrentRepos returns the names of repo secrets (without the "repo-" prefix)
// that have the ArgoCD repository label.
func (p *Plugin) ListCurrentRepos(kubecontext, namespace string) ([]string, error) {
	cmd := execCommand("kubectl", "--context", kubecontext,
		"get", "secrets", "-n", namespace,
		"-l", "argocd.argoproj.io/secret-type=repository",
		"-o", "jsonpath={.items[*].metadata.name}")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	raw := strings.TrimSpace(string(output))
	if raw == "" {
		return nil, nil
	}

	var names []string
	for _, fullName := range strings.Fields(raw) {
		// Secret names are "repo-<name>", strip the prefix to match config names
		name := strings.TrimPrefix(fullName, "repo-")
		names = append(names, name)
	}
	return names, nil
}

// ListCurrentApps returns the names of ArgoCD Application resources in the namespace.
func (p *Plugin) ListCurrentApps(kubecontext, namespace string) ([]string, error) {
	cmd := execCommand("kubectl", "--context", kubecontext,
		"get", "applications.argoproj.io", "-n", namespace,
		"-o", "jsonpath={.items[*].metadata.name}")
	output, err := cmd.Output()
	if err != nil {
		// If the CRD doesn't exist yet, treat as empty
		if strings.Contains(err.Error(), "the server doesn't have a resource type") {
			return nil, nil
		}
		return nil, err
	}

	raw := strings.TrimSpace(string(output))
	if raw == "" {
		return nil, nil
	}

	return strings.Fields(raw), nil
}

// deleteRepo deletes a repository secret by name.
func (p *Plugin) deleteRepo(name, kubecontext, namespace string) error {
	secretName := "repo-" + name
	return p.runKubectl(kubecontext, "delete", "secret", secretName, "-n", namespace)
}

// deleteApp deletes an ArgoCD Application by name.
func (p *Plugin) deleteApp(name, kubecontext, namespace string) error {
	return p.runKubectl(kubecontext, "delete", "application.argoproj.io", name, "-n", namespace)
}

func (p *Plugin) configureIngress(cfg *template.ArgoCDIngressTemplate, kubecontext string, namespace string) error {
	p.Log.Info("Configuring ingress for ArgoCD UI...\n")

	// Set server.insecure=true in argocd-cmd-params-cm ConfigMap
	// This disables internal TLS so the ingress controller can proxy HTTP to the backend
	p.Log.Debug("Configuring argocd-server to disable internal TLS...\n")
	cmManifest := fmt.Sprintf(`apiVersion: v1
kind: ConfigMap
metadata:
  name: argocd-cmd-params-cm
  namespace: %s
data:
  server.insecure: "true"`, namespace)

	err := retry.Run(3, 5*time.Second, p.Log.Warn, func() error {
		cmd := execCommand("kubectl", "--context", kubecontext, "apply", "-f", "-")
		cmd.Stdin = strings.NewReader(cmManifest)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	})
	if err != nil {
		return fmt.Errorf("failed to configure argocd-cmd-params-cm: %w", err)
	}

	// Restart argocd-server to pick up the ConfigMap change
	p.Log.Debug("Restarting argocd-server...\n")
	if err := p.runKubectl(kubecontext, "rollout", "restart", "deployment/argocd-server", "-n", namespace); err != nil {
		return fmt.Errorf("failed to restart argocd-server: %w", err)
	}

	if err := p.waitForDeployment(kubecontext, namespace, "argocd-server", p.Timeout); err != nil {
		return fmt.Errorf("argocd-server not ready after restart: %w", err)
	}

	// Build Ingress manifest
	ingressCfg := k8s.IngressConfig{
		Name:        "argocd-server-ingress",
		Namespace:   namespace,
		Host:        cfg.Host,
		ServiceName: "argocd-server",
		ServicePort: 80,
		TLS:         cfg.TLS,
	}
	if cfg.TLS {
		ingressCfg.TLSSecret = "argocd-tls"
		ingressCfg.Annotations = map[string]string{
			"cert-manager.io/cluster-issuer": `"letsencrypt-prod"`,
		}
	}
	manifest := k8s.IngressManifest(ingressCfg)

	p.Log.Debug("Applying Ingress resource for host '%s'...\n", cfg.Host)
	err = retry.Run(3, 5*time.Second, p.Log.Warn, func() error {
		ingressCmd := execCommand("kubectl", "--context", kubecontext, "apply", "-f", "-")
		ingressCmd.Stdin = strings.NewReader(manifest)
		ingressCmd.Stdout = os.Stdout
		ingressCmd.Stderr = os.Stderr
		return ingressCmd.Run()
	})
	if err != nil {
		return fmt.Errorf("failed to apply ingress: %w", err)
	}

	p.Log.Success("Ingress configured: http://%s\n", cfg.Host)
	if cfg.TLS {
		p.Log.Info("  HTTPS: https://%s (TLS via cert-manager)\n", cfg.Host)
	}

	return nil
}

func (p *Plugin) runKubectl(kubecontext string, args ...string) error {
	fullArgs := append([]string{"--context", kubecontext}, args...)
	return retry.Run(3, 5*time.Second, p.Log.Warn, func() error {
		cmd := execCommand("kubectl", fullArgs...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	})
}

func (p *Plugin) runKubectlApply(kubecontext string, namespace string, url string) error {
	return retry.Run(3, 5*time.Second, p.Log.Warn, func() error {
		cmd := execCommand("kubectl", "--context", kubecontext, "apply", "-n", namespace, "-f", url, "--server-side", "--force-conflicts")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	})
}

func (p *Plugin) waitForDeployment(kubecontext string, namespace string, name string, timeout time.Duration) error {
	cmd := execCommand("kubectl", "--context", kubecontext, "rollout", "status", "deployment/"+name, "-n", namespace, "--timeout", timeout.String())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
