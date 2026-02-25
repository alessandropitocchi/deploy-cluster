# Istio Plugin

The Istio plugin installs the Istio service mesh, providing traffic management, security (mTLS), and observability for your Kubernetes cluster.

## Overview

Istio is an open-source service mesh that provides:
- **Traffic Management**: Load balancing, traffic routing, canary deployments
- **Security**: Automatic mTLS encryption, authentication, authorization
- **Observability**: Metrics, tracing, and logging for all service communications

## Configuration

```yaml
plugins:
  istio:
    enabled: true
    version: "1.24.0"           # Optional: Istio version (default: 1.24.0)
    profile: default            # Optional: Profile - default, demo, minimal, empty (default: default)
    revision: ""                # Optional: Revision for canary upgrades
    ingressGateway: true        # Optional: Enable ingress gateway (default: false)
    egressGateway: false        # Optional: Enable egress gateway (default: false)
    values: {}                  # Optional: Additional Helm values
```

## Profiles

| Profile | Description | Use Case |
|---------|-------------|----------|
| `default` | Production-ready configuration | Production clusters |
| `demo` | Full configuration with access to all features | Development, testing |
| `minimal` | Minimal control plane, no gateways | Minimal resource usage |
| `empty` | No components (for custom installations) | Advanced users |
| `ambient` | Ambient mesh mode (beta) | New Istio architecture |

## Examples

### Basic Installation

```yaml
plugins:
  istio:
    enabled: true
    profile: demo
```

### Production Setup

```yaml
plugins:
  istio:
    enabled: true
    profile: default
    ingressGateway: true
    egressGateway: true
```

### With Ingress Gateway

```yaml
plugins:
  istio:
    enabled: true
    profile: default
    ingressGateway: true
```

### Canary Upgrade

```yaml
plugins:
  istio:
    enabled: true
    profile: default
    revision: "1-24-0"  # Install as canary revision
```

## Usage

### Enable Sidecar Injection

After installing Istio, enable automatic sidecar injection on namespaces:

```bash
# Enable injection on a namespace
kubectl label namespace default istio-injection=enabled

# Disable injection
kubectl label namespace default istio-injection=disabled --overwrite
```

### Deploy an Application

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: default
spec:
  replicas: 3
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
    spec:
      containers:
        - name: app
          image: my-app:latest
          ports:
            - containerPort: 8080
```

With `istio-injection=enabled` on the namespace, Istio automatically injects the sidecar proxy.

### Traffic Routing

```yaml
apiVersion: networking.istio.io/v1beta1
kind: VirtualService
metadata:
  name: my-app
spec:
  hosts:
    - my-app.example.com
  http:
    - route:
        - destination:
            host: my-app
            subset: v1
          weight: 90
        - destination:
            host: my-app
            subset: v2
          weight: 10
```

## Verification

After installation, verify Istio is running:

```bash
# Check Istio control plane
kubectl get pods -n istio-system

# Check istiod
kubectl get deployment istiod -n istio-system

# Check ingress gateway (if enabled)
kubectl get service istio-ingressgateway -n istio-system

# View Istio configuration
istioctl proxy-config service
```

## Accessing the Dashboard

Istio integrates with the monitoring stack (Grafana, Kiali):

```bash
# Port-forward to Kiali (if installed)
kubectl port-forward svc/kiali -n istio-system 20001:20001

# Open http://localhost:20001
```

## Troubleshooting

### Sidecar not injected

1. Check namespace label:
   ```bash
   kubectl get namespace <namespace> -o yaml | grep istio-injection
   ```

2. Check istiod logs:
   ```bash
   kubectl logs -n istio-system deployment/istiod
   ```

3. Verify pod has sidecar:
   ```bash
   kubectl get pod <pod-name> -o jsonpath='{.spec.containers[*].name}'
   # Should show: my-app istio-proxy
   ```

### mTLS not working

1. Check PeerAuthentication policy:
   ```bash
   kubectl get peerauthentication -A
   ```

2. Verify destination rules:
   ```bash
   kubectl get destinationrule -A
   ```

## Integration with Other Plugins

### With Ingress

Istio ingress gateway can replace or complement NGINX ingress:

```yaml
plugins:
  ingress:
    enabled: true
    type: nginx
  istio:
    enabled: true
    profile: default
    ingressGateway: true
```

### With Monitoring

Istio metrics are automatically scraped by Prometheus when monitoring is enabled:

```yaml
plugins:
  monitoring:
    enabled: true
    type: prometheus
  istio:
    enabled: true
    profile: default
```

Access Istio dashboards in Grafana:
- Istio Service Dashboard
- Istio Workload Dashboard
- Istio Mesh Dashboard

### With External DNS

Istio ingress gateway works with External DNS:

```yaml
plugins:
  externalDNS:
    enabled: true
    provider: cloudflare
    zone: example.com
  istio:
    enabled: true
    profile: default
    ingressGateway: true
```

## Security Best Practices

1. **Enable mTLS in strict mode**:
   ```yaml
   apiVersion: security.istio.io/v1beta1
   kind: PeerAuthentication
   metadata:
     name: default
     namespace: istio-system
   spec:
     mtls:
       mode: STRICT
   ```

2. **Use Authorization Policies**:
   ```yaml
   apiVersion: security.istio.io/v1beta1
   kind: AuthorizationPolicy
   metadata:
     name: my-app
   spec:
     selector:
       matchLabels:
         app: my-app
     rules:
       - from:
           - source:
               principals: ["cluster.local/ns/default/sa/allowed-service"]
   ```

## See Also

- [Istio Documentation](https://istio.io/latest/docs/)
- [Istio Best Practices](https://istio.io/latest/docs/ops/best-practices/)
- [Istio Security](https://istio.io/latest/docs/concepts/security/)
