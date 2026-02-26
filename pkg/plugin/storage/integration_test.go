package storage

import (
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/alessandropitocchi/deploy-cluster/pkg/template"
	"github.com/alessandropitocchi/deploy-cluster/pkg/logger"
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
	return logger.New("[storage]", logger.LevelQuiet)
}

func TestInstallLocalPath_Commands(t *testing.T) {
	cmds := setupFakeExec(t)
	p := New(quietLogger(), 5*time.Minute)
	cfg := &template.StorageTemplate{Enabled: true, Type: "local-path"}

	err := p.Install(cfg, "kind-test", "kind")
	if err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	if len(*cmds) < 3 {
		t.Fatalf("expected at least 3 commands, got %d", len(*cmds))
	}

	// Command 1: kubectl apply
	apply := (*cmds)[0]
	if apply.name != "kubectl" {
		t.Errorf("cmd[0] name = %q, want kubectl", apply.name)
	}
	assertContains(t, apply.args, "--context", "apply should have --context")
	assertContains(t, apply.args, "kind-test", "apply should have kubecontext value")
	assertContains(t, apply.args, "-f", "apply should have -f")

	// Command 2: kubectl rollout status with timeout
	rollout := (*cmds)[1]
	if rollout.name != "kubectl" {
		t.Errorf("cmd[1] name = %q, want kubectl", rollout.name)
	}
	assertContains(t, rollout.args, "rollout", "rollout command expected")
	assertContains(t, rollout.args, "deployment/local-path-provisioner", "should wait for local-path-provisioner")
	assertContains(t, rollout.args, "--timeout", "should have --timeout")
	assertContains(t, rollout.args, "5m0s", "should use plugin timeout value")
}

func TestInstallLocalPath_CustomTimeout(t *testing.T) {
	cmds := setupFakeExec(t)
	p := New(quietLogger(), 30*time.Second)
	cfg := &template.StorageTemplate{Enabled: true, Type: "local-path"}

	if err := p.Install(cfg, "kind-test", "kind"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	// Find the rollout command and check timeout
	for _, cmd := range *cmds {
		if containsArg(cmd.args, "rollout") {
			assertContains(t, cmd.args, "30s", "should use custom timeout 30s")
			return
		}
	}
	t.Fatal("no rollout command found")
}

func TestInstallLocalPath_Kubecontext(t *testing.T) {
	cmds := setupFakeExec(t)
	p := New(quietLogger(), 5*time.Minute)
	cfg := &template.StorageTemplate{Enabled: true, Type: "local-path"}

	if err := p.Install(cfg, "kind-my-cluster", "kind"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	for _, cmd := range *cmds {
		assertContains(t, cmd.args, "kind-my-cluster", "all commands should use the provided kubecontext")
	}
}

func TestIsInstalled_Kubecontext(t *testing.T) {
	cmds := setupFakeExec(t)
	p := New(quietLogger(), 5*time.Minute)

	installed, err := p.IsInstalled("kind-check")
	if err != nil {
		t.Fatalf("IsInstalled() error = %v", err)
	}
	if !installed {
		t.Error("IsInstalled should return true when command succeeds")
	}

	if len(*cmds) != 1 {
		t.Fatalf("expected 1 command, got %d", len(*cmds))
	}
	assertContains(t, (*cmds)[0].args, "kind-check", "should use provided kubecontext")
	assertContains(t, (*cmds)[0].args, "local-path-provisioner", "should check local-path-provisioner deployment")
}

func TestUninstallLocalPath_Commands(t *testing.T) {
	cmds := setupFakeExec(t)
	p := New(quietLogger(), 5*time.Minute)
	cfg := &template.StorageTemplate{Enabled: true, Type: "local-path"}

	if err := p.Uninstall(cfg, "kind-test"); err != nil {
		t.Fatalf("Uninstall() error = %v", err)
	}

	if len(*cmds) != 1 {
		t.Fatalf("expected 1 command, got %d", len(*cmds))
	}
	assertContains(t, (*cmds)[0].args, "delete", "should run kubectl delete")
	assertContains(t, (*cmds)[0].args, "-f", "should delete via manifest URL")
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
