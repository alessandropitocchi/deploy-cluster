# Drift Detection

The `drift` command detects differences between your cluster's actual state and the desired state defined in your template.

## Overview

Drift occurs when the cluster state diverges from the template configuration. This can happen due to:
- Manual changes made directly to the cluster
- Failed or partial deployments
- Resources deleted outside of deploy-cluster
- Version upgrades applied outside of deploy-cluster

## Usage

```bash
deploy-cluster drift [flags]
```

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-t, --template` | `template.yaml` | Template file |
| `-e, --env` | `.env` | Environment file |
| `--exit-error` | `false` | Exit with error code if drift detected |

## Drift Types

### Missing Resources
Resources defined in the template but not present in the cluster:

```
- [monitoring] kube-prometheus-stack: Monitoring is enabled but not installed in cluster
```

### Orphan Resources
Resources present in the cluster but not defined in the template:

```
? [custom-apps] redis: Custom app "redis" is installed but not in template
```

### Modified Resources
Resources where configuration differs between cluster and template:

```
~ [cert-manager] cert-manager: Version drift: cluster=v1.16.2, template=v1.16.3
```

## Examples

### Basic Drift Detection

```bash
deploy-cluster drift
```

Output:
```
Drift Detection Results:
------------------------------------------------------------

  Missing (in template, not in cluster):
    - [storage] local-path-provisioner: Storage plugin is enabled but not installed in cluster

  Orphans (in cluster, not in template):
    ? [custom-apps] mysql: Custom app "mysql" is installed but not in template

  Modified (drift detected):
    ~ [monitoring] kube-prometheus-stack: Version drift: cluster=72.6.0, template=72.6.2

------------------------------------------------------------
Total: 3 drift items (1 missing, 1 modified, 1 orphans)
```

### CI/CD Integration

Use `--exit-error` for CI/CD pipelines:

```bash
# In your CI pipeline
deploy-cluster drift --template template.yaml --exit-error

# This will fail the build if drift is detected
```

### With Specific Template

```bash
deploy-cluster drift --template production.yaml
```

## Common Scenarios

### After Manual Changes

Someone manually deleted a resource:

```bash
$ deploy-cluster drift

Drift Detection Results:
------------------------------------------------------------

  Missing (in template, not in cluster):
    - [ingress] ingress-nginx-controller: Ingress (nginx) is enabled but not installed in cluster

------------------------------------------------------------
Total: 1 drift items (1 missing, 0 modified, 0 orphans)
```

Fix by re-applying the template:
```bash
deploy-cluster upgrade --template template.yaml
```

### After Adding Resources

You installed something manually that should be in the template:

```bash
$ deploy-cluster drift

Drift Detection Results:
------------------------------------------------------------

  Orphans (in cluster, not in template):
    ? [custom-apps] postgresql: Custom app "postgresql" is installed but not in template

------------------------------------------------------------
Total: 1 drift items (0 missing, 0 modified, 1 orphans)
```

Fix by adding to template or uninstalling:
```bash
# Option 1: Add to template, then upgrade
# Edit template.yaml to include postgresql
deploy-cluster upgrade --template template.yaml

# Option 2: Uninstall the orphan
helm uninstall postgresql
```

### Version Drift

The cluster has a different version than specified in template:

```bash
$ deploy-cluster drift

Drift Detection Results:
------------------------------------------------------------

  Modified (drift detected):
    ~ [monitoring] kube-prometheus-stack: Version drift: cluster=72.5.0, template=72.6.2

------------------------------------------------------------
Total: 1 drift items (0 missing, 1 modified, 0 orphans)
```

Fix by upgrading:
```bash
deploy-cluster upgrade --template template.yaml
```

## GitOps Workflow

Drift detection is essential for GitOps workflows:

```bash
# 1. Make changes to template.yaml
# 2. Validate changes
deploy-cluster lint --template template.yaml

# 3. Check current drift
deploy-cluster drift --template template.yaml

# 4. Preview changes
deploy-cluster upgrade --template template.yaml --dry-run

# 5. Apply changes
deploy-cluster upgrade --template template.yaml

# 6. Verify no drift remains
deploy-cluster drift --template template.yaml --exit-error
```

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Drift Detection

on:
  schedule:
    - cron: '0 */6 * * *'  # Every 6 hours

jobs:
  drift:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      
      - name: Setup deploy-cluster
        run: |
          go build -o deploy-cluster ./cmd/deploycluster
          
      - name: Configure kubectl
        run: |
          echo "${{ secrets.KUBECONFIG }}" | base64 -d > ~/.kube/config
          
      - name: Detect Drift
        run: |
          ./deploy-cluster drift --template template.yaml --exit-error
```

### GitLab CI Example

```yaml
drift-detection:
  script:
    - go build -o deploy-cluster ./cmd/deploycluster
    - ./deploy-cluster drift --template template.yaml --exit-error
  only:
    - schedules
```

## Best Practices

1. **Run drift detection regularly**:
   ```bash
   # Add to your daily checks
   deploy-cluster drift
   ```

2. **Use in CI/CD**:
   - Run on schedule (e.g., every 6 hours)
   - Run after any manual cluster access
   - Alert on drift detection

3. **Fix drift immediately**:
   - Don't let drift accumulate
   - Prefer `upgrade` over manual fixes
   - Document any intentional drift

4. **Version control your templates**:
   - Keep templates in git
   - Review drift detection in PR checks
   - Use `--exit-error` in CI

## Troubleshooting

### False Positives

Some resources may appear as drift but are actually fine:

- **System-generated resources**: These are filtered out automatically
- **CRD-managed resources**: May show as orphans if CRD is missing
- **Pending deletions**: Resources being deleted may temporarily appear

### Detection Failures

If drift detection fails:

1. Check cluster connectivity:
   ```bash
   kubectl cluster-info
   ```

2. Verify kubeconfig:
   ```bash
   kubectl config current-context
   ```

3. Check permissions:
   ```bash
   kubectl auth can-i list deployments
   ```

## See Also

- [Upgrade Command](commands.md#upgrade) - Fix drift by upgrading
- [Status Command](commands.md#status) - View cluster status
- [GitOps Best Practices](https://www.gitops.tech/)
