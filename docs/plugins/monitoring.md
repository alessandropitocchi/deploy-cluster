# Plugin: Monitoring

Installs the [kube-prometheus-stack](https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-prometheus-stack) via Helm, which includes Prometheus, Grafana, Alertmanager, node-exporter, and kube-state-metrics.

## Configuration

```yaml
plugins:
  monitoring:
    enabled: true
    type: prometheus
    version: "72.6.2"          # optional
    ingress:
      enabled: true
      host: grafana.localhost
```

| Field | Type | Default | Required | Description |
|-------|------|---------|:---:|-------------|
| `enabled` | bool | `false` | yes | Enable the plugin |
| `type` | string | - | yes | Stack type. Values: `prometheus` |
| `version` | string | `72.6.2` | no | Helm chart version |
| `ingress.enabled` | bool | `false` | no | Create an Ingress for Grafana |
| `ingress.host` | string | - | if ingress enabled | Hostname for Grafana |
| `values` | object | - | no | Additional Helm values (inline) |
| `valuesFile` | string | - | no | Path to external values file |

## How It Works

The plugin uses Helm to install the OCI chart:

```
oci://ghcr.io/prometheus-community/charts/kube-prometheus-stack
```

- Namespace: `monitoring`
- Release name: `kube-prometheus-stack`
- `--wait` flag to wait for all pods to be ready

## Accessing Grafana

### With ingress (HTTPRoute)

If `ingress.enabled: true`, an HTTPRoute is created to expose Grafana via the shared Gateway:

```
http://grafana.localhost
```

Credentials: `admin` / `prom-operator`

The HTTPRoute references the shared Gateway created by the ingress plugin (`shared-gateway` in the ingress controller namespace).

### Without ingress (port-forward)

```bash
kubectl port-forward svc/kube-prometheus-stack-grafana -n monitoring 3000:80
# http://localhost:3000
```

## Accessing Prometheus

```bash
kubectl port-forward svc/kube-prometheus-stack-prometheus -n monitoring 9090:9090
# http://localhost:9090
```

## Accessing Alertmanager

```bash
kubectl port-forward svc/kube-prometheus-stack-alertmanager -n monitoring 9093:9093
# http://localhost:9093
```

## Installed Components

| Component | Description |
|-----------|-------------|
| Prometheus Operator | Manages Prometheus/ServiceMonitor/AlertmanagerConfig resources |
| Prometheus | Metrics collection and PromQL queries |
| Grafana | Dashboards and visualization (with preconfigured dashboards) |
| Alertmanager | Alert management and routing |
| node-exporter | OS-level node metrics |
| kube-state-metrics | Kubernetes object state metrics |

## Preconfigured Grafana Dashboards

The chart includes dashboards for:
- Kubernetes cluster overview
- Node metrics
- Pod metrics
- Namespace resources
- Persistent volumes
- API server
- etcd
- CoreDNS

## Custom Values

You can customize the Helm deployment using `values` (inline) or `valuesFile` (external file):

```yaml
plugins:
  monitoring:
    enabled: true
    type: prometheus
    values:
      grafana:
        adminPassword: mypassword
        persistence:
          enabled: true
          size: 10Gi
      prometheus:
        prometheusSpec:
          retention: 30d
          storageSpec:
            volumeClaimTemplate:
              spec:
                resources:
                  requests:
                    storage: 50Gi
```

Or use an external values file:
```yaml
plugins:
  monitoring:
    enabled: true
    type: prometheus
    valuesFile: ./monitoring-values.yaml
```

## Upgrade

Upgrade uses `helm upgrade` which updates the existing release. Change `version` in the template and run `upgrade` to update the stack.

## Verification

```bash
# Pods
kubectl get pods -n monitoring

# Services
kubectl get svc -n monitoring

# ServiceMonitors
kubectl get servicemonitors -n monitoring

# Helm release
helm list -n monitoring
```
