package customapps

import (
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/alepito/deploy-cluster/pkg/logger"
	"github.com/alepito/deploy-cluster/pkg/template"
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
	return logger.New("[customApps]", logger.LevelQuiet)
}

func TestInstall_HelmArgs(t *testing.T) {
	cmds := setupFakeExec(t)
	p := New(quietLogger(), 5*time.Minute)
	apps := []template.CustomAppTemplate{
		{Name: "my-app", ChartName: "oci://ghcr.io/my/chart", Version: "1.0.0"},
	}

	if err := p.Install(apps, "kind-test", "kind"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	if len(*cmds) < 1 {
		t.Fatal("expected at least 1 command")
	}

	helm := (*cmds)[0]
	if helm.name != "helm" {
		t.Errorf("cmd name = %q, want helm", helm.name)
	}
	assertContains(t, helm.args, "upgrade", "should use upgrade")
	assertContains(t, helm.args, "--install", "should use --install")
	assertContains(t, helm.args, "my-app", "should use app name as release name")
	assertContains(t, helm.args, "oci://ghcr.io/my/chart", "should use chart ref")
	assertContains(t, helm.args, "--version", "should specify version")
	assertContains(t, helm.args, "1.0.0", "should use specified version")
	assertContains(t, helm.args, "--namespace", "should specify namespace")
	assertContains(t, helm.args, "my-app", "should use app name as namespace when not specified")
	assertContains(t, helm.args, "--timeout", "should have --timeout")
	assertContains(t, helm.args, "5m0s", "should use plugin timeout")
	assertContains(t, helm.args, "--wait", "should use --wait")
}

func TestInstall_CustomTimeout(t *testing.T) {
	cmds := setupFakeExec(t)
	p := New(quietLogger(), 45*time.Second)
	apps := []template.CustomAppTemplate{
		{Name: "my-app", ChartName: "oci://ghcr.io/my/chart"},
	}

	if err := p.Install(apps, "kind-test", "kind"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	assertContains(t, (*cmds)[0].args, "45s", "should use custom timeout 45s")
}

func TestInstall_CustomNamespace(t *testing.T) {
	cmds := setupFakeExec(t)
	p := New(quietLogger(), 5*time.Minute)
	apps := []template.CustomAppTemplate{
		{Name: "my-app", ChartName: "oci://ghcr.io/my/chart", Namespace: "custom-ns"},
	}

	if err := p.Install(apps, "kind-test", "kind"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	assertContains(t, (*cmds)[0].args, "custom-ns", "should use custom namespace")
}

func TestInstallAll_MultipleApps(t *testing.T) {
	cmds := setupFakeExec(t)
	p := New(quietLogger(), 5*time.Minute)
	apps := []template.CustomAppTemplate{
		{Name: "app-one", ChartName: "chart-one"},
		{Name: "app-two", ChartName: "chart-two"},
	}

	if err := p.InstallAll(apps, "kind-test"); err != nil {
		t.Fatalf("InstallAll() error = %v", err)
	}

	// Should have at least 2 helm commands (one per app)
	helmCmds := 0
	for _, cmd := range *cmds {
		if cmd.name == "helm" {
			helmCmds++
		}
	}
	if helmCmds < 2 {
		t.Errorf("expected at least 2 helm commands, got %d", helmCmds)
	}
}

func TestInstall_WithIngress(t *testing.T) {
	cmds := setupFakeExec(t)
	p := New(quietLogger(), 5*time.Minute)
	apps := []template.CustomAppTemplate{
		{
			Name:      "my-app",
			ChartName: "oci://ghcr.io/my/chart",
			Ingress: &template.CustomAppIngressTemplate{
				Enabled:     true,
				Host:        "my-app.local",
				ServiceName: "my-svc",
				ServicePort: 8080,
			},
		},
	}

	if err := p.Install(apps, "kind-test", "kind"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	// Should have helm + kubectl apply for ingress
	if len(*cmds) < 2 {
		t.Fatalf("expected at least 2 commands, got %d", len(*cmds))
	}

	lastCmd := (*cmds)[len(*cmds)-1]
	if lastCmd.name != "kubectl" {
		t.Errorf("last cmd = %q, want kubectl for ingress", lastCmd.name)
	}
	assertContains(t, lastCmd.args, "apply", "should apply ingress manifest")
}

func TestIsInstalled_HelmStatus(t *testing.T) {
	cmds := setupFakeExec(t)
	p := New(quietLogger(), 5*time.Minute)

	installed, err := p.IsAppInstalled("my-app", "my-ns", "kind-test")
	if err != nil {
		t.Fatalf("IsInstalled() error = %v", err)
	}
	if !installed {
		t.Error("should return true when helm status succeeds")
	}
	if len(*cmds) != 1 {
		t.Fatalf("expected 1 command, got %d", len(*cmds))
	}
	assertContains(t, (*cmds)[0].args, "my-app", "should check release name")
	assertContains(t, (*cmds)[0].args, "my-ns", "should check in correct namespace")
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
