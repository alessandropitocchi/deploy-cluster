// Package drift provides drift detection capabilities for comparing cluster state with templates.
package drift

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/alepito/deploy-cluster/pkg/logger"
	"github.com/alepito/deploy-cluster/pkg/template"
)

// cmdRunner is a function type for executing commands, replaceable in tests.
type cmdRunner func(name string, arg ...string) *exec.Cmd

// execCommand is the default command runner.
var execCommand cmdRunner = exec.Command

// ChangeType represents the type of drift change.
type ChangeType string

const (
	ChangeTypeAdd     ChangeType = "ADD"     // Resource in cluster but not in template
	ChangeTypeRemove  ChangeType = "REMOVE"  // Resource in template but not in cluster
	ChangeTypeModify  ChangeType = "MODIFY"  // Resource differs between cluster and template
	ChangeTypeOrphan  ChangeType = "ORPHAN"  // Resource in cluster, unknown to template
)

// Change represents a single drift change.
type Change struct {
	Type       ChangeType
	Plugin     string
	Resource   string
	Property   string
	ClusterVal string
	TemplateVal string
	Message    string
}

// Result contains all drift detection results.
type Result struct {
	Changes []Change
	HasDrift bool
}

// Detector performs drift detection.
type Detector struct {
	log *logger.Logger
}

// NewDetector creates a new drift detector.
func NewDetector(log *logger.Logger) *Detector {
	return &Detector{log: log}
}

// Detect performs drift detection for all enabled plugins.
func (d *Detector) Detect(tmpl *template.Template, kubecontext string) (*Result, error) {
	var changes []Change

	d.log.Info("Detecting drift for cluster '%s'...\n", tmpl.Name)

	// Check storage drift
	if tmpl.Plugins.Storage != nil && tmpl.Plugins.Storage.Enabled {
		if c, err := d.detectStorageDrift(tmpl.Plugins.Storage, kubecontext); err == nil {
			changes = append(changes, c...)
		}
	} else {
		// Check for orphan storage
		if c, err := d.detectOrphanStorage(kubecontext); err == nil {
			changes = append(changes, c...)
		}
	}

	// Check ingress drift
	if tmpl.Plugins.Ingress != nil && tmpl.Plugins.Ingress.Enabled {
		if c, err := d.detectIngressDrift(tmpl.Plugins.Ingress, kubecontext); err == nil {
			changes = append(changes, c...)
		}
	} else {
		if c, err := d.detectOrphanIngress(kubecontext); err == nil {
			changes = append(changes, c...)
		}
	}

	// Check cert-manager drift
	if tmpl.Plugins.CertManager != nil && tmpl.Plugins.CertManager.Enabled {
		if c, err := d.detectCertManagerDrift(tmpl.Plugins.CertManager, kubecontext); err == nil {
			changes = append(changes, c...)
		}
	} else {
		if c, err := d.detectOrphanCertManager(kubecontext); err == nil {
			changes = append(changes, c...)
		}
	}

	// Check external-dns drift
	if tmpl.Plugins.ExternalDNS != nil && tmpl.Plugins.ExternalDNS.Enabled {
		if c, err := d.detectExternalDNSDrift(tmpl.Plugins.ExternalDNS, kubecontext); err == nil {
			changes = append(changes, c...)
		}
	} else {
		if c, err := d.detectOrphanExternalDNS(kubecontext); err == nil {
			changes = append(changes, c...)
		}
	}

	// Check istio drift
	if tmpl.Plugins.Istio != nil && tmpl.Plugins.Istio.Enabled {
		if c, err := d.detectIstioDrift(tmpl.Plugins.Istio, kubecontext); err == nil {
			changes = append(changes, c...)
		}
	} else {
		if c, err := d.detectOrphanIstio(kubecontext); err == nil {
			changes = append(changes, c...)
		}
	}

	// Check monitoring drift
	if tmpl.Plugins.Monitoring != nil && tmpl.Plugins.Monitoring.Enabled {
		if c, err := d.detectMonitoringDrift(tmpl.Plugins.Monitoring, kubecontext); err == nil {
			changes = append(changes, c...)
		}
	} else {
		if c, err := d.detectOrphanMonitoring(kubecontext); err == nil {
			changes = append(changes, c...)
		}
	}

	// Check dashboard drift
	if tmpl.Plugins.Dashboard != nil && tmpl.Plugins.Dashboard.Enabled {
		if c, err := d.detectDashboardDrift(tmpl.Plugins.Dashboard, kubecontext); err == nil {
			changes = append(changes, c...)
		}
	} else {
		if c, err := d.detectOrphanDashboard(kubecontext); err == nil {
			changes = append(changes, c...)
		}
	}

	// Check custom apps drift
	if c, err := d.detectCustomAppsDrift(tmpl.Plugins.CustomApps, kubecontext); err == nil {
		changes = append(changes, c...)
	}

	// Check ArgoCD drift
	if tmpl.Plugins.ArgoCD != nil && tmpl.Plugins.ArgoCD.Enabled {
		if c, err := d.detectArgoCDDrift(tmpl.Plugins.ArgoCD, kubecontext); err == nil {
			changes = append(changes, c...)
		}
	} else {
		if c, err := d.detectOrphanArgoCD(kubecontext); err == nil {
			changes = append(changes, c...)
		}
	}

	return &Result{
		Changes:  changes,
		HasDrift: len(changes) > 0,
	}, nil
}

// Storage drift detection
func (d *Detector) detectStorageDrift(cfg *template.StorageTemplate, kubecontext string) ([]Change, error) {
	var changes []Change

	// Check if storage is installed
	exists, err := d.resourceExists(kubecontext, "deployment", "local-path-provisioner", "local-path-storage")
	if err != nil {
		return nil, err
	}

	if !exists {
		changes = append(changes, Change{
			Type:     ChangeTypeRemove,
			Plugin:   "storage",
			Resource: "local-path-provisioner",
			Message:  "Storage plugin is enabled but not installed in cluster",
		})
	}

	return changes, nil
}

func (d *Detector) detectOrphanStorage(kubecontext string) ([]Change, error) {
	var changes []Change

	exists, err := d.resourceExists(kubecontext, "deployment", "local-path-provisioner", "local-path-storage")
	if err != nil {
		return nil, err
	}

	if exists {
		changes = append(changes, Change{
			Type:     ChangeTypeOrphan,
			Plugin:   "storage",
			Resource: "local-path-provisioner",
			Message:  "Storage plugin is installed but not enabled in template",
		})
	}

	return changes, nil
}

// Ingress drift detection
func (d *Detector) detectIngressDrift(cfg *template.IngressTemplate, kubecontext string) ([]Change, error) {
	var changes []Change

	var deploymentName, namespace string
	if cfg.Type == "nginx" {
		deploymentName = "ingress-nginx-controller"
		namespace = "ingress-nginx"
	} else {
		// Traefik
		deploymentName = "traefik"
		namespace = "kube-system"
	}

	exists, err := d.resourceExists(kubecontext, "deployment", deploymentName, namespace)
	if err != nil {
		return nil, err
	}

	if !exists {
		changes = append(changes, Change{
			Type:     ChangeTypeRemove,
			Plugin:   "ingress",
			Resource: deploymentName,
			Message:  fmt.Sprintf("Ingress (%s) is enabled but not installed in cluster", cfg.Type),
		})
	}

	return changes, nil
}

func (d *Detector) detectOrphanIngress(kubecontext string) ([]Change, error) {
	var changes []Change

	// Check for nginx
	exists, _ := d.resourceExists(kubecontext, "deployment", "ingress-nginx-controller", "ingress-nginx")
	if exists {
		changes = append(changes, Change{
			Type:     ChangeTypeOrphan,
			Plugin:   "ingress",
			Resource: "ingress-nginx-controller",
			Message:  "NGINX ingress is installed but not enabled in template",
		})
	}

	// Check for traefik
	exists, _ = d.resourceExists(kubecontext, "deployment", "traefik", "kube-system")
	if exists {
		changes = append(changes, Change{
			Type:     ChangeTypeOrphan,
			Plugin:   "ingress",
			Resource: "traefik",
			Message:  "Traefik ingress is installed but not enabled in template",
		})
	}

	return changes, nil
}

// Cert-manager drift detection
func (d *Detector) detectCertManagerDrift(cfg *template.CertManagerTemplate, kubecontext string) ([]Change, error) {
	var changes []Change

	exists, err := d.resourceExists(kubecontext, "deployment", "cert-manager", "cert-manager")
	if err != nil {
		return nil, err
	}

	if !exists {
		changes = append(changes, Change{
			Type:     ChangeTypeRemove,
			Plugin:   "cert-manager",
			Resource: "cert-manager",
			Message:  "Cert-manager is enabled but not installed in cluster",
		})
		return changes, nil
	}

	// Check version if specified
	if cfg.Version != "" {
		// Get installed version from deployment labels
		version, err := d.getDeploymentImageTag(kubecontext, "cert-manager", "cert-manager", "cert-manager")
		if err == nil && version != "" && !strings.Contains(version, cfg.Version) {
			changes = append(changes, Change{
				Type:        ChangeTypeModify,
				Plugin:      "cert-manager",
				Resource:    "cert-manager",
				Property:    "version",
				ClusterVal:  version,
				TemplateVal: cfg.Version,
				Message:     fmt.Sprintf("Version drift: cluster=%s, template=%s", version, cfg.Version),
			})
		}
	}

	return changes, nil
}

func (d *Detector) detectOrphanCertManager(kubecontext string) ([]Change, error) {
	var changes []Change

	exists, err := d.resourceExists(kubecontext, "deployment", "cert-manager", "cert-manager")
	if err != nil {
		return nil, err
	}

	if exists {
		changes = append(changes, Change{
			Type:     ChangeTypeOrphan,
			Plugin:   "cert-manager",
			Resource: "cert-manager",
			Message:  "Cert-manager is installed but not enabled in template",
		})
	}

	return changes, nil
}

// External DNS drift detection
func (d *Detector) detectExternalDNSDrift(cfg *template.ExternalDNSTemplate, kubecontext string) ([]Change, error) {
	var changes []Change

	exists, err := d.helmReleaseExists(kubecontext, "external-dns", "external-dns")
	if err != nil {
		return nil, err
	}

	if !exists {
		changes = append(changes, Change{
			Type:     ChangeTypeRemove,
			Plugin:   "external-dns",
			Resource: "external-dns",
			Message:  "External DNS is enabled but not installed in cluster",
		})
	}

	return changes, nil
}

func (d *Detector) detectOrphanExternalDNS(kubecontext string) ([]Change, error) {
	var changes []Change

	exists, err := d.helmReleaseExists(kubecontext, "external-dns", "external-dns")
	if err != nil {
		return nil, err
	}

	if exists {
		changes = append(changes, Change{
			Type:     ChangeTypeOrphan,
			Plugin:   "external-dns",
			Resource: "external-dns",
			Message:  "External DNS is installed but not enabled in template",
		})
	}

	return changes, nil
}

// Istio drift detection
func (d *Detector) detectIstioDrift(cfg *template.IstioTemplate, kubecontext string) ([]Change, error) {
	var changes []Change

	exists, err := d.resourceExists(kubecontext, "deployment", "istiod", "istio-system")
	if err != nil {
		return nil, err
	}

	if !exists {
		changes = append(changes, Change{
			Type:     ChangeTypeRemove,
			Plugin:   "istio",
			Resource: "istiod",
			Message:  "Istio is enabled but not installed in cluster",
		})
		return changes, nil
	}

	// Check ingress gateway
	if cfg.IngressGateway {
		exists, _ := d.resourceExists(kubecontext, "deployment", "istio-ingressgateway", "istio-system")
		if !exists {
			changes = append(changes, Change{
				Type:     ChangeTypeRemove,
				Plugin:   "istio",
				Resource: "istio-ingressgateway",
				Message:  "Istio ingress gateway is enabled but not installed",
			})
		}
	}

	return changes, nil
}

func (d *Detector) detectOrphanIstio(kubecontext string) ([]Change, error) {
	var changes []Change

	exists, err := d.resourceExists(kubecontext, "deployment", "istiod", "istio-system")
	if err != nil {
		return nil, err
	}

	if exists {
		changes = append(changes, Change{
			Type:     ChangeTypeOrphan,
			Plugin:   "istio",
			Resource: "istiod",
			Message:  "Istio is installed but not enabled in template",
		})
	}

	return changes, nil
}

// Monitoring drift detection
func (d *Detector) detectMonitoringDrift(cfg *template.MonitoringTemplate, kubecontext string) ([]Change, error) {
	var changes []Change

	exists, err := d.helmReleaseExists(kubecontext, "kube-prometheus-stack", "monitoring")
	if err != nil {
		return nil, err
	}

	if !exists {
		changes = append(changes, Change{
			Type:     ChangeTypeRemove,
			Plugin:   "monitoring",
			Resource: "kube-prometheus-stack",
			Message:  "Monitoring is enabled but not installed in cluster",
		})
		return changes, nil
	}

	// Check version drift
	if cfg.Version != "" {
		chartVersion, err := d.getHelmChartVersion(kubecontext, "kube-prometheus-stack", "monitoring")
		if err == nil && chartVersion != "" && chartVersion != cfg.Version {
			changes = append(changes, Change{
				Type:        ChangeTypeModify,
				Plugin:      "monitoring",
				Resource:    "kube-prometheus-stack",
				Property:    "version",
				ClusterVal:  chartVersion,
				TemplateVal: cfg.Version,
				Message:     fmt.Sprintf("Version drift: cluster=%s, template=%s", chartVersion, cfg.Version),
			})
		}
	}

	return changes, nil
}

func (d *Detector) detectOrphanMonitoring(kubecontext string) ([]Change, error) {
	var changes []Change

	exists, err := d.helmReleaseExists(kubecontext, "kube-prometheus-stack", "monitoring")
	if err != nil {
		return nil, err
	}

	if exists {
		changes = append(changes, Change{
			Type:     ChangeTypeOrphan,
			Plugin:   "monitoring",
			Resource: "kube-prometheus-stack",
			Message:  "Monitoring is installed but not enabled in template",
		})
	}

	return changes, nil
}

// Dashboard drift detection
func (d *Detector) detectDashboardDrift(cfg *template.DashboardTemplate, kubecontext string) ([]Change, error) {
	var changes []Change

	exists, err := d.helmReleaseExists(kubecontext, "headlamp", "headlamp")
	if err != nil {
		return nil, err
	}

	if !exists {
		changes = append(changes, Change{
			Type:     ChangeTypeRemove,
			Plugin:   "dashboard",
			Resource: "headlamp",
			Message:  "Dashboard is enabled but not installed in cluster",
		})
	}

	return changes, nil
}

func (d *Detector) detectOrphanDashboard(kubecontext string) ([]Change, error) {
	var changes []Change

	exists, err := d.helmReleaseExists(kubecontext, "headlamp", "headlamp")
	if err != nil {
		return nil, err
	}

	if exists {
		changes = append(changes, Change{
			Type:     ChangeTypeOrphan,
			Plugin:   "dashboard",
			Resource: "headlamp",
			Message:  "Dashboard is installed but not enabled in template",
		})
	}

	return changes, nil
}

// Custom apps drift detection
func (d *Detector) detectCustomAppsDrift(apps []template.CustomAppTemplate, kubecontext string) ([]Change, error) {
	var changes []Change

	// Get all Helm releases
	releases, err := d.getHelmReleases(kubecontext)
	if err != nil {
		return nil, err
	}

	// Check for apps that should be installed
	for _, app := range apps {
		found := false
		for _, release := range releases {
			if release == app.Name {
				found = true
				break
			}
		}
		if !found {
			changes = append(changes, Change{
				Type:     ChangeTypeRemove,
				Plugin:   "custom-apps",
				Resource: app.Name,
				Message:  fmt.Sprintf("Custom app %q is in template but not installed", app.Name),
			})
		}
	}

	// Check for orphan apps (Helm releases not in template)
	appNames := make(map[string]bool)
	for _, app := range apps {
		appNames[app.Name] = true
	}

	for _, release := range releases {
		// Skip known system releases
		if isSystemRelease(release) {
			continue
		}
		if !appNames[release] {
			changes = append(changes, Change{
				Type:     ChangeTypeOrphan,
				Plugin:   "custom-apps",
				Resource: release,
				Message:  fmt.Sprintf("Custom app %q is installed but not in template", release),
			})
		}
	}

	return changes, nil
}

// ArgoCD drift detection
func (d *Detector) detectArgoCDDrift(cfg *template.ArgoCDTemplate, kubecontext string) ([]Change, error) {
	var changes []Change

	namespace := cfg.Namespace
	if namespace == "" {
		namespace = "argocd"
	}

	exists, err := d.resourceExists(kubecontext, "deployment", "argocd-server", namespace)
	if err != nil {
		return nil, err
	}

	if !exists {
		changes = append(changes, Change{
			Type:     ChangeTypeRemove,
			Plugin:   "argocd",
			Resource: "argocd-server",
			Message:  "ArgoCD is enabled but not installed in cluster",
		})
		return changes, nil
	}

	// Check for repos in template but not in cluster
	for _, repo := range cfg.Repos {
		exists, _ := d.argocdRepoExists(kubecontext, repo.Name, namespace)
		if !exists {
			changes = append(changes, Change{
				Type:     ChangeTypeRemove,
				Plugin:   "argocd",
				Resource: fmt.Sprintf("repository/%s", repo.Name),
				Message:  fmt.Sprintf("ArgoCD repo %q is in template but not in cluster", repo.Name),
			})
		}
	}

	// Check for apps in template but not in cluster
	for _, app := range cfg.Apps {
		exists, _ := d.resourceExists(kubecontext, "application", app.Name, namespace)
		if !exists {
			changes = append(changes, Change{
				Type:     ChangeTypeRemove,
				Plugin:   "argocd",
				Resource: fmt.Sprintf("application/%s", app.Name),
				Message:  fmt.Sprintf("ArgoCD app %q is in template but not in cluster", app.Name),
			})
		}
	}

	return changes, nil
}

func (d *Detector) detectOrphanArgoCD(kubecontext string) ([]Change, error) {
	var changes []Change

	// Check default namespace
	exists, err := d.resourceExists(kubecontext, "deployment", "argocd-server", "argocd")
	if err != nil {
		return nil, err
	}

	if exists {
		changes = append(changes, Change{
			Type:     ChangeTypeOrphan,
			Plugin:   "argocd",
			Resource: "argocd-server",
			Message:  "ArgoCD is installed but not enabled in template",
		})
	}

	return changes, nil
}

// Helper methods

func (d *Detector) argocdRepoExists(kubecontext, name, namespace string) (bool, error) {
	// ArgoCD repos are stored as secrets with label argocd.argoproj.io/secret-type=repository
	cmd := execCommand("kubectl", "--context", kubecontext, "get", "secret", "-n", namespace,
		"-l", "argocd.argoproj.io/secret-type=repository",
		"-o", fmt.Sprintf("jsonpath={.items[?(@.metadata.name==\"%s\")].metadata.name}", name))
	out, err := cmd.Output()
	if err != nil {
		return false, nil
	}
	return len(out) > 0 && string(out) == name, nil
}

func (d *Detector) resourceExists(kubecontext, resourceType, name, namespace string) (bool, error) {
	cmd := execCommand("kubectl", "--context", kubecontext, "get", resourceType, name, "-n", namespace)
	if err := cmd.Run(); err != nil {
		return false, nil
	}
	return true, nil
}

func (d *Detector) helmReleaseExists(kubecontext, name, namespace string) (bool, error) {
	cmd := execCommand("helm", "status", name, "--namespace", namespace, "--kube-context", kubecontext)
	if err := cmd.Run(); err != nil {
		return false, nil
	}
	return true, nil
}

func (d *Detector) getDeploymentImageTag(kubecontext, name, namespace, containerName string) (string, error) {
	cmd := execCommand("kubectl", "--context", kubecontext, "get", "deployment", name, "-n", namespace,
		"-o", fmt.Sprintf("jsonpath={.spec.template.spec.containers[?(@.name==\"%s\")].image}", containerName))
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	image := string(out)
	parts := strings.Split(image, ":")
	if len(parts) > 1 {
		return parts[len(parts)-1], nil
	}
	return "", nil
}

func (d *Detector) getHelmChartVersion(kubecontext, name, namespace string) (string, error) {
	cmd := execCommand("helm", "list", "-n", namespace, "--kube-context", kubecontext, "-f", name, "-o", "json")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}

	var releases []struct {
		Chart string `json:"chart"`
	}
	if err := json.Unmarshal(out, &releases); err != nil {
		return "", err
	}

	if len(releases) > 0 {
		// Chart format: name-version
		parts := strings.Split(releases[0].Chart, "-")
		if len(parts) > 1 {
			return parts[len(parts)-1], nil
		}
	}

	return "", nil
}

func (d *Detector) getHelmReleases(kubecontext string) ([]string, error) {
	cmd := execCommand("helm", "list", "-A", "--kube-context", kubecontext, "-q")
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	var releases []string
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			releases = append(releases, line)
		}
	}

	return releases, nil
}

func isSystemRelease(name string) bool {
	systemReleases := []string{
		"kube-prometheus-stack",
		"headlamp",
		"external-dns",
	}
	for _, sys := range systemReleases {
		if name == sys {
			return true
		}
	}
	return false
}

// FormatResult formats the drift result for display.
func FormatResult(result *Result) string {
	if !result.HasDrift {
		return "✓ No drift detected. Cluster matches template."
	}

	var lines []string
	lines = append(lines, "")
	lines = append(lines, "Drift Detection Results:")
	lines = append(lines, strings.Repeat("-", 60))

	// Group by type
	var adds, removes, modifies, orphans []Change
	for _, c := range result.Changes {
		switch c.Type {
		case ChangeTypeAdd:
			adds = append(adds, c)
		case ChangeTypeRemove:
			removes = append(removes, c)
		case ChangeTypeModify:
			modifies = append(modifies, c)
		case ChangeTypeOrphan:
			orphans = append(orphans, c)
		}
	}

	// Print in order: Remove, Add, Modify, Orphan
	if len(removes) > 0 {
		lines = append(lines, "\n  Missing (in template, not in cluster):")
		for _, c := range removes {
			lines = append(lines, fmt.Sprintf("    - [%s] %s: %s", c.Plugin, c.Resource, c.Message))
		}
	}

	if len(adds) > 0 {
		lines = append(lines, "\n  To Add (will be created on next apply):")
		for _, c := range adds {
			lines = append(lines, fmt.Sprintf("    + [%s] %s: %s", c.Plugin, c.Resource, c.Message))
		}
	}

	if len(modifies) > 0 {
		lines = append(lines, "\n  Modified (drift detected):")
		for _, c := range modifies {
			lines = append(lines, fmt.Sprintf("    ~ [%s] %s: %s", c.Plugin, c.Resource, c.Message))
		}
	}

	if len(orphans) > 0 {
		lines = append(lines, "\n  Orphans (in cluster, not in template):")
		for _, c := range orphans {
			lines = append(lines, fmt.Sprintf("    ? [%s] %s: %s", c.Plugin, c.Resource, c.Message))
		}
	}

	lines = append(lines, "")
	lines = append(lines, strings.Repeat("-", 60))
	lines = append(lines, fmt.Sprintf("Total: %d drift items (%d missing, %d modified, %d orphans)",
		len(result.Changes), len(removes), len(modifies), len(orphans)))

	return strings.Join(lines, "\n")
}
