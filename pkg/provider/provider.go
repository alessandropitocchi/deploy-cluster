package provider

import "github.com/alessandropitocchi/deploy-cluster/pkg/template"

// Provider defines the interface for cluster providers (kind, k3d, etc.)
type Provider interface {
	// Name returns the provider name
	Name() string

	// Create creates a new cluster with the given template
	Create(tmpl *template.Template) error

	// Delete removes the cluster
	Delete(name string) error

	// GetKubeconfig returns the kubeconfig for the cluster
	GetKubeconfig(name string) (string, error)

	// Exists checks if a cluster with the given name exists
	Exists(name string) (bool, error)

	// KubeContext returns the kubectl context name for the cluster
	KubeContext(name string) string
}
