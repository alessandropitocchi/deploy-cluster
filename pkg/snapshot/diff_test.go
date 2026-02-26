package snapshot

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/alessandropitocchi/deploy-cluster/pkg/logger"
)

func writeTestFile(t *testing.T, path string, res map[string]interface{}) {
	t.Helper()
	data, _ := json.MarshalIndent(res, "", "  ")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write test file %s: %v", path, err)
	}
}

func TestReadResourceEntry(t *testing.T) {
	dir := t.TempDir()

	res := map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]interface{}{
			"name":      "my-app",
			"namespace": "default",
		},
	}
	path := filepath.Join(dir, "my-app.yaml")
	writeTestFile(t, path, res)

	entry, err := readResourceEntry(path)
	if err != nil {
		t.Fatalf("readResourceEntry() error = %v", err)
	}
	if entry.Kind != "Deployment" {
		t.Errorf("kind = %q, want %q", entry.Kind, "Deployment")
	}
	if entry.Name != "my-app" {
		t.Errorf("name = %q, want %q", entry.Name, "my-app")
	}
	if entry.Namespace != "default" {
		t.Errorf("namespace = %q, want %q", entry.Namespace, "default")
	}
}

func TestReadResourceEntry_ClusterScoped(t *testing.T) {
	dir := t.TempDir()

	res := map[string]interface{}{
		"apiVersion": "rbac.authorization.k8s.io/v1",
		"kind":       "ClusterRole",
		"metadata": map[string]interface{}{
			"name": "admin",
		},
	}
	path := filepath.Join(dir, "admin.yaml")
	writeTestFile(t, path, res)

	entry, err := readResourceEntry(path)
	if err != nil {
		t.Fatalf("readResourceEntry() error = %v", err)
	}
	if entry.Namespace != "" {
		t.Errorf("namespace = %q, want empty", entry.Namespace)
	}
}

func TestReadResourceEntry_Invalid(t *testing.T) {
	dir := t.TempDir()

	res := map[string]interface{}{
		"metadata": map[string]interface{}{"name": "test"},
	}
	path := filepath.Join(dir, "bad.yaml")
	writeTestFile(t, path, res)

	_, err := readResourceEntry(path)
	if err == nil {
		t.Fatal("expected error for missing kind")
	}
}

func TestReadResourceEntry_MissingName(t *testing.T) {
	dir := t.TempDir()

	res := map[string]interface{}{
		"kind":     "ConfigMap",
		"metadata": map[string]interface{}{},
	}
	path := filepath.Join(dir, "noname.yaml")
	writeTestFile(t, path, res)

	_, err := readResourceEntry(path)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestDiffEntry_String(t *testing.T) {
	tests := []struct {
		entry DiffEntry
		want  string
	}{
		{
			entry: DiffEntry{Kind: "Deployment", Name: "my-app", Namespace: "default"},
			want:  "Deployment default/my-app",
		},
		{
			entry: DiffEntry{Kind: "ClusterRole", Name: "admin"},
			want:  "ClusterRole admin",
		},
	}
	for _, tt := range tests {
		got := tt.entry.String()
		if got != tt.want {
			t.Errorf("String() = %q, want %q", got, tt.want)
		}
	}
}

func TestSortEntries(t *testing.T) {
	entries := []DiffEntry{
		{Kind: "Service", Name: "svc-b", Namespace: "default"},
		{Kind: "Deployment", Name: "app-a", Namespace: "default"},
		{Kind: "Deployment", Name: "app-b", Namespace: "default"},
		{Kind: "ClusterRole", Name: "admin"},
	}

	sortEntries(entries)

	if entries[0].Kind != "ClusterRole" {
		t.Errorf("first entry should be ClusterRole, got %s", entries[0].Kind)
	}
	if entries[1].Name != "app-a" {
		t.Errorf("second entry should be app-a, got %s", entries[1].Name)
	}
	if entries[2].Name != "app-b" {
		t.Errorf("third entry should be app-b, got %s", entries[2].Name)
	}
	if entries[3].Kind != "Service" {
		t.Errorf("fourth entry should be Service, got %s", entries[3].Kind)
	}
}

func TestDiffSnapshot_WithMockKubectl(t *testing.T) {
	// Create a fake snapshot directory structure
	snapshotBase := t.TempDir()
	snapDir := filepath.Join(snapshotBase, "test-diff")

	for _, d := range []string{
		filepath.Join(snapDir, "namespaced", "default", "deployments"),
		filepath.Join(snapDir, "namespaced", "default", "configmaps"),
		filepath.Join(snapDir, "cluster-scoped", "clusterroles"),
	} {
		if err := os.MkdirAll(d, 0755); err != nil {
			t.Fatalf("failed to create dir %s: %v", d, err)
		}
	}

	// Write resources
	writeTestResource(t, filepath.Join(snapDir, "namespaced", "default", "deployments", "my-app.yaml"),
		"Deployment", "my-app", "default")
	writeTestResource(t, filepath.Join(snapDir, "namespaced", "default", "configmaps", "my-config.yaml"),
		"ConfigMap", "my-config", "default")
	writeTestResource(t, filepath.Join(snapDir, "cluster-scoped", "clusterroles", "custom-role.yaml"),
		"ClusterRole", "custom-role", "")

	// Write metadata
	meta := &Metadata{Name: "test-diff", ResourceCount: 3}
	if err := SaveMetadata(snapDir, meta); err != nil {
		t.Fatalf("failed to save metadata: %v", err)
	}

	// Mock kubectl: "my-app" exists, "my-config" not found, "custom-role" exists
	origExec := execCommand
	defer func() { execCommand = origExec }()
	execCommand = func(name string, args ...string) *exec.Cmd {
		for i, arg := range args {
			if arg == "get" && i+2 < len(args) {
				resourceName := args[i+2]
				switch resourceName {
				case "my-app", "custom-role":
					return exec.Command("echo", "found")
				default:
					return exec.Command("echo", "") // empty = not found
				}
			}
		}
		return exec.Command("echo", "")
	}

	// Test readResourceEntry + resourceExistsInCluster directly since DiffSnapshot uses snapshotDir()
	log := logger.New("[test]", logger.LevelQuiet)

	entry1, _ := readResourceEntry(filepath.Join(snapDir, "namespaced", "default", "deployments", "my-app.yaml"))
	if !resourceExistsInCluster(entry1, "test-context") {
		t.Error("my-app should exist in cluster (mocked)")
	}

	entry2, _ := readResourceEntry(filepath.Join(snapDir, "namespaced", "default", "configmaps", "my-config.yaml"))
	if resourceExistsInCluster(entry2, "test-context") {
		t.Error("my-config should not exist in cluster (mocked)")
	}

	_ = log
}

func writeTestResource(t *testing.T, path, kind, name, namespace string) {
	t.Helper()
	res := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       kind,
		"metadata":   map[string]interface{}{"name": name},
	}
	if namespace != "" {
		res["metadata"].(map[string]interface{})["namespace"] = namespace
	}
	data, _ := json.MarshalIndent(res, "", "  ")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write test resource: %v", err)
	}
}
