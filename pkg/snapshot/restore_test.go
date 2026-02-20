package snapshot

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"testing"

	"github.com/alepito/deploy-cluster/pkg/logger"
)

func TestRestoreSnapshot_DirectoryNotFound(t *testing.T) {
	log := logger.New("[test]", logger.LevelQuiet)

	err := RestoreSnapshot("/nonexistent/path", RestoreOptions{
		Kubecontext: "test-context",
		Log:         log,
	})

	// Should not error — all phases just skip if dirs don't exist
	if err != nil {
		t.Fatalf("RestoreSnapshot() error = %v", err)
	}
}

func TestRestoreSnapshot_DryRun(t *testing.T) {
	dir := t.TempDir()

	// Create a minimal snapshot structure
	crdDir := filepath.Join(dir, "crds")
	if err := os.MkdirAll(crdDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(crdDir, "test-crd.yaml"), []byte(`{"apiVersion":"apiextensions.k8s.io/v1","kind":"CustomResourceDefinition"}`), 0644); err != nil {
		t.Fatal(err)
	}

	nsDir := filepath.Join(dir, "namespaces")
	if err := os.MkdirAll(nsDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nsDir, "default.yaml"), []byte(`{"apiVersion":"v1","kind":"Namespace"}`), 0644); err != nil {
		t.Fatal(err)
	}

	log := logger.New("[test]", logger.LevelQuiet)

	err := RestoreSnapshot(dir, RestoreOptions{
		Kubecontext: "test-context",
		Log:         log,
		DryRun:      true,
	})

	if err != nil {
		t.Fatalf("RestoreSnapshot() dry-run error = %v", err)
	}
}

func TestApplyOrderedSubdirs_Priority(t *testing.T) {
	dir := t.TempDir()

	// Create subdirectories matching namespaced priority
	subdirs := []string{"deployments", "secrets", "services", "configmaps", "serviceaccounts", "ingresses"}
	for _, sub := range subdirs {
		subdir := filepath.Join(dir, sub)
		if err := os.MkdirAll(subdir, 0755); err != nil {
			t.Fatal(err)
		}
	}

	entries, _ := os.ReadDir(dir)
	type dirEntry struct {
		name     string
		priority int
	}

	var dirs []dirEntry
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		p := 10
		if v, ok := namespacedPriority[entry.Name()]; ok {
			p = v
		}
		dirs = append(dirs, dirEntry{name: entry.Name(), priority: p})
	}

	sort.Slice(dirs, func(i, j int) bool {
		if dirs[i].priority != dirs[j].priority {
			return dirs[i].priority < dirs[j].priority
		}
		return dirs[i].name < dirs[j].name
	})

	// Verify order: serviceaccounts(0) → configmaps/secrets(1) → services(3) → deployments(4) → ingresses(5)
	expected := []string{"serviceaccounts", "configmaps", "secrets", "services", "deployments", "ingresses"}
	for i, d := range dirs {
		if d.name != expected[i] {
			t.Errorf("position %d: got %q, want %q", i, d.name, expected[i])
		}
	}
}

func TestClusterScopedPriority(t *testing.T) {
	// Verify clusterroles < clusterrolebindings < persistentvolumes
	if clusterScopedPriority["clusterroles"] >= clusterScopedPriority["clusterrolebindings"] {
		t.Error("clusterroles should have lower priority than clusterrolebindings")
	}
	if clusterScopedPriority["clusterrolebindings"] >= clusterScopedPriority["persistentvolumes"] {
		t.Error("clusterrolebindings should have lower priority than persistentvolumes")
	}
}

func TestDirExists(t *testing.T) {
	dir := t.TempDir()

	if !dirExists(dir) {
		t.Error("existing directory should return true")
	}

	if dirExists(filepath.Join(dir, "nonexistent")) {
		t.Error("nonexistent directory should return false")
	}

	// File should return false
	f := filepath.Join(dir, "file.txt")
	if err := os.WriteFile(f, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	if dirExists(f) {
		t.Error("file should return false")
	}
}

func TestApplyFile_DryRun(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "test.yaml")
	if err := os.WriteFile(f, []byte(`{"apiVersion":"v1","kind":"ConfigMap"}`), 0644); err != nil {
		t.Fatal(err)
	}

	log := logger.New("[test]", logger.LevelQuiet)

	// Dry-run should not call kubectl
	err := applyFile(f, RestoreOptions{
		Kubecontext: "test",
		Log:         log,
		DryRun:      true,
	})
	if err != nil {
		t.Fatalf("applyFile dry-run error = %v", err)
	}
}

func TestApplyFile_KubectlFailure(t *testing.T) {
	// Save and restore original execCommand
	orig := execCommand
	defer func() { execCommand = orig }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("false")
	}

	dir := t.TempDir()
	f := filepath.Join(dir, "test.yaml")
	if err := os.WriteFile(f, []byte(`{"apiVersion":"v1","kind":"ConfigMap"}`), 0644); err != nil {
		t.Fatal(err)
	}

	log := logger.New("[test]", logger.LevelQuiet)

	err := applyFile(f, RestoreOptions{
		Kubecontext: "test",
		Log:         log,
		DryRun:      false,
	})
	if err == nil {
		t.Fatal("expected error when kubectl fails")
	}
}
