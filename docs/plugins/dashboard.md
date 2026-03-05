# Plugin: Dashboard

Installs [Headlamp](https://headlamp.dev/) as a web dashboard for Kubernetes, via Helm.

## Configuration

```yaml
plugins:
  dashboard:
    enabled: true
    type: headlamp
    version: "0.40.0"          # optional
    ingress:
      enabled: true
      host: headlamp.localhost
```

| Field | Type | Default | Required | Description |
|-------|------|---------|:---:|-------------|
| `enabled` | bool | `false` | yes | Enable the plugin |
| `type` | string | - | yes | Dashboard type. Values: `headlamp` |
| `version` | string | `0.40.0` | no | Helm chart version |
| `ingress.enabled` | bool | `false` | no | Create an Ingress for Headlamp |
| `ingress.host` | string | - | if ingress enabled | Hostname for Headlamp |
| `values` | object | - | no | Additional Helm values (inline) |
| `valuesFile` | string | - | no | Path to external values file |

## How It Works

The plugin:
1. Installs Headlamp via Helm from the chart repository `https://kubernetes-sigs.github.io/headlamp/`
2. Creates a `ClusterRoleBinding` that assigns `cluster-admin` to the `headlamp` service account
3. If ingress is enabled, creates an Ingress resource

- Namespace: `headlamp`
- Release name: `headlamp`

## Access

### With ingress (HTTPRoute)

If `ingress.enabled: true`, an HTTPRoute is created to expose Headlamp via the shared Gateway:

```
http://headlamp.localhost
```

### Without ingress (port-forward)

```bash
kubectl port-forward svc/headlamp -n headlamp 4466:80
# http://localhost:4466
```

## Authentication

Headlamp requires a token for login. The plugin creates a ClusterRoleBinding with `cluster-admin`, so you can use the service account token:

```bash
# Create a token for the headlamp service account
kubectl create token headlamp -n headlamp
```

Copy the token and paste it into the Headlamp login screen.

For a long-lived token:

```bash
kubectl apply -f - <<EOF
apiVersion: v1
kind: Secret
metadata:
  name: headlamp-token
  namespace: headlamp
  annotations:
    kubernetes.io/service-account.name: headlamp
type: kubernetes.io/service-account-token
EOF

# Retrieve the token
kubectl get secret headlamp-token -n headlamp -o jsonpath="{.data.token}" | base64 -d
```

## Features

Headlamp offers:
- Cluster, namespace, workload, service, and storage views
- Real-time pod logs
- Container exec
- Built-in YAML editor
- Multi-cluster support
- Extensible plugin system

## Custom Values

You can customize the Helm deployment using `values` (inline) or `valuesFile` (external file):

```yaml
plugins:
  dashboard:
    enabled: true
    type: headlamp
    values:
      replicaCount: 2
      service:
        type: LoadBalancer
      resources:
        requests:
          memory: "256Mi"
          cpu: "250m"
```

Or use an external values file:
```yaml
plugins:
  dashboard:
    enabled: true
    type: headlamp
    valuesFile: ./headlamp-values.yaml
```

## Verification

```bash
kubectl get pods -n headlamp
kubectl get svc -n headlamp
helm list -n headlamp
```
