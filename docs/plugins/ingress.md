# Plugin: Ingress

Installs an ingress controller with Gateway API support in the cluster to expose services via HTTP/HTTPS through hostnames.

## Configuration

```yaml
plugins:
  ingress:
    enabled: true
    type: traefik  # or nginx-gateway-fabric
```

| Field | Type | Default | Required | Description |
|-------|------|---------|:---:|-------------|
| `enabled` | bool | `false` | yes | Enable the plugin |
| `type` | string | - | yes | Controller type: `traefik` or `nginx-gateway-fabric` |
| `version` | string | - | no | Chart version (default: latest) |
| `values` | object | - | no | Additional Helm values (inline) |
| `valuesFile` | string | - | no | Path to external values file |

## Supported Types

### `traefik`

[Traefik](https://traefik.io/) — modern HTTP reverse proxy and load balancer with native Gateway API support.

Features:
- Native Gateway API support
- Traditional Ingress support
- Automatic HTTPS with Let's Encrypt
- Middleware support

Installation method: Helm chart from `https://traefik.github.io/charts`

Creates:
- Traefik deployment in namespace `traefik`
- Gateway `shared-gateway` in namespace `traefik` (for HTTPRoutes)

### `nginx-gateway-fabric`

[NGINX Gateway Fabric](https://github.com/nginxinc/nginx-gateway-fabric) — F5's implementation of the Gateway API using NGINX.

Features:
- Full Gateway API conformance
- High performance NGINX data plane
- Suitable for production workloads
- Advanced traffic management

Installation method: Helm OCI chart from `oci://ghcr.io/nginx/charts/nginx-gateway-fabric`

Creates:
- NGINX Gateway Fabric deployment in namespace `nginx-gateway`
- Gateway `shared-gateway` in namespace `nginx-gateway` (for HTTPRoutes)

## Prerequisites

### kind Provider

The kind provider automatically configures the control-plane node with:
- Port mapping `80:80` and `443:443`

This happens at cluster creation time.

### k3d Provider

The k3d provider automatically configures:
- Port mapping `80:80` and `443:443` on the loadbalancer
- Traefik is disabled when using `nginx-gateway-fabric` (k3d ships Traefik by default)

## How It Works

### Traditional Ingress (Traefik)

Create `Ingress` resources to expose services:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-app
spec:
  ingressClassName: traefik
  rules:
    - host: myapp.localhost
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: my-app
                port:
                  number: 80
```

### Gateway API (Recommended)

Create `Gateway` and `HTTPRoute` resources:

```yaml
apiVersion: gateway.networking.k8s.io/v1
kind: Gateway
metadata:
  name: my-gateway
spec:
  gatewayClassName: traefik-gateway  # or "nginx" for nginx-gateway-fabric
  listeners:
    - name: http
      protocol: HTTP
      port: 80
---
apiVersion: gateway.networking.k8s.io/v1
kind: HTTPRoute
metadata:
  name: my-route
spec:
  parentRefs:
    - name: my-gateway
  hostnames:
    - myapp.localhost
  rules:
    - matches:
        - path:
            type: PathPrefix
            value: /
      backendRefs:
        - name: my-app
          port: 80
```

The service becomes reachable at `http://myapp.localhost`.

## Integration with Other Plugins

When ingress is enabled, the plugin creates a **shared Gateway** that other plugins use to expose their UI via **HTTPRoute** resources (Gateway API).

| Plugin | Default Hostname | Configuration | Resource Created |
|--------|-----------------|---------------|------------------|
| Monitoring (Grafana) | `grafana.localhost` | `monitoring.ingress.host` | HTTPRoute |
| Dashboard (Headlamp) | `headlamp.localhost` | `dashboard.ingress.host` | HTTPRoute |
| ArgoCD | `argocd.localhost` | `argocd.ingress.host` | HTTPRoute |
| Custom Apps | configurable | `customApps[].ingress.host` | HTTPRoute |

## Verification

### Traefik

```bash
# Controller pods
kubectl get pods -n traefik

# Gateway classes
kubectl get gatewayclass

# Created gateways and routes
kubectl get gateway,httproute --all-namespaces

# Test
curl http://argocd.localhost
```

### NGINX Gateway Fabric

```bash
# Controller pods
kubectl get pods -n nginx-gateway

# Gateway classes
kubectl get gatewayclass

# Test
curl http://argocd.localhost
```

## Custom Values

Both ingress controllers support custom Helm values:

```yaml
plugins:
  ingress:
    enabled: true
    type: traefik
    version: "34.0.0"  # Optional: specify chart version
    values:
      replicaCount: 2
      resources:
        requests:
          memory: "256Mi"
          cpu: "250m"
      additionalArguments:
        - "--log.level=DEBUG"
```

Or use an external values file:
```yaml
plugins:
  ingress:
    enabled: true
    type: traefik
    version: "34.0.0"
    valuesFile: ./traefik-values.yaml
```

## Notes

- Host ports 80 and 443 must be free
- On macOS/Linux, `*.localhost` resolves automatically to `127.0.0.1`
- For custom hostnames, add an entry to `/etc/hosts`
- Gateway API is the future standard; consider migrating from traditional Ingress
