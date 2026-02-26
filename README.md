# klastr

CLI tool for deploying Kubernetes clusters with plugin support.

Create local clusters (kind/k3d) or deploy to existing clusters. Automatically install components like storage, ingress, cert-manager, monitoring, dashboard, ArgoCD and custom Helm apps — all defined in a single template file.

## Requirements

- Go 1.21+ | Docker | kubectl | [Helm](https://helm.sh/) 3.x
- For local clusters: [kind](https://kind.sigs.k8s.io/) or [k3d](https://k3d.io/)

## Installation

### macOS / Linux

**Using the install script (recommended):**
```bash
curl -fsSL https://raw.githubusercontent.com/alessandropitocchi/deploy-cluster/main/install.sh | bash
```

**Or download manually from GitHub Releases:**
```bash
# Download latest release (replace v1.0.0 with the latest version)
curl -LO https://github.com/alessandropitocchi/deploy-cluster/releases/latest/download/klastr_$(uname -s)_$(uname -m).tar.gz

# Extract
tar -xzf klastr_*.tar.gz

# Move to PATH
sudo mv klastr /usr/local/bin/
```

### Build from Source

```bash
go build -o klastr ./cmd/deploycluster
sudo mv klastr /usr/local/bin/
```

### Verify Installation

```bash
klastr --version
klastr check
```

## Quick Start

```bash
# Check prerequisites
./klastr check

# Generate a single template file
./klastr init

# Or generate a directory structure (recommended for complex setups)
./klastr init --dir --output my-cluster/

# Validate the template before creating
./klastr lint --template template.yaml
# Or for directory: ./klastr lint --template my-cluster/

# Create the cluster with all configured plugins
./klastr run --template template.yaml
# Or for directory: ./klastr run --template my-cluster/

# Check status
./klastr status --template template.yaml

# Update plugins without recreating the cluster
./klastr upgrade --template template.yaml

# Detect drift between cluster and template
./klastr drift --template template.yaml

# Switch kubectl context between clusters
./klastr switch my-cluster

# Save a snapshot of cluster resources
./klastr snapshot save my-snapshot --template template.yaml

# Restore a snapshot (preview first with --dry-run)
./klastr snapshot restore my-snapshot --dry-run --template template.yaml
./klastr snapshot restore my-snapshot --template template.yaml

# List and delete snapshots
./klastr snapshot list
./klastr snapshot delete my-snapshot

# Destroy the cluster
./klastr destroy --template template.yaml
```

### Multi-Environment Support

Manage multiple environments (dev, staging, production) with overlays:

```bash
# Create environments
./klastr env create dev
./klastr env create staging
./klastr env create production

# Deploy specific environment
./klastr run --environment dev
./klastr run --environment staging
./klastr run --environment production

# List and show environments
./klastr env list
./klastr env show production
```

See [Multi-Environment Configuration](docs/configuration.md#multi-environment-configuration) for details.

## Configuration Examples

### Local Cluster (kind)

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

### Existing Cluster

```yaml
name: my-production-cluster
provider:
  type: existing
  kubeconfig: ~/.kube/config
  context: production

plugins:
  certManager:
    enabled: true
  monitoring:
    enabled: true
    type: prometheus
    ingress:
      enabled: true
      host: grafana.mycompany.com
```

See [Existing Cluster Provider](docs/providers/existing.md) for more details.

## Directory-Based Configuration

For complex setups, you can organize your configuration across multiple files instead of a single `template.yaml`:

```bash
# Generate a directory structure
./klastr init --dir --output my-cluster/
```

This creates:
```
my-cluster/
├── klastr.yaml          # Main config (provider, cluster)
├── plugins/
│   ├── storage.yaml
│   ├── ingress.yaml
│   ├── cert-manager.yaml
│   └── ...
├── apps/                # Custom application configs
├── .env.example         # Environment variables template
└── README.md
```

**Benefits:**
- Better organization for complex configurations
- Easier to manage with Git (clearer diffs)
- Team members can work on different plugins independently
- Environment-specific overlays

**Loading order:**
1. `klastr.yaml` - Main configuration
2. `plugins/*.yaml` - Plugin configs (alphabetical)
3. `apps/*.yaml` - Custom apps (alphabetical)

Later files override earlier ones for simple fields. Lists (like custom apps) are additive.

## Available Plugins

| Plugin | Description | Installation |
|--------|-------------|--------------|
| [Storage](docs/plugins/storage.md) | StorageClass provisioner (local-path) | kubectl apply |
| [Ingress](docs/plugins/ingress.md) | NGINX controller for kind | kubectl apply |
| [Cert-Manager](docs/plugins/cert-manager.md) | TLS certificate management | kubectl apply |
| [External DNS](docs/plugins/external-dns.md) | Automatic DNS management | Helm |
| [Istio](docs/plugins/istio.md) | Service mesh with mTLS | istioctl |
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
klastr snapshot save before-upgrade --template template.yaml

# Save only specific namespaces
klastr snapshot save my-snap --namespace app,monitoring --template template.yaml

# Preview what a restore would apply
klastr snapshot restore before-upgrade --dry-run --template template.yaml

# Restore resources to the cluster
klastr snapshot restore before-upgrade --template template.yaml
```

Snapshots are stored at `~/.klastr/snapshots/<name>/` with one file per resource. The restore follows a dependency-aware order: CRDs → Namespaces → cluster-scoped → namespaced resources.

> **Note:** Snapshots may contain Kubernetes Secrets in plain text.

## Development

### Building from Source

```bash
# Clone the repository
git clone https://github.com/alessandropitocchi/deploy-cluster.git
cd deploy-cluster

# Build
make build

# Run tests
make test-short

# Create a release (requires git tag)
git tag v1.0.0
git push origin v1.0.0
# GitHub Actions will automatically create the release with binaries
```

### Project Structure

```
.
├── cmd/deploycluster/    # CLI commands
├── pkg/
│   ├── plugin/          # Plugin implementations
│   ├── provider/        # Cluster providers (kind, k3d)
│   ├── template/        # Configuration loading
│   └── env/             # Multi-environment support
├── docs/                # Documentation
├── examples/            # Example configurations
└── e2e/                 # End-to-end tests
```

## Documentation

| Section | Description |
|---------|-------------|
| [Getting Started](docs/getting-started.md) | Installation and first use |
| [CLI Commands](docs/commands.md) | Complete command reference |
| [Configuration](docs/configuration.md) | `template.yaml` file structure |
| [Architecture](docs/architecture.md) | Project architecture and design |
| [Provider: kind](docs/providers/kind.md) | kind provider details |
| [Provider: k3d](docs/providers/k3d.md) | k3d provider details |
| [Provider: existing](docs/providers/existing.md) | Existing cluster provider |
| [Plugins](docs/plugins/) | Documentation for each plugin |
