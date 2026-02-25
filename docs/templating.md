# Advanced Templating

deploy-cluster supports Go templates in `template.yaml` files, allowing dynamic configuration based on environment variables, git information, and more.

## Overview

Templates use Go's `text/template` syntax with additional helper functions. This enables:
- Environment-specific configuration (dev/staging/prod)
- Dynamic values from CI/CD pipelines
- Reusable templates with variables
- Git-based versioning

## Basic Syntax

Templates use double curly braces:

```yaml
name: "{{ .Env.CLUSTER_NAME }}"
cluster:
  workers: {{ .Env.WORKERS }}
```

## Built-in Variables

### Environment Variables

Access environment variables using `.Env.VAR_NAME`:

```yaml
name: "{{ .Env.CLUSTER_NAME }}"
plugins:
  externalDNS:
    credentials:
      apiToken: "{{ .Env.CF_API_TOKEN }}"
```

### Git Information

| Variable | Description | Example |
|----------|-------------|---------|
| `.GitCommit` | Short commit hash | `abc1234` |
| `.GitBranch` | Current branch | `main`, `feature/auth` |
| `.GitTag` | Current tag | `v1.2.3` |

```yaml
plugins:
  argocd:
    apps:
      - name: my-app
        values:
          image:
            tag: "{{ .GitCommit }}"
```

### Cluster Information

| Variable | Description |
|----------|-------------|
| `.Cluster.name` | Cluster name |
| `.Cluster.provider` | Provider (kind, k3d) |
| `.Cluster.controlPlanes` | Number of control planes |
| `.Cluster.workers` | Number of workers |

## Template Functions

### `env` - Environment Variable with Default

```yaml
name: "{{ env "CLUSTER_NAME" "my-cluster" }}"
```

Returns the environment variable value, or the default if not set.

### `required` - Required Environment Variable

```yaml
name: "{{ required "CLUSTER_NAME" }}"
```

Fails if the environment variable is not set.

### `default` - Default Value

```yaml
workers: {{ default 2 .Env.WORKERS }}
```

Returns the value if set, otherwise the default.

### String Functions

```yaml
# Upper case
name: "{{ upper .Env.CLUSTER_NAME }}"

# Lower case
name: "{{ lower .Env.CLUSTER_NAME }}"

# Title case
name: "{{ title .Env.CLUSTER_NAME }}"

# Replace
name: "{{ replace "-" "_" .Env.CLUSTER_NAME }}"

# Contains (in conditionals)
{{ if contains "prod" .Env.CLUSTER_NAME }}...{{ end }}

# Trim
name: "{{ trim .Env.CLUSTER_NAME }}"
```

### Indentation

```yaml
config: |
{{ indent 2 .Env.CONFIG_CONTENT }}
```

### Quote

```yaml
name: {{ quote .Env.CLUSTER_NAME }}
# Results in: name: "my-cluster"
```

## Examples

### Environment-Specific Configuration

```yaml
name: "{{ env "CLUSTER_NAME" "dev-cluster" }}"
provider:
  type: kind
cluster:
  controlPlanes: 1
  workers: {{ env "WORKERS" "2" }}
plugins:
  monitoring:
    enabled: {{ if eq .Env.ENV "prod" }}true{{ else }}false{{ end }}
    type: prometheus
```

### CI/CD Integration

```yaml
name: "ci-{{ .GitBranch | replace "/" "-" }}"
cluster:
  workers: 2
plugins:
  argocd:
    enabled: true
    apps:
      - name: my-app
        namespace: default
        repoURL: https://github.com/org/repo.git
        targetRevision: "{{ .GitCommit }}"
        values:
          image:
            tag: "{{ .GitCommit }}"
          ingress:
            host: "{{ .GitBranch | replace "/" "-" | lower }}.preview.example.com"
```

### Multi-Environment Template

```yaml
name: "{{ env "ENV" "dev" }}-cluster"
provider:
  type: kind
cluster:
  workers: {{ if eq (env "ENV" "dev") "prod" }}5{{ else }}2{{ end }}
plugins:
  istio:
    enabled: {{ if eq (env "ENV" "dev") "prod" }}true{{ else }}false{{ end }}
    profile: {{ if eq (env "ENV" "dev") "prod" }}default{{ else }}demo{{ end }}
  monitoring:
    enabled: {{ if eq (env "ENV" "dev") "prod" }}true{{ else }}false{{ end }}
```

### Dynamic Ingress Hosts

```yaml
plugins:
  ingress:
    enabled: true
    type: nginx
  monitoring:
    enabled: true
    type: prometheus
    ingress:
      enabled: true
      host: "grafana.{{ env "ZONE" "localhost" }}"
  externalDNS:
    enabled: {{ if ne (env "ENV" "dev") "dev" }}true{{ else }}false{{ end }}
    provider: cloudflare
    zone: "{{ env "ZONE" "example.com" }}"
```

## Using with deploy-cluster

Templates are processed automatically when loading the template file:

```bash
# Set environment variables
export CLUSTER_NAME="prod-cluster"
export WORKERS="5"
export CF_API_TOKEN="your-token"

# Create cluster (template is processed automatically)
deploy-cluster create --template template.yaml
```

### Loading Additional Env Files

You can specify additional environment files:

```bash
# Load from .env file
deploy-cluster create --template template.yaml --env .env

# Or use environment variables directly
CLUSTER_NAME=prod WORKERS=5 deploy-cluster create --template template.yaml
```

## Validation

The `lint` command validates templates including template syntax:

```bash
deploy-cluster lint --template template.yaml
```

If template processing fails, you'll get an error:

```
Error: template processing failed: required environment variable "CLUSTER_NAME" is not set
```

## Best Practices

1. **Use defaults for development**:
   ```yaml
   name: "{{ env "CLUSTER_NAME" "dev-cluster" }}"
   ```

2. **Use required for production**:
   ```yaml
   name: "{{ required "CLUSTER_NAME" }}"
   ```

3. **Keep secrets in environment**:
   ```yaml
   # Good
   apiToken: "{{ .Env.API_TOKEN }}"
   
   # Bad - never hardcode secrets
   apiToken: "hardcoded-secret"
   ```

4. **Use git commit for image tags**:
   ```yaml
   image:
     tag: "{{ .GitCommit }}"
   ```

5. **Document required variables**:
   Add a comment at the top of your template:
   ```yaml
   # Required environment variables:
   # - CLUSTER_NAME: Name of the cluster
   # - ZONE: DNS zone for ingress
   # - CF_API_TOKEN: Cloudflare API token
   ```

## Troubleshooting

### Template not processed

If your template expressions appear literally in the output, check:
- Syntax: Use `{{` and `}}` (no spaces inside braces for simple variables)
- Quotes: YAML strings with templates should be quoted

### Environment variable not found

Environment variables are case-sensitive. Check:
```bash
env | grep CLUSTER_NAME
```

### Syntax errors

Use the `lint` command to catch syntax errors:
```bash
deploy-cluster lint --template template.yaml
```

## See Also

- [Go Template Documentation](https://golang.org/pkg/text/template/)
- [Sprig Template Functions](https://masterminds.github.io/sprig/) (additional functions available)
