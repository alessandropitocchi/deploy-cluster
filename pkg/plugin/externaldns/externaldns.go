// Package externaldns provides the External DNS plugin for automatic DNS management.
package externaldns

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/alepito/deploy-cluster/pkg/logger"
	"github.com/alepito/deploy-cluster/pkg/retry"
	"github.com/alepito/deploy-cluster/pkg/template"
)

// execCommand is a package-level variable for creating exec.Cmd, replaceable in tests.
var execCommand = exec.Command

const (
	defaultVersion   = "1.15.0"
	defaultNamespace = "external-dns"
	helmRepo         = "https://kubernetes-sigs.github.io/external-dns/"
	chartName        = "external-dns/external-dns"
)

// Plugin implements the plugin.Plugin interface for External DNS.
type Plugin struct {
	Log     *logger.Logger
	Timeout time.Duration
}

// New creates a new External DNS plugin.
func New(log *logger.Logger, timeout time.Duration) *Plugin {
	return &Plugin{Log: log, Timeout: timeout}
}

// Name returns the plugin name.
func (p *Plugin) Name() string {
	return "external-dns"
}

// Install installs the External DNS plugin.
func (p *Plugin) Install(cfg interface{}, kubecontext string, providerType string) error {
	dnsCfg, ok := cfg.(*template.ExternalDNSTemplate)
	if !ok {
		return fmt.Errorf("invalid config type for external-dns plugin: expected *template.ExternalDNSTemplate")
	}

	version := dnsCfg.Version
	if version == "" {
		version = defaultVersion
	}

	p.Log.Info("Installing External DNS %s (provider: %s)...\n", version, dnsCfg.Provider)

	// Add Helm repo
	p.Log.Debug("Adding External DNS Helm repository...\n")
	if err := p.addHelmRepo(); err != nil {
		return fmt.Errorf("failed to add Helm repository: %w", err)
	}

	// Build Helm values
	values, err := p.buildValues(dnsCfg)
	if err != nil {
		return fmt.Errorf("failed to build Helm values: %w", err)
	}

	// Install via Helm
	args := []string{
		"upgrade", "--install", "external-dns", chartName,
		"--version", version,
		"--namespace", defaultNamespace,
		"--create-namespace",
		"--kube-context", kubecontext,
		"--wait",
		"--timeout", p.Timeout.String(),
	}

	if values != "" {
		args = append(args, "--values", "-")
	}

	err = retry.Run(3, 5*time.Second, p.Log.Warn, func() error {
		cmd := execCommand("helm", args...)
		if values != "" {
			cmd.Stdin = strings.NewReader(values)
		}
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	})
	if err != nil {
		return fmt.Errorf("failed to install External DNS: %w", err)
	}

	p.Log.Success("External DNS %s installed successfully\n", version)
	p.Log.Info("Provider: %s\n", dnsCfg.Provider)
	if dnsCfg.Zone != "" {
		p.Log.Info("Zone: %s\n", dnsCfg.Zone)
	}
	return nil
}

// Uninstall removes the External DNS plugin.
func (p *Plugin) Uninstall(cfg interface{}, kubecontext string) error {
	p.Log.Info("Uninstalling External DNS...\n")

	cmd := execCommand("helm", "uninstall", "external-dns",
		"--namespace", defaultNamespace,
		"--kube-context", kubecontext)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to uninstall External DNS: %w", err)
	}

	p.Log.Success("External DNS uninstalled\n")
	return nil
}

// IsInstalled checks if External DNS is installed.
func (p *Plugin) IsInstalled(kubecontext string) (bool, error) {
	cmd := execCommand("helm", "status", "external-dns",
		"--namespace", defaultNamespace, "--kube-context", kubecontext)
	if err := cmd.Run(); err != nil {
		return false, nil
	}
	return true, nil
}

// Upgrade re-applies the External DNS configuration (idempotent).
func (p *Plugin) Upgrade(cfg interface{}, kubecontext string, providerType string) error {
	// For External DNS, upgrade is the same as install (idempotent)
	return p.Install(cfg, kubecontext, providerType)
}

// DryRun shows what would be installed.
func (p *Plugin) DryRun(cfg interface{}, kubecontext string, providerType string) error {
	dnsCfg, ok := cfg.(*template.ExternalDNSTemplate)
	if !ok {
		return fmt.Errorf("invalid config type for external-dns plugin")
	}

	version := dnsCfg.Version
	if version == "" {
		version = defaultVersion
	}

	fmt.Printf("[external-dns] Would install version: %s\n", version)

	installed, err := p.IsInstalled(kubecontext)
	if err != nil {
		return err
	}
	if installed {
		fmt.Println("  Status: already installed (would upgrade)")
	} else {
		fmt.Println("  Status: not installed")
	}
	fmt.Printf("  Provider: %s\n", dnsCfg.Provider)
	fmt.Printf("  Chart: %s\n", chartName)
	fmt.Printf("  Namespace: %s\n", defaultNamespace)
	if dnsCfg.Zone != "" {
		fmt.Printf("  Zone: %s\n", dnsCfg.Zone)
	}
	return nil
}

func (p *Plugin) addHelmRepo() error {
	cmd := execCommand("helm", "repo", "add", "external-dns", helmRepo)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		// Ignore error if repo already exists
		return nil
	}

	cmd = execCommand("helm", "repo", "update")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (p *Plugin) buildValues(cfg *template.ExternalDNSTemplate) (string, error) {
	var values []string

	// Provider configuration
	switch cfg.Provider {
	case "cloudflare":
		values = append(values, p.cloudflareValues(cfg.Credentials)...)
	case "route53":
		values = append(values, p.route53Values(cfg.Credentials)...)
	case "google":
		values = append(values, p.googleValues(cfg.Credentials)...)
	case "azure":
		values = append(values, p.azureValues(cfg.Credentials)...)
	case "digitalocean":
		values = append(values, p.digitalOceanValues(cfg.Credentials)...)
	default:
		return "", fmt.Errorf("unsupported provider: %s", cfg.Provider)
	}

	// Source configuration
	source := cfg.Source
	if source == "" {
		source = "ingress" // default
	}
	switch source {
	case "ingress":
		values = append(values, "sources:\n  - ingress")
	case "service":
		values = append(values, "sources:\n  - service")
	case "both":
		values = append(values, "sources:\n  - ingress\n  - service")
	}

	// Policy: sync for actual DNS management
	values = append(values, "policy: sync")

	// Registry: txt for ownership tracking
	values = append(values, "registry: txt")
	if cfg.Zone != "" {
		values = append(values, fmt.Sprintf("txtOwnerId: \"%s\"", cfg.Zone))
	}

	// Domain filter
	if cfg.Zone != "" {
		values = append(values, fmt.Sprintf("domainFilters:\n  - \"%s\"", cfg.Zone))
	}

	return strings.Join(values, "\n"), nil
}

func (p *Plugin) cloudflareValues(creds map[string]string) []string {
	var values []string
	values = append(values, "provider:\n  name: cloudflare")

	// Cloudflare credentials
	apiToken := creds["apiToken"]
	if apiToken == "" {
		apiToken = os.Getenv("CF_API_TOKEN")
	}
	if apiToken != "" {
		values = append(values, fmt.Sprintf("env:\n  - name: CF_API_TOKEN\n    value: \"%s\"", apiToken))
	}

	return values
}

func (p *Plugin) route53Values(creds map[string]string) []string {
	var values []string
	values = append(values, "provider:\n  name: aws")

	// AWS credentials can be provided via env vars or IRSA
	region := creds["region"]
	if region == "" {
		region = os.Getenv("AWS_REGION")
	}
	if region == "" {
		region = "us-east-1"
	}

	var envVars []string
	envVars = append(envVars, fmt.Sprintf("  - name: AWS_REGION\n    value: \"%s\"", region))

	if accessKey := creds["accessKey"]; accessKey != "" {
		envVars = append(envVars, fmt.Sprintf("  - name: AWS_ACCESS_KEY_ID\n    value: \"%s\"", accessKey))
	}
	if secretKey := creds["secretKey"]; secretKey != "" {
		envVars = append(envVars, fmt.Sprintf("  - name: AWS_SECRET_ACCESS_KEY\n    value: \"%s\"", secretKey))
	}

	if len(envVars) > 0 {
		values = append(values, "env:\n"+strings.Join(envVars, "\n"))
	}

	return values
}

func (p *Plugin) googleValues(creds map[string]string) []string {
	var values []string
	values = append(values, "provider:\n  name: google")

	// Google Cloud credentials
	project := creds["project"]
	if project == "" {
		project = os.Getenv("GOOGLE_PROJECT")
	}
	if project != "" {
		values = append(values, fmt.Sprintf("extraArgs:\n  google-project: \"%s\"", project))
	}

	return values
}

func (p *Plugin) azureValues(creds map[string]string) []string {
	var values []string
	values = append(values, "provider:\n  name: azure")

	// Azure configuration via secret
	// In production, users should create the azure-config-file secret manually
	values = append(values, "extraVolumes:\n  - name: azure-config\n    secret:\n      secretName: azure-config-file")
	values = append(values, "extraVolumeMounts:\n  - name: azure-config\n    mountPath: /etc/kubernetes")

	return values
}

func (p *Plugin) digitalOceanValues(creds map[string]string) []string {
	var values []string
	values = append(values, "provider:\n  name: digitalocean")

	// DigitalOcean token
	token := creds["token"]
	if token == "" {
		token = os.Getenv("DO_TOKEN")
	}
	if token != "" {
		values = append(values, fmt.Sprintf("env:\n  - name: DO_TOKEN\n    value: \"%s\"", token))
	}

	return values
}
