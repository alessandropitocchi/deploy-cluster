# External DNS Plugin

The External DNS plugin automatically manages DNS records for your Kubernetes Ingresses and Services. It synchronizes exposed Kubernetes Services and Ingresses with DNS providers.

## Overview

External DNS makes Kubernetes resources discoverable via public DNS servers. It watches for new Ingresses and Services and automatically creates corresponding DNS records in your DNS provider.

## Supported Providers

| Provider | Authentication | Notes |
|----------|---------------|-------|
| `cloudflare` | API Token | Recommended for Cloudflare zones |
| `route53` | IAM Role or Access Keys | AWS Route53 |
| `google` | Service Account | Google Cloud DNS |
| `azure` | Service Principal | Azure DNS |
| `digitalocean` | API Token | DigitalOcean DNS |

## Configuration

```yaml
plugins:
  externalDNS:
    enabled: true
    version: "1.15.0"           # Optional: chart version (default: 1.15.0)
    provider: cloudflare        # Required: DNS provider
    zone: example.com           # Optional: DNS zone to manage
    source: ingress             # Optional: ingress, service, or both (default: ingress)
    credentials:                # Provider-specific credentials
      apiToken: ${CF_API_TOKEN} # Or set via environment variable
```

## Provider Configuration

### Cloudflare

```yaml
plugins:
  externalDNS:
    enabled: true
    provider: cloudflare
    zone: example.com
    credentials:
      apiToken: ${CF_API_TOKEN}  # Cloudflare API Token
```

**Required permissions for API Token:**
- Zone:Read
- DNS:Edit

Environment variable alternative:
```bash
export CF_API_TOKEN="your-api-token"
```

### AWS Route53

```yaml
plugins:
  externalDNS:
    enabled: true
    provider: route53
    zone: example.com
    credentials:
      region: us-east-1
      accessKey: ${AWS_ACCESS_KEY_ID}
      secretKey: ${AWS_SECRET_ACCESS_KEY}
```

Or use IAM Roles for Service Accounts (IRSA) - recommended for EKS:
```yaml
plugins:
  externalDNS:
    enabled: true
    provider: route53
    zone: example.com
    # No credentials needed - uses IRSA
```

### Google Cloud DNS

```yaml
plugins:
  externalDNS:
    enabled: true
    provider: google
    zone: example.com
    credentials:
      project: my-gcp-project
```

Requires a Google Cloud service account key mounted as a secret (see External DNS documentation).

### Azure DNS

```yaml
plugins:
  externalDNS:
    enabled: true
    provider: azure
    zone: example.com
```

Requires creating an `azure-config-file` secret manually with Azure service principal credentials.

### DigitalOcean

```yaml
plugins:
  externalDNS:
    enabled: true
    provider: digitalocean
    zone: example.com
    credentials:
      token: ${DO_TOKEN}
```

## Usage with Ingress

Once External DNS is installed, it will automatically watch for Ingress resources with hostnames matching your configured zone:

```yaml
apiVersion: networking.k8s.io/v1
kind: Ingress
metadata:
  name: my-app
  annotations:
    kubernetes.io/ingress.class: nginx
spec:
  rules:
    - host: my-app.example.com  # This DNS record will be created automatically
      http:
        paths:
          - path: /
            pathType: Prefix
            backend:
              service:
                name: my-app
                port:
                  number: 80
```

External DNS will create an `A` or `CNAME` record pointing to your ingress controller's external IP.

## Source Configuration

The `source` option controls which Kubernetes resources External DNS watches:

| Source | Description |
|--------|-------------|
| `ingress` | Watch Ingress resources (default) |
| `service` | Watch Service resources with `LoadBalancer` type |
| `both` | Watch both Ingress and Service resources |

```yaml
plugins:
  externalDNS:
    enabled: true
    provider: cloudflare
    source: both  # Watch both Ingresses and Services
```

## Complete Example

```yaml
name: my-cluster
provider:
  type: kind
cluster:
  controlPlanes: 1
  workers: 2
plugins:
  ingress:
    enabled: true
    type: nginx
  certManager:
    enabled: true
  externalDNS:
    enabled: true
    provider: cloudflare
    zone: example.com
    source: ingress
    credentials:
      apiToken: ${CF_API_TOKEN}
  monitoring:
    enabled: true
    type: prometheus
    ingress:
      enabled: true
      host: grafana.example.com  # DNS will be created automatically!
```

## Verification

After installation, verify External DNS is running:

```bash
# Check the deployment
kubectl get deployment external-dns -n external-dns

# View logs
kubectl logs -n external-dns deployment/external-dns -f
```

## Troubleshooting

### DNS records not being created

1. Check External DNS logs for errors:
   ```bash
   kubectl logs -n external-dns deployment/external-dns
   ```

2. Verify the Ingress has a valid hostname matching your zone

3. Ensure credentials are correct and have necessary permissions

4. Check that the Ingress has an external IP assigned:
   ```bash
   kubectl get ingress
   ```

### Permission denied errors

- **Cloudflare**: Ensure API Token has `Zone:Read` and `DNS:Edit` permissions
- **AWS**: Verify IAM policy allows `route53:ChangeResourceRecordSets` and `route53:ListHostedZones`
- **Google**: Ensure service account has `dns.admin` role

## See Also

- [External DNS Documentation](https://kubernetes-sigs.github.io/external-dns/)
- [External DNS GitHub](https://github.com/kubernetes-sigs/external-dns)
