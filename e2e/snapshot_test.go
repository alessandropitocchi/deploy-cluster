package e2e

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestSnapshotLifecycle tests snapshot save, list, and restore
func TestSnapshotLifecycle(t *testing.T) {
	if os.Getenv("RUN_E2E") != "1" {
		t.Skip("Skipping e2e test. Set RUN_E2E=1 to run.")
	}

	clusterName := "e2e-snapshot-test"
	defer cleanupCluster(t, clusterName)

	tmpDir := t.TempDir()
	templateFile := filepath.Join(tmpDir, "template.yaml")
	
	// Create cluster with some resources
	template := fmt.Sprintf(`
name: %s
provider:
  type: kind
cluster:
  controlPlanes: 1
  workers: 0
  version: v1.31.0
plugins:
  storage:
    enabled: true
    type: local-path
`, clusterName)

	if err := os.WriteFile(templateFile, []byte(template), 0644); err != nil {
		t.Fatalf("Failed to write template: %v", err)
	}

	// Create cluster
	output, err := runKlastr(t, "run", "-f", templateFile)
	if err != nil {
		t.Fatalf("Failed to create cluster: %v\nOutput: %s", err, output)
	}

	// Wait for cluster to be ready
	contextName := "kind-" + clusterName
	if err := waitForClusterReady(t, contextName, 5*time.Minute); err != nil {
		t.Fatalf("Cluster not ready: %v", err)
	}

	snapshotName := "e2e-test-snapshot"

	t.Run("SaveSnapshot", func(t *testing.T) {
		// Cleanup any existing snapshot (ignore error if doesn't exist)
		_, _ = runKlastr(t, "snapshot", "delete", snapshotName)

		output, err := runKlastr(t, "snapshot", "save", snapshotName, "-f", templateFile)
		if err != nil {
			t.Fatalf("Failed to save snapshot: %v\nOutput: %s", err, output)
		}

		if !strings.Contains(output, "Snapshot saved") && !strings.Contains(output, "saved") {
			t.Logf("Warning: unexpected output: %s", output)
		}
	})

	t.Run("ListSnapshots", func(t *testing.T) {
		output, err := runKlastr(t, "snapshot", "list")
		if err != nil {
			t.Fatalf("Failed to list snapshots: %v\nOutput: %s", err, output)
		}

		if !strings.Contains(output, snapshotName) {
			t.Errorf("Snapshot %s not found in list: %s", snapshotName, output)
		}
	})

	t.Run("RestoreSnapshotDryRun", func(t *testing.T) {
		output, err := runKlastr(t, "snapshot", "restore", snapshotName, "--dry-run", "-f", templateFile)
		if err != nil {
			t.Fatalf("Dry-run restore failed: %v\nOutput: %s", err, output)
		}

		if !strings.Contains(output, "dry-run") && !strings.Contains(output, "Dry-run") {
			t.Logf("Warning: dry-run not indicated in output: %s", output)
		}
	})

	t.Run("DeleteSnapshot", func(t *testing.T) {
		output, err := runKlastr(t, "snapshot", "delete", snapshotName)
		if err != nil {
			t.Fatalf("Failed to delete snapshot: %v\nOutput: %s", err, output)
		}

		// Verify snapshot is deleted
		listOutput, _ := runKlastr(t, "snapshot", "list")
		if strings.Contains(listOutput, snapshotName) {
			t.Errorf("Snapshot %s still exists after delete", snapshotName)
		}
	})
}

// TestSnapshotWithNamespace tests snapshot with namespace filtering
func TestSnapshotWithNamespace(t *testing.T) {
	if os.Getenv("RUN_E2E") != "1" {
		t.Skip("Skipping e2e test. Set RUN_E2E=1 to run.")
	}

	clusterName := "e2e-snapshot-ns-test"
	defer cleanupCluster(t, clusterName)

	tmpDir := t.TempDir()
	templateFile := filepath.Join(tmpDir, "template.yaml")
	
	template := fmt.Sprintf(`
name: %s
provider:
  type: kind
cluster:
  controlPlanes: 1
  workers: 0
  version: v1.31.0
`, clusterName)

	if err := os.WriteFile(templateFile, []byte(template), 0644); err != nil {
		t.Fatalf("Failed to write template: %v", err)
	}

	// Create cluster
	output, err := runKlastr(t, "run", "-f", templateFile)
	if err != nil {
		t.Fatalf("Failed to create cluster: %v\nOutput: %s", err, output)
	}

	// Create a test namespace
	contextName := "kind-" + clusterName
	cmd := exec.Command("kubectl", "--context", contextName, "create", "ns", "e2e-test-ns")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("Failed to create namespace: %v\n%s", err, output)
	}

	snapshotName := "e2e-ns-snapshot"

	// Save snapshot with namespace filter
	output, err = runKlastr(t, "snapshot", "save", snapshotName, "--namespace", "e2e-test-ns", "-f", templateFile)
	if err != nil {
		t.Fatalf("Failed to save snapshot: %v\nOutput: %s", err, output)
	}

	// Cleanup (ignore error)
	_, _ = runKlastr(t, "snapshot", "delete", snapshotName)
}


