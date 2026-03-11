# Agent Gateway 设计文档

> 关联总览：[系统设计总览](system-design-overview.md)
> 关联：[Event Bus 设计](event-bus.md) | [Operator 设计](operator.md)

## 一、职责边界

Agent Gateway 是所有 AI Agent 对外请求的**唯一出口**，提供透明代理、身份识别和审计记录功能。

**该做（✅）**
- 反向代理 Agent 的所有出站请求（LLM 推理 / Gitea API / Git HTTP clone&push）
- 从 Bearer Token 中识别 Agent 身份（`agent_id`、`agent_role`）
- 将请求元数据**异步**写入 Redis Stream `events:gateway`
- 转发请求到实际后端（LiteLLM / Gitea）

**不该做（❌）**
- 不做业务逻辑判断（内容过滤、权限决策）
- 不直接写 ClickHouse（通过 Event Bus 异步）
- 不修改请求/响应内容（透明代理）
- 不做 Rate Limiting（限流在 LiteLLM 层）
- 不存储请求/响应 body（body 写入 Event Bus，由 ClickHouse Writer 决定是否持久化）

## 二、认证机制

### Token 格式

使用 **JWT（HS256）**，由 Operator 在 Reconcile 创建 Agent 时签发，存入 K8S Secret。

```json
{
  "sub": "agent-developer-1",
  "agent_id": "agent-developer-1",
  "agent_role": "developer",
  "iat": 1741651200,
  "exp": 0
}
```

> `exp = 0` 表示永不过期。Token 与 Agent 生命周期绑定：Agent CR 删除时，`agent-{name}-jwt-token` Secret 随 ownerReference 级联删除，Token 自然失效。

### JWT 签名密钥

Gateway 用于验签的密钥存于：

```
K8S Secret: agent-gateway-jwt-secret
  Namespace: control-plane
  Key: secret   ← 随机 256-bit hex 字符串
```

Operator 用于签发 JWT 也使用同一个密钥，通过 Secret 挂载到 Operator Pod：

```yaml
# Operator Deployment 中挂载
env:
- name: JWT_SIGNING_SECRET
  valueFrom:
    secretKeyRef:
      name: agent-gateway-jwt-secret
      key: secret
```

Gateway ConfigMap 中引用 Secret 名称（不直接含密钥值）：

```yaml
auth:
  jwtSecretRef: "agent-gateway-jwt-secret"  # Gateway 启动时加载密钥
```

**初始化**：首次部署时手动创建（或由 Helm chart 生成）：

```bash
kubectl create secret generic agent-gateway-jwt-secret \
  --from-literal=secret=$(openssl rand -hex 32) \
  -n control-plane
```

### JWT Token 存储与挂载

Operator 签发后，将 JWT 存入独立 Secret：

```
K8S Secret: agent-{name}-jwt-token
  Namespace: agents
  Key: token   ← JWT 字符串
```

挂载到 Agent Pod（文件形式，不用环境变量，避免 `kubectl describe` 泄漏）：

```yaml
volumes:
- name: jwt-token
  secret:
    secretName: agent-developer-1-jwt-token
volumeMounts:
- name: jwt-token
  mountPath: /agent/secrets/jwt-token
  subPath: token
  readOnly: true
```

OpenClaw entrypoint.sh 启动时从文件读取并注入 `openclaw.json`：

```bash
JWT_TOKEN=$(cat /agent/secrets/jwt-token)
# 写入 openclaw.json 的 apiKey 字段
```

### 认证流程

```
Agent 请求
  │  Authorization: Bearer <jwt>
  ▼
Gateway 中间件
  ├─ 验证 JWT 签名（使用 agent-gateway-jwt-secret）
  ├─ 检查 exp 字段：exp=0 视为永不过期，直接跳过时间校验
  ├─ 解析 agent_id / agent_role
  ├─ 注入请求头：X-Agent-ID, X-Agent-Role（供后端日志使用）
  └─ 继续转发
```

未携带 Token 或 Token 无效 → 返回 `401 Unauthorized`，不转发请求，异步写入 `events:gateway` 记录拒绝事件。

### Token 轮换

当前版本不做自动轮换（`exp=0`，Token 与 Agent 生命周期一致）。

若需强制失效（如 Agent 疑似被入侵），操作流程：

```bash
# 1. 删除旧 JWT Secret（触发 Pod 因找不到 Volume 而重启）
kubectl delete secret agent-developer-1-jwt-token -n agents

# 2. Operator 自动检测 Secret 缺失，在下次 Reconcile 时重新签发新 Token
# （Operator Watch agents namespace 的 Secret 变化，或通过定期 Reconcile 发现）

# 3. Pod 以新 Token 重建，旧 Token 无法再通过 Gateway 验证
# （因为旧 Token 的 iat 与新 Token 不同，Gateway 无法区分）
```

> **注意**：由于 `exp=0`，Gateway 本身无法主动拒绝旧 Token（无过期时间可检查）。强制失效的唯一手段是**轮换 JWT 签名密钥**（`agent-gateway-jwt-secret`），但这会使所有 Agent 的 Token 同时失效，需要 Operator 批量重新签发。这是当前方案的已知局限，MVP 阶段可接受。

## 三、路由规则

| 请求路径前缀 | 转发目标 | 说明 |
|------------|---------|------|
| `/v1/*`（OpenAI 兼容接口） | `LiteLLM Proxy :4000` | LLM 推理请求 |
| `/api/v1/*`（Gitea REST API） | `Gitea :3000` | Gitea API 操作 |
| `/git/*`（Git HTTP Smart Protocol） | `Gitea :3000` | Git clone / push / fetch |

路由规则通过配置文件（ConfigMap）管理，不硬编码。

### 路径重写规则

```
/api/v1/{owner}/{repo}.git/*  →  Gitea: /{owner}/{repo}.git/*
/v1/chat/completions           →  LiteLLM: /v1/chat/completions
/v1/models                     →  LiteLLM: /v1/models
```

## 四、审计事件写入

每次请求完成后，Gateway **异步**（非阻塞）向 Redis Stream `events:gateway` 写入审计事件：

```json
{
  "event_id": "uuid-v4",
  "event_type": "gateway.{request_type}",
  "timestamp": "2026-03-11T13:00:00.000Z",
  "agent_id": "agent-developer-1",
  "agent_role": "developer",
  "payload": {
    "trace_id": "uuid",
    "method": "POST",
    "path": "/v1/chat/completions",
    "status_code": 200,
    "model": "gpt-4o",
    "tokens_in": 1200,
    "tokens_out": 350,
    "latency_ms": 2100,
    "request_size_bytes": 4096,
    "response_size_bytes": 1024
  }
}
```

`event_type` 枚举：

| `event_type` | 触发条件 |
|-------------|---------|
| `gateway.llm_inference` | 转发到 LiteLLM 的请求 |
| `gateway.gitea_api` | 转发到 Gitea REST API 的请求 |
| `gateway.git_http` | 转发到 Gitea Git HTTP 的请求 |
| `gateway.heartbeat` | Agent 心跳请求（特殊 path `/heartbeat`） |
| `gateway.auth_failed` | Token 验证失败 |

> **重要**：request_body 和 response_body 不写入 Event Bus（避免 Redis 内存压力）。完整的 LLM 请求/响应 body 由 Gateway 直接记录到结构化 log，由 log collector 另行处理（当前版本暂不实现）。

## 五、心跳处理

Agent Runtime 每 5 分钟向 Gateway 的特殊端点 `POST /heartbeat` 发送心跳。

Gateway 处理逻辑：
1. 验证 Token，提取 `agent_id`
2. 返回 `200 OK`（响应体：`{"status":"ok","timestamp":"..."}`）
3. 写入 `events:gateway` Stream（`event_type: gateway.heartbeat`）

Backend 通过消费 ClickHouse 的 `audit.traces` 表检测心跳超时（详见 [系统设计总览 §10](system-design-overview.md#十agent-健康检查与异常恢复)）。

## 六、错误处理

| 场景 | Gateway 行为 |
|------|------------|
| 后端（LiteLLM/Gitea）不可达 | 返回 `502 Bad Gateway`，写入审计事件（status_code=502） |
| 后端请求超时（默认 60s） | 返回 `504 Gateway Timeout`，写入审计事件 |
| Redis 不可达（审计写入失败） | **不影响请求转发**，降级为本地日志记录，不返回错误给 Agent |
| JWT 验证失败 | 返回 `401 Unauthorized`，**不转发**，写入审计事件 |

## 七、性能设计

- **异步审计**：事件写入 Redis Stream 使用独立 goroutine，不阻塞请求链路
- **连接池**：维护到 LiteLLM 和 Gitea 的 HTTP 连接池（Keep-Alive）
- **流式响应透传**：LLM streaming 响应（SSE）直接透传，不缓冲

## 八、部署配置

```yaml
# Gateway ConfigMap
upstream:
  litellm: "http://litellm.llm-gateway.svc:4000"
  gitea: "http://gitea.devops-infra.svc:3000"

redis:
  addr: "redis.storage.svc:6379"
  stream: "events:gateway"

auth:
  jwtSecretRef: "agent-gateway-jwt-secret"

timeout:
  read: 30s
  write: 60s
  idle: 120s
```

## 九、高可用与运维

### 9.1 多副本高可用

Gateway 完全无状态（JWT 验证只需密钥，Redis 写入为 fire-and-forget），支持水平扩展：

```yaml
# Gateway Deployment
replicas: 2                         # 生产建议 2 副本
strategy:
  type: RollingUpdate
  rollingUpdate:
    maxUnavailable: 0               # 零停机滚动更新
    maxSurge: 1
```

流量通过 K8S Service（ClusterIP）负载均衡，Agent 连接 `http://agent-gateway.control-plane.svc`，无需感知副本数量。

Redis 写入失败时降级为本地日志，不影响请求转发，副本间无共享状态，无需 leader election。

### 9.2 速率保护

防止单个 Agent 异常时（死循环、失控）刷爆 Redis Stream：

**两层保护：**

**① Gateway 层：per-Agent 请求频率限制**

在 Gateway ConfigMap 中配置：

```yaml
rateLimit:
  enabled: true
  perAgent:
    requestsPerMinute: 60          # 每分钟最多 60 次请求（1 次/秒）
    burstSize: 10                  # 允许短时间突发 10 次
  strategy: token-bucket           # 令牌桶算法，内存实现，不依赖 Redis
```

超出限制时：返回 `429 Too Many Requests`，写入 `events:gateway`（`event_type: gateway.rate_limited`），**不写入被限流的请求详情**（避免 Redis 被刷爆的同时产生更多 Redis 写入）。

**② Redis Stream 层：MAXLEN 硬顶**

已在 `event-bus.md §2.2` 定义，`events:gateway MAXLEN ~ 100000`，旧消息自动裁剪。即使 Gateway 层失效，Redis 也不会无限增长。

### 9.3 Git Large File 处理

Agent 执行 `git clone` 大仓库时可能耗时很长，Gateway 需要特殊处理：

```yaml
timeout:
  default:
    read: 30s
    write: 60s
  gitHttp:                         # /git/* 路径单独配置
    read: 300s                     # clone 最长允许 5 分钟
    write: 300s
    idle: 60s                      # 无数据传输 60s 后断开
```

对于超大仓库（>500MB），Agent SKILL.md 中约定使用 `--depth=1` 浅克隆：

```bash
git clone --depth=1 http://agent-gateway.../git/{org}/{repo}.git
```

### 9.4 request/response body 记录

MVP 阶段不记录完整 body（避免 Redis 内存压力和敏感数据泄漏）。

当前记录：请求元数据（method、path、status_code、latency、token 数量）

未来扩展方案（post-MVP）：Gateway 将 body 写入结构化日志文件 → Fluentd/Vector 采集 → 独立存储。此时无需修改现有 Event Bus 架构。
