package snapshot

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/alessandropitocchi/deploy-cluster/pkg/logger"
	"github.com/alessandropitocchi/deploy-cluster/pkg/retry"
)

// RestoreOptions configures the restore operation.
type RestoreOptions struct {
	Kubecontext string
	Log         *logger.Logger
	DryRun      bool
}

// namespacedPriority defines the apply order for namespaced resource types.
var namespacedPriority = map[string]int{
	"serviceaccounts":          0,
	"secrets":                  1,
	"configmaps":               1,
	"persistentvolumeclaims":   2,
	"services":                 3,
	"deployments":              4,
	"statefulsets":             4,
	"daemonsets":               4,
	"ingresses":                5,
	"jobs":                     6,
	"cronjobs":                 6,
}

// clusterScopedPriority defines the apply order for cluster-scoped resource types.
var clusterScopedPriority = map[string]int{
	"clusterroles":        0,
	"clusterrolebindings": 1,
	"persistentvolumes":   2,
}

// RestoreSnapshot applies resources from a snapshot directory to the cluster.
func RestoreSnapshot(dir string, opts RestoreOptions) error {
	// Phase 1: CRDs
	crdDir := filepath.Join(dir, "crds")
	if dirExists(crdDir) {
		opts.Log.Info("Phase 1: Restoring CRDs...\n")
		if err := applyDirectory(crdDir, opts); err != nil {
			return fmt.Errorf("failed to restore CRDs: %w", err)
		}
		if !opts.DryRun {
			opts.Log.Debug("Waiting 5s for CRD propagation...\n")
			time.Sleep(5 * time.Second)
		}
	}

	// Phase 2: Namespaces
	nsDir := filepath.Join(dir, "namespaces")
	if dirExists(nsDir) {
		opts.Log.Info("Phase 2: Restoring Namespaces...\n")
		if err := applyDirectory(nsDir, opts); err != nil {
			return fmt.Errorf("failed to restore namespaces: %w", err)
		}
	}

	// Phase 3: Cluster-scoped resources
	clusterDir := filepath.Join(dir, "cluster-scoped")
	if dirExists(clusterDir) {
		opts.Log.Info("Phase 3: Restoring cluster-scoped resources...\n")
		if err := applyOrderedSubdirs(clusterDir, clusterScopedPriority, opts); err != nil {
			return fmt.Errorf("failed to restore cluster-scoped resources: %w", err)
		}
	}

	// Phase 4: Namespaced resources
	namespacedDir := filepath.Join(dir, "namespaced")
	if dirExists(namespacedDir) {
		opts.Log.Info("Phase 4: Restoring namespaced resources...\n")
		entries, err := os.ReadDir(namespacedDir)
		if err != nil {
			return fmt.Errorf("failed to read namespaced directory: %w", err)
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			nsPath := filepath.Join(namespacedDir, entry.Name())
			opts.Log.Info("  Namespace: %s\n", entry.Name())
			if err := applyOrderedSubdirs(nsPath, namespacedPriority, opts); err != nil {
				return fmt.Errorf("failed to restore resources in namespace %s: %w", entry.Name(), err)
			}
		}
	}

	return nil
}

// applyDirectory applies all .yaml files in a directory (no ordering).
func applyDirectory(dir string, opts RestoreOptions) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.yaml"))
	if err != nil {
		return err
	}
	for _, f := range files {
		if err := applyFile(f, opts); err != nil {
			return err
		}
	}
	return nil
}

// applyOrderedSubdirs applies subdirectories in priority order.
func applyOrderedSubdirs(dir string, priority map[string]int, opts RestoreOptions) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	type dirEntry struct {
		name     string
		priority int
	}

	var dirs []dirEntry
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		p := 10 // default priority
		if v, ok := priority[entry.Name()]; ok {
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

	for _, d := range dirs {
		subdir := filepath.Join(dir, d.name)
		opts.Log.Debug("  Applying %s...\n", d.name)
		if err := applyDirectory(subdir, opts); err != nil {
			return fmt.Errorf("failed to apply %s: %w", d.name, err)
		}
	}
	return nil
}

// applyFile applies a single YAML file using kubectl apply.
func applyFile(path string, opts RestoreOptions) error {
	if opts.DryRun {
		name := filepath.Base(path)
		dir := filepath.Base(filepath.Dir(path))
		opts.Log.Info("  [dry-run] Would apply %s/%s\n", dir, name)
		return nil
	}

	return retry.Run(3, 5*time.Second, opts.Log.Warn, func() error {
		args := []string{"--context", opts.Kubecontext, "apply", "-f", path, "--server-side", "--force-conflicts"}
		cmd := execCommand("kubectl", args...)
		output, err := cmd.CombinedOutput()
		if err != nil {
			// Include output in error for debugging
			errMsg := strings.TrimSpace(string(output))
			if errMsg != "" {
				return fmt.Errorf("%s: %w", errMsg, err)
			}
			return err
		}
		return nil
	})
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
