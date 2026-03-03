package ingress

import (
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/alessandropitocchi/deploy-cluster/pkg/logger"
	"github.com/alessandropitocchi/deploy-cluster/pkg/template"
)

type capturedCmd struct {
	name string
	args []string
}

func setupFakeExec(t *testing.T) *[]capturedCmd {
	t.Helper()
	var cmds []capturedCmd
	orig := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		cmds = append(cmds, capturedCmd{name, args})
		return exec.Command("true")
	}
	t.Cleanup(func() { execCommand = orig })
	return &cmds
}

func quietLogger() *logger.Logger {
	return logger.New("[ingress]", logger.LevelQuiet)
}

func TestInstallTraefik_Commands(t *testing.T) {
	cmds := setupFakeExec(t)
	p := New(quietLogger(), 5*time.Minute)
	cfg := &template.IngressTemplate{Enabled: true, Type: "traefik"}

	// This will fail because fake exec returns success but we need to check commands
	_ = p.Install(cfg, "kind-test", "kind")

	// Should have helm repo add, helm repo update, kubectl create namespace, helm upgrade
	foundHelm := false
	for _, cmd := range *cmds {
		if cmd.name == "helm" {
			foundHelm = true
			break
		}
	}
	if !foundHelm {
		t.Error("expected helm commands for traefik installation")
	}
}

func TestInstallNginxGatewayFabric_Commands(t *testing.T) {
	cmds := setupFakeExec(t)
	p := New(quietLogger(), 5*time.Minute)
	cfg := &template.IngressTemplate{Enabled: true, Type: "nginx-gateway-fabric"}

	// This will fail because fake exec returns success but we need to check commands
	_ = p.Install(cfg, "kind-test", "kind")

	// Should have sh for kustomize CRDs and helm for installation
	foundSh := false
	foundHelm := false
	for _, cmd := range *cmds {
		if cmd.name == "sh" {
			foundSh = true
		}
		if cmd.name == "helm" {
			foundHelm = true
		}
	}
	if !foundSh {
		t.Error("expected sh command for kustomize CRDs")
	}
	if !foundHelm {
		t.Error("expected helm command for nginx-gateway-fabric installation")
	}
}

func TestIsInstalled_Commands(t *testing.T) {
	cmds := setupFakeExec(t)
	p := New(quietLogger(), 5*time.Minute)

	installed, err := p.IsInstalled("kind-check")
	if err != nil {
		t.Fatalf("IsInstalled() error = %v", err)
	}
	if !installed {
		t.Error("should return true when command succeeds")
	}
	if len(*cmds) < 1 {
		t.Fatalf("expected at least 1 command, got %d", len(*cmds))
	}
	// Should check for traefik first
	assertContains(t, (*cmds)[0].args, "traefik", "should check traefik deployment")
}

// Helpers

func assertContains(t *testing.T, args []string, want string, msg string) {
	t.Helper()
	if !containsArg(args, want) {
		t.Errorf("%s: args %v do not contain %q", msg, args, want)
	}
}

func containsArg(args []string, s string) bool {
	for _, a := range args {
		if strings.Contains(a, s) {
			return true
		}
	}
	return false
}
