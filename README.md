# EntropyGen — AI Agent Operations Platform

A Kubernetes-native platform for running autonomous AI agents that collaborate through Gitea. Agents observe repositories, implement issues, review code, and monitor infrastructure — all orchestrated via a control panel.

## Architecture

```
Frontend (React)  →  Backend API  →  K8s CRD (Agent)
                           ↓               ↓
                       ClickHouse     Operator
                       Redis          (reconciles pods)
                           ↑
              Gateway  ←  Agent Pods  →  Gitea
           (JWT proxy)   (LLM + tools)
```

**Components:**
| Component | Path | Port |
|-----------|------|------|
| Control Panel (frontend) | `frontend/` | 3000 |
| Backend API | `cmd/backend/` | 8080 |
| Agent Gateway | `cmd/gateway/` | 8090 |
| Event Collector | `cmd/event-collector/` | — |
| K8s Operator | `cmd/operator/` | — |

---

## Local Development

### Prerequisites

- Go 1.22+
- Node.js 20+
- Docker + Docker Compose

### 1. Start infrastructure services

```bash
./scripts/dev-up.sh
```

Starts Redis (`:6380`), ClickHouse (`:9000`), and Gitea (`http://localhost:3000`) via Docker Compose. Waits for all services to be healthy.

To stop:
```bash
./scripts/dev-down.sh
```

### 2. Start the backend

```bash
./scripts/dev-backend.sh
```

Starts the control panel backend on `http://localhost:8080`.

**Default credentials:** `admin` / `admin`

> The backend connects to Redis and ClickHouse. K8s is optional — if no cluster is configured, agent CRUD operations will return errors but all other features (auth, audit log, monitoring) work normally.

### 3. Start the frontend

```bash
./scripts/dev-frontend.sh
```

Starts the Vite dev server on `http://localhost:3000`. API requests to `/api/*` are proxied to the backend at `:8080`.

Open `http://localhost:3000` and sign in with `admin` / `admin`.

### All three in parallel

```bash
./scripts/dev-up.sh          # wait for healthy, then in separate terminals:
./scripts/dev-backend.sh &
./scripts/dev-frontend.sh
```

---

## Production Deployment (Kubernetes)

### 1. Initialize secrets

```bash
./scripts/init-secrets.sh --namespace control-plane
```

### 2. Install via Helm

```bash
# Apply CRDs first
kubectl apply -f k8s/helm/templates/crd.yaml

# Install the platform
helm install aidevops-platform k8s/helm/ \
  --namespace control-plane \
  --create-namespace \
  --values k8s/helm/values.yaml
```

### 3. Configure Ingress

Add to `/etc/hosts` (or configure DNS):

```
<cluster-ingress-ip>  control.devops.local
<cluster-ingress-ip>  gitea.devops.local
```

Access the control panel at `http://control.devops.local`.

---

## Environment Variables (backend)

| Variable | Default | Description |
|----------|---------|-------------|
| `LISTEN_ADDR` | `:8080` | HTTP listen address |
| `ADMIN_USERNAME` | — | Control panel admin username |
| `ADMIN_PASSWORD_HASH` | — | bcrypt hash of admin password |
| `JWT_SECRET` | — | JWT signing secret |
| `REDIS_ADDR` | `redis.storage.svc:6379` | Redis address |
| `CLICKHOUSE_ADDR` | — | ClickHouse native address |
| `CLICKHOUSE_DB` | `audit` | ClickHouse database |
| `LITELLM_ADDR` | — | LiteLLM proxy address |
| `AGENT_NAMESPACE` | `agents` | K8s namespace for agent pods |

To generate a new password hash:

```bash
go run scripts/genhash.go <password>
```

---

## Project Structure

```
.
├── cmd/                    # Entrypoints (backend, gateway, operator, ...)
├── frontend/               # React control panel
│   └── src/
│       ├── api/            # Axios API clients
│       ├── components/     # Shared components
│       ├── pages/          # Route pages
│       ├── stores/         # Zustand state
│       └── hooks/          # Custom hooks (WebSocket, ...)
├── internal/               # Go packages
│   ├── backend/            # Control panel API handlers
│   ├── gateway/            # Agent JWT proxy
│   ├── operator/           # K8s controller
│   └── event-collector/    # Gitea webhook → Redis streams
├── k8s/
│   └── helm/               # Helm chart
└── scripts/                # Dev and ops scripts
```
