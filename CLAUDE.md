# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**deploy-cluster** is a CLI tool for deploying Kubernetes clusters on kind (Kubernetes in Docker) or k3d (K3s in Docker). It allows users to define cluster topology and install plugins (storage, ingress, cert-manager, monitoring, dashboard, custom Helm apps, ArgoCD) from a single YAML template file.

## Architecture

### Core Concepts

- **Providers**: Abstraction layer for cluster providers. Interface in `pkg/provider/provider.go`. Implemented: **kind**, **k3d**.
- **Plugins**: Modular components installed on clusters. Each plugin has its own package under `pkg/plugin/`. Implemented: **storage** (local-path-provisioner), **ingress** (nginx or traefik), **cert-manager**, **monitoring** (kube-prometheus-stack via Helm), **dashboard** (Headlamp via Helm), **customApps** (arbitrary Helm charts), **ArgoCD**.
- **Template**: Single `template.yaml` defines cluster topology + all plugins. Parsed and validated in `pkg/template/`.

### Plugin Installation Order

`create` and `upgrade` install plugins in this order: **storage → ingress → cert-manager → monitoring → dashboard → customApps → ArgoCD**. Storage first so PVCs are available; ingress before others so Ingress resources work; ArgoCD last because it may depend on everything else.

### Ingress Integration

When `plugins.ingress` is enabled:
- **kind**: config automatically adds `ingress-ready=true` label and port mappings (80/443) to the first control-plane node
- **k3d**: port mappings go on the loadbalancer; if ingress type is nginx, Traefik is disabled (`--disable=traefik`); if type is traefik, the built-in Traefik is used as-is

Several plugins support optional `ingress` sub-config to expose their UI (ArgoCD, monitoring/Grafana, dashboard/Headlamp, customApps).

## Commands

| Command | Description |
|---------|-------------|
| `init` | Generate starter `template.yaml` |
| `create` | Create cluster + install all enabled plugins |
| `upgrade` | Update plugins on existing cluster (diff-based for ArgoCD repos/apps) |
| `upgrade --dry-run` | Preview changes without applying |
| `status` | Show cluster and plugin status |
| `destroy` | Delete cluster |
| `get clusters [--provider]` | List clusters (kind and/or k3d) |
| `get nodes <name> [--provider]` | List cluster nodes |
| `get kubeconfig <name> [--provider]` | Print kubeconfig |
| `snapshot save <name>` | Export cluster resources to a local snapshot |
| `snapshot restore <name>` | Restore resources from a snapshot (supports `--dry-run`) |
| `snapshot list` | List all saved snapshots |
| `snapshot delete <name>` | Delete a snapshot from disk |

## Development

### Tech Stack
- **Language**: Go
- **CLI Framework**: cobra
- **Template Format**: YAML (gopkg.in/yaml.v3)
- **Cluster Interaction**: kubectl and helm commands via `os/exec`

### Build & Test
```bash
go build -o deploy-cluster ./cmd/deploycluster
go test ./...
```

### Project Structure
```
cmd/deploycluster/          # CLI entrypoint and cobra commands
  main.go                   # Entry point
  root.go                   # Root command
  helpers.go                # Shared getProvider() helper
  create.go                 # create command
  upgrade.go                # upgrade command (with --dry-run)
  destroy.go                # destroy command
  status.go                 # status command
  init.go                   # init command
  get.go                    # get subcommands
  snapshot.go               # snapshot save/restore/list/delete subcommands
pkg/
  snapshot/
    snapshot.go             # Orchestration: Save/Restore/List/Delete
    metadata.go             # Metadata struct + YAML serialization
    discovery.go            # kubectl api-resources discovery + filtering
    export.go               # Resource export + sanitization + system resource filtering
    restore.go              # Ordered restore (CRDs → Namespaces → cluster-scoped → namespaced)
  template/
    template.go             # Template structs, Load(), Save(), Validate()
    env.go                  # .env file loading
  provider/
    provider.go             # Provider interface (Name, Create, Delete, Exists, KubeContext, GetKubeconfig)
    kind/
      kind.go               # kind provider (generates kind config with ingress labels/ports)
    k3d/
      k3d.go                # k3d provider (generates k3d SimpleConfig, loadbalancer ports, traefik disable)
  plugin/
    argocd/
      argocd.go             # ArgoCD plugin (Install, Upgrade, DryRun, repos/apps diff, ingress, insecure mode)
    storage/
      storage.go            # Storage plugin (local-path-provisioner)
    ingress/
      ingress.go            # Ingress plugin (nginx or traefik, provider-aware manifest URL)
    certmanager/
      certmanager.go        # Cert-manager plugin (TLS certificates)
    monitoring/
      monitoring.go         # Monitoring plugin (kube-prometheus-stack via Helm OCI, Grafana ingress)
    dashboard/
      dashboard.go          # Dashboard plugin (Headlamp via Helm, ingress)
    customapps/
      customapps.go         # Custom apps plugin (arbitrary Helm charts with values/ingress)
```

### Key Design Decisions
- Provider abstraction: `KubeContext()` method avoids hardcoding `kind-<name>` everywhere
- Kind config generates `kubeadmConfigPatches` with `ingress-ready=true` label when ingress plugin is enabled
- k3d config uses SimpleConfig (k3d.io/v1alpha5), ports on loadbalancer, disables Traefik when nginx is chosen
- Ingress plugin is provider-aware: uses kind-specific or cloud nginx manifest URL based on provider type
- ArgoCD `Upgrade()` is diff-based: applies all desired repos/apps (idempotent), removes those no longer in template
- ArgoCD insecure mode uses `argocd-cmd-params-cm` ConfigMap (not container args patching)
- Repo name generation is centralized in `repoName()` — used by both `addRepository` and `Upgrade` diff logic
- Template `Validate()` runs inside `Load()` — invalid templates fail early
- No generic Plugin interface — each plugin has typed template (ArgoCD receives `*ArgoCDTemplate`, etc.)
- Helm-based plugins (monitoring, dashboard, customApps) use `helm upgrade --install` for idempotency
- customApps: inline `values` are written to temp files, `valuesFile` takes precedence over inline values
- Plugin installation is idempotent (`kubectl apply` or `helm upgrade --install`)
- Snapshot uses dynamic resource discovery via `kubectl api-resources` to capture all resource types including CRDs
- Snapshot restore follows dependency order: CRDs → Namespaces → cluster-scoped → namespaced (with per-type priority)
- Snapshot sanitizes exported resources (removes resourceVersion, uid, managedFields, status, etc.)
- Snapshot filters system resources (ownerReferences, default SA, kube-root-ca.crt, kubernetes service)
- Snapshots stored at `~/.deploy-cluster/snapshots/<name>/` with one file per resource

### Testing
Tests are colocated with source files (`*_test.go` in same package). Run with `go test ./...`.

Key test areas:
- `pkg/template/`: Load/Save round-trip, validation (all error cases), env file parsing
- `pkg/plugin/argocd/`: repoName generation, diff logic (add/remove repos/apps)
- `pkg/plugin/storage/`: type routing, error messages
- `pkg/plugin/ingress/`: type routing, error messages
- `pkg/plugin/certmanager/`: version handling, manifest URL generation
- `pkg/plugin/monitoring/`: type routing, chart version
- `pkg/plugin/dashboard/`: type routing, chart version
- `pkg/plugin/customapps/`: values resolution (inline, file, precedence)
- `pkg/provider/kind/`: generateKindConfig (with/without ingress), KubeContext
- `pkg/provider/k3d/`: generateK3dConfig (single/multi node, version, ingress nginx/traefik), KubeContext
- `pkg/snapshot/`: metadata round-trip, api-resources parsing, sanitizeResource, isSystemResource, restore ordering, dry-run
