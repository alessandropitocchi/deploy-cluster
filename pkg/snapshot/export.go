package snapshot

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alessandropitocchi/deploy-cluster/pkg/logger"
)

// ExportOptions configures which resources to export.
type ExportOptions struct {
	Kubecontext    string
	Namespaces     []string // nil = discover non-system namespaces
	SkipNamespaces []string
	ExcludeSecrets bool
	Log            *logger.Logger
}

// defaultSkipNamespaces are namespaces excluded by default.
var defaultSkipNamespaces = map[string]bool{
	"kube-system":        true,
	"kube-public":        true,
	"kube-node-lease":    true,
	"local-path-storage": true,
}

// ExportResources discovers and exports all resources from the cluster into dir.
// Returns the total number of resources exported.
func ExportResources(dir string, opts ExportOptions) (int, error) {
	resources, err := DiscoverResources(opts.Kubecontext)
	if err != nil {
		return 0, err
	}

	skipNs := make(map[string]bool)
	for k, v := range defaultSkipNamespaces {
		skipNs[k] = v
	}
	for _, ns := range opts.SkipNamespaces {
		skipNs[ns] = true
	}

	// Determine target namespaces
	namespaces := opts.Namespaces
	if len(namespaces) == 0 {
		namespaces, err = discoverNamespaces(opts.Kubecontext, skipNs)
		if err != nil {
			return 0, fmt.Errorf("failed to discover namespaces: %w", err)
		}
	}

	count := 0

	// Export CRDs
	crdCount, err := exportCRDs(dir, opts.Kubecontext, opts.Log)
	if err != nil {
		return 0, fmt.Errorf("failed to export CRDs: %w", err)
	}
	count += crdCount

	// Export Namespaces
	nsCount, err := exportNamespaces(dir, opts.Kubecontext, namespaces, opts.Log)
	if err != nil {
		return 0, fmt.Errorf("failed to export namespaces: %w", err)
	}
	count += nsCount

	// Split resources into cluster-scoped and namespaced
	var clusterScoped, namespacedRes []APIResource
	for _, r := range resources {
		if r.Name == "customresourcedefinitions" || r.Name == "namespaces" {
			continue // already handled
		}
		if r.Namespaced {
			namespacedRes = append(namespacedRes, r)
		} else {
			clusterScoped = append(clusterScoped, r)
		}
	}

	// Export cluster-scoped resources
	for _, r := range clusterScoped {
		if opts.ExcludeSecrets && r.Name == "secrets" {
			continue
		}
		n, err := exportClusterScopedResource(dir, opts.Kubecontext, r, opts.Log)
		if err != nil {
			opts.Log.Debug("Skipping cluster-scoped resource %s: %v\n", r.GroupResource(), err)
			continue
		}
		count += n
	}

	// Export namespaced resources
	for _, ns := range namespaces {
		for _, r := range namespacedRes {
			if opts.ExcludeSecrets && r.Name == "secrets" {
				continue
			}
			n, err := exportNamespacedResource(dir, opts.Kubecontext, ns, r, opts.Log)
			if err != nil {
				opts.Log.Debug("Skipping %s in %s: %v\n", r.GroupResource(), ns, err)
				continue
			}
			count += n
		}
	}

	return count, nil
}

func discoverNamespaces(kubecontext string, skipNs map[string]bool) ([]string, error) {
	cmd := execCommand("kubectl", "--context", kubecontext, "get", "namespaces", "-o", "jsonpath={.items[*].metadata.name}")
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var namespaces []string
	for _, ns := range strings.Fields(string(output)) {
		if !skipNs[ns] {
			namespaces = append(namespaces, ns)
		}
	}
	return namespaces, nil
}

func exportCRDs(dir, kubecontext string, log *logger.Logger) (int, error) {
	items, err := getResourceItems(kubecontext, "customresourcedefinitions", "", "")
	if err != nil {
		log.Debug("No CRDs found: %v\n", err)
		return 0, nil
	}

	crdDir := filepath.Join(dir, "crds")
	count := 0
	for _, item := range items {
		name := getResourceName(item)
		if name == "" {
			continue
		}
		sanitized := sanitizeResource(item)
		if err := writeResource(crdDir, name, sanitized); err != nil {
			return count, err
		}
		count++
	}
	if count > 0 {
		log.Info("Exported %d CRDs\n", count)
	}
	return count, nil
}

func exportNamespaces(dir, kubecontext string, namespaces []string, log *logger.Logger) (int, error) {
	nsDir := filepath.Join(dir, "namespaces")
	count := 0
	for _, ns := range namespaces {
		items, err := getResourceItems(kubecontext, "namespaces", "", ns)
		if err != nil {
			continue
		}
		for _, item := range items {
			name := getResourceName(item)
			if name == "" || name != ns {
				continue
			}
			sanitized := sanitizeResource(item)
			if err := writeResource(nsDir, name, sanitized); err != nil {
				return count, err
			}
			count++
		}
	}
	if count > 0 {
		log.Info("Exported %d namespaces\n", count)
	}
	return count, nil
}

func exportClusterScopedResource(dir, kubecontext string, r APIResource, log *logger.Logger) (int, error) {
	items, err := getResourceItems(kubecontext, r.GroupResource(), "", "")
	if err != nil {
		return 0, err
	}

	resDir := filepath.Join(dir, "cluster-scoped", r.Name)
	count := 0
	for _, item := range items {
		if isSystemResource(item) {
			continue
		}
		name := getResourceName(item)
		if name == "" {
			continue
		}
		sanitized := sanitizeResource(item)
		if err := writeResource(resDir, name, sanitized); err != nil {
			return count, err
		}
		count++
	}
	if count > 0 {
		log.Debug("Exported %d %s (cluster-scoped)\n", count, r.Name)
	}
	return count, nil
}

func exportNamespacedResource(dir, kubecontext, namespace string, r APIResource, log *logger.Logger) (int, error) {
	items, err := getResourceItems(kubecontext, r.GroupResource(), namespace, "")
	if err != nil {
		return 0, err
	}

	resDir := filepath.Join(dir, "namespaced", namespace, r.Name)
	count := 0
	for _, item := range items {
		if isSystemResource(item) {
			continue
		}
		name := getResourceName(item)
		if name == "" {
			continue
		}
		sanitized := sanitizeResource(item)
		if err := writeResource(resDir, name, sanitized); err != nil {
			return count, err
		}
		count++
	}
	if count > 0 {
		log.Debug("Exported %d %s in %s\n", count, r.Name, namespace)
	}
	return count, nil
}

// getResourceItems fetches resources using kubectl and returns the items array.
// If name is non-empty, it fetches that specific resource; otherwise lists all.
func getResourceItems(kubecontext, resource, namespace, name string) ([]map[string]interface{}, error) {
	args := []string{"--context", kubecontext, "get", resource, "-o", "json"}
	if namespace != "" {
		args = append(args, "-n", namespace)
	}
	if name != "" {
		args = append(args, name)
	}

	cmd := execCommand("kubectl", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		return nil, fmt.Errorf("failed to parse kubectl output: %w", err)
	}

	// Single resource (when name is specified)
	if result["kind"] != nil && !strings.HasSuffix(result["kind"].(string), "List") {
		return []map[string]interface{}{result}, nil
	}

	// List of resources
	items, ok := result["items"].([]interface{})
	if !ok {
		return nil, nil
	}

	var resources []map[string]interface{}
	for _, item := range items {
		if m, ok := item.(map[string]interface{}); ok {
			resources = append(resources, m)
		}
	}
	return resources, nil
}

// sanitizeResource removes cluster-specific fields from a resource.
func sanitizeResource(res map[string]interface{}) map[string]interface{} {
	// Deep copy to avoid modifying the original
	result := deepCopyMap(res)

	// Remove status
	delete(result, "status")

	// Sanitize metadata
	if meta, ok := result["metadata"].(map[string]interface{}); ok {
		delete(meta, "resourceVersion")
		delete(meta, "uid")
		delete(meta, "creationTimestamp")
		delete(meta, "generation")
		delete(meta, "managedFields")
		delete(meta, "selfLink")

		// Remove last-applied-configuration annotation
		if annotations, ok := meta["annotations"].(map[string]interface{}); ok {
			delete(annotations, "kubectl.kubernetes.io/last-applied-configuration")
			if len(annotations) == 0 {
				delete(meta, "annotations")
			}
		}
	}

	return result
}

// isSystemResource returns true if the resource should be filtered out.
func isSystemResource(res map[string]interface{}) bool {
	meta, ok := res["metadata"].(map[string]interface{})
	if !ok {
		return true
	}

	name, _ := meta["name"].(string)
	namespace, _ := meta["namespace"].(string)
	kind, _ := res["kind"].(string)

	// Skip resources with ownerReferences (managed by a controller)
	if owners, ok := meta["ownerReferences"]; ok {
		if arr, ok := owners.([]interface{}); ok && len(arr) > 0 {
			return true
		}
	}

	// Skip the default kubernetes service
	if kind == "Service" && name == "kubernetes" && namespace == "default" {
		return true
	}

	// Skip auto-created kube-root-ca.crt ConfigMaps
	if kind == "ConfigMap" && name == "kube-root-ca.crt" {
		return true
	}

	// Skip default ServiceAccounts
	if kind == "ServiceAccount" && name == "default" {
		return true
	}

	return false
}

func getResourceName(res map[string]interface{}) string {
	meta, ok := res["metadata"].(map[string]interface{})
	if !ok {
		return ""
	}
	name, _ := meta["name"].(string)
	return name
}

func writeResource(dir, name string, res map[string]interface{}) error {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Sanitize filename: replace characters that are invalid in file paths
	safeName := strings.ReplaceAll(name, "/", "_")
	safeName = strings.ReplaceAll(safeName, ":", "_")

	data, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal resource %s: %w", name, err)
	}

	path := filepath.Join(dir, safeName+".yaml")
	return os.WriteFile(path, data, 0644)
}

func deepCopyMap(m map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m {
		switch val := v.(type) {
		case map[string]interface{}:
			result[k] = deepCopyMap(val)
		case []interface{}:
			result[k] = deepCopySlice(val)
		default:
			result[k] = v
		}
	}
	return result
}

func deepCopySlice(s []interface{}) []interface{} {
	result := make([]interface{}, len(s))
	for i, v := range s {
		switch val := v.(type) {
		case map[string]interface{}:
			result[i] = deepCopyMap(val)
		case []interface{}:
			result[i] = deepCopySlice(val)
		default:
			result[i] = v
		}
	}
	return result
}
