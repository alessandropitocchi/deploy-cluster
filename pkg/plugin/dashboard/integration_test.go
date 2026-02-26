package dashboard

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
	return logger.New("[dashboard]", logger.LevelQuiet)
}

func TestInstallHeadlamp_HelmArgs(t *testing.T) {
	cmds := setupFakeExec(t)
	p := New(quietLogger(), 5*time.Minute)
	cfg := &template.DashboardTemplate{Enabled: true, Type: "headlamp"}

	if err := p.Install(cfg, "kind-test", "kind"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	if len(*cmds) < 3 {
		t.Fatal("expected at least 3 commands (repo add, repo update, upgrade)")
	}

	// cmds[0] = repo add, cmds[1] = repo update, cmds[2] = upgrade --install
	helm := (*cmds)[2]
	if helm.name != "helm" {
		t.Errorf("cmd name = %q, want helm", helm.name)
	}
	assertContains(t, helm.args, "upgrade", "should use upgrade --install")
	assertContains(t, helm.args, "--install", "should use upgrade --install")
	assertContains(t, helm.args, "headlamp", "should install headlamp")
	assertContains(t, helm.args, "--namespace", "should specify namespace")
	assertContains(t, helm.args, "--kube-context", "should specify kube-context")
	assertContains(t, helm.args, "kind-test", "should use provided kubecontext")
	assertContains(t, helm.args, "--timeout", "should have --timeout")
	assertContains(t, helm.args, "5m0s", "should use plugin timeout")
	assertContains(t, helm.args, "--wait", "should use --wait")
}

func TestInstallHeadlamp_CustomTimeout(t *testing.T) {
	cmds := setupFakeExec(t)
	p := New(quietLogger(), 3*time.Minute)
	cfg := &template.DashboardTemplate{Enabled: true, Type: "headlamp"}

	if err := p.Install(cfg, "kind-test", "kind"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	// cmds[0] = repo add, cmds[1] = repo update, cmds[2] = upgrade --install
	assertContains(t, (*cmds)[2].args, "3m0s", "should use custom timeout 3m")
}

func TestInstallHeadlamp_WithIngress(t *testing.T) {
	cmds := setupFakeExec(t)
	p := New(quietLogger(), 5*time.Minute)
	cfg := &template.DashboardTemplate{
		Enabled: true,
		Type:    "headlamp",
		Ingress: &template.DashboardIngressTemplate{
			Enabled: true,
			Host:    "headlamp.local",
		},
	}

	if err := p.Install(cfg, "kind-test", "kind"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	// Should have helm + ClusterRoleBinding + ingress apply
	if len(*cmds) < 3 {
		t.Fatalf("expected at least 3 commands, got %d", len(*cmds))
	}

	// Find the ingress apply command
	lastKubectl := (*cmds)[len(*cmds)-1]
	if lastKubectl.name != "kubectl" {
		t.Errorf("last cmd = %q, want kubectl for ingress", lastKubectl.name)
	}
	assertContains(t, lastKubectl.args, "apply", "should apply ingress manifest")
}

func TestIsInstalled_HelmStatus(t *testing.T) {
	cmds := setupFakeExec(t)
	p := New(quietLogger(), 5*time.Minute)

	installed, err := p.IsInstalled("kind-test")
	if err != nil {
		t.Fatalf("IsInstalled() error = %v", err)
	}
	if !installed {
		t.Error("should return true when helm status succeeds")
	}
	if len(*cmds) != 1 {
		t.Fatalf("expected 1 command, got %d", len(*cmds))
	}
	assertContains(t, (*cmds)[0].args, "status", "should run helm status")
	assertContains(t, (*cmds)[0].args, "headlamp", "should check headlamp release")
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
