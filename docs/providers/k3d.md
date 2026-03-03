# k3d Provider

The `k3d` provider creates local Kubernetes clusters using [k3d](https://k3d.io/), a lightweight wrapper to run [k3s](https://k3s.io/) (Rancher Lab's minimal Kubernetes distribution) in Docker.

## Features

- **Lightweight**: Uses k3s, a lightweight Kubernetes distribution
- **Fast**: Quick cluster creation and teardown
- **Built-in ServiceLB**: Includes built-in service load balancer
- **Gateway API Support**: Traefik and NGINX Gateway Fabric with Gateway API
- **Multi-node**: Support for control planes and worker nodes

## Requirements

- [Docker](https://docs.docker.com/get-docker/)
- [k3d](https://k3d.io/v5.6.0/#installation) v5.x or later

Install k3d:
```bash
# macOS
brew install k3d

# Linux
curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | bash

# Windows (via Chocolatey)
choco install k3d
```

## Configuration

```yaml
name: my-k3d-cluster
provider:
  type: k3d
cluster:
  controlPlanes: 1    # Number of control plane nodes (server nodes)
  workers: 2          # Number of worker nodes (agent nodes)
  version: v1.31.0    # Optional: Kubernetes version

plugins:
  storage:
    enabled: true
    type: local-path
  
  ingress:
    enabled: true
    type: traefik    # or nginx-gateway-fabric
  
  # ... other plugins
```

## Provider-Specific Behavior

### Ingress Controller

k3d has different ingress handling compared to kind:

- **Traefik**: Full installation with Gateway API support
  - Set `plugins.ingress.type: traefik`
  - Installs via Helm with Gateway API enabled
  - k3d's built-in Traefik is disabled
  
- **NGINX Gateway Fabric**: F5's Gateway API implementation
  - Set `plugins.ingress.type: nginx-gateway-fabric`
  - Installs official manifests from GitHub releases
  - k3d's built-in Traefik is disabled automatically

### Port Exposure

k3d automatically exposes ports via the load balancer. The provider configures:

- Port 80 → LoadBalancer → Ingress Controller
- Port 443 → LoadBalancer → Ingress Controller (if TLS enabled)

### Storage

k3s includes `local-path-provisioner` by default. When you enable the storage plugin:
- If using k3d's default setup, the local-path-provisioner is already present
- The provider will verify it's running

## Examples

### Basic Single-Node Cluster

```yaml
name: k3d-single
provider:
  type: k3d
cluster:
  controlPlanes: 1
  workers: 0
```

### Multi-Node Cluster with Traefik

```yaml
name: k3d-multi
provider:
  type: k3d
cluster:
  controlPlanes: 1
  workers: 2

plugins:
  ingress:
    enabled: true
    type: traefik
  
  monitoring:
    enabled: true
    type: prometheus
    ingress:
      enabled: true
      host: grafana.localhost
```

### With NGINX Gateway Fabric

```yaml
name: k3d-nginx-gateway
provider:
  type: k3d
cluster:
  controlPlanes: 1
  workers: 2

plugins:
  ingress:
    enabled: true
    type: nginx-gateway-fabric  # This will disable built-in Traefik
```

## Commands

```bash
# Create k3d cluster
klastr run --template template-k3d.yaml

# Check status
klastr status --template template-k3d.yaml

# Upgrade plugins
klastr upgrade --template template-k3d.yaml

# Destroy cluster
klastr destroy --template template-k3d.yaml
```

## k3d vs kind

| Feature | kind | k3d |
|---------|------|-----|
| Kubernetes | Standard | k3s (lightweight) |
| Resource Usage | Higher | Lower |
| Startup Time | Slower | Faster |
| Default Ingress | None | Traefik |
| Multi-node | Yes | Yes |
| Docker-in-Docker | Native | Requires configuration |

## Troubleshooting

### "k3d not found"

Install k3d:
```bash
curl -s https://raw.githubusercontent.com/k3d-io/k3d/main/install.sh | bash
```

### Port conflicts

k3d uses ports 80 and 443 by default. If these are in use:
```bash
# Check what's using the ports
lsof -i :80
lsof -i :443

# Or use a different port mapping in k3d config
```

### Cluster fails to start

Check Docker resources:
```bash
# Check Docker is running
docker info

# Check available resources
docker system df
```

### Ingress not working

Verify the ingress controller is running:
```bash
# Traefik
kubectl get pods -n traefik

# NGINX Gateway Fabric
kubectl get pods -n nginx-gateway

# Check Gateway classes
kubectl get gatewayclass
```

## See Also

- [k3d Documentation](https://k3d.io/v5.6.0/)
- [k3s Documentation](https://docs.k3s.io/)
- [Provider: kind](kind.md) - Alternative local provider
