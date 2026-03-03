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

## Supported Types

### `traefik`

[Traefik](https://traefik.io/) — modern HTTP reverse proxy and load balancer with native Gateway API support.

Features:
- Native Gateway API support (experimental)
- Traditional Ingress support
- Automatic HTTPS with Let's Encrypt
- Middleware support

Installation method: Helm chart from `https://traefik.github.io/charts`

### `nginx-gateway-fabric`

[NGINX Gateway Fabric](https://github.com/nginxinc/nginx-gateway-fabric) — F5's implementation of the Gateway API using NGINX.

Features:
- Full Gateway API conformance
- High performance NGINX data plane
- Suitable for production workloads
- Advanced traffic management

Installation method: Official manifests from GitHub releases

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

When ingress is enabled, several plugins can automatically create an Ingress for their UI:

| Plugin | Default Hostname | Configuration |
|--------|-----------------|---------------|
| Monitoring (Grafana) | `grafana.localhost` | `monitoring.ingress.host` |
| Dashboard (Headlamp) | `headlamp.localhost` | `dashboard.ingress.host` |
| ArgoCD | `argocd.localhost` | `argocd.ingress.host` |
| Custom Apps | configurable | `customApps[].ingress.host` |

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

## Notes

- Host ports 80 and 443 must be free
- On macOS/Linux, `*.localhost` resolves automatically to `127.0.0.1`
- For custom hostnames, add an entry to `/etc/hosts`
- Gateway API is the future standard; consider migrating from traditional Ingress
