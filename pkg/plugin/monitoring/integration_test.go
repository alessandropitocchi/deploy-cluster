package monitoring

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
	return logger.New("[monitoring]", logger.LevelQuiet)
}

func TestInstallPrometheus_HelmArgs(t *testing.T) {
	cmds := setupFakeExec(t)
	p := New(quietLogger(), 5*time.Minute)
	cfg := &template.MonitoringTemplate{Enabled: true, Type: "prometheus"}

	if err := p.Install(cfg, "kind-test", "kind"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	if len(*cmds) < 1 {
		t.Fatal("expected at least 1 command")
	}

	helm := (*cmds)[0]
	if helm.name != "helm" {
		t.Errorf("cmd name = %q, want helm", helm.name)
	}
	assertContains(t, helm.args, "upgrade", "should use upgrade --install")
	assertContains(t, helm.args, "--install", "should use upgrade --install")
	assertContains(t, helm.args, "kube-prometheus-stack", "should install kube-prometheus-stack")
	assertContains(t, helm.args, "--namespace", "should specify namespace")
	assertContains(t, helm.args, "monitoring", "should use monitoring namespace")
	assertContains(t, helm.args, "--kube-context", "should specify kube-context")
	assertContains(t, helm.args, "kind-test", "should use provided kubecontext")
	assertContains(t, helm.args, "--timeout", "should have --timeout")
	assertContains(t, helm.args, "5m0s", "should use plugin timeout")
	assertContains(t, helm.args, "--wait", "should use --wait")
	assertContains(t, helm.args, "--create-namespace", "should create namespace")
}

func TestInstallPrometheus_CustomTimeout(t *testing.T) {
	cmds := setupFakeExec(t)
	p := New(quietLogger(), 10*time.Minute)
	cfg := &template.MonitoringTemplate{Enabled: true, Type: "prometheus"}

	if err := p.Install(cfg, "kind-test", "kind"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	assertContains(t, (*cmds)[0].args, "10m0s", "should use custom timeout 10m")
}

func TestInstallPrometheus_CustomVersion(t *testing.T) {
	cmds := setupFakeExec(t)
	p := New(quietLogger(), 5*time.Minute)
	cfg := &template.MonitoringTemplate{Enabled: true, Type: "prometheus", Version: "70.0.0"}

	if err := p.Install(cfg, "kind-test", "kind"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	assertContains(t, (*cmds)[0].args, "70.0.0", "should use custom chart version")
}

func TestInstallPrometheus_WithIngress(t *testing.T) {
	cmds := setupFakeExec(t)
	p := New(quietLogger(), 5*time.Minute)
	cfg := &template.MonitoringTemplate{
		Enabled: true,
		Type:    "prometheus",
		Ingress: &template.MonitoringIngressTemplate{
			Enabled: true,
			Host:    "grafana.local",
		},
	}

	if err := p.Install(cfg, "kind-test", "kind"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	// Should have helm install + kubectl apply for ingress
	if len(*cmds) < 2 {
		t.Fatalf("expected at least 2 commands (helm + ingress apply), got %d", len(*cmds))
	}

	// Last command should be kubectl apply for ingress
	ingressCmd := (*cmds)[len(*cmds)-1]
	if ingressCmd.name != "kubectl" {
		t.Errorf("last cmd = %q, want kubectl for ingress", ingressCmd.name)
	}
	assertContains(t, ingressCmd.args, "apply", "should apply ingress manifest")
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
	cmd := (*cmds)[0]
	if cmd.name != "helm" {
		t.Errorf("cmd name = %q, want helm", cmd.name)
	}
	assertContains(t, cmd.args, "status", "should run helm status")
	assertContains(t, cmd.args, "kube-prometheus-stack", "should check release name")
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
