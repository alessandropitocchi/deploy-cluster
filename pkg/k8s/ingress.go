package k8s

import (
	"fmt"
	"sort"
	"strings"
)

// IngressConfig describes a Kubernetes Ingress resource to generate.
type IngressConfig struct {
	Name        string
	Namespace   string
	Host        string
	ServiceName string
	ServicePort int
	TLS         bool   // enable TLS section
	TLSSecret   string // secret name for TLS cert
	// Extra annotations merged on top of defaults.
	// Setting a key here overrides the default value.
	Annotations map[string]string
}

// IngressManifest returns a YAML manifest string for a Kubernetes Ingress resource.
// Default annotations: backend-protocol HTTP, ssl-redirect false.
// If TLS is true, ssl-redirect is set to "true" and a tls section is added.
func IngressManifest(cfg IngressConfig) string {
	port := cfg.ServicePort
	if port == 0 {
		port = 80
	}

	// Build annotations
	annotations := map[string]string{
		"nginx.ingress.kubernetes.io/backend-protocol": `"HTTP"`,
		"nginx.ingress.kubernetes.io/ssl-redirect":     `"false"`,
	}
	if cfg.TLS {
		annotations["nginx.ingress.kubernetes.io/ssl-redirect"] = `"true"`
	}
	for k, v := range cfg.Annotations {
		annotations[k] = v
	}

	// Sort annotation keys for deterministic output
	keys := make([]string, 0, len(annotations))
	for k := range annotations {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var annotationLines strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&annotationLines, "    %s: %s\n", k, annotations[k])
	}

	tlsSection := ""
	if cfg.TLS && cfg.TLSSecret != "" {
		tlsSection = fmt.Sprintf(`  tls:
    - hosts:
        - %s
      secretName: %s
`, cfg.Host, cfg.TLSSecret)
	}

	return fmt.Sprintf(`apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: %s
  namespace: %s
  annotations:
%sspec:
  ingressClassName: nginx
  rules:
    - host: %s
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: %s
                port:
                  number: %d
%s`, cfg.Name, cfg.Namespace, annotationLines.String(), cfg.Host, cfg.ServiceName, port, tlsSection)
}
