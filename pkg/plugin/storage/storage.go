package storage

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
	localPathProvisionerURL = "https://raw.githubusercontent.com/rancher/local-path-provisioner/v0.0.30/deploy/local-path-storage.yaml"
)

// Plugin implements the plugin.Plugin interface for storage.
type Plugin struct {
	Log     *logger.Logger
	Timeout time.Duration
}

// New creates a new storage plugin.
func New(log *logger.Logger, timeout time.Duration) *Plugin {
	return &Plugin{Log: log, Timeout: timeout}
}

// Name returns the plugin name.
func (p *Plugin) Name() string {
	return "storage"
}

// Install installs the storage plugin.
func (p *Plugin) Install(cfg interface{}, kubecontext string, providerType string) error {
	storageCfg, ok := cfg.(*template.StorageTemplate)
	if !ok {
		return fmt.Errorf("invalid config type for storage plugin: expected *template.StorageTemplate")
	}

	switch storageCfg.Type {
	case "local-path":
		return p.installLocalPath(kubecontext)
	default:
		return fmt.Errorf("unsupported storage type: %s (supported: local-path)", storageCfg.Type)
	}
}

// Uninstall removes the storage plugin.
func (p *Plugin) Uninstall(cfg interface{}, kubecontext string) error {
	storageCfg, ok := cfg.(*template.StorageTemplate)
	if !ok {
		return fmt.Errorf("invalid config type for storage plugin")
	}

	switch storageCfg.Type {
	case "local-path":
		return p.uninstallLocalPath(kubecontext)
	default:
		return fmt.Errorf("unsupported storage type: %s", storageCfg.Type)
	}
}

// IsInstalled checks if the storage plugin is installed.
func (p *Plugin) IsInstalled(kubecontext string) (bool, error) {
	cmd := execCommand("kubectl", "--context", kubecontext,
		"get", "deployment", "local-path-provisioner", "-n", "local-path-storage")
	if err := cmd.Run(); err != nil {
		return false, nil
	}
	return true, nil
}

// Upgrade re-applies the storage configuration (idempotent).
func (p *Plugin) Upgrade(cfg interface{}, kubecontext string, providerType string) error {
	// For storage, upgrade is the same as install (idempotent)
	return p.Install(cfg, kubecontext, providerType)
}

// DryRun shows what would be installed.
func (p *Plugin) DryRun(cfg interface{}, kubecontext string, providerType string) error {
	storageCfg, ok := cfg.(*template.StorageTemplate)
	if !ok {
		return fmt.Errorf("invalid config type for storage plugin")
	}

	fmt.Printf("[storage] Would install: %s\n", storageCfg.Type)
	
	installed, err := p.IsInstalled(kubecontext)
	if err != nil {
		return err
	}
	if installed {
		fmt.Println("  Status: already installed (would skip)")
	} else {
		fmt.Println("  Status: not installed")
		fmt.Printf("  Manifest: %s\n", localPathProvisionerURL)
	}
	return nil
}

func (p *Plugin) installLocalPath(kubecontext string) error {
	p.Log.Info("Installing local-path-provisioner...\n")

	// Apply manifest with retry
	err := retry.Run(3, 5*time.Second, p.Log.Warn, func() error {
		cmd := execCommand("kubectl", "--context", kubecontext,
			"apply", "-f", localPathProvisionerURL)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	})
	if err != nil {
		return fmt.Errorf("failed to apply local-path-provisioner manifest: %w", err)
	}

	// Wait for deployment
	p.Log.Info("Waiting for local-path-provisioner to be ready...\n")
	waitCmd := execCommand("kubectl", "--context", kubecontext,
		"rollout", "status", "deployment/local-path-provisioner",
		"-n", "local-path-storage", "--timeout", p.Timeout.String())
	waitCmd.Stdout = os.Stdout
	waitCmd.Stderr = os.Stderr
	if err := waitCmd.Run(); err != nil {
		return fmt.Errorf("local-path-provisioner not ready: %w", err)
	}

	// Set as default StorageClass
	p.Log.Debug("Setting local-path as default StorageClass...\n")
	patchCmd := execCommand("kubectl", "--context", kubecontext,
		"patch", "storageclass", "local-path",
		"-p", `{"metadata":{"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}`)
	patchCmd.Stdout = os.Stdout
	patchCmd.Stderr = os.Stderr
	if err := patchCmd.Run(); err != nil {
		// Non-fatal: the storageclass might already be default or have a different name
		p.Log.Warn("Warning: could not set local-path as default StorageClass: %v\n", err)
	}

	// Unset kind's default standard StorageClass if present
	unsetCmd := execCommand("kubectl", "--context", kubecontext,
		"patch", "storageclass", "standard",
		"-p", `{"metadata":{"annotations":{"storageclass.kubernetes.io/is-default-class":"false"}}}`)
	if err := unsetCmd.Run(); err != nil {
		// Ignore: 'standard' storageclass might not exist
		_ = err
	}

	p.Log.Success("local-path-provisioner installed successfully\n")
	p.Log.Info("Default StorageClass: local-path\n")
	return nil
}

func (p *Plugin) uninstallLocalPath(kubecontext string) error {
	p.Log.Info("Uninstalling local-path-provisioner...\n")

	cmd := execCommand("kubectl", "--context", kubecontext,
		"delete", "-f", localPathProvisionerURL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("failed to delete local-path-provisioner: %w", err)
		}
	}

	p.Log.Success("local-path-provisioner uninstalled\n")
	return nil
}
