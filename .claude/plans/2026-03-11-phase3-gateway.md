# Phase 3: Agent Gateway — 透明代理 + JWT 认证 + 审计

> 关联设计：[Agent Gateway 设计](../designs/agent-gateway.md) | [Event Bus 设计](../designs/event-bus.md)
> 依赖 Phase：Phase 1（common 包、Redis client、Event model）
> 可并行：Phase 2 (Operator)、Phase 4 (Event Collector) 可同步进行

## 目标

实现 Agent Gateway：所有 Agent 对外请求的唯一出口，提供：
- JWT Bearer Token 认证（HS256，由 Operator 签发）
- 透明反向代理（LiteLLM / Gitea API / Git HTTP）
- 异步审计事件写入 Redis Stream（仅 LLM 请求记录 body）
- Per-Agent 速率限制（60 req/min，突发 10）
- 特殊心跳端点（`POST /heartbeat`）

---

## 依赖的外部服务

| 服务 | 用途 |
|------|------|
| LiteLLM Proxy (:4000) | 转发 `/v1/*` LLM 推理请求 |
| Gitea (:3000) | 转发 `/api/v1/*` 和 `/git/*` |
| Redis (`events:gateway`) | 异步写入审计事件 |
| `agent-gateway-jwt-secret` | 验证 Agent JWT Token |

---

## 任务拆解

### 3.1 Gateway 主程序

**文件**：`cmd/gateway/main.go`

- [ ] 从环境变量 / ConfigMap 读取配置：
  ```go
  type Config struct {
      ListenAddr    string        // :8080
      LiteLLMAddr   string        // http://litellm.llm-gateway.svc:4000
      GiteaAddr     string        // http://gitea.devops-infra.svc:3000
      RedisAddr     string        // redis.storage.svc:6379
      JWTSecretRef  string        // K8S Secret name
      RateLimit     RateLimitConfig
      Timeout       TimeoutConfig
  }
  ```
- [ ] 初始化 Redis client、JWT verifier、Rate limiter、反向代理
- [ ] 启动 HTTP server（标准库 `net/http`）

### 3.2 JWT 认证中间件

**文件**：`internal/gateway/handler/auth.go`

- [ ] 中间件从 `Authorization: Bearer <jwt>` 提取 Token
- [ ] 验签（HS256，`agent-gateway-jwt-secret`）
- [ ] `exp=0` 特殊处理：跳过时间校验
- [ ] 解析 `agent_id`、`agent_role`，注入请求上下文
- [ ] 注入下游请求头：`X-Agent-ID`、`X-Agent-Role`
- [ ] Token 无效 → `401 Unauthorized`，写入 `events:gateway` 拒绝事件，**不转发**

**测试（TDD）**：
- 有效 JWT → 请求通过，context 中有 agent_id/role
- 无效 JWT → 401，审计事件 event_type=gateway.auth_failed
- 缺少 Authorization header → 401
- exp=0 的 Token 永不过期，时间任意均通过

### 3.3 反向代理核心

**文件**：`internal/gateway/handler/proxy.go`

**路由规则**（按前缀匹配）：

| 路径前缀 | 转发目标 | 超时 |
|---------|---------|------|
| `/v1/` | LiteLLM `:4000` | 60s（LLM 推理） |
| `/api/v1/` | Gitea `:3000` | 30s |
| `/git/` | Gitea `:3000`（路径重写去掉 `/git` 前缀） | 300s（clone） |

- [ ] 使用 `httputil.ReverseProxy`
- [ ] Git HTTP 路径重写：`/git/{owner}/{repo}.git/*` → `/{owner}/{repo}.git/*`
- [ ] 保持 SSE/流式响应透传（LLM streaming，`Transfer-Encoding: chunked`）
- [ ] 后端不可达 → `502 Bad Gateway`
- [ ] 超时 → `504 Gateway Timeout`
- [ ] 响应完成后触发异步审计写入（不阻塞响应返回）

**测试**：
- LLM 请求正确转发到 LiteLLM，路径不变
- Git clone 路径 `/git/org/repo.git/info/refs` 转发到 Gitea 时变为 `/org/repo.git/info/refs`
- 后端超时 → 504，审计事件 status_code=504

### 3.4 审计事件写入

**文件**：`internal/gateway/audit/event_writer.go`

- [ ] 异步 goroutine，buffered channel 接收审计请求（channel size=1000）
- [ ] 构建 `common/models.Event`，`event_type` 根据路径前缀判断
- [ ] **LLM 推理专用**：从请求/响应读取 body，截断到 64KB，设 `_body_truncated`
- [ ] 调用 `redisclient.StreamWriter.Add("events:gateway", event)`
- [ ] Redis 不可达时：降级写入结构化 log（`logger.Warn`），**不影响请求链路**

**body 记录规则**：
```
gateway.llm_inference:
  - request_body:  POST body（OpenAI messages JSON），截断到 64KB
  - response_body: 响应 body，截断到 64KB
  - _body_truncated: true/false
其他 event_type: request_body="", response_body=""
```

**测试**：
- LLM 请求 → 审计事件含 request_body、response_body、model、tokens
- Gitea API 请求 → 审计事件 request_body=""
- body 超 64KB → 截断，_body_truncated=true
- Redis 不可达 → 请求正常返回，日志中有 warn

### 3.5 心跳端点

**文件**：`internal/gateway/handler/heartbeat.go`

- [ ] `POST /heartbeat`：验证 JWT，返回 `{"status":"ok","timestamp":"..."}` 200
- [ ] 写入 `events:gateway`（`event_type: gateway.heartbeat`）

**测试**：有效 JWT → 200，审计事件写入；无效 JWT → 401

### 3.6 速率限制

**文件**：`internal/gateway/ratelimit/token_bucket.go`

- [ ] Per-Agent 令牌桶（内存，不依赖 Redis）
- [ ] 配置：`requestsPerMinute=60`，`burstSize=10`
- [ ] 超限 → `429 Too Many Requests`，写入审计事件（`event_type: gateway.rate_limited`），**不写被限流请求详情**
- [ ] `sync.Map` 存储各 Agent 的令牌桶，定期清理长期不活跃的桶

**测试**：
- 连续 70 次请求（同 agent_id）→ 第 61 次返回 429
- 不同 agent_id 相互独立，不共享桶

### 3.7 错误处理

| 场景 | Gateway 行为 |
|------|------------|
| 后端不可达 | 502，写审计（status_code=502） |
| 后端超时（读取 60s） | 504，写审计 |
| Redis 不可达 | 降级 log，不影响转发 |
| JWT 验证失败 | 401，写审计，不转发 |
| 速率超限 | 429，写审计 |

### 3.8 高可用与部署

- [ ] Gateway 完全无状态（JWT 验签只需密钥，Redis 写入 fire-and-forget）
- [ ] 填充 `k8s/helm/templates/gateway-deployment.yaml`：
  - `replicas: 2`，RollingUpdate（maxUnavailable=0，maxSurge=1）
  - 挂载 `agent-gateway-jwt-secret`
  - ConfigMap 挂载路由配置
- [ ] 健康检查端点：`GET /healthz`（200 即可）

---

## 验收标准

- [ ] `go test ./internal/gateway/...` 全部通过（覆盖率 ≥ 80%）
- [ ] 在本地启动 Gateway，使用有效 JWT 发送 LLM 代理请求
  - 请求正确转发到 LiteLLM mock
  - Redis Stream 中出现 `events:gateway` 事件，payload 含 request_body（测试 mock body）
- [ ] Git clone 路径重写正确（curl 验证）
- [ ] 连续请求触发 429 速率限制
- [ ] 无效 JWT → 401，事件写入 Redis
