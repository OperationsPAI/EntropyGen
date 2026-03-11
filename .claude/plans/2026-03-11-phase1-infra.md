# Phase 1: 基础设施层 — CRD + RBAC + Helm + 公共包

> 关联设计：[系统设计总览](../designs/system-design-overview.md)
> 依赖 Phase：无（最先执行）
> 可并行：Phase 7 (agent-tooling) 可同步进行

## 目标

搭建整个项目的骨架，所有后续 Phase 都依赖这里的产出：

1. Go module 初始化（单 module，`github.com/entropyGen/entropyGen`）
2. Agent CRD 定义（YAML + Go types）
3. K8S RBAC 预设（Operator ClusterRole + Agent ClusterRole）
4. Helm Chart 框架（values.yaml + 所有 template 骨架）
5. `internal/common` 公共包（event model、Redis/ClickHouse/Gitea client 封装、logger）

---

## 依赖的外部服务

| 服务 | 说明 |
|------|------|
| K8S API | 用于 CRD 安装和 RBAC 创建 |
| Redis | 公共包中的 Stream client |
| ClickHouse | 公共包中的 native protocol client |
| Gitea | 公共包中的 API client |

---

## 任务拆解

### 1.1 Go 模块初始化

- [ ] 确认 `go.mod` module 名称和 Go 版本（1.23）
- [ ] 添加核心依赖：
  - `sigs.k8s.io/controller-runtime`（Operator 框架）
  - `k8s.io/client-go`
  - `k8s.io/apimachinery`
  - `github.com/redis/go-redis/v9`
  - `github.com/ClickHouse/clickhouse-go/v2`
  - `code.gitea.io/sdk/gitea`
  - `github.com/golang-jwt/jwt/v5`
  - `go.uber.org/zap`（结构化日志）
  - `github.com/gin-gonic/gin`（Backend HTTP 框架）
  - `github.com/gorilla/websocket`
  - `github.com/spf13/cobra`（gitea-cli CLI 框架）
- [ ] 运行 `go mod tidy`

### 1.2 Agent CRD 定义

- [ ] 写入 `k8s/crds/agent-crd.yaml`（来自设计文档 §五完整 YAML）
  - group: `aidevops.io`, version: `v1alpha1`, kind: `Agent`
  - spec 字段：role、soul、skills、cron、llm、gitea、kubernetes、resources、memory、paused
  - status 字段：phase、conditions、giteaUser、lastAction、tokenUsage、podName、startedAt
  - additionalPrinterColumns、subresources.status
- [ ] 写入 `internal/operator/api/types.go`：Go struct 定义（AgentSpec、AgentStatus、Agent、AgentList）
- [ ] 运行 `controller-gen object:headerFile="..." paths="./internal/operator/api/..."` 生成 `zz_generated.deepcopy.go`
- [ ] 写入 `internal/operator/api/register.go`：将 Agent 类型注册到 scheme

**验收**：`kubectl apply -f k8s/crds/agent-crd.yaml` 无错误，`kubectl get crd agents.aidevops.io` 存在。

### 1.3 K8S RBAC

- [ ] 写入 `k8s/rbac/operator-clusterrole.yaml`：Operator ClusterRole（来自设计文档 §7.2-7.6 完整 YAML）
  - agent CRD CRUD
  - deployments/configmaps/secrets/serviceaccounts/pvcs CRUD
  - rolebindings create（agents + app-staging）
  - pods/pods/log get/list/watch
  - events create/patch
  - leases CRUD（Leader Election）
- [ ] 写入 `k8s/rbac/agent-clusterroles.yaml`：
  - `agent-readonly-role`（observer/developer/reviewer）
  - `sre-agent-role` Role（app-staging namespace，Deployment/Service/Ingress）

### 1.4 Helm Chart 框架

- [ ] 写入 `k8s/helm/Chart.yaml`（apiVersion v2，name: aidevops-platform，version: 0.1.0）
- [ ] 写入 `k8s/helm/values.yaml`：所有组件的可配置项
  ```yaml
  global:
    registry: registry.devops.local/platform
    tag: latest

  operator:
    replicas: 1
    image: operator

  gateway:
    replicas: 2
    image: gateway
    jwtSecretRef: agent-gateway-jwt-secret

  backend:
    replicas: 1
    image: backend
    adminUsername: admin
    # adminPasswordHash from secret

  eventCollector:
    replicas: 1
    image: event-collector

  frontend:
    replicas: 1
    image: control-panel-frontend

  redis:
    addr: redis.storage.svc:6379
    maxmemory: 512mb

  clickhouse:
    addr: clickhouse.storage.svc:9000
    database: audit

  litellm:
    addr: http://litellm.llm-gateway.svc:4000

  gitea:
    addr: http://gitea.devops-infra.svc:3000
    # adminToken from secret

  agentNamespace: agents
  ```
- [ ] 创建所有 template 骨架文件（空文件，后续各 Phase 填充）：
  - `operator-deployment.yaml`、`gateway-deployment.yaml`、`backend-deployment.yaml`
  - `event-collector-deployment.yaml`、`frontend-deployment.yaml`
  - `gitea-init-job.yaml`、`ingress.yaml`、`networkpolicies.yaml`
  - `configmaps.yaml`（Gateway config）、`secrets.yaml`（secret template）
  - `agent-namespace.yaml`（创建 agents namespace）
  - `rbac.yaml`（引用 k8s/rbac/）
  - `crd.yaml`（引用 k8s/crds/）

### 1.5 `internal/common` 公共包

#### 1.5.1 Event Model（`internal/common/models/event.go`）

- [ ] 定义统一事件结构体：
  ```go
  type Event struct {
      EventID   string          `json:"event_id"`
      EventType string          `json:"event_type"`
      Timestamp time.Time       `json:"timestamp"`
      AgentID   string          `json:"agent_id"`
      AgentRole string          `json:"agent_role"`
      Payload   json.RawMessage `json:"payload"`
  }
  ```
- [ ] 定义各 Payload 结构（GatewayLLMPayload、GatewayGiteaPayload、GiteaIssuePayload 等）
- [ ] 定义 event_type 常量

**测试**：JSON 序列化/反序列化往返，payload 字段正确提取。

#### 1.5.2 Redis Stream Client（`internal/common/redisclient/`）

- [ ] `stream_writer.go`：封装 `XADD ... MAXLEN ~ N`，支持 fire-and-forget 异步写入
- [ ] `stream_reader.go`：封装 `XREADGROUP`，支持 Consumer Group 读取 + ACK
- [ ] `pending.go`：封装 `XAUTOCLAIM`，处理 pending 消息（服务重启恢复）

**测试**：写入一条事件，消费组读取，ACK 后不再出现在 pending 列表。

#### 1.5.3 ClickHouse Client（`internal/common/chclient/`）

- [ ] `clickhouse_client.go`：建立连接池（native protocol, TCP 9000）
- [ ] `ddl.go`：`audit.traces` 表的 CREATE TABLE SQL + 两个物化视图的 CREATE SQL（幂等，加 `IF NOT EXISTS`）
- [ ] `insert.go`：批量 INSERT 接口（传入 `[]models.AuditTrace`）
- [ ] `query.go`：常用查询封装（traces 分页、token_usage_daily、agent_operations_hourly、heartbeat 检测）

**测试**：建表（幂等），插入 3 条记录，查询验证字段映射正确。

#### 1.5.4 Gitea API Client（`internal/common/giteaclient/`）

- [ ] `client.go`：封装 `code.gitea.io/sdk/gitea`，提供统一初始化入口
- [ ] 暴露常用操作方法：CreateUser、CreateToken、AddCollaborator、DeleteUser

**测试**：Mock Gitea HTTP 服务器，验证 Admin Token 正确传递、参数正确。

#### 1.5.5 Logger（`internal/common/logger/`）

- [ ] `logger.go`：基于 `go.uber.org/zap`，提供结构化日志
- [ ] 支持 JSON 格式（生产）和 Console 格式（开发）
- [ ] 自动注入 `service`、`version` 字段

### 1.6 示例 Agent CR

- [ ] 写入 `k8s/examples/observer.yaml`、`developer.yaml`、`reviewer.yaml`、`sre.yaml`
- [ ] 覆盖 Agent CRD 所有字段，便于测试和演示

### 1.7 init-secrets.sh

- [ ] 写入 `scripts/init-secrets.sh`：
  - 创建 `agent-gateway-jwt-secret`
  - 创建 `gitea-webhook-secret`
  - 创建 `gitea-admin-token`（提示用户填入）
  - 创建 `clickhouse-creds`

---

## 验收标准

- [ ] `go build ./...` 无错误（空实现不报错）
- [ ] `kubectl apply -f k8s/crds/agent-crd.yaml` 成功
- [ ] `kubectl apply -f k8s/rbac/` 成功
- [ ] `go test ./internal/common/...` 全部通过
- [ ] `helm template aidevops-platform k8s/helm/` 无语法错误
