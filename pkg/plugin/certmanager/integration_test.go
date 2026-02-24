package certmanager

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
	return logger.New("[cert-manager]", logger.LevelQuiet)
}

func TestInstall_Commands(t *testing.T) {
	cmds := setupFakeExec(t)
	p := New(quietLogger(), 5*time.Minute)
	cfg := &template.CertManagerTemplate{Enabled: true, Version: "v1.16.3"}

	if err := p.Install(cfg, "kind-test", "kind"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	if len(*cmds) < 3 {
		t.Fatalf("expected at least 3 commands (apply + 2 rollouts), got %d", len(*cmds))
	}

	// Command 1: kubectl apply with manifest URL
	apply := (*cmds)[0]
	assertContains(t, apply.args, "apply", "first cmd should be apply")
	assertContains(t, apply.args, "v1.16.3", "should use specified version in URL")

	// Command 2: rollout status cert-manager-webhook
	webhook := (*cmds)[1]
	assertContains(t, webhook.args, "cert-manager-webhook", "should wait for webhook")
	assertContains(t, webhook.args, "--timeout", "webhook should have --timeout")
	assertContains(t, webhook.args, "5m0s", "webhook should use plugin timeout")

	// Command 3: rollout status cert-manager controller
	ctrl := (*cmds)[2]
	assertContains(t, ctrl.args, "deployment/cert-manager", "should wait for controller")
	assertContains(t, ctrl.args, "--timeout", "controller should have --timeout")
	assertContains(t, ctrl.args, "5m0s", "controller should use plugin timeout")
}

func TestInstall_CustomTimeout(t *testing.T) {
	cmds := setupFakeExec(t)
	p := New(quietLogger(), 10*time.Minute)
	cfg := &template.CertManagerTemplate{Enabled: true}

	if err := p.Install(cfg, "kind-test", "kind"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	// Both rollouts should use the custom timeout
	rollouts := 0
	for _, cmd := range *cmds {
		if containsArg(cmd.args, "rollout") {
			assertContains(t, cmd.args, "10m0s", "rollout should use custom timeout 10m")
			rollouts++
		}
	}
	if rollouts != 2 {
		t.Errorf("expected 2 rollout commands, got %d", rollouts)
	}
}

func TestInstall_DefaultVersion(t *testing.T) {
	cmds := setupFakeExec(t)
	p := New(quietLogger(), 5*time.Minute)
	cfg := &template.CertManagerTemplate{Enabled: true} // no version

	if err := p.Install(cfg, "kind-test", "kind"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	assertContains(t, (*cmds)[0].args, "v1.16.3", "should use default version")
}

func TestInstall_CustomVersion(t *testing.T) {
	cmds := setupFakeExec(t)
	p := New(quietLogger(), 5*time.Minute)
	cfg := &template.CertManagerTemplate{Enabled: true, Version: "v1.14.0"}

	if err := p.Install(cfg, "kind-test", "kind"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	assertContains(t, (*cmds)[0].args, "v1.14.0", "should use custom version")
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
