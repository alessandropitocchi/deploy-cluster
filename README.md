# deploy-cluster

CLI tool for deploying local Kubernetes clusters with plugin support.

Create clusters with configurable topology and automatically install components like storage, ingress, cert-manager, monitoring, dashboard, ArgoCD and custom Helm apps — all defined in a single template file.

## Requirements

- Go 1.21+ | Docker | [kind](https://kind.sigs.k8s.io/) | kubectl | [Helm](https://helm.sh/) 3.x

## Installation

```bash
go build -o deploy-cluster ./cmd/deploycluster
```

## Quick Start

```bash
# Check prerequisites
./deploy-cluster check

# Interactive wizard to generate the template
./deploy-cluster init

# Validate the template before creating
./deploy-cluster lint --template template.yaml

# Create the cluster with all configured plugins
./deploy-cluster create --template template.yaml

# Check status
./deploy-cluster status --template template.yaml

# Update plugins without recreating the cluster
./deploy-cluster upgrade --template template.yaml

# Switch kubectl context between clusters
./deploy-cluster switch my-cluster

# Save a snapshot of cluster resources
./deploy-cluster snapshot save my-snapshot --template template.yaml

# Restore a snapshot (preview first with --dry-run)
./deploy-cluster snapshot restore my-snapshot --dry-run --template template.yaml
./deploy-cluster snapshot restore my-snapshot --template template.yaml

# List and delete snapshots
./deploy-cluster snapshot list
./deploy-cluster snapshot delete my-snapshot

# Destroy the cluster
./deploy-cluster destroy --template template.yaml
```

## Configuration Example

```yaml
name: my-cluster
provider:
  type: kind
cluster:
  controlPlanes: 1
  workers: 2
  version: v1.31.0
plugins:
  storage:
    enabled: true
    type: local-path
  ingress:
    enabled: true
    type: nginx
  certManager:
    enabled: true
  externalDNS:
    enabled: true
    provider: cloudflare
    zone: example.com
    credentials:
      apiToken: ${CF_API_TOKEN}
  monitoring:
    enabled: true
    type: prometheus
    ingress:
      enabled: true
      host: grafana.localhost
  dashboard:
    enabled: true
    type: headlamp
    ingress:
      enabled: true
      host: headlamp.localhost
  customApps:
    - name: redis
      chart: oci://registry-1.docker.io/bitnamicharts/redis
      version: "21.1.5"
      namespace: redis
      values:
        architecture: standalone
  argocd:
    enabled: true
    ingress:
      enabled: true
      host: argocd.localhost
```

## Available Plugins

| Plugin | Description | Installation |
|--------|-------------|--------------|
| [Storage](docs/plugins/storage.md) | StorageClass provisioner (local-path) | kubectl apply |
| [Ingress](docs/plugins/ingress.md) | NGINX controller for kind | kubectl apply |
| [Cert-Manager](docs/plugins/cert-manager.md) | TLS certificate management | kubectl apply |
| [External DNS](docs/plugins/external-dns.md) | Automatic DNS management | Helm |
| [Monitoring](docs/plugins/monitoring.md) | Prometheus + Grafana | Helm |
| [Dashboard](docs/plugins/dashboard.md) | Headlamp | Helm |
| [Custom Apps](docs/plugins/custom-apps.md) | Custom Helm charts | Helm |
| [ArgoCD](docs/plugins/argocd.md) | GitOps | kubectl apply |

## UI Access

With ingress enabled:

| Service | URL | Credentials |
|---------|-----|-------------|
| Grafana | `http://grafana.localhost` | admin / prom-operator |
| Headlamp | `http://headlamp.localhost` | `kubectl create token headlamp -n headlamp` |
| ArgoCD | `http://argocd.localhost` | admin / `kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" \| base64 -d` |

## Snapshots

The `snapshot` command exports Kubernetes resources from a running cluster to disk and can restore them later. Useful for backup/restore, cluster migration, and disaster recovery.

```bash
# Save all non-system resources
deploy-cluster snapshot save before-upgrade --template template.yaml

# Save only specific namespaces
deploy-cluster snapshot save my-snap --namespace app,monitoring --template template.yaml

# Preview what a restore would apply
deploy-cluster snapshot restore before-upgrade --dry-run --template template.yaml

# Restore resources to the cluster
deploy-cluster snapshot restore before-upgrade --template template.yaml
```

Snapshots are stored at `~/.deploy-cluster/snapshots/<name>/` with one file per resource. The restore follows a dependency-aware order: CRDs → Namespaces → cluster-scoped → namespaced resources.

> **Note:** Snapshots may contain Kubernetes Secrets in plain text.

## Documentation

| Section | Description |
|---------|-------------|
| [Getting Started](docs/getting-started.md) | Installation and first use |
| [CLI Commands](docs/commands.md) | Complete command reference |
| [Configuration](docs/configuration.md) | `template.yaml` file structure |
| [Architecture](docs/architecture.md) | Project architecture and design |
| [Provider: kind](docs/providers/kind.md) | kind provider details |
| [Plugins](docs/plugins/) | Documentation for each plugin |
