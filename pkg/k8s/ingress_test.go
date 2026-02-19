package k8s

import (
	"strings"
	"testing"
)

func TestIngressManifest_Basic(t *testing.T) {
	m := IngressManifest(IngressConfig{
		Name:        "grafana-ingress",
		Namespace:   "monitoring",
		Host:        "grafana.local",
		ServiceName: "grafana",
		ServicePort: 80,
	})

	checks := []string{
		"name: grafana-ingress",
		"namespace: monitoring",
		"host: grafana.local",
		"name: grafana",
		"number: 80",
		"ingressClassName: nginx",
		`backend-protocol: "HTTP"`,
		`ssl-redirect: "false"`,
	}
	for _, c := range checks {
		if !strings.Contains(m, c) {
			t.Errorf("manifest missing %q", c)
		}
	}

	if strings.Contains(m, "tls:") {
		t.Error("basic manifest should not contain tls section")
	}
}

func TestIngressManifest_DefaultPort(t *testing.T) {
	m := IngressManifest(IngressConfig{
		Name:        "test-ingress",
		Namespace:   "default",
		Host:        "test.local",
		ServiceName: "test",
	})
	if !strings.Contains(m, "number: 80") {
		t.Error("default port should be 80")
	}
}

func TestIngressManifest_CustomPort(t *testing.T) {
	m := IngressManifest(IngressConfig{
		Name:        "test-ingress",
		Namespace:   "default",
		Host:        "test.local",
		ServiceName: "test",
		ServicePort: 8080,
	})
	if !strings.Contains(m, "number: 8080") {
		t.Error("should use custom port 8080")
	}
}

func TestIngressManifest_WithTLS(t *testing.T) {
	m := IngressManifest(IngressConfig{
		Name:        "argocd-ingress",
		Namespace:   "argocd",
		Host:        "argocd.example.com",
		ServiceName: "argocd-server",
		ServicePort: 80,
		TLS:         true,
		TLSSecret:   "argocd-tls",
		Annotations: map[string]string{
			"cert-manager.io/cluster-issuer": `"letsencrypt-prod"`,
		},
	})

	checks := []string{
		`ssl-redirect: "true"`,
		"tls:",
		"secretName: argocd-tls",
		"- argocd.example.com",
		`cert-manager.io/cluster-issuer: "letsencrypt-prod"`,
	}
	for _, c := range checks {
		if !strings.Contains(m, c) {
			t.Errorf("TLS manifest missing %q", c)
		}
	}
}

func TestIngressManifest_TLSWithoutSecret(t *testing.T) {
	m := IngressManifest(IngressConfig{
		Name:        "test-ingress",
		Namespace:   "default",
		Host:        "test.local",
		ServiceName: "test",
		TLS:         true,
	})

	// TLS flag set but no secret: ssl-redirect should be true but no tls section
	if !strings.Contains(m, `ssl-redirect: "true"`) {
		t.Error("TLS should set ssl-redirect to true")
	}
	if strings.Contains(m, "tls:") {
		t.Error("should not have tls section without TLSSecret")
	}
}

func TestIngressManifest_ExtraAnnotations(t *testing.T) {
	m := IngressManifest(IngressConfig{
		Name:        "test-ingress",
		Namespace:   "default",
		Host:        "test.local",
		ServiceName: "test",
		Annotations: map[string]string{
			"custom.io/foo": `"bar"`,
		},
	})

	if !strings.Contains(m, `custom.io/foo: "bar"`) {
		t.Error("should contain custom annotation")
	}
	// Default annotations should still be present
	if !strings.Contains(m, `backend-protocol: "HTTP"`) {
		t.Error("should still contain default backend-protocol annotation")
	}
}
