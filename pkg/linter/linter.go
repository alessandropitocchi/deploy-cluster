// Package linter provides template validation and best practice checks.
package linter

import (
	"fmt"
	"net"
	"regexp"
	"strings"

	"github.com/alepito/deploy-cluster/pkg/template"
)

// Severity represents the severity level of a lint issue.
type Severity string

const (
	SeverityError   Severity = "ERROR"
	SeverityWarning Severity = "WARN"
	SeverityInfo    Severity = "INFO"
)

// Issue represents a single lint issue.
type Issue struct {
	Severity Severity
	Path     string
	Message  string
}

// Result contains all issues found during linting.
type Result struct {
	Issues []Issue
	Valid  bool
}

// Linter checks templates for issues.
type Linter struct {
	strict bool
}

// New creates a new Linter.
func New(strict bool) *Linter {
	return &Linter{strict: strict}
}

// Lint checks a template for issues.
func (l *Linter) Lint(t *template.Template) *Result {
	var issues []Issue

	// Run all checks
	issues = append(issues, l.checkClusterName(t)...)
	issues = append(issues, l.checkKubernetesVersion(t)...)
	issues = append(issues, l.checkTopology(t)...)
	issues = append(issues, l.checkIngressHosts(t)...)
	issues = append(issues, l.checkResourceReferences(t)...)
	issues = append(issues, l.checkBestPractices(t)...)

	// Determine if valid (no errors)
	valid := true
	for _, issue := range issues {
		if issue.Severity == SeverityError {
			valid = false
			break
		}
	}

	return &Result{
		Issues: issues,
		Valid:  valid,
	}
}

// checkClusterName validates the cluster name.
func (l *Linter) checkClusterName(t *template.Template) []Issue {
	var issues []Issue

	if t.Name == "" {
		issues = append(issues, Issue{
			Severity: SeverityError,
			Path:     "name",
			Message:  "cluster name is required",
		})
		return issues
	}

	// Check valid DNS subdomain name
	if !isValidDNSName(t.Name) {
		issues = append(issues, Issue{
			Severity: SeverityError,
			Path:     "name",
			Message:  fmt.Sprintf("cluster name %q is not a valid DNS subdomain name (must match [a-z0-9]([-a-z0-9]*[a-z0-9])?)", t.Name),
		})
	}

	// Check length
	if len(t.Name) > 63 {
		issues = append(issues, Issue{
			Severity: SeverityError,
			Path:     "name",
			Message:  fmt.Sprintf("cluster name %q is too long (max 63 characters)", t.Name),
		})
	}

	// Warning for very short names
	if len(t.Name) < 3 {
		issues = append(issues, Issue{
			Severity: SeverityWarning,
			Path:     "name",
			Message:  fmt.Sprintf("cluster name %q is very short, consider a more descriptive name", t.Name),
		})
	}

	return issues
}

// checkKubernetesVersion validates the Kubernetes version.
func (l *Linter) checkKubernetesVersion(t *template.Template) []Issue {
	var issues []Issue

	if t.Cluster.Version == "" {
		// Default version is OK
		return issues
	}

	// Check version format (vX.Y.Z)
	versionRegex := regexp.MustCompile(`^v\d+\.\d+\.\d+$`)
	if !versionRegex.MatchString(t.Cluster.Version) {
		issues = append(issues, Issue{
			Severity: SeverityError,
			Path:     "cluster.version",
			Message:  fmt.Sprintf("Kubernetes version %q has invalid format (expected vX.Y.Z)", t.Cluster.Version),
		})
		return issues
	}

	// Check for very old versions
	oldVersions := []string{"v1.20", "v1.21", "v1.22", "v1.23", "v1.24"}
	for _, old := range oldVersions {
		if strings.HasPrefix(t.Cluster.Version, old) {
			issues = append(issues, Issue{
				Severity: SeverityWarning,
				Path:     "cluster.version",
				Message:  fmt.Sprintf("Kubernetes version %q is quite old, consider upgrading to v1.28+", t.Cluster.Version),
			})
			break
		}
	}

	// Check for EOL versions
	eolVersions := []string{"v1.19", "v1.18", "v1.17", "v1.16", "v1.15"}
	for _, eol := range eolVersions {
		if strings.HasPrefix(t.Cluster.Version, eol) {
			issues = append(issues, Issue{
				Severity: SeverityError,
				Path:     "cluster.version",
				Message:  fmt.Sprintf("Kubernetes version %q is end-of-life, please use v1.25 or newer", t.Cluster.Version),
			})
			break
		}
	}

	return issues
}

// checkTopology validates cluster topology.
func (l *Linter) checkTopology(t *template.Template) []Issue {
	var issues []Issue

	// Control planes
	if t.Cluster.ControlPlanes < 1 {
		issues = append(issues, Issue{
			Severity: SeverityError,
			Path:     "cluster.controlPlanes",
			Message:  "at least 1 control plane is required",
		})
	}

	if t.Cluster.ControlPlanes > 1 && t.Cluster.ControlPlanes%2 == 0 {
		issues = append(issues, Issue{
			Severity: SeverityWarning,
			Path:     "cluster.controlPlanes",
			Message:  fmt.Sprintf("using %d control planes (even number) is not recommended for etcd quorum, use odd numbers (1, 3, 5)", t.Cluster.ControlPlanes),
		})
	}

	// Workers
	if t.Cluster.Workers < 0 {
		issues = append(issues, Issue{
			Severity: SeverityError,
			Path:     "cluster.workers",
			Message:  "workers cannot be negative",
		})
	}

	if t.Cluster.Workers == 0 {
		issues = append(issues, Issue{
			Severity: SeverityInfo,
			Path:     "cluster.workers",
			Message:  "no worker nodes configured, workloads will run on control planes",
		})
	}

	return issues
}

// checkIngressHosts validates ingress host configurations.
func (l *Linter) checkIngressHosts(t *template.Template) []Issue {
	var issues []Issue
	hosts := make(map[string]string) // host -> path

	// Check monitoring ingress
	if t.Plugins.Monitoring != nil && t.Plugins.Monitoring.Enabled {
		if ing := t.Plugins.Monitoring.Ingress; ing != nil && ing.Enabled {
			if ing.Host == "" {
				issues = append(issues, Issue{
					Severity: SeverityError,
					Path:     "plugins.monitoring.ingress.host",
					Message:  "host is required when ingress is enabled",
				})
			} else {
				if otherPath, exists := hosts[ing.Host]; exists {
					issues = append(issues, Issue{
						Severity: SeverityError,
						Path:     "plugins.monitoring.ingress.host",
						Message:  fmt.Sprintf("host %q is already used by %s", ing.Host, otherPath),
					})
				} else {
					hosts[ing.Host] = "plugins.monitoring.ingress.host"
				}

				if !isValidHost(ing.Host) {
					issues = append(issues, Issue{
						Severity: SeverityWarning,
						Path:     "plugins.monitoring.ingress.host",
						Message:  fmt.Sprintf("host %q does not look like a valid hostname", ing.Host),
					})
				}
			}
		}
	}

	// Check dashboard ingress
	if t.Plugins.Dashboard != nil && t.Plugins.Dashboard.Enabled {
		if ing := t.Plugins.Dashboard.Ingress; ing != nil && ing.Enabled {
			if ing.Host == "" {
				issues = append(issues, Issue{
					Severity: SeverityError,
					Path:     "plugins.dashboard.ingress.host",
					Message:  "host is required when ingress is enabled",
				})
			} else {
				if otherPath, exists := hosts[ing.Host]; exists {
					issues = append(issues, Issue{
						Severity: SeverityError,
						Path:     "plugins.dashboard.ingress.host",
						Message:  fmt.Sprintf("host %q is already used by %s", ing.Host, otherPath),
					})
				} else {
					hosts[ing.Host] = "plugins.dashboard.ingress.host"
				}

				if !isValidHost(ing.Host) {
					issues = append(issues, Issue{
						Severity: SeverityWarning,
						Path:     "plugins.dashboard.ingress.host",
						Message:  fmt.Sprintf("host %q does not look like a valid hostname", ing.Host),
					})
				}
			}
		}
	}

	// Check ArgoCD ingress
	if t.Plugins.ArgoCD != nil && t.Plugins.ArgoCD.Enabled {
		if ing := t.Plugins.ArgoCD.Ingress; ing != nil && ing.Enabled {
			if ing.Host == "" {
				issues = append(issues, Issue{
					Severity: SeverityError,
					Path:     "plugins.argocd.ingress.host",
					Message:  "host is required when ingress is enabled",
				})
			} else {
				if otherPath, exists := hosts[ing.Host]; exists {
					issues = append(issues, Issue{
						Severity: SeverityError,
						Path:     "plugins.argocd.ingress.host",
						Message:  fmt.Sprintf("host %q is already used by %s", ing.Host, otherPath),
					})
				} else {
					hosts[ing.Host] = "plugins.argocd.ingress.host"
				}

				if !isValidHost(ing.Host) {
					issues = append(issues, Issue{
						Severity: SeverityWarning,
						Path:     "plugins.argocd.ingress.host",
						Message:  fmt.Sprintf("host %q does not look like a valid hostname", ing.Host),
					})
				}
			}
		}
	}

	// Check custom apps ingress
	for i, app := range t.Plugins.CustomApps {
		if app.Ingress != nil && app.Ingress.Enabled {
			if app.Ingress.Host == "" {
				issues = append(issues, Issue{
					Severity: SeverityError,
					Path:     fmt.Sprintf("plugins.customApps[%d].ingress.host", i),
					Message:  "host is required when ingress is enabled",
				})
			} else {
				if otherPath, exists := hosts[app.Ingress.Host]; exists {
					issues = append(issues, Issue{
						Severity: SeverityError,
						Path:     fmt.Sprintf("plugins.customApps[%d].ingress.host", i),
						Message:  fmt.Sprintf("host %q is already used by %s", app.Ingress.Host, otherPath),
					})
				} else {
					hosts[app.Ingress.Host] = fmt.Sprintf("plugins.customApps[%d].ingress.host", i)
				}

				if !isValidHost(app.Ingress.Host) {
					issues = append(issues, Issue{
						Severity: SeverityWarning,
						Path:     fmt.Sprintf("plugins.customApps[%d].ingress.host", i),
						Message:  fmt.Sprintf("host %q does not look like a valid hostname", app.Ingress.Host),
					})
				}
			}
		}
	}

	return issues
}

// checkResourceReferences validates references between resources.
func (l *Linter) checkResourceReferences(t *template.Template) []Issue {
	var issues []Issue

	// Check if ingress is needed but not enabled
	needsIngress := false
	needsIngressPaths := []string{}

	if t.Plugins.Monitoring != nil && t.Plugins.Monitoring.Enabled {
		if ing := t.Plugins.Monitoring.Ingress; ing != nil && ing.Enabled {
			needsIngress = true
			needsIngressPaths = append(needsIngressPaths, "plugins.monitoring.ingress")
		}
	}

	if t.Plugins.Dashboard != nil && t.Plugins.Dashboard.Enabled {
		if ing := t.Plugins.Dashboard.Ingress; ing != nil && ing.Enabled {
			needsIngress = true
			needsIngressPaths = append(needsIngressPaths, "plugins.dashboard.ingress")
		}
	}

	if t.Plugins.ArgoCD != nil && t.Plugins.ArgoCD.Enabled {
		if ing := t.Plugins.ArgoCD.Ingress; ing != nil && ing.Enabled {
			needsIngress = true
			needsIngressPaths = append(needsIngressPaths, "plugins.argocd.ingress")
		}
	}

	for i, app := range t.Plugins.CustomApps {
		if app.Ingress != nil && app.Ingress.Enabled {
			needsIngress = true
			needsIngressPaths = append(needsIngressPaths, fmt.Sprintf("plugins.customApps[%d].ingress", i))
		}
	}

	if needsIngress {
		if t.Plugins.Ingress == nil || !t.Plugins.Ingress.Enabled {
			issues = append(issues, Issue{
				Severity: SeverityError,
				Path:     "plugins.ingress",
				Message:  fmt.Sprintf("ingress plugin must be enabled when using ingress configurations: %v", needsIngressPaths),
			})
		}
	}

	// Check if cert-manager TLS is needed but not enabled
	if t.Plugins.ArgoCD != nil && t.Plugins.ArgoCD.Enabled {
		if ing := t.Plugins.ArgoCD.Ingress; ing != nil && ing.Enabled && ing.TLS {
			if t.Plugins.CertManager == nil || !t.Plugins.CertManager.Enabled {
				issues = append(issues, Issue{
					Severity: SeverityWarning,
					Path:     "plugins.certManager",
					Message:  "cert-manager should be enabled when using ArgoCD ingress with TLS",
				})
			}
		}
	}

	return issues
}

// checkBestPractices checks for best practice recommendations.
func (l *Linter) checkBestPractices(t *template.Template) []Issue {
	var issues []Issue

	// Storage recommendation for production-like clusters
	if t.Cluster.Workers >= 2 {
		if t.Plugins.Storage == nil || !t.Plugins.Storage.Enabled {
			issues = append(issues, Issue{
				Severity: SeverityInfo,
				Path:     "plugins.storage",
				Message:  "consider enabling storage plugin for multi-node clusters (PVC support)",
			})
		}
	}

	// Monitoring recommendation
	if t.Cluster.Workers >= 2 {
		if t.Plugins.Monitoring == nil || !t.Plugins.Monitoring.Enabled {
			issues = append(issues, Issue{
				Severity: SeverityInfo,
				Path:     "plugins.monitoring",
				Message:  "consider enabling monitoring for better observability",
			})
		}
	}

	// ArgoCD apps without repos
	if t.Plugins.ArgoCD != nil && t.Plugins.ArgoCD.Enabled {
		if len(t.Plugins.ArgoCD.Apps) > 0 && len(t.Plugins.ArgoCD.Repos) == 0 {
			issues = append(issues, Issue{
				Severity: SeverityWarning,
				Path:     "plugins.argocd",
				Message:  "ArgoCD has applications configured but no repositories defined",
			})
		}
	}

	// Check for duplicate custom app names
	appNames := make(map[string]int)
	for i, app := range t.Plugins.CustomApps {
		if idx, exists := appNames[app.Name]; exists {
			issues = append(issues, Issue{
				Severity: SeverityError,
				Path:     fmt.Sprintf("plugins.customApps[%d].name", i),
				Message:  fmt.Sprintf("duplicate app name %q (also at customApps[%d])", app.Name, idx),
			})
		} else {
			appNames[app.Name] = i
		}
	}

	// Check for empty ArgoCD repo URLs
	for i, repo := range t.Plugins.ArgoCD.Repos {
		if repo.URL == "" {
			issues = append(issues, Issue{
				Severity: SeverityError,
				Path:     fmt.Sprintf("plugins.argocd.repos[%d].url", i),
				Message:  "repository URL is required",
			})
		}
	}

	// Check for empty ArgoCD app names and repoURLs
	for i, app := range t.Plugins.ArgoCD.Apps {
		if app.Name == "" {
			issues = append(issues, Issue{
				Severity: SeverityError,
				Path:     fmt.Sprintf("plugins.argocd.apps[%d].name", i),
				Message:  "application name is required",
			})
		}
		if app.RepoURL == "" {
			issues = append(issues, Issue{
				Severity: SeverityError,
				Path:     fmt.Sprintf("plugins.argocd.apps[%d].repoURL", i),
				Message:  "application repoURL is required",
			})
		}
	}

	return issues
}

// isValidDNSName checks if a string is a valid DNS subdomain name.
func isValidDNSName(name string) bool {
	if name == "" {
		return false
	}
	// DNS subdomain regex: [a-z0-9]([-a-z0-9]*[a-z0-9])?
	regex := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	return regex.MatchString(name)
}

// isValidHost checks if a string looks like a valid hostname.
func isValidHost(host string) bool {
	if host == "" {
		return false
	}

	// Check for localhost (valid for local development)
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}

	// Try to parse as hostname
	if _, err := net.LookupHost(host); err == nil {
		return true
	}

	// Check basic hostname format
	regex := regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?)*$`)
	return regex.MatchString(host)
}

// FormatResult formats the lint result for display.
func FormatResult(result *Result) string {
	if len(result.Issues) == 0 {
		return "✓ No issues found!"
	}

	var lines []string
	lines = append(lines, "")

	for _, issue := range result.Issues {
		var icon string
		switch issue.Severity {
		case SeverityError:
			icon = "✗"
		case SeverityWarning:
			icon = "⚠"
		case SeverityInfo:
			icon = "ℹ"
		}
		lines = append(lines, fmt.Sprintf("  %s [%s] %s: %s", icon, issue.Severity, issue.Path, issue.Message))
	}

	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("Total: %d issues (%d errors, %d warnings, %d info)",
		len(result.Issues),
		countBySeverity(result.Issues, SeverityError),
		countBySeverity(result.Issues, SeverityWarning),
		countBySeverity(result.Issues, SeverityInfo),
	))

	return strings.Join(lines, "\n")
}

func countBySeverity(issues []Issue, sev Severity) int {
	count := 0
	for _, issue := range issues {
		if issue.Severity == sev {
			count++
		}
	}
	return count
}
