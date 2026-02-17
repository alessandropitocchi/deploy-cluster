package storage

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/alepito/deploy-cluster/pkg/config"
)

const (
	localPathProvisionerURL = "https://raw.githubusercontent.com/rancher/local-path-provisioner/v0.0.30/deploy/local-path-storage.yaml"
)

type Plugin struct {
	Verbose bool
}

func New() *Plugin {
	return &Plugin{Verbose: true}
}

func (p *Plugin) Name() string {
	return "storage"
}

func (p *Plugin) log(format string, args ...any) {
	if p.Verbose {
		fmt.Printf(format, args...)
	}
}

func (p *Plugin) Install(cfg *config.StorageConfig, kubecontext string) error {
	switch cfg.Type {
	case "local-path":
		return p.installLocalPath(kubecontext)
	default:
		return fmt.Errorf("unsupported storage type: %s (supported: local-path)", cfg.Type)
	}
}

func (p *Plugin) Uninstall(cfg *config.StorageConfig, kubecontext string) error {
	switch cfg.Type {
	case "local-path":
		return p.uninstallLocalPath(kubecontext)
	default:
		return fmt.Errorf("unsupported storage type: %s", cfg.Type)
	}
}

func (p *Plugin) IsInstalled(kubecontext string) (bool, error) {
	cmd := exec.Command("kubectl", "--context", kubecontext,
		"get", "deployment", "local-path-provisioner", "-n", "local-path-storage")
	if err := cmd.Run(); err != nil {
		return false, nil
	}
	return true, nil
}

func (p *Plugin) installLocalPath(kubecontext string) error {
	p.log("[storage] Installing local-path-provisioner...\n")

	// Apply manifest
	cmd := exec.Command("kubectl", "--context", kubecontext,
		"apply", "-f", localPathProvisionerURL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to apply local-path-provisioner manifest: %w", err)
	}

	// Wait for deployment
	p.log("[storage] Waiting for local-path-provisioner to be ready...\n")
	waitCmd := exec.Command("kubectl", "--context", kubecontext,
		"rollout", "status", "deployment/local-path-provisioner",
		"-n", "local-path-storage", "--timeout", (2 * time.Minute).String())
	waitCmd.Stdout = os.Stdout
	waitCmd.Stderr = os.Stderr
	if err := waitCmd.Run(); err != nil {
		return fmt.Errorf("local-path-provisioner not ready: %w", err)
	}

	// Set as default StorageClass
	p.log("[storage] Setting local-path as default StorageClass...\n")
	patchCmd := exec.Command("kubectl", "--context", kubecontext,
		"patch", "storageclass", "local-path",
		"-p", `{"metadata":{"annotations":{"storageclass.kubernetes.io/is-default-class":"true"}}}`)
	patchCmd.Stdout = os.Stdout
	patchCmd.Stderr = os.Stderr
	if err := patchCmd.Run(); err != nil {
		// Non-fatal: the storageclass might already be default or have a different name
		p.log("[storage] Warning: could not set local-path as default StorageClass: %v\n", err)
	}

	// Unset kind's default standard StorageClass if present
	unsetCmd := exec.Command("kubectl", "--context", kubecontext,
		"patch", "storageclass", "standard",
		"-p", `{"metadata":{"annotations":{"storageclass.kubernetes.io/is-default-class":"false"}}}`)
	if err := unsetCmd.Run(); err != nil {
		// Ignore: 'standard' storageclass might not exist
		_ = err
	}

	p.log("[storage] ✓ local-path-provisioner installed successfully\n")
	p.log("[storage] Default StorageClass: local-path\n")
	return nil
}

func (p *Plugin) uninstallLocalPath(kubecontext string) error {
	p.log("[storage] Uninstalling local-path-provisioner...\n")

	cmd := exec.Command("kubectl", "--context", kubecontext,
		"delete", "-f", localPathProvisionerURL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		if !strings.Contains(err.Error(), "not found") {
			return fmt.Errorf("failed to delete local-path-provisioner: %w", err)
		}
	}

	p.log("[storage] ✓ local-path-provisioner uninstalled\n")
	return nil
}
