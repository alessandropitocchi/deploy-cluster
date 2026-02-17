package argocd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"text/template"
	"time"

	"github.com/alepito/deploy-cluster/pkg/config"
	"gopkg.in/yaml.v3"
)

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

type Plugin struct {
	Verbose bool
}

func New() *Plugin {
	return &Plugin{Verbose: true}
}

func (p *Plugin) Name() string {
	return "argocd"
}

func (p *Plugin) log(format string, args ...interface{}) {
	if p.Verbose {
		fmt.Printf(format, args...)
	}
}

func (p *Plugin) Install(cfg *config.ArgoCDConfig, kubecontext string) error {
	// Set defaults
	namespace := cfg.Namespace
	if namespace == "" {
		namespace = "argocd"
	}

	version := cfg.Version
	if version == "" {
		version = "stable"
	}

	p.log("[argocd] Installing ArgoCD...\n")
	p.log("[argocd] Namespace: %s\n", namespace)
	p.log("[argocd] Version: %s\n", version)

	// Create namespace
	p.log("[argocd] Creating namespace '%s'...\n", namespace)
	if err := p.runKubectl(kubecontext, "create", "namespace", namespace); err != nil {
		// Ignore if already exists
		if !strings.Contains(err.Error(), "already exists") {
			return fmt.Errorf("failed to create namespace: %w", err)
		}
		p.log("[argocd] Namespace already exists, continuing...\n")
	}

	// Install ArgoCD
	manifestURL := fmt.Sprintf("https://raw.githubusercontent.com/argoproj/argo-cd/%s/manifests/install.yaml", version)
	p.log("[argocd] Applying manifest from %s...\n", manifestURL)

	if err := p.runKubectlApply(kubecontext, namespace, manifestURL); err != nil {
		return fmt.Errorf("failed to install ArgoCD: %w", err)
	}

	// Wait for ArgoCD to be ready
	p.log("[argocd] Waiting for ArgoCD server to be ready...\n")
	if err := p.waitForDeployment(kubecontext, namespace, "argocd-server", 5*time.Minute); err != nil {
		return fmt.Errorf("ArgoCD server not ready: %w", err)
	}
	p.log("[argocd] ✓ ArgoCD server is ready\n")

	// Add repositories
	if len(cfg.Repos) > 0 {
		p.log("[argocd] Adding repositories...\n")
		for _, repo := range cfg.Repos {
			if err := p.addRepository(repo, kubecontext, namespace); err != nil {
				return fmt.Errorf("failed to add repository %s: %w", repo.URL, err)
			}
		}
		p.log("[argocd] ✓ Repositories added\n")
	}

	// Create applications
	if len(cfg.Apps) > 0 {
		p.log("[argocd] Creating applications...\n")
		for _, app := range cfg.Apps {
			if err := p.createApplication(app, kubecontext, namespace); err != nil {
				return fmt.Errorf("failed to create application %s: %w", app.Name, err)
			}
		}
		p.log("[argocd] ✓ Applications created\n")
	}

	// Print access info
	p.log("\n[argocd] ✓ ArgoCD installed successfully!\n")
	p.log("\n[argocd] To access ArgoCD UI:\n")
	p.log("  kubectl port-forward svc/argocd-server -n %s 8080:443\n", namespace)
	p.log("  Open: https://localhost:8080\n")
	p.log("\n[argocd] Get admin password:\n")
	p.log("  kubectl -n %s get secret argocd-initial-admin-secret -o jsonpath=\"{.data.password}\" | base64 -d\n", namespace)

	if len(cfg.Repos) > 0 {
		p.log("\n[argocd] Configured repositories:\n")
		for _, repo := range cfg.Repos {
			p.log("  • %s (%s)\n", repo.URL, repo.Name)
		}
	}

	if len(cfg.Apps) > 0 {
		p.log("\n[argocd] Configured applications:\n")
		for _, app := range cfg.Apps {
			if app.Chart != "" {
				p.log("  • %s (chart: %s@%s → %s)\n", app.Name, app.Chart, app.TargetRevision, app.Namespace)
			} else {
				p.log("  • %s (path: %s@%s → %s)\n", app.Name, app.Path, app.TargetRevision, app.Namespace)
			}
		}
	}

	return nil
}

func (p *Plugin) addRepository(repo config.ArgoCDRepoConfig, kubecontext string, namespace string) error {
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
		p.log("[argocd] Adding repository: %s (using SSH key from $%s)\n", repo.URL, repo.SSHKeyEnv)
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
		p.log("[argocd] Adding repository: %s (using SSH key from %s)\n", repo.URL, repo.SSHKeyFile)
	} else {
		p.log("[argocd] Adding repository: %s\n", repo.URL)
	}

	// Template functions
	funcMap := template.FuncMap{
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
	tmpl, err := template.New("repo").Funcs(funcMap).Parse(repoSecretTemplate)
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
		p.log("[argocd] Using insecure mode (skip TLS verification)\n")
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

	// Apply manifest
	cmd := exec.Command("kubectl", "--context", kubecontext, "apply", "-f", "-")
	cmd.Stdin = &manifest
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (p *Plugin) createApplication(app config.ArgoCDAppConfig, kubecontext string, argoNamespace string) error {
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

	p.log("[argocd] Creating application '%s'...\n", app.Name)
	p.log("[argocd] Application manifest:\n---\n%s---\n", manifest)

	// Apply manifest
	cmd := exec.Command("kubectl", "--context", kubecontext, "apply", "-f", "-")
	cmd.Stdin = strings.NewReader(manifest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (p *Plugin) Upgrade(cfg *config.ArgoCDConfig, kubecontext string) error {
	namespace := cfg.Namespace
	if namespace == "" {
		namespace = "argocd"
	}

	version := cfg.Version
	if version == "" {
		version = "stable"
	}

	p.log("[argocd] Upgrading ArgoCD...\n")

	// Re-apply ArgoCD manifest (idempotent, updates version if changed)
	manifestURL := fmt.Sprintf("https://raw.githubusercontent.com/argoproj/argo-cd/%s/manifests/install.yaml", version)
	p.log("[argocd] Applying manifest from %s...\n", manifestURL)
	if err := p.runKubectlApply(kubecontext, namespace, manifestURL); err != nil {
		return fmt.Errorf("failed to apply ArgoCD manifest: %w", err)
	}

	p.log("[argocd] Waiting for ArgoCD server to be ready...\n")
	if err := p.waitForDeployment(kubecontext, namespace, "argocd-server", 5*time.Minute); err != nil {
		return fmt.Errorf("ArgoCD server not ready: %w", err)
	}
	p.log("[argocd] ✓ ArgoCD server is ready\n")

	// --- Repos diff ---
	desiredRepos := make(map[string]config.ArgoCDRepoConfig)
	for _, repo := range cfg.Repos {
		name := p.repoName(repo)
		desiredRepos[name] = repo
	}

	currentRepos, err := p.ListCurrentRepos(kubecontext, namespace)
	if err != nil {
		return fmt.Errorf("failed to list current repos: %w", err)
	}

	// Add/update desired repos (kubectl apply is idempotent)
	for _, repo := range cfg.Repos {
		if err := p.addRepository(repo, kubecontext, namespace); err != nil {
			return fmt.Errorf("failed to add/update repository %s: %w", repo.URL, err)
		}
	}
	p.log("[argocd] ✓ Repositories applied (%d)\n", len(cfg.Repos))

	// Remove repos that are no longer in config
	removedRepos := 0
	for _, currentName := range currentRepos {
		if _, desired := desiredRepos[currentName]; !desired {
			p.log("[argocd] Removing repository '%s'...\n", currentName)
			if err := p.deleteRepo(currentName, kubecontext, namespace); err != nil {
				return fmt.Errorf("failed to delete repository %s: %w", currentName, err)
			}
			removedRepos++
		}
	}
	if removedRepos > 0 {
		p.log("[argocd] ✓ Removed %d repository(ies)\n", removedRepos)
	}

	// --- Apps diff ---
	desiredApps := make(map[string]config.ArgoCDAppConfig)
	for _, app := range cfg.Apps {
		desiredApps[app.Name] = app
	}

	currentApps, err := p.ListCurrentApps(kubecontext, namespace)
	if err != nil {
		return fmt.Errorf("failed to list current apps: %w", err)
	}

	// Add/update desired apps (kubectl apply is idempotent)
	for _, app := range cfg.Apps {
		if err := p.createApplication(app, kubecontext, namespace); err != nil {
			return fmt.Errorf("failed to add/update application %s: %w", app.Name, err)
		}
	}
	p.log("[argocd] ✓ Applications applied (%d)\n", len(cfg.Apps))

	// Remove apps that are no longer in config
	removedApps := 0
	for _, currentName := range currentApps {
		if _, desired := desiredApps[currentName]; !desired {
			p.log("[argocd] Removing application '%s'...\n", currentName)
			if err := p.deleteApp(currentName, kubecontext, namespace); err != nil {
				return fmt.Errorf("failed to delete application %s: %w", currentName, err)
			}
			removedApps++
		}
	}
	if removedApps > 0 {
		p.log("[argocd] ✓ Removed %d application(s)\n", removedApps)
	}

	p.log("\n[argocd] ✓ ArgoCD upgrade completed\n")
	return nil
}

// repoName returns the secret name for a repo config (without the "repo-" prefix).
func (p *Plugin) repoName(repo config.ArgoCDRepoConfig) string {
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

// listCurrentRepos returns the names of repo secrets (without the "repo-" prefix)
// that have the ArgoCD repository label.
func (p *Plugin) ListCurrentRepos(kubecontext, namespace string) ([]string, error) {
	cmd := exec.Command("kubectl", "--context", kubecontext,
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

// listCurrentApps returns the names of ArgoCD Application resources in the namespace.
func (p *Plugin) ListCurrentApps(kubecontext, namespace string) ([]string, error) {
	cmd := exec.Command("kubectl", "--context", kubecontext,
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

func (p *Plugin) Uninstall(kubecontext string, namespace string) error {
	if namespace == "" {
		namespace = "argocd"
	}

	p.log("[argocd] Uninstalling ArgoCD from namespace '%s'...\n", namespace)

	if err := p.runKubectl(kubecontext, "delete", "namespace", namespace); err != nil {
		return fmt.Errorf("failed to delete namespace: %w", err)
	}

	p.log("[argocd] ✓ ArgoCD uninstalled\n")
	return nil
}

func (p *Plugin) IsInstalled(kubecontext string, namespace string) (bool, error) {
	if namespace == "" {
		namespace = "argocd"
	}

	cmd := exec.Command("kubectl", "--context", kubecontext, "get", "deployment", "argocd-server", "-n", namespace)
	if err := cmd.Run(); err != nil {
		return false, nil
	}
	return true, nil
}

func (p *Plugin) runKubectl(kubecontext string, args ...string) error {
	fullArgs := append([]string{"--context", kubecontext}, args...)
	cmd := exec.Command("kubectl", fullArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (p *Plugin) runKubectlApply(kubecontext string, namespace string, url string) error {
	cmd := exec.Command("kubectl", "--context", kubecontext, "apply", "-n", namespace, "-f", url, "--server-side", "--force-conflicts")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (p *Plugin) waitForDeployment(kubecontext string, namespace string, name string, timeout time.Duration) error {
	cmd := exec.Command("kubectl", "--context", kubecontext, "rollout", "status", "deployment/"+name, "-n", namespace, "--timeout", timeout.String())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
