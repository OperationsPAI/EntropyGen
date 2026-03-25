# Local Development Environment (minikube)

This document describes how to set up and use a local Kubernetes development environment using minikube for the AI DevOps Platform.

## Prerequisites

- Docker (running and accessible by current user)
- Linux amd64 (the setup script downloads linux-amd64 binaries)

All other dependencies (minikube, kubectl, helm, skaffold) are automatically installed by the setup script into `~/.local/bin`.

## Quick Start

```bash
# 1. Install tools and start minikube cluster
./scripts/minikube-setup.sh start

# 2. Add ~/.local/bin to PATH (add to .bashrc/.zshrc for persistence)
export PATH="$HOME/.local/bin:$PATH"

# 3. Point docker CLI to minikube's docker daemon (images build directly inside the cluster)
eval $(minikube -p aidevops docker-env)

# 4. Deploy the full platform
skaffold run -p minikube

# 5. (Optional) Dev mode — watches for file changes and auto-rebuilds
skaffold dev -p minikube
```

## Architecture Differences from Production

| Aspect | Production | minikube |
|--------|-----------|----------|
| Registry | `10.10.10.240/library` (Harbor) | Local docker daemon (`push: false`) |
| Image prefix | `10.10.10.240/library/` | `aidevops/` |
| StorageClass | `juicefs-sc` | `standard` (hostpath) |
| Ingress | nginx ingress controller | Disabled (use NodePort / port-forward) |
| Resource limits | Full allocation | Reduced (~50% of production) |
| LLM config | Pre-configured API key | Must be provided manually |

## Cluster Configuration

The minikube setup script creates a profile named `aidevops` with:

| Parameter | Default | Environment Variable |
|-----------|---------|---------------------|
| CPUs | 4 | `MINIKUBE_CPUS` |
| Memory | 8192 MB | `MINIKUBE_MEMORY` |
| Disk | 40 GB | `MINIKUBE_DISK` |
| K8s version | v1.31.0 | `MINIKUBE_K8S_VERSION` |
| Driver | docker | — |
| Addons | default-storageclass, storage-provisioner, metrics-server | — |

Customize by setting environment variables before running the script:

```bash
MINIKUBE_CPUS=2 MINIKUBE_MEMORY=4096 ./scripts/minikube-setup.sh start
```

## Skaffold Profile

The `minikube` profile in `skaffold.yaml` applies:

- **Build**: `push: false`, concurrency 2 — builds images directly into minikube's Docker
- **Deploy**: Helm release with `values.yaml` + `values-minikube.yaml` overlay
- **Image names**: `aidevops/{backend,operator,gateway,event-collector,control-panel-frontend,agent-runtime}`

## Values Override (`k8s/helm/values-minikube.yaml`)

Key overrides:
- `storage.className: standard` — uses minikube's built-in hostpath provisioner
- `storage.rolesData.size: 1Gi` — smaller PVC for local dev
- `ingress.enabled: false` — no ingress controller needed
- All component resources reduced for laptop-friendly footprint
- `redis.maxmemory: 256mb` — smaller Redis allocation

## Accessing Services

```bash
# Frontend (NodePort 30083)
minikube service -p aidevops -n aidevops control-panel-frontend

# Gitea (NodePort 30030)
minikube service -p aidevops -n aidevops gitea

# Any service via port-forward
kubectl -n aidevops port-forward svc/control-panel-backend 8081:8081
kubectl -n aidevops port-forward svc/gateway 8080:8080
```

## LLM Configuration

The minikube profile leaves `llm.apiKey` and `llm.baseURL` empty. Provide them at deploy time:

```bash
skaffold run -p minikube \
  --set llm.apiKey=sk-your-key \
  --set llm.baseURL=https://your-api-endpoint/v1
```

Or edit `k8s/helm/values-minikube.yaml` directly.

## Lifecycle Commands

```bash
./scripts/minikube-setup.sh start    # Install deps + start cluster
./scripts/minikube-setup.sh stop     # Pause cluster (preserves state)
./scripts/minikube-setup.sh delete   # Destroy cluster completely
./scripts/minikube-setup.sh status   # Check cluster status
```

## Troubleshooting

### Images not found / ImagePullBackOff

Make sure you ran `eval $(minikube -p aidevops docker-env)` in the **same shell** before `skaffold run`. Each new terminal session needs this command.

### PVC Pending

Verify the `standard` StorageClass exists:
```bash
kubectl get sc
```
If missing, enable the addon: `minikube -p aidevops addons enable default-storageclass storage-provisioner`

### Insufficient resources

Reduce resource allocation:
```bash
MINIKUBE_CPUS=2 MINIKUBE_MEMORY=4096 ./scripts/minikube-setup.sh start
```

### Clean restart

```bash
./scripts/minikube-setup.sh delete
./scripts/minikube-setup.sh start
```

## Related Documents

- [System Design Overview](system-design-overview.md)
- [Operator](operator.md)
- [Agent Gateway](agent-gateway.md)
