package argocd

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

func quietLog() *logger.Logger {
	return logger.New("[argocd]", logger.LevelQuiet)
}

func TestInstall_FullFlow(t *testing.T) {
	cmds := setupFakeExec(t)
	p := New(quietLog(), 5*time.Minute)
	cfg := &template.ArgoCDTemplate{
		Enabled:   true,
		Namespace: "argocd",
		Version:   "stable",
	}

	if err := p.Install(cfg, "kind-test", "kind"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	if len(*cmds) < 3 {
		t.Fatalf("expected at least 3 commands (ns create + apply + rollout), got %d", len(*cmds))
	}

	// Command 1: create namespace
	nsCmd := (*cmds)[0]
	if nsCmd.name != "kubectl" {
		t.Errorf("cmd[0] name = %q, want kubectl", nsCmd.name)
	}
	assertContains(t, nsCmd.args, "create", "should create namespace")
	assertContains(t, nsCmd.args, "namespace", "should create namespace")
	assertContains(t, nsCmd.args, "argocd", "should use argocd namespace")

	// Command 2: kubectl apply manifest
	applyCmd := (*cmds)[1]
	assertContains(t, applyCmd.args, "apply", "should apply manifest")
	assertContains(t, applyCmd.args, "argocd", "should apply to argocd namespace")
	assertContains(t, applyCmd.args, "--server-side", "should use server-side apply")

	// Command 3: rollout status
	rollout := (*cmds)[2]
	assertContains(t, rollout.args, "rollout", "should wait for rollout")
	assertContains(t, rollout.args, "deployment/argocd-server", "should wait for argocd-server")
	assertContains(t, rollout.args, "--timeout", "should have --timeout")
	assertContains(t, rollout.args, "5m0s", "should use plugin timeout")
}

func TestInstall_CustomTimeout(t *testing.T) {
	cmds := setupFakeExec(t)
	p := New(quietLog(), 15*time.Minute)
	cfg := &template.ArgoCDTemplate{
		Enabled:   true,
		Namespace: "argocd",
		Version:   "stable",
	}

	if err := p.Install(cfg, "kind-test", "kind"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	for _, cmd := range *cmds {
		if containsArg(cmd.args, "rollout") {
			assertContains(t, cmd.args, "15m0s", "should use custom timeout 15m")
			return
		}
	}
	t.Fatal("no rollout command found")
}

func TestInstall_CustomNamespace(t *testing.T) {
	cmds := setupFakeExec(t)
	p := New(quietLog(), 5*time.Minute)
	cfg := &template.ArgoCDTemplate{
		Enabled:   true,
		Namespace: "gitops",
		Version:   "stable",
	}

	if err := p.Install(cfg, "kind-test", "kind"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	// Namespace creation should use "gitops"
	assertContains(t, (*cmds)[0].args, "gitops", "should use custom namespace")
}

func TestInstall_WithIngress(t *testing.T) {
	cmds := setupFakeExec(t)
	p := New(quietLog(), 5*time.Minute)
	cfg := &template.ArgoCDTemplate{
		Enabled:   true,
		Namespace: "argocd",
		Version:   "stable",
		Ingress: &template.ArgoCDIngressTemplate{
			Enabled: true,
			Host:    "argocd.local",
		},
	}

	if err := p.Install(cfg, "kind-test", "kind"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	// Should have: ns create + apply manifest + rollout + configmap apply + rollout restart + rollout status + ingress apply
	if len(*cmds) < 7 {
		t.Fatalf("expected at least 7 commands for install with ingress, got %d", len(*cmds))
	}

	// Find the configmap apply (argocd-cmd-params-cm)
	foundCM := false
	for _, cmd := range *cmds {
		if cmd.name == "kubectl" && containsArg(cmd.args, "apply") {
			// One of the apply commands should be for the configmap (via stdin)
			foundCM = true
			break
		}
	}
	if !foundCM {
		t.Error("should apply argocd-cmd-params-cm configmap")
	}
}

func TestInstall_WithRepos(t *testing.T) {
	cmds := setupFakeExec(t)
	p := New(quietLog(), 5*time.Minute)
	cfg := &template.ArgoCDTemplate{
		Enabled:   true,
		Namespace: "argocd",
		Version:   "stable",
		Repos: []template.ArgoCDRepoTemplate{
			{Name: "my-repo", URL: "https://github.com/user/repo.git"},
		},
	}

	if err := p.Install(cfg, "kind-test", "kind"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	// Should have kubectl apply for the repo secret
	found := false
	for _, cmd := range *cmds {
		if cmd.name == "kubectl" && containsArg(cmd.args, "apply") {
			found = true
		}
	}
	if !found {
		t.Error("should apply repo secret")
	}
}

func TestInstall_WithApps(t *testing.T) {
	cmds := setupFakeExec(t)
	p := New(quietLog(), 5*time.Minute)
	cfg := &template.ArgoCDTemplate{
		Enabled:   true,
		Namespace: "argocd",
		Version:   "stable",
		Apps: []template.ArgoCDAppTemplate{
			{
				Name:           "my-app",
				RepoURL:        "https://github.com/user/repo.git",
				Path:           "manifests",
				TargetRevision: "main",
				Namespace:      "default",
			},
		},
	}

	if err := p.Install(cfg, "kind-test", "kind"); err != nil {
		t.Fatalf("Install() error = %v", err)
	}

	// Should have kubectl apply for the Application resource
	applyCount := 0
	for _, cmd := range *cmds {
		if cmd.name == "kubectl" && containsArg(cmd.args, "apply") {
			applyCount++
		}
	}
	// At least: manifest apply + app apply
	if applyCount < 2 {
		t.Errorf("expected at least 2 kubectl apply commands, got %d", applyCount)
	}
}

func TestIsInstalled_Commands(t *testing.T) {
	cmds := setupFakeExec(t)
	p := New(quietLog(), 5*time.Minute)

	installed, err := p.IsInstalledInNamespace("kind-test", "argocd")
	if err != nil {
		t.Fatalf("IsInstalled() error = %v", err)
	}
	if !installed {
		t.Error("should return true when command succeeds")
	}
	if len(*cmds) != 1 {
		t.Fatalf("expected 1 command, got %d", len(*cmds))
	}
	assertContains(t, (*cmds)[0].args, "argocd-server", "should check argocd-server deployment")
	assertContains(t, (*cmds)[0].args, "argocd", "should use argocd namespace")
}

func TestIsInstalled_CustomNamespace(t *testing.T) {
	cmds := setupFakeExec(t)
	p := New(quietLog(), 5*time.Minute)

	_, _ = p.IsInstalledInNamespace("kind-test", "gitops")

	assertContains(t, (*cmds)[0].args, "gitops", "should use custom namespace")
}

func TestWaitForDeployment_Timeout(t *testing.T) {
	cmds := setupFakeExec(t)
	p := New(quietLog(), 7*time.Minute)

	if err := p.waitForDeployment("kind-test", "argocd", "argocd-server", p.Timeout); err != nil {
		t.Fatalf("waitForDeployment() error = %v", err)
	}

	if len(*cmds) != 1 {
		t.Fatalf("expected 1 command, got %d", len(*cmds))
	}
	cmd := (*cmds)[0]
	assertContains(t, cmd.args, "rollout", "should use rollout status")
	assertContains(t, cmd.args, "deployment/argocd-server", "should wait for argocd-server")
	assertContains(t, cmd.args, "7m0s", "should use 7m timeout")
}

func TestUpgrade_Flow(t *testing.T) {
	cmds := setupFakeExec(t)
	p := New(quietLog(), 5*time.Minute)
	cfg := &template.ArgoCDTemplate{
		Enabled:   true,
		Namespace: "argocd",
		Version:   "stable",
	}

	if err := p.Upgrade(cfg, "kind-test", "kind"); err != nil {
		t.Fatalf("Upgrade() error = %v", err)
	}

	if len(*cmds) < 2 {
		t.Fatalf("expected at least 2 commands (apply + rollout), got %d", len(*cmds))
	}

	// Should apply manifest
	assertContains(t, (*cmds)[0].args, "apply", "first command should apply manifest")

	// Should wait for rollout with plugin timeout
	rollout := (*cmds)[1]
	assertContains(t, rollout.args, "rollout", "should wait for rollout")
	assertContains(t, rollout.args, "5m0s", "should use plugin timeout")
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
