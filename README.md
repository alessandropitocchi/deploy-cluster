# deploy-cluster

CLI tool per il deploy di cluster Kubernetes locali con supporto plugin.

Permette di creare cluster con topologia configurabile (numero di worker e control plane) e installare automaticamente componenti come ArgoCD, definendo repository e applicazioni direttamente da configurazione.

## Requisiti

- Go 1.21+
- Docker
- [kind](https://kind.sigs.k8s.io/)
- kubectl

## Installazione

```bash
go build -o deploy-cluster ./cmd/deploycluster
```

## Quick Start

```bash
# Genera il file di configurazione
./deploy-cluster init

# Modifica cluster.yaml secondo le tue esigenze
# Crea il cluster
./deploy-cluster create --config cluster.yaml

# Verifica lo stato
./deploy-cluster status --config cluster.yaml
./deploy-cluster get clusters
./deploy-cluster get nodes my-cluster

# Aggiorna i plugin senza ricreare il cluster
./deploy-cluster upgrade --config cluster.yaml

# Distruggi il cluster
./deploy-cluster destroy --config cluster.yaml
```

## Comandi

| Comando | Descrizione |
|---------|-------------|
| `init` | Genera un file `cluster.yaml` di partenza |
| `create` | Crea il cluster e installa i plugin configurati |
| `upgrade` | Aggiorna i plugin di un cluster esistente (diff-based) |
| `status` | Mostra lo stato del cluster e dei plugin installati |
| `destroy` | Distrugge il cluster |
| `get clusters` | Lista tutti i cluster esistenti |
| `get nodes <nome>` | Lista i nodi di un cluster |
| `get kubeconfig <nome>` | Ottieni il kubeconfig di un cluster |

### Flag principali

| Flag | Comando | Default | Descrizione |
|------|---------|---------|-------------|
| `-c, --config` | create, upgrade, status, destroy | `cluster.yaml` | File di configurazione |
| `-e, --env` | create, upgrade | `.env` | File con variabili d'ambiente per i secret |
| `-o, --output` | init | `cluster.yaml` | Path del file di output |
| `-n, --name` | destroy | - | Nome del cluster (override del config) |

## Configurazione

Il file `cluster.yaml` definisce l'intera configurazione del cluster e dei plugin.

### Esempio completo

```yaml
name: my-cluster
provider:
  type: kind
cluster:
  controlPlanes: 1
  workers: 2
  version: v1.31.0
plugins:
  storage:
    enabled: true
    type: local-path
  argocd:
    enabled: true
    namespace: argocd
    version: stable
    repos:
      - name: my-gitops-repo
        url: git@github.com:user/gitops-repo.git
        type: git
        sshKeyFile: ~/.ssh/id_ed25519
    apps:
      - name: nginx
        namespace: demo-app
        repoURL: https://charts.bitnami.com/bitnami
        chart: nginx
        targetRevision: 18.2.4
        values:
          replicaCount: 2
          service:
            type: ClusterIP
```

### Cluster

| Campo | Tipo | Default | Descrizione |
|-------|------|---------|-------------|
| `name` | string | `my-cluster` | Nome del cluster |
| `provider.type` | string | `kind` | Provider (`kind`) |
| `cluster.controlPlanes` | int | `1` | Numero di control plane |
| `cluster.workers` | int | `2` | Numero di worker |
| `cluster.version` | string | `v1.31.0` | Versione Kubernetes |

### Plugin Storage

Installa un provisioner per StorageClass nel cluster. Viene installato prima degli altri plugin, in modo che eventuali PVC richiesti da ArgoCD o altre app siano già disponibili.

| Campo | Tipo | Default | Descrizione |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Abilita l'installazione dello storage |
| `type` | string | **obbligatorio** | Tipo di provisioner: `local-path` |

#### Tipi supportati

| Tipo | Descrizione |
|------|-------------|
| `local-path` | [Rancher local-path-provisioner](https://github.com/rancher/local-path-provisioner) — crea volumi sul filesystem del nodo. Ideale per cluster locali di sviluppo. Viene impostato come StorageClass di default. |

#### Esempio

```yaml
plugins:
  storage:
    enabled: true
    type: local-path
```

Dopo l'installazione, la StorageClass `local-path` diventa il default del cluster. Qualsiasi PVC senza `storageClassName` esplicito userà questo provisioner.

```bash
# Verifica
kubectl get storageclass
# NAME                   PROVISIONER             RECLAIMPOLICY   DEFAULT
# local-path (default)   rancher.io/local-path   Delete          Yes
```

### Plugin ArgoCD

| Campo | Tipo | Default | Descrizione |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Abilita l'installazione di ArgoCD |
| `namespace` | string | `argocd` | Namespace di installazione |
| `version` | string | `stable` | Versione di ArgoCD |

#### Repository (`repos`)

| Campo | Tipo | Default | Descrizione |
|-------|------|---------|-------------|
| `name` | string | auto-generato | Nome della repository |
| `url` | string | **obbligatorio** | URL della repository |
| `type` | string | `git` | Tipo: `git` o `helm` |
| `insecure` | bool | auto | Salta verifica TLS (auto per URL non HTTPS) |
| `username` | string | - | Username per repo private (HTTPS) |
| `password` | string | - | Password/token per repo private (HTTPS) |
| `sshKeyEnv` | string | - | Variabile d'ambiente con la chiave SSH privata |
| `sshKeyFile` | string | - | Path al file della chiave SSH privata |

#### Applicazioni (`apps`)

| Campo | Tipo | Default | Descrizione |
|-------|------|---------|-------------|
| `name` | string | **obbligatorio** | Nome dell'Application in ArgoCD |
| `namespace` | string | `default` | Namespace di destinazione |
| `project` | string | `default` | Progetto ArgoCD |
| `repoURL` | string | **obbligatorio** | URL del chart repo o del repo Git |
| `chart` | string | - | Nome del chart Helm (per Helm repo) |
| `path` | string | `.` | Path nel repo Git (per sorgenti Git) |
| `targetRevision` | string | `HEAD` | Versione del chart o branch/tag |
| `values` | map | - | Valori Helm inline |
| `valuesFile` | string | - | Path a un file di values esterno |
| `autoSync` | bool | `true` | Abilita sync automatico con prune e selfHeal |

### Autenticazione repository private

#### SSH key da file

```yaml
repos:
  - name: private-repo
    url: git@github.com:user/repo.git
    sshKeyFile: ~/.ssh/id_ed25519
```

#### SSH key da variabile d'ambiente

Crea un file `.env`:

```bash
ARGOCD_SSH_KEY="-----BEGIN OPENSSH PRIVATE KEY-----
...
-----END OPENSSH PRIVATE KEY-----"
```

```yaml
repos:
  - name: private-repo
    url: git@github.com:user/repo.git
    sshKeyEnv: ARGOCD_SSH_KEY
```

#### HTTPS con token

```yaml
repos:
  - name: private-repo
    url: https://github.com/user/repo.git
    username: git
    password: ghp_xxxxxxxxxxxxx
```

### Tipi di Application

#### Helm chart da repository pubblica

```yaml
apps:
  - name: nginx
    namespace: demo-app
    repoURL: https://charts.bitnami.com/bitnami
    chart: nginx
    targetRevision: 18.2.4
    values:
      replicaCount: 3
```

#### Helm chart con values da file

```yaml
apps:
  - name: nginx
    namespace: demo-app
    repoURL: https://charts.bitnami.com/bitnami
    chart: nginx
    targetRevision: 18.2.4
    valuesFile: ./nginx-values.yaml
```

#### Manifesti da Git repo

```yaml
apps:
  - name: my-app
    namespace: demo-app
    repoURL: git@github.com:user/gitops-repo.git
    path: environments/dev
    targetRevision: main
```

## Upgrade del cluster

Il comando `upgrade` aggiorna un cluster esistente applicando solo le differenze rispetto alla configurazione attuale. Il cluster non viene ricreato.

```bash
./deploy-cluster upgrade --config cluster.yaml
```

Cosa fa:
- **Storage**: se abilitato e non installato, lo installa. Se già presente, ri-applica il manifest (idempotente).
- **ArgoCD**: se abilitato e non installato, fa un'installazione completa. Se già presente:
  - Ri-applica il manifest ArgoCD (aggiorna la versione se cambiata)
  - **Repos**: applica quelli desiderati (idempotente), elimina quelli non più in configurazione
  - **Apps**: applica quelle desiderate (idempotente), elimina quelle non più in configurazione
  - Se disabilitato ma presente nel cluster, mostra un warning (non disinstalla automaticamente)

## Status del cluster

Il comando `status` mostra lo stato corrente del cluster e dei plugin installati.

```bash
./deploy-cluster status --config cluster.yaml
```

Output di esempio:

```
Cluster: my-cluster
Provider: kind
Status: running

Storage: installed (local-path-provisioner)

ArgoCD: installed (namespace: argocd)
  Repos (1):
    - app-repo
  Apps (2):
    - nginx
    - my-app
```

## Accesso ad ArgoCD

Dopo la creazione del cluster con ArgoCD abilitato:

```bash
# Port forward per accedere alla UI
kubectl port-forward svc/argocd-server -n argocd 8080:443

# Apri https://localhost:8080

# Ottieni la password admin
kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" | base64 -d
```
