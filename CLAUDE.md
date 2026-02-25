# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**deploy-cluster** is a CLI tool for deploying Kubernetes clusters on kind (Kubernetes in Docker) or k3d (K3s in Docker). It allows users to define cluster topology and install plugins (storage, ingress, cert-manager, monitoring, dashboard, custom Helm apps, ArgoCD) from a single YAML template file.

## Architecture

### Core Concepts

- **Providers**: Abstraction layer for cluster providers. Interface in `pkg/provider/provider.go`. Implemented: **kind**, **k3d**.
- **Plugins**: Modular components installed on clusters. Each plugin implements the unified `Plugin` interface in `pkg/plugin/plugin.go`. Implemented: **storage** (local-path-provisioner), **ingress** (nginx or traefik), **cert-manager**, **external-dns** (automatic DNS management), **istio** (service mesh), **monitoring** (kube-prometheus-stack via Helm), **dashboard** (Headlamp via Helm), **customApps** (arbitrary Helm charts), **ArgoCD**.
- **Plugin Manager**: Orchestrates plugin installation in `pkg/plugin/manager.go`. Handles install order, parallel execution, and result tracking.
- **Linter**: Validates templates for errors and best practices in `pkg/linter/linter.go`.
- **Template**: Single `template.yaml` defines cluster topology + all plugins. Parsed and validated in `pkg/template/`.

### Plugin Installation Order

`create` and `upgrade` install plugins in this order: **storage → ingress → cert-manager → monitoring → dashboard → customApps → ArgoCD**. Storage first so PVCs are available; ingress before others so Ingress resources work; ArgoCD last because it may depend on everything else.

### Unified Plugin Interface

All plugins implement this interface:

```go
type Plugin interface {
    Name() string
    Install(cfg interface{}, kubecontext string, providerType string) error
    IsInstalled(kubecontext string) (bool, error)
    Upgrade(cfg interface{}, kubecontext string, providerType string) error
    Uninstall(cfg interface{}, kubecontext string) error
    DryRun(cfg interface{}, kubecontext string, providerType string) error
}
```

The `cfg` parameter is type-asserted by each plugin to its specific configuration struct.

### Ingress Integration

When `plugins.ingress` is enabled:
- **kind**: config automatically adds `ingress-ready=true` label and port mappings (80/443) to the first control-plane node
- **k3d**: port mappings go on the loadbalancer; if ingress type is nginx, Traefik is disabled (`--disable=traefik`); if type is traefik, the built-in Traefik is used as-is

Several plugins support optional `ingress` sub-config to expose their UI (ArgoCD, monitoring/Grafana, dashboard/Headlamp, customApps).

## Commands

| Command | Description |
|---------|-------------|
| `init` | Generate starter `template.yaml` via interactive wizard |
| `lint` | Validate template for errors and best practices |
| `create` | Create cluster + install all enabled plugins |
| `upgrade` | Update plugins on existing cluster (diff-based for ArgoCD repos/apps) |
| `upgrade --dry-run` | Preview changes without applying |
| `drift` | Detect drift between cluster and template |
| `uninstall` | Uninstall plugins from cluster (keeps cluster) |
| `status` | Show cluster and plugin status |
| `destroy` | Delete cluster |
| `check` | Verify that all prerequisites are installed |
| `switch` | Switch kubectl context between clusters |
| `get clusters [--provider]` | List clusters (kind and/or k3d) |
| `get nodes <name> [--provider]` | List cluster nodes |
| `get kubeconfig <name> [--provider]` | Print kubeconfig |
| `snapshot save <name>` | Export cluster resources to a local snapshot |
| `snapshot restore <name>` | Restore resources from a snapshot (supports `--dry-run`) |
| `snapshot list` | List all saved snapshots |
| `snapshot delete <name>` | Delete a snapshot from disk |

## Development

### Tech Stack
- **Language**: Go 1.21+
- **CLI Framework**: cobra
- **Template Format**: YAML (gopkg.in/yaml.v3)
- **Cluster Interaction**: kubectl and helm commands via `os/exec`

### Build & Test
```bash
go build -o deploy-cluster ./cmd/deploycluster
go test ./...
```

### Linting
```bash
# Run golangci-lint
~/go/bin/golangci-lint run ./...
```

### Project Structure
```
cmd/deploycluster/          # CLI entrypoint and cobra commands
  main.go                   # Entry point
  root.go                   # Root command
  helpers.go                # Shared helpers (getProvider, newLogger)
  init.go                   # init command
  lint.go                   # lint command
  create.go                 # create command
  upgrade.go                # upgrade command
  uninstall.go              # uninstall command
  destroy.go                # destroy command
  status.go                 # status command
  check.go                  # check command
  switch.go                 # switch command
  get.go                    # get subcommands
  snapshot.go               # snapshot subcommands
  plugins.go                # Plugin orchestration (installPlugins, upgradePlugins)
pkg/
  plugin/
    plugin.go               # Plugin interface and Registry
    manager.go              # Plugin Manager for orchestration
    argocd/                 # ArgoCD plugin
    certmanager/            # Cert-manager plugin
    customapps/             # Custom apps plugin
    dashboard/              # Dashboard plugin
    externaldns/            # External DNS plugin
    ingress/                # Ingress plugin
    istio/                  # Istio plugin
    monitoring/             # Monitoring plugin
    storage/                # Storage plugin
  linter/
    linter.go               # Template validation and linting
  templating/
    templating.go           # Go template processing
  template/
    template.go             # Template structs, Load(), Save(), Validate()
    env.go                  # .env file loading
  provider/
    provider.go             # Provider interface
    kind/                   # kind provider
    k3d/                    # k3d provider
  snapshot/                 # Snapshot system
  drift/                    # Drift detection engine
  k8s/                      # Kubernetes helpers
  logger/                   # Structured logging
  retry/                    # Retry logic
```

### Key Design Decisions
- **Unified Plugin Interface**: All plugins implement the same interface for consistent handling
- **Plugin Manager**: Centralized orchestration with support for parallel installation
- **Provider abstraction**: `KubeContext()` method avoids hardcoding `kind-<name>` everywhere
- **Kind config**: Generates `kubeadmConfigPatches` with `ingress-ready=true` label when ingress plugin is enabled
- **k3d config**: Uses SimpleConfig (k3d.io/v1alpha5), ports on loadbalancer, disables Traefik when nginx is chosen
- **Ingress plugin**: Provider-aware, uses kind-specific or cloud nginx manifest URL based on provider type
- **ArgoCD Upgrade**: Diff-based - applies all desired repos/apps, removes those no longer in template
- **ArgoCD insecure mode**: Uses `argocd-cmd-params-cm` ConfigMap (not container args patching)
- **Template validation**: `Validate()` runs inside `Load()` — invalid templates fail early
- **Helm-based plugins**: Use `helm upgrade --install` for idempotency
- **customApps**: Inline `values` are written to temp files, `valuesFile` takes precedence over inline values
- **Snapshot system**: Dynamic resource discovery, dependency-aware restore, resource sanitization
- **Linter**: Comprehensive validation including best practices and dependency checking
- **Drift Detection**: Compare cluster state against template to detect configuration drift, orphaned resources, and missing resources

### Testing
Tests are colocated with source files (`*_test.go` in same package). Run with `go test ./...`.

Key test areas:
- `pkg/template/`: Load/Save round-trip, validation, env file parsing
- `pkg/plugin/`: All plugin implementations (argocd, storage, ingress, certmanager, monitoring, dashboard, customapps)
- `pkg/linter/`: Lint checks, issue formatting
- `pkg/provider/kind/` and `pkg/provider/k3d/`: Provider implementations
- `pkg/snapshot/`: Metadata, discovery, export, restore, diff
- `pkg/drift/`: Detection engine, report generation, history management

### Adding a New Plugin

1. Create package under `pkg/plugin/<name>/`
2. Implement the `Plugin` interface:
   - `Name() string` - Return plugin identifier
   - `Install(cfg interface{}, kubecontext, providerType string) error`
   - `IsInstalled(kubecontext string) (bool, error)`
   - `Upgrade(cfg interface{}, kubecontext, providerType string) error`
   - `Uninstall(cfg interface{}, kubecontext string) error`
   - `DryRun(cfg interface{}, kubecontext, providerType string) error`
3. Add config struct to `pkg/template/template.go` if needed
4. Register in `cmd/deploycluster/plugins.go` `createRegistry()`
5. Add to `plugin.InstallOrder` if it has dependencies
6. Add extractor to `plugin.Extractors` for config extraction
7. Update documentation in `docs/plugins/<name>.md`
