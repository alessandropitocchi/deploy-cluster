package k8s

import (
	"fmt"
)

// GatewayConfig describes a Gateway API Gateway resource to generate.
type GatewayConfig struct {
	Name             string
	Namespace        string
	GatewayClassName string // e.g., "traefik" or "nginx"
	Hosts            []string // Hosts to create listeners for (empty = all hosts)
	Port             int32    // Listener port (default: 8000 for Traefik)
	AllowAllRoutes   bool     // Allow routes from all namespaces (default: true)
}

// GatewayManifest returns a YAML manifest string for a Gateway API Gateway resource.
func GatewayManifest(cfg GatewayConfig) string {
	// Default port for Traefik
	port := cfg.Port
	if port == 0 {
		port = 8000
	}

	// Build allowed routes section
	allowedRoutes := ""
	if cfg.AllowAllRoutes {
		allowedRoutes = `
      allowedRoutes:
        namespaces:
          from: All`
	}

	// Build listeners for each unique host
	listeners := ""
	for i, host := range cfg.Hosts {
		if i > 0 {
			listeners += "\n"
		}
		listeners += fmt.Sprintf(`    - name: http-%d
      protocol: HTTP
      port: %d
      hostname: %s%s`, i, port, host, allowedRoutes)
	}

	// If no hosts specified, create a generic listener
	if len(cfg.Hosts) == 0 {
		listeners = fmt.Sprintf(`    - name: http
      protocol: HTTP
      port: %d%s`, port, allowedRoutes)
	}

	return fmt.Sprintf(`apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: %s
  namespace: %s
spec:
  gatewayClassName: %s
  listeners:
%s`, cfg.Name, cfg.Namespace, cfg.GatewayClassName, listeners)
}

// HTTPRouteConfig describes a Gateway API HTTPRoute resource to generate.
type HTTPRouteConfig struct {
	Name        string
	Namespace   string
	Host        string
	GatewayName string  // Name of the Gateway to attach to
	GatewayNamespace string // Namespace of the Gateway (empty = same namespace)
	ServiceName string
	ServicePort int32
	Path        string  // defaults to "/"
	PathType    string  // defaults to "PathPrefix"
}

// HTTPRouteManifest returns a YAML manifest string for a Gateway API HTTPRoute resource.
func HTTPRouteManifest(cfg HTTPRouteConfig) string {
	port := cfg.ServicePort
	if port == 0 {
		port = 80
	}

	path := cfg.Path
	if path == "" {
		path = "/"
	}

	pathType := cfg.PathType
	if pathType == "" {
		pathType = "PathPrefix"
	}

	// Build parent reference
	parentRef := fmt.Sprintf(`    - name: %s`, cfg.GatewayName)
	if cfg.GatewayNamespace != "" {
		parentRef += fmt.Sprintf(`
      namespace: %s`, cfg.GatewayNamespace)
	}

	return fmt.Sprintf(`apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: %s
  namespace: %s
spec:
  parentRefs:
%s
  hostnames:
    - %s
  rules:
    - matches:
        - path:
            type: %s
            value: %s
      backendRefs:
        - name: %s
          port: %d`, cfg.Name, cfg.Namespace, parentRef, cfg.Host, pathType, path, cfg.ServiceName, port)
}

// GetGatewayClassName returns the appropriate GatewayClass name for the ingress type.
func GetGatewayClassName(ingressType string) string {
	switch ingressType {
	case "traefik":
		return "traefik"
	case "nginx-gateway-fabric":
		return "nginx"
	default:
		return "traefik"
	}
}
