# deploy-cluster

CLI tool per il deploy di cluster Kubernetes locali con supporto plugin.

Permette di creare cluster con topologia configurabile (numero di worker e control plane) e installare automaticamente componenti come storage (local-path-provisioner), ingress (nginx), cert-manager, monitoring (Prometheus/Grafana), dashboard (Headlamp), ArgoCD e applicazioni custom via Helm, definendo tutto da un singolo file di configurazione.

## Requisiti

- Go 1.21+
- Docker
- [kind](https://kind.sigs.k8s.io/)
- kubectl
- [Helm](https://helm.sh/) (per monitoring, dashboard e customApps)

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

# Preview delle modifiche
./deploy-cluster upgrade --config cluster.yaml --dry-run

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
| `upgrade --dry-run` | Mostra le modifiche senza applicarle |
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
| `--dry-run` | upgrade | `false` | Preview modifiche senza applicarle |

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
  ingress:
    enabled: true
    type: nginx
  certManager:
    enabled: true
    version: v1.16.3
  monitoring:
    enabled: true
    type: prometheus
    ingress:
      enabled: true
      host: grafana.localhost
  dashboard:
    enabled: true
    type: headlamp
    ingress:
      enabled: true
      host: headlamp.localhost
  customApps:
    - name: redis
      chart: oci://registry-1.docker.io/bitnamicharts/redis
      version: "21.1.5"
      namespace: redis
      values:
        architecture: standalone
        auth:
          enabled: false
    - name: rabbitmq
      chart: oci://registry-1.docker.io/bitnamicharts/rabbitmq
      version: "14.0.0"
      namespace: rabbitmq
      valuesFile: ./rabbitmq-values.yaml
      ingress:
        enabled: true
        host: rabbitmq.localhost
        serviceName: rabbitmq
        servicePort: 15672
  argocd:
    enabled: true
    namespace: argocd
    version: stable
    ingress:
      enabled: true
      host: argocd.localhost
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

### Ordine di installazione

I plugin vengono installati in quest'ordine: **storage → ingress → cert-manager → monitoring → dashboard → customApps → ArgoCD**. Storage per primo in modo che i PVC siano disponibili; ingress prima degli altri per consentire l'accesso via hostname; ArgoCD per ultimo perché potrebbe dipendere da tutti gli altri.

### Cluster

| Campo | Tipo | Default | Descrizione |
|-------|------|---------|-------------|
| `name` | string | `my-cluster` | Nome del cluster |
| `provider.type` | string | `kind` | Provider (`kind`) |
| `cluster.controlPlanes` | int | `1` | Numero di control plane |
| `cluster.workers` | int | `2` | Numero di worker |
| `cluster.version` | string | `v1.31.0` | Versione Kubernetes |

### Plugin Storage

Installa un provisioner per StorageClass nel cluster. Viene installato prima degli altri plugin, in modo che eventuali PVC richiesti da altri componenti siano già disponibili.

| Campo | Tipo | Default | Descrizione |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Abilita l'installazione dello storage |
| `type` | string | **obbligatorio** | Tipo di provisioner: `local-path` |

#### Tipi supportati

| Tipo | Descrizione |
|------|-------------|
| `local-path` | [Rancher local-path-provisioner](https://github.com/rancher/local-path-provisioner) — crea volumi sul filesystem del nodo. Ideale per cluster locali di sviluppo. Viene impostato come StorageClass di default. |

```yaml
plugins:
  storage:
    enabled: true
    type: local-path
```

### Plugin Ingress

Installa un ingress controller nel cluster per esporre i servizi via HTTP/HTTPS. Quando abilitato, il nodo control-plane di kind viene configurato automaticamente con la label `ingress-ready=true` e i port mapping per le porte 80 e 443.

| Campo | Tipo | Default | Descrizione |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Abilita l'installazione dell'ingress controller |
| `type` | string | **obbligatorio** | Tipo di controller: `nginx` |

#### Tipi supportati

| Tipo | Descrizione |
|------|-------------|
| `nginx` | [ingress-nginx](https://kubernetes.github.io/ingress-nginx/) — controller ufficiale NGINX per Kubernetes. Usa il manifest specifico per kind. |

```yaml
plugins:
  ingress:
    enabled: true
    type: nginx
```

Dopo l'installazione, le risorse `Ingress` con `ingressClassName: nginx` vengono gestite automaticamente. I servizi diventano raggiungibili su `http://<host>` tramite gli hostname configurati (es. `argocd.localhost`, `grafana.localhost`).

### Plugin Cert-Manager

Installa [cert-manager](https://cert-manager.io/) per la gestione automatica dei certificati TLS nel cluster.

| Campo | Tipo | Default | Descrizione |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Abilita l'installazione di cert-manager |
| `version` | string | `v1.16.3` | Versione di cert-manager |

```yaml
plugins:
  certManager:
    enabled: true
    version: v1.16.3
```

### Plugin Monitoring

Installa [kube-prometheus-stack](https://github.com/prometheus-community/helm-charts/tree/main/charts/kube-prometheus-stack) via Helm (chart OCI), che include Prometheus, Grafana, Alertmanager, node-exporter e kube-state-metrics.

| Campo | Tipo | Default | Descrizione |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Abilita l'installazione del monitoring |
| `type` | string | **obbligatorio** | Tipo di stack: `prometheus` |
| `version` | string | `72.6.2` | Versione del chart Helm |
| `ingress.enabled` | bool | `false` | Crea un Ingress per Grafana |
| `ingress.host` | string | - | Hostname per Grafana (es. `grafana.localhost`) |

```yaml
plugins:
  monitoring:
    enabled: true
    type: prometheus
    ingress:
      enabled: true
      host: grafana.localhost
```

Dopo l'installazione:

```bash
# Con ingress: http://grafana.localhost (admin/prom-operator)

# Senza ingress (port-forward):
kubectl port-forward svc/kube-prometheus-stack-grafana -n monitoring 3000:80
# http://localhost:3000 (admin/prom-operator)

# Prometheus
kubectl port-forward svc/kube-prometheus-stack-prometheus -n monitoring 9090:9090
# http://localhost:9090
```

### Plugin Dashboard

Installa [Headlamp](https://headlamp.dev/) come dashboard Kubernetes via Helm.

| Campo | Tipo | Default | Descrizione |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Abilita l'installazione del dashboard |
| `type` | string | **obbligatorio** | Tipo di dashboard: `headlamp` |
| `version` | string | `0.25.0` | Versione del chart Helm |
| `ingress.enabled` | bool | `false` | Crea un Ingress per Headlamp |
| `ingress.host` | string | - | Hostname per Headlamp (es. `headlamp.localhost`) |

```yaml
plugins:
  dashboard:
    enabled: true
    type: headlamp
    ingress:
      enabled: true
      host: headlamp.localhost
```

### Custom Apps

Permette di installare qualsiasi chart Helm arbitrario senza dover creare un plugin dedicato. Ogni entry nella lista diventa un `helm upgrade --install`.

| Campo | Tipo | Default | Descrizione |
|-------|------|---------|-------------|
| `name` | string | **obbligatorio** | Nome della release Helm |
| `chart` | string | **obbligatorio** | Chart Helm (OCI, repo URL, path locale) |
| `version` | string | - | Versione del chart |
| `namespace` | string | uguale a `name` | Namespace di installazione |
| `values` | map | - | Valori Helm inline |
| `valuesFile` | string | - | Path a un file di values esterno |
| `ingress.enabled` | bool | `false` | Crea un Ingress per l'app |
| `ingress.host` | string | - | Hostname |
| `ingress.serviceName` | string | uguale a `name` | Nome del service backend |
| `ingress.servicePort` | int | `80` | Porta del service backend |

#### Esempi

```yaml
plugins:
  customApps:
    # Chart OCI con values inline
    - name: redis
      chart: oci://registry-1.docker.io/bitnamicharts/redis
      version: "21.1.5"
      namespace: redis
      values:
        architecture: standalone
        auth:
          enabled: false

    # Chart con values da file e ingress
    - name: rabbitmq
      chart: oci://registry-1.docker.io/bitnamicharts/rabbitmq
      version: "14.0.0"
      namespace: rabbitmq
      valuesFile: ./rabbitmq-values.yaml
      ingress:
        enabled: true
        host: rabbitmq.localhost
        serviceName: rabbitmq
        servicePort: 15672

    # Chart senza versione specifica (latest)
    - name: whoami
      chart: oci://ghcr.io/traefik/charts/whoami
      namespace: whoami
      ingress:
        enabled: true
        host: whoami.localhost
```

### Plugin ArgoCD

| Campo | Tipo | Default | Descrizione |
|-------|------|---------|-------------|
| `enabled` | bool | `false` | Abilita l'installazione di ArgoCD |
| `namespace` | string | `argocd` | Namespace di installazione |
| `version` | string | `stable` | Versione di ArgoCD |
| `ingress.enabled` | bool | `false` | Crea un Ingress per la UI di ArgoCD |
| `ingress.host` | string | - | Hostname (es. `argocd.localhost`) |
| `ingress.tls` | bool | `false` | Abilita TLS via cert-manager |

Quando l'ingress è abilitato, ArgoCD server viene configurato automaticamente in modalità `--insecure` (TLS interno disabilitato) tramite la ConfigMap `argocd-cmd-params-cm`, in modo che l'ingress nginx possa fare proxy HTTP.

```yaml
plugins:
  argocd:
    enabled: true
    namespace: argocd
    version: stable
    ingress:
      enabled: true
      host: argocd.localhost
```

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

### Tipi di Application ArgoCD

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
# Preview delle modifiche
./deploy-cluster upgrade --config cluster.yaml --dry-run

# Applica le modifiche
./deploy-cluster upgrade --config cluster.yaml
```

Cosa fa:
- **Storage**: se abilitato e non installato, lo installa. Se già presente, ri-applica il manifest (idempotente).
- **Ingress**: se abilitato e non installato, lo installa. Se già presente, ri-applica il manifest (idempotente).
- **Cert-Manager**: se abilitato e non installato, lo installa. Se già presente, ri-applica il manifest (aggiorna versione se cambiata).
- **Monitoring**: se abilitato e non installato, lo installa via Helm. Se già presente, `helm upgrade` aggiorna (idempotente).
- **Dashboard**: se abilitato e non installato, lo installa via Helm. Se già presente, `helm upgrade` aggiorna.
- **Custom Apps**: ogni app viene installata/aggiornata con `helm upgrade --install` (idempotente).
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

Ingress: installed (nginx)

Cert-manager: installed

Monitoring: installed (prometheus)

Dashboard: installed (headlamp)

Custom Apps (2 configured):
  - redis: installed
  - rabbitmq: installed

ArgoCD: installed (namespace: argocd)
  Repos (1):
    - app-repo
  Apps (2):
    - nginx
    - my-app
```

## Accesso alle UI

Con ingress abilitato, le UI sono accessibili direttamente via hostname:

| Servizio | URL | Credenziali |
|----------|-----|-------------|
| ArgoCD | `http://argocd.localhost` | admin / `kubectl -n argocd get secret argocd-initial-admin-secret -o jsonpath="{.data.password}" \| base64 -d` |
| Grafana | `http://grafana.localhost` | admin / prom-operator |
| Headlamp | `http://headlamp.localhost` | Service Account token |
| Prometheus | port-forward: `kubectl port-forward svc/kube-prometheus-stack-prometheus -n monitoring 9090:9090` | - |

Senza ingress, usare `kubectl port-forward`:

```bash
# ArgoCD
kubectl port-forward svc/argocd-server -n argocd 8080:443
# https://localhost:8080

# Grafana
kubectl port-forward svc/kube-prometheus-stack-grafana -n monitoring 3000:80
# http://localhost:3000

# Headlamp
kubectl port-forward svc/headlamp -n headlamp 4466:80
# http://localhost:4466
```
