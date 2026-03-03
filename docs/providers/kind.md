# Provider: kind

[kind](https://kind.sigs.k8s.io/) (Kubernetes IN Docker) creates Kubernetes clusters using Docker containers as nodes.

## Configuration

```yaml
provider:
  type: kind
```

## How It Works

The kind provider:

1. Generates a kind configuration (`kind.x-k8s.io/v1alpha4`) based on `template.yaml`
2. Creates nodes as Docker containers (one per control plane, one per worker)
3. Automatically configures kubeconfig with context `kind-<cluster-name>`

## Automatic Ingress Configuration

When the **ingress** plugin is enabled, the provider automatically adds to the first control-plane node:

- **Port mapping** `80:80` and `443:443` — exposes HTTP/HTTPS ports on the host

This configuration happens at cluster creation time.

## Custom Port Mappings

You can expose additional ports from the cluster to your host machine using `extraPortMappings`:

```yaml
cluster:
  controlPlanes: 1
  workers: 2
  extraPortMappings:
    # Database access
    - containerPort: 5432
      hostPort: 5432
      protocol: TCP
    # Redis
    - containerPort: 6379
      hostPort: 6379
    # Custom service with specific bind address
    - containerPort: 8080
      hostPort: 8080
      listenAddress: 127.0.0.1
```

| Field | Description | Required |
|-------|-------------|----------|
| `containerPort` | Port inside the container/node | Yes |
| `hostPort` | Port on your host machine | Yes |
| `protocol` | `TCP` (default), `UDP`, or `SCTP` | No |
| `listenAddress` | Bind address on host (default: `0.0.0.0`) | No |

**Note:** Port mappings are applied to the **first control-plane node** only.

## Example Generated kind Config

With 1 control plane, 2 workers, and ingress enabled:

```yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
    image: kindest/node:v1.31.0
    kubeadmConfigPatches:
      - |
        kind: InitConfiguration
        nodeRegistration:
          kubeletExtraArgs:
            node-labels: "ingress-ready=true"
    extraPortMappings:
      - containerPort: 80
        hostPort: 80
        protocol: TCP
      - containerPort: 443
        hostPort: 443
        protocol: TCP
  - role: worker
    image: kindest/node:v1.31.0
  - role: worker
    image: kindest/node:v1.31.0
```

## Kubeconfig

The Kubernetes context follows the format `kind-<cluster-name>`:

```bash
# Use the context
kubectl cluster-info --context kind-my-cluster

# Export kubeconfig
klastr get kubeconfig my-cluster > kubeconfig.yaml
```

## Useful Commands

```bash
# List kind clusters
klastr get clusters
# or: kind get clusters

# List nodes
klastr get nodes my-cluster
# or: kind get nodes --name my-cluster

# Access a Docker node
docker exec -it my-cluster-control-plane bash
```

## Limitations

- Requires Docker to be running
- Ports 80/443 must be free on the host (if ingress is enabled)
- Custom port mappings (`extraPortMappings`) must not conflict with ports 80/443 when ingress is enabled
- The cluster does not survive Docker restarts (containers are stopped)
- Supports only one Kubernetes version per cluster
