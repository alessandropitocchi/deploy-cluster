package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/alessandropitocchi/deploy-cluster/pkg/logger"
)

// DiffOptions configures the diff operation.
type DiffOptions struct {
	Kubecontext string
	Log         *logger.Logger
}

// DiffEntry represents a single resource in the diff result.
type DiffEntry struct {
	Kind      string
	Name      string
	Namespace string
}

// String returns a human-readable representation of the entry.
func (e DiffEntry) String() string {
	if e.Namespace != "" {
		return fmt.Sprintf("%s %s/%s", e.Kind, e.Namespace, e.Name)
	}
	return fmt.Sprintf("%s %s", e.Kind, e.Name)
}

// DiffResult holds the comparison between a snapshot and the live cluster.
type DiffResult struct {
	ToRestore []DiffEntry // in snapshot, not in cluster
	Existing  []DiffEntry // in both
}

// DiffSnapshot compares a saved snapshot against the live cluster.
// For each resource in the snapshot, it checks if the resource exists in the cluster.
func DiffSnapshot(name string, opts DiffOptions) (*DiffResult, error) {
	dir, err := snapshotDir(name)
	if err != nil {
		return nil, err
	}

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil, fmt.Errorf("snapshot %q not found", name)
	}

	result := &DiffResult{}

	// Scan all subdirectories for .yaml files
	dirs := []string{"crds", "namespaces", "cluster-scoped", "namespaced"}
	for _, subdir := range dirs {
		base := filepath.Join(dir, subdir)
		if !dirExists(base) {
			continue
		}
		if err := filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() || !strings.HasSuffix(info.Name(), ".yaml") {
				return nil
			}
			// Skip metadata.yaml
			if info.Name() == "metadata.yaml" {
				return nil
			}

			entry, err := readResourceEntry(path)
			if err != nil {
				opts.Log.Debug("Skipping %s: %v\n", path, err)
				return nil
			}

			exists := resourceExistsInCluster(entry, opts.Kubecontext)
			if exists {
				result.Existing = append(result.Existing, entry)
			} else {
				result.ToRestore = append(result.ToRestore, entry)
			}
			return nil
		}); err != nil {
			return nil, fmt.Errorf("failed to scan %s: %w", subdir, err)
		}
	}

	// Sort for deterministic output
	sortEntries(result.ToRestore)
	sortEntries(result.Existing)

	return result, nil
}

// readResourceEntry reads a snapshot file and extracts kind, name, and namespace.
func readResourceEntry(path string) (DiffEntry, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return DiffEntry{}, err
	}

	var res map[string]interface{}
	if err := json.Unmarshal(data, &res); err != nil {
		return DiffEntry{}, fmt.Errorf("failed to parse %s: %w", path, err)
	}

	kind, _ := res["kind"].(string)
	meta, _ := res["metadata"].(map[string]interface{})
	if kind == "" || meta == nil {
		return DiffEntry{}, fmt.Errorf("missing kind or metadata in %s", path)
	}

	name, _ := meta["name"].(string)
	namespace, _ := meta["namespace"].(string)
	if name == "" {
		return DiffEntry{}, fmt.Errorf("missing name in %s", path)
	}

	return DiffEntry{Kind: kind, Name: name, Namespace: namespace}, nil
}

// resourceExistsInCluster checks if a resource exists in the cluster using kubectl get.
func resourceExistsInCluster(entry DiffEntry, kubecontext string) bool {
	args := []string{"--context", kubecontext, "get", strings.ToLower(entry.Kind), entry.Name}
	if entry.Namespace != "" {
		args = append(args, "-n", entry.Namespace)
	}
	args = append(args, "--no-headers", "--ignore-not-found")

	cmd := execCommand("kubectl", args...)
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(output)) != ""
}

func sortEntries(entries []DiffEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Kind != entries[j].Kind {
			return entries[i].Kind < entries[j].Kind
		}
		if entries[i].Namespace != entries[j].Namespace {
			return entries[i].Namespace < entries[j].Namespace
		}
		return entries[i].Name < entries[j].Name
	})
}
