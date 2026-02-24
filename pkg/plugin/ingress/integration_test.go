package ingress

import (
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/alepito/deploy-cluster/pkg/template"
	"github.com/alepito/deploy-cluster/pkg/logger"
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

func TestInstallNginx_Commands(t *testing.T) {
	cmds := setupFakeExec(t)
	p := New(quietLogger(), 5*time.Minute)
	cfg := &template.IngressTemplate{Enabled: true, Type: "nginx"}

	if err := p.Install(cfg, "kind-test", "kind"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	if len(*cmds) < 2 {
		t.Fatalf("expected at least 2 commands, got %d", len(*cmds))
	}

	// Command 1: kubectl apply
	apply := (*cmds)[0]
	if apply.name != "kubectl" {
		t.Errorf("cmd[0] name = %q, want kubectl", apply.name)
	}
	assertContains(t, apply.args, "apply", "should run kubectl apply")
	assertContains(t, apply.args, "kind-test", "should use kubecontext")

	// Command 2: kubectl rollout status
	rollout := (*cmds)[1]
	assertContains(t, rollout.args, "rollout", "should wait for rollout")
	assertContains(t, rollout.args, "deployment/ingress-nginx-controller", "should wait for nginx controller")
	assertContains(t, rollout.args, "--timeout", "should have --timeout")
	assertContains(t, rollout.args, "5m0s", "should use plugin timeout")
}

func TestInstallNginx_CustomTimeout(t *testing.T) {
	cmds := setupFakeExec(t)
	p := New(quietLogger(), 2*time.Minute)
	cfg := &template.IngressTemplate{Enabled: true, Type: "nginx"}

	if err := p.Install(cfg, "kind-test", "kind"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	for _, cmd := range *cmds {
		if containsArg(cmd.args, "rollout") {
			assertContains(t, cmd.args, "2m0s", "should use custom timeout 2m")
			return
		}
	}
	t.Fatal("no rollout command found")
}

func TestInstallNginx_Namespace(t *testing.T) {
	cmds := setupFakeExec(t)
	p := New(quietLogger(), 5*time.Minute)
	cfg := &template.IngressTemplate{Enabled: true, Type: "nginx"}

	if err := p.Install(cfg, "kind-test", "kind"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	rollout := (*cmds)[1]
	assertContains(t, rollout.args, "ingress-nginx", "rollout should use ingress-nginx namespace")
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
	if len(*cmds) != 1 {
		t.Fatalf("expected 1 command, got %d", len(*cmds))
	}
	assertContains(t, (*cmds)[0].args, "ingress-nginx-controller", "should check nginx controller deployment")
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
