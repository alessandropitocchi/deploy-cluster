# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**deploy-cluster** is a CLI tool for deploying Kubernetes clusters on kind (Kubernetes in Docker). It allows users to define cluster topology and install plugins (storage, ingress, ArgoCD) from a single YAML configuration file.

## Architecture

### Core Concepts

- **Providers**: Abstraction layer for cluster providers. Interface in `pkg/provider/provider.go`. Currently implemented: **kind**.
- **Plugins**: Modular components installed on clusters. Each plugin has its own package under `pkg/plugin/`. Implemented: **storage** (local-path-provisioner), **ingress** (nginx), **ArgoCD**.
- **Config**: Single `cluster.yaml` defines cluster topology + all plugins. Parsed and validated in `pkg/config/`.

### Plugin Installation Order

`create` and `upgrade` install plugins in this order: **storage → ingress → ArgoCD**. Storage first so PVCs are available; ingress before ArgoCD so Ingress resources work immediately.

## Commands

| Command | Description |
|---------|-------------|
| `init` | Generate starter `cluster.yaml` |
| `create` | Create cluster + install all enabled plugins |
| `upgrade` | Update plugins on existing cluster (diff-based for ArgoCD repos/apps) |
| `upgrade --dry-run` | Preview changes without applying |
| `status` | Show cluster and plugin status |
| `destroy` | Delete cluster |
| `get clusters` | List kind clusters |
| `get nodes <name>` | List cluster nodes |
| `get kubeconfig <name>` | Print kubeconfig |

## Development

### Tech Stack
- **Language**: Go
- **CLI Framework**: cobra
- **Config Format**: YAML (gopkg.in/yaml.v3)
- **Cluster Interaction**: kubectl commands via `os/exec`

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
pkg/
  config/
    config.go               # Config structs, Load(), Save(), Validate()
    env.go                  # .env file loading
  provider/
    provider.go             # Provider interface (Name, Create, Delete, Exists, KubeContext, GetKubeconfig)
    kind/
      kind.go               # kind provider implementation
  plugin/
    argocd/
      argocd.go             # ArgoCD plugin (Install, Upgrade, DryRun, repos/apps diff)
    storage/
      storage.go            # Storage plugin (local-path-provisioner)
    ingress/
      ingress.go            # Ingress plugin (nginx)
```

### Key Design Decisions
- Provider abstraction: `KubeContext()` method avoids hardcoding `kind-<name>` everywhere
- ArgoCD `Upgrade()` is diff-based: applies all desired repos/apps (idempotent), removes those no longer in config
- Repo name generation is centralized in `repoName()` — used by both `addRepository` and `Upgrade` diff logic
- Config `Validate()` runs inside `Load()` — invalid configs fail early
- No generic Plugin interface — each plugin has typed config (ArgoCD receives `*ArgoCDConfig`, etc.)
- Plugin installation is idempotent (`kubectl apply`)

### Testing
Tests are colocated with source files (`*_test.go` in same package). Run with `go test ./...`.

Key test areas:
- `pkg/config/`: Load/Save round-trip, validation (all error cases), env file parsing
- `pkg/plugin/argocd/`: repoName generation, diff logic (add/remove repos/apps)
- `pkg/plugin/storage/`: type routing, error messages
- `pkg/plugin/ingress/`: type routing, error messages
- `pkg/provider/kind/`: generateKindConfig, KubeContext
