# Existing Cluster Provider

The `existing` provider allows you to deploy-cluster plugins to an existing Kubernetes cluster that was created outside of deploy-cluster.

## Use Cases

- **Managed Kubernetes**: EKS, GKE, AKS, or other cloud-managed clusters
- **Existing kind/k3d clusters**: Clusters created manually or by other tools
- **On-premises clusters**: Self-managed Kubernetes clusters
- **CI/CD pipelines**: Clusters created by pipeline orchestrators

## Configuration

```yaml
name: my-cluster
provider:
  type: existing
  # Optional: path to kubeconfig file
  # Defaults to KUBECONFIG env var or ~/.kube/config
  kubeconfig: ~/.kube/config
  
  # Optional: kubectl context to use
  # Defaults to current context
  context: my-cluster-context

# Cluster section is optional for existing clusters
cluster:
  controlPlanes: 1
  workers: 0

plugins:
  # ... your plugins configuration
```

## How It Works

1. **Validation**: The provider verifies that:
   - The kubeconfig file exists and is readable
   - The cluster is accessible via `kubectl cluster-info`
   - The specified context exists (if provided)

2. **No Cluster Creation**: Unlike `kind` or `k3d` providers, the `existing` provider does not create or delete clusters. It only:
   - Validates connectivity
   - Installs/upgrades/uninstalls plugins
   - Runs drift detection

3. **Kubecontext**: The provider uses the specified context or the current context from kubeconfig.

## Examples

### Basic Usage

```bash
# Use current kubeconfig and context
deploy-cluster create --template template-existing.yaml

# Specify kubeconfig and context
deploy-cluster create --template template-existing.yaml
```

With template:
```yaml
name: production
provider:
  type: existing
  kubeconfig: ~/.kube/production-config
  context: prod-cluster
```

### With Environment Variables

```bash
export KUBECONFIG=~/.kube/my-cluster-config
deploy-cluster create --template template.yaml
```

### Drift Detection on Existing Cluster

```bash
deploy-cluster drift --template template-existing.yaml
```

### Upgrade Plugins

```bash
deploy-cluster upgrade --template template-existing.yaml
```

## Limitations

- **No Cluster Deletion**: The `destroy` command will fail with existing clusters
- **No Node Management**: Control plane and worker counts are informational only
- **Provider-Specific Features**: Some features (like kind's ingress-ready label) are not applicable

## Security Notes

- The kubeconfig is read but never modified
- Cluster credentials are not stored by deploy-cluster
- All kubectl/helm operations use the provided kubeconfig

## Troubleshooting

### "kubeconfig not found"

Ensure the kubeconfig path is correct:
```bash
ls -la ~/.kube/config
# or
export KUBECONFIG=/path/to/your/config
deploy-cluster create --template template.yaml
```

### "cannot connect to existing cluster"

Verify kubectl connectivity:
```bash
kubectl cluster-info --kubeconfig /path/to/config --context your-context
```

### Context not found

List available contexts:
```bash
kubectl config get-contexts --kubeconfig /path/to/config
```
