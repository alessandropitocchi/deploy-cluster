# CLI Commands

## Overview

| Command | Description |
|---------|-------------|
| [`init`](#init) | Generate a `template.yaml` file via interactive wizard |
| [`lint`](#lint) | Validate template for errors and best practices |
| [`create`](#create) | Create the cluster and install configured plugins |
| [`upgrade`](#upgrade) | Update plugins on an existing cluster |
| [`drift`](#drift) | Detect drift between cluster and template |
| [`uninstall`](#uninstall) | Uninstall plugins from a cluster (keeps cluster) |
| [`status`](#status) | Show cluster and plugin status |
| [`destroy`](#destroy) | Destroy the cluster |
| [`get`](#get) | Subcommands for retrieving cluster information |
| [`check`](#check) | Verify that all prerequisites are installed |
| [`switch`](#switch) | Switch kubectl context between clusters |
| [`snapshot`](#snapshot) | Save, restore, list, and delete cluster snapshots |

---

## `init`

Generates a `template.yaml` template file via an interactive wizard that guides you through plugin and option selection.

```bash
deploy-cluster init [flags]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-o, --output` | `template.yaml` | Output file path |

### Example

```bash
# Generate template.yaml in the current directory
deploy-cluster init

# Generate with custom name
deploy-cluster init -o my-cluster.yaml
```

The wizard asks in sequence:
1. **Cluster** — name, Kubernetes version, number of control planes and workers
2. **Plugins** — multi-select plugins to enable
3. **Ingress** — hostname for each service with a UI (if ingress enabled)
4. **ArgoCD** — namespace and version (if ArgoCD enabled)

---

## `lint`

Validates the template file for errors, warnings, and best practices before creating or upgrading a cluster.

```bash
deploy-cluster lint [flags]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-t, --template` | `template.yaml` | Template file |
| `--strict` | `false` | Treat warnings as errors |

### Checks Performed

| Category | Checks |
|----------|--------|
| **Cluster Name** | Valid DNS subdomain format, length (3-63 chars) |
| **Kubernetes Version** | Valid format (vX.Y.Z), EOL version detection |
| **Topology** | Control planes (odd numbers recommended), workers |
| **Ingress Hosts** | Uniqueness, valid hostname format |
| **Dependencies** | Ingress plugin required if ingress hosts configured |
| **Best Practices** | Storage for multi-node, monitoring, ArgoCD repos |

### Example

```bash
# Basic lint
$ deploy-cluster lint

  ✓ No issues found!

# Lint with warnings
$ deploy-cluster lint --template my-cluster.yaml

  ⚠ [WARN] cluster.controlPlanes: using 2 control planes (even number) is not recommended for etcd quorum, use odd numbers (1, 3, 5)
  ℹ [INFO] plugins.storage: consider enabling storage plugin for multi-node clusters (PVC support)

Total: 2 issues (0 errors, 1 warnings, 1 info)

# Strict mode (treats warnings as errors)
$ deploy-cluster lint --strict
```

---

## `create`

Creates a new Kubernetes cluster and installs all plugins enabled in the template.

```bash
deploy-cluster create [flags]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-t, --template` | `template.yaml` | Template file |
| `-e, --env` | `.env` | File with environment variables for secrets |
| `--timeout` | `5m` | Timeout for plugin operations (kubectl/helm) |

### Installation Order

Plugins are installed in this order:

1. **Storage** — to make PVCs available
2. **Ingress** — to expose services via hostname
3. **Cert-Manager** — for TLS certificates
4. **External DNS** — for automatic DNS management
5. **Istio** — service mesh (installs before apps that may use it)
6. **Monitoring** — Prometheus + Grafana
7. **Dashboard** — Headlamp
8. **Custom Apps** — custom Helm charts
9. **ArgoCD** — GitOps (last, as it may depend on others)

### Example

```bash
deploy-cluster create --template template.yaml
deploy-cluster create --template template.yaml --env production.env
deploy-cluster create --template template.yaml --timeout 10m
```

---

## `upgrade`

Updates an existing cluster by applying only the differences from the current template. The cluster is not recreated.

```bash
deploy-cluster upgrade [flags]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-t, --template` | `template.yaml` | Template file |
| `-e, --env` | `.env` | File with environment variables for secrets |
| `--dry-run` | `false` | Show changes without applying them |
| `--timeout` | `5m` | Timeout for plugin operations (kubectl/helm) |

### Per-plugin Behavior

| Plugin | Behavior |
|--------|----------|
| Storage | Re-apply manifest (idempotent) |
| Ingress | Re-apply manifest (idempotent) |
| Cert-Manager | Re-apply manifest (updates version if changed) |
| External DNS | `helm upgrade` (idempotent) |
| Istio | `istioctl install` (handles upgrades) |
| Monitoring | `helm upgrade` (idempotent) |
| Dashboard | `helm upgrade` (idempotent) |
| Custom Apps | `helm upgrade --install` for each app |
| ArgoCD | Re-apply manifest + diff repos/apps (adds new, removes those deleted from config) |

### Example

```bash
# Preview
deploy-cluster upgrade --template template.yaml --dry-run

# Apply
deploy-cluster upgrade --template template.yaml
```

---

## `drift`

Detects drift between the cluster's actual state and the desired state defined in the template. Identifies missing resources, orphan resources, and configuration differences.

```bash
deploy-cluster drift [flags]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-t, --template` | `template.yaml` | Template file |
| `-e, --env` | `.env` | Environment file |
| `--exit-error` | `false` | Exit with error code if drift detected |

### Drift Types

| Type | Description | Icon |
|------|-------------|------|
| **Missing** | In template but not in cluster | `-` |
| **Orphan** | In cluster but not in template | `?` |
| **Modified** | Different configuration | `~` |

### Example

```bash
# Basic drift detection
deploy-cluster drift

# With specific template
deploy-cluster drift --template production.yaml

# Exit with error if drift detected (useful for CI/CD)
deploy-cluster drift --exit-error
```

### Sample Output

```
Drift Detection Results:
------------------------------------------------------------

  Missing (in template, not in cluster):
    - [storage] local-path-provisioner: Storage plugin is enabled but not installed

  Orphans (in cluster, not in template):
    ? [custom-apps] redis: Custom app "redis" is installed but not in template

  Modified (drift detected):
    ~ [monitoring] kube-prometheus-stack: Version drift: cluster=72.6.0, template=72.6.2

------------------------------------------------------------
Total: 3 drift items (1 missing, 1 modified, 1 orphans)
```

See [Drift Detection](drift.md) for detailed documentation.

---

## `uninstall`

Uninstalls all enabled plugins from an existing cluster **without destroying the cluster itself**. Useful for resetting plugin state or cleaning up before a fresh plugin install.

```bash
deploy-cluster uninstall [flags]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-t, --template` | `template.yaml` | Template file |
| `-e, --env` | `.env` | Environment file for secrets |
| `--fail-fast` | `false` | Stop at first plugin failure |

### Uninstall Order

Plugins are uninstalled in reverse installation order:

1. **ArgoCD**
2. **Custom Apps**
3. **Dashboard**
4. **Monitoring**
5. **Cert-Manager**
6. **Ingress**
7. **Storage**

### Example

```bash
# Uninstall all plugins
deploy-cluster uninstall --template template.yaml

# Uninstall with fail-fast
deploy-cluster uninstall --template template.yaml --fail-fast
```

---

## `status`

Shows the current cluster status: existence, installed plugins, ArgoCD repositories and applications.

```bash
deploy-cluster status [flags]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-t, --template` | `template.yaml` | Template file |

### Example

```bash
deploy-cluster status --template template.yaml
```

---

## `destroy`

Destroys the cluster. Deletes all associated Docker containers.

```bash
deploy-cluster destroy [flags]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-t, --template` | `template.yaml` | Template file |
| `-n, --name` | - | Cluster name (overrides template) |

### Example

```bash
deploy-cluster destroy --template template.yaml
deploy-cluster destroy --name my-cluster
```

---

## `get`

Subcommands for retrieving cluster information.

### `get clusters`

List all existing kind clusters.

```bash
deploy-cluster get clusters
```

### `get nodes <name>`

List nodes of a specific cluster.

```bash
deploy-cluster get nodes my-cluster
```

### `get kubeconfig <name>`

Print the kubeconfig of a cluster.

```bash
deploy-cluster get kubeconfig my-cluster

# Save to file
deploy-cluster get kubeconfig my-cluster > kubeconfig.yaml
```

---

## `check`

Verifies that all required tools are installed and shows their versions.

```bash
deploy-cluster check
```

### Example

```bash
$ deploy-cluster check

Prerequisites
─────────────
✓ docker     27.5.1
✓ kind       v0.25.0
✓ kubectl    v1.31.4
✓ helm       v3.16.3

All prerequisites satisfied!
```

If a tool is missing:

```
✗ kind       not found → https://kind.sigs.k8s.io/
```

The command exits with an error if any prerequisite is missing.

---

## `switch`

Switches the active kubectl context to a kind cluster. Without arguments, lists all clusters and highlights the current context.

```bash
deploy-cluster switch [cluster-name]
```

### Example

```bash
# List clusters with current context marker
$ deploy-cluster switch

KIND CLUSTERS
─────────────
  my-cluster
● dev-cluster (current)
  staging

# Switch to a specific cluster
$ deploy-cluster switch my-cluster
Switched to context 'kind-my-cluster'
```

---

## `snapshot`

Manage cluster snapshots — export Kubernetes resources to disk or S3 and restore them later.

### `snapshot save <name>`

Export all non-system resources from the cluster to a local snapshot or S3.

```bash
deploy-cluster snapshot save <name> [flags]
```

#### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-t, --template` | `template.yaml` | Template file |
| `-e, --env` | `.env` | Environment file for secrets |
| `--namespace` | *(all non-system)* | Comma-separated list of namespaces to snapshot |
| `--exclude-secrets` | `false` | Exclude Kubernetes Secrets from the snapshot |
| `--s3` | `false` | Upload snapshot to S3 |
| `--s3-bucket` | *(env var)* | S3 bucket name |
| `--s3-prefix` | *(env var)* | S3 key prefix |
| `--s3-region` | *(env var)* | AWS region |
| `--s3-endpoint` | *(env var)* | S3 endpoint URL (for S3-compatible services) |

#### What gets exported

- Dynamic resource discovery via `kubectl api-resources` (captures CRDs too)
- One file per resource, stored at `~/.deploy-cluster/snapshots/<name>/` (local) or S3
- Sanitized: removes `resourceVersion`, `uid`, `managedFields`, `status`, etc.

#### What gets excluded

- **System namespaces**: `kube-system`, `kube-public`, `kube-node-lease`, `local-path-storage`
- **Transient resources**: events, endpoints, pods, replicasets, nodes, leases
- **Controller-managed**: resources with `ownerReferences`
- **Auto-created**: `kube-root-ca.crt` ConfigMaps, `default` ServiceAccounts, `kubernetes` Service

#### Examples

```bash
# Save all non-system resources locally
deploy-cluster snapshot save before-upgrade --template template.yaml

# Save only specific namespaces
deploy-cluster snapshot save my-snap --namespace app,monitoring --template template.yaml

# Save to S3 using flags
deploy-cluster snapshot save my-snap --s3 --s3-bucket my-backups --s3-prefix clusters/prod/

# Save to S3 using environment variables
export DEPLOY_CLUSTER_S3_BUCKET=my-backups
export DEPLOY_CLUSTER_S3_PREFIX=clusters/prod/
export AWS_REGION=us-east-1
deploy-cluster snapshot save my-snap --s3
```

### `snapshot restore <name>`

Restore resources from a saved snapshot to the cluster.

```bash
deploy-cluster snapshot restore <name> [flags]
```

#### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-t, --template` | `template.yaml` | Template file |
| `-e, --env` | `.env` | Environment file for secrets |
| `--dry-run` | `false` | Preview what would be applied without making changes |
| `--s3` | `false` | Restore snapshot from S3 |
| `--s3-bucket` | *(env var)* | S3 bucket name |
| `--s3-prefix` | *(env var)* | S3 key prefix |
| `--s3-region` | *(env var)* | AWS region |
| `--s3-endpoint` | *(env var)* | S3 endpoint URL |

#### Restore Order

Resources are applied in dependency order:

1. **CRDs** + 5s wait for propagation
2. **Namespaces**
3. **Cluster-scoped**: ClusterRoles → ClusterRoleBindings → PersistentVolumes → other
4. **Namespaced** (per namespace): ServiceAccounts → Secrets/ConfigMaps → PVCs → Services → Deployments/StatefulSets/DaemonSets → Ingresses → Jobs/CronJobs → custom resources

Each `kubectl apply` uses retry with exponential backoff for transient errors.

#### Examples

```bash
# Preview restore from local snapshot
deploy-cluster snapshot restore before-upgrade --dry-run --template template.yaml

# Restore from S3
deploy-cluster snapshot restore my-snap --s3 --s3-bucket my-backups --s3-prefix clusters/prod/

# Apply restore
deploy-cluster snapshot restore before-upgrade --template template.yaml
```

### `snapshot list`

Display all saved snapshots with metadata.

```bash
deploy-cluster snapshot list [flags]
```

#### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--s3` | `false` | List snapshots in S3 |
| `--s3-bucket` | *(env var)* | S3 bucket name |
| `--s3-prefix` | *(env var)* | S3 key prefix |

#### Examples

```bash
# List local snapshots
$ deploy-cluster snapshot list

SNAPSHOTS
─────────────────────────────────────────────────────────────
• before-upgrade
  Cluster: my-cluster (kind)
  Resources: 42
  Created: 2025-01-15 10:30:00

# List S3 snapshots
$ deploy-cluster snapshot list --s3 --s3-bucket my-backups

S3 SNAPSHOTS
─────────────────────────────────────────────────────────────
• before-upgrade
• after-migration
```

### `snapshot delete <name>`

Permanently delete a snapshot from disk or S3.

```bash
deploy-cluster snapshot delete <name> [flags]
```

#### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--s3` | `false` | Delete snapshot from S3 |
| `--s3-bucket` | *(env var)* | S3 bucket name |
| `--s3-prefix` | *(env var)* | S3 key prefix |

#### Examples

```bash
# Delete local snapshot
deploy-cluster snapshot delete before-upgrade

# Delete S3 snapshot
deploy-cluster snapshot delete my-snap --s3 --s3-bucket my-backups
```

---

## S3 Snapshot Storage

Snapshots can be stored in Amazon S3 or S3-compatible services (MinIO, Wasabi, etc.) for backup and disaster recovery.

### Configuration Methods

You can configure S3 in three ways (priority: flags > template config > env vars):

#### 1. Template Configuration (Recommended)

Add to your `template.yaml`:

```yaml
snapshot:
  enabled: true
  bucket: my-k8s-backups
  prefix: clusters/production/
  region: us-east-1
  # endpoint: http://localhost:9000  # For S3-compatible services
```

With this configuration, all snapshot commands automatically use S3:
```bash
deploy-cluster snapshot save my-backup    # Saves to S3 automatically
deploy-cluster snapshot list              # Lists from S3 automatically
deploy-cluster snapshot restore my-backup # Restores from S3 automatically
```

#### 2. Environment Variables

| Environment Variable | Description | Required |
|---------------------|-------------|----------|
| `DEPLOY_CLUSTER_S3_BUCKET` | S3 bucket name | Yes |
| `DEPLOY_CLUSTER_S3_PREFIX` | Key prefix (e.g., `clusters/prod/`) | No |
| `AWS_REGION` | AWS region | Yes |
| `AWS_ACCESS_KEY_ID` | AWS access key | Yes* |
| `AWS_SECRET_ACCESS_KEY` | AWS secret key | Yes* |
| `DEPLOY_CLUSTER_S3_ENDPOINT` | Custom endpoint for S3-compatible services | No |

\* Or use IAM roles/instance profiles when running on AWS.

#### 3. Command Flags

Use `--s3` flag with optional configuration flags:

```bash
deploy-cluster snapshot save my-backup --s3 \
  --s3-bucket my-k8s-backups \
  --s3-prefix clusters/production/ \
  --s3-region us-east-1
```

### Examples

#### AWS S3 with Template Config

```yaml
# template.yaml
name: production
provider:
  type: kind

snapshot:
  enabled: true
  bucket: my-k8s-backups
  prefix: clusters/production/
  region: us-east-1
```

```bash
# All commands automatically use S3
deploy-cluster snapshot save before-upgrade
deploy-cluster snapshot list
deploy-cluster snapshot restore before-upgrade
```

#### AWS S3 with Environment Variables

```bash
export DEPLOY_CLUSTER_S3_BUCKET=my-k8s-backups
export DEPLOY_CLUSTER_S3_PREFIX=clusters/production/
export AWS_REGION=us-east-1
export AWS_ACCESS_KEY_ID=AKIA...
export AWS_SECRET_ACCESS_KEY=...

# Must use --s3 flag with env var config
deploy-cluster snapshot save before-upgrade --s3
deploy-cluster snapshot list --s3
deploy-cluster snapshot restore before-upgrade --s3
```

#### MinIO (S3-Compatible)

```yaml
# template.yaml
snapshot:
  enabled: true
  bucket: k8s-backups
  prefix: clusters/
  region: us-east-1
  endpoint: http://localhost:9000
```

Or via environment:
```bash
export DEPLOY_CLUSTER_S3_BUCKET=k8s-backups
export DEPLOY_CLUSTER_S3_PREFIX=clusters/
export AWS_REGION=us-east-1
export AWS_ACCESS_KEY_ID=minioadmin
export AWS_SECRET_ACCESS_KEY=minioadmin
export DEPLOY_CLUSTER_S3_ENDPOINT=http://localhost:9000

deploy-cluster snapshot save my-snap --s3
```

### S3 Storage Structure

Snapshots are stored in S3 with the following structure:

```
s3://bucket-name/prefix/snapshot-name/
├── metadata.yaml
├── crds/
├── namespaces/
├── cluster-scoped/
└── namespaced/
    └── namespace-name/
        ├── deployments/
        ├── services/
        └── ...
```

> **Security note:** Snapshots may contain Kubernetes Secrets in plain text. Always encrypt your S3 buckets and use IAM policies to restrict access.
