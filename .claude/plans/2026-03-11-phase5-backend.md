# Phase 5: Control Panel Backend — REST API + ClickHouse Writer + WebSocket

> 关联设计：[Control Panel Backend 设计](../designs/control-panel-backend.md) | [Event Bus 设计](../designs/event-bus.md)
> 依赖 Phase：Phase 1（common 包）、Phase 3（events:gateway 格式）、Phase 4（events:gitea/k8s 格式）
> 被依赖：Phase 6 (Frontend)

## 目标

实现 Control Panel Backend，功能包括：
1. REST API（Agent 管理、LLM 配置、审计查询、训练数据导出）
2. JWT 认证（管理员登录）
3. WebSocket 实时事件推送
4. ClickHouse Writer（消费 Redis Stream，批量写入）
5. Dead Letter Queue（写入失败本地缓存 + 定期重试）
6. 心跳超时检测（定时查 ClickHouse，更新 Agent CR status）

---

## 依赖的外部服务

| 服务 | 用途 |
|------|------|
| K8S API | agent CR CRUD，Pod 日志查询 |
| ClickHouse (:9000) | 查询审计数据、批量写入 audit.traces |
| Redis (`events:*`) | Consumer Group 消费，WebSocket 推送 |
| LiteLLM (:4000) | 代理 LLM 模型配置管理 |

---

## 任务拆解

### 5.1 Backend 主程序框架

**文件**：`cmd/backend/main.go`

- [ ] 初始化 Gin router、ClickHouse client、Redis client、K8S client
- [ ] 启动后台 goroutine：
  - ClickHouse Writer（Consumer Group `ch-writer`）
  - WebSocket Pusher（Consumer Group `ws-push`）
  - 心跳检测定时器（每 5 分钟）
  - DLQ 重试扫描（每 10 分钟）
- [ ] JWT 认证中间件（管理员 Token，与 Agent JWT 不同）

### 5.2 认证

**文件**：`internal/backend/handler/auth.go`

- [ ] `POST /api/auth/login`：验证 `ADMIN_USERNAME` + bcrypt 密码 → 签发 24h JWT
- [ ] `GET /api/auth/me`：返回当前用户信息
- [ ] `POST /api/auth/logout`：客户端删除 Token（Backend 无状态，返回 200）
- [ ] 中间件：所有 `/api/*` 路由（除 `/api/auth/login` 和 `/api/health`）需要有效 JWT

**测试**：正确密码 → JWT；错误密码 → 401；JWT 过期 → 401。

### 5.3 Agent 管理 API

**文件**：`internal/backend/handler/agents.go` + `internal/backend/k8sclient/agent_cr.go`

- [ ] `GET /api/agents` → `client-go` list agent CRs，返回列表（含 status）
- [ ] `POST /api/agents` → create agent CR（校验 role 枚举、必填字段）
- [ ] `GET /api/agents/:name` → get agent CR
- [ ] `PUT /api/agents/:name` → patch agent CR spec（SOUL.md/LLM/resources/cron）
- [ ] `DELETE /api/agents/:name` → delete agent CR（Operator Finalizer 负责清理）
- [ ] `POST /api/agents/:name/pause` → patch `spec.paused=true`
- [ ] `POST /api/agents/:name/resume` → patch `spec.paused=false`
- [ ] `GET /api/agents/:name/logs` → `client-go` pods/log subresource，最后 200 行

**错误响应格式**（统一）：
```json
{
  "success": false,
  "error": "agent not found",
  "code": "AGENT_NOT_FOUND",
  "detail": "agent 'developer-1' does not exist in namespace 'agents'"
}
```

**测试**：创建 → 列表可查；pause → status.phase=Paused；删除 → 列表不再出现。

### 5.4 LLM 配置管理（代理到 LiteLLM）

**文件**：`internal/backend/handler/llm.go`

- [ ] `GET /api/llm/models` → 转发 `GET /model/info` 到 LiteLLM
- [ ] `POST /api/llm/models` → 转发 `POST /model/new`
- [ ] `PUT /api/llm/models/:id` → 转发 `POST /model/update`
- [ ] `DELETE /api/llm/models/:id` → 转发 `POST /model/delete`
- [ ] `GET /api/llm/health` → 转发 `GET /health`
- [ ] 透明转发：接收前端 JSON → 直接传给 LiteLLM → 返回结果

**测试**：Mock LiteLLM，验证请求转发路径、参数透传正确。

### 5.5 审计数据查询

**文件**：`internal/backend/handler/audit.go`

- [ ] `GET /api/audit/traces`：支持参数过滤 + 分页
  - 参数：`agent_id`、`request_type`、`start`、`end`、`page`（默认 1）、`limit`（默认 50，最大 200）
  - 调用 `chclient.QueryTraces(filters, page, limit)`
- [ ] `GET /api/audit/traces/:trace_id`：单条完整 trace
- [ ] `GET /api/audit/stats/token-usage`：`token_usage_daily` 查询（7 天/30 天）
- [ ] `GET /api/audit/stats/agent-activity`：今日 Agent 活跃度排行
- [ ] `GET /api/audit/stats/operations`：`agent_operations_hourly` 查询
- [ ] `GET /api/audit/export`：流式 JSONL 导出（训练数据 trajectory）
  - 支持参数：`agent_id`、`start`、`end`
  - 先 COUNT 查询，响应头含 `X-Total-Count`
  - 流式 SELECT，100 行 flush 一次
  - 从 `request_body` 提取 messages，从 `response_body` 提取 response，组装 JSONL
  - 并发限制：全局最多 2 个并发导出（超出 429）

**测试**：分页查询返回正确条数；导出 JSONL 每行格式正确（`{"messages":[...],"response":{...}}`）。

### 5.6 WebSocket 实时推送

**文件**：`internal/backend/wspush/websocket_pusher.go` + `internal/backend/handler/ws.go`

- [ ] `WS /api/ws/events`：Upgrade 到 WebSocket
  - 可选参数 `?agent_id=xxx` 过滤特定 Agent 事件
  - 连接建立后注册到 Pusher hub
- [ ] Pusher 订阅 Redis Consumer Group `ws-push`（三个 Stream）
  - `XREADGROUP GROUP ws-push backend-1 COUNT 50 BLOCK 1000`
  - 读到事件 → 广播到匹配的 WebSocket 连接
  - ACK（ws-push 不需要持久化，消费即 ACK）
- [ ] 客户端断开 → 从 hub 注销
- [ ] ping/pong 心跳（30s），断连自动清理

**测试**：WebSocket 连接后，向 Redis 写入测试事件，验证客户端收到；agent_id 过滤正确。

### 5.7 ClickHouse Writer（内嵌后台服务）

**文件**：`internal/backend/chwriter/clickhouse_writer.go`

- [ ] Consumer Group `ch-writer`，消费三个 Stream
  - `events:gateway` → `audit.traces`（含 request_body/response_body 映射）
  - `events:gitea` → `audit.traces`（request_type 来自 event_type）
  - `events:k8s` → `audit.traces`
- [ ] 批量写入策略：
  - 每个 Stream 独立 buffer（100 条）
  - 触发：buffer 满 OR 等待超 5 秒
  - 写入成功 → XACK
  - 写入失败 → 不 ACK，重试 3 次，失败后写 DLQ
- [ ] 字段映射（`events:gateway → audit.traces`）：
  ```
  trace_id, agent_id, agent_role, method, path, status_code,
  model, tokens_in, tokens_out, latency_ms,
  request_body, response_body → 直接映射
  event_type → request_type（去掉 "gateway." 前缀）
  timestamp → created_at
  ```
- [ ] 启动时执行 `XAUTOCLAIM`，处理上次重启未 ACK 的 pending 消息

**Dead Letter Queue（DLQ）**：

- [ ] 写入失败超 3 次 → 序列化为 JSONL 存入 `backend-dlq` PVC
  - 路径：`/var/lib/backend/dlq/events-{timestamp}-{stream}.jsonl`
- [ ] XACK（从 pending 移除，不阻塞后续消息）
- [ ] DLQ 扫描（每 10 分钟）：尝试重新写入，成功删文件，失败保留（最多 7 天）

**测试**：
- 写入 5 条 events:gateway 事件 → ClickHouse audit.traces 出现对应记录
- 模拟 ClickHouse 不可达 → 事件写入 DLQ 文件，恢复后重试写入成功

### 5.8 心跳超时检测

**文件**：`internal/backend/heartbeat/detector.go`

- [ ] 定时器每 5 分钟执行一次：
  ```sql
  SELECT agent_id, max(created_at) AS last_heartbeat
  FROM audit.traces
  WHERE request_type = 'heartbeat'
    AND created_at >= now() - INTERVAL 1 HOUR
  GROUP BY agent_id
  HAVING last_heartbeat < now() - INTERVAL 15 MINUTE;
  ```
- [ ] 查到超时 Agent → K8S API patch CR `status.phase=Error`
- [ ] 向 `events:k8s` Stream 写入 `agent.heartbeat_timeout` 告警
- [ ] WebSocket 推送触发（通过 Pusher hub）

**测试**：模拟 ClickHouse 中 agent-x 无心跳记录 → 验证 K8S CR status.phase=Error，告警事件写入。

### 5.9 ClickHouse 表初始化

- [ ] Backend 启动时，调用 `chclient.EnsureTables()`（幂等 DDL）：
  - `audit.traces` 表
  - `token_usage_daily` 物化视图
  - `agent_operations_hourly` 物化视图

### 5.10 Backend Helm Template

- [ ] 填充 `k8s/helm/templates/backend-deployment.yaml`：
  - 挂载 `backend-dlq` PVC（1Gi）
  - 环境变量：CLICKHOUSE_ADDR、REDIS_ADDR、LITELLM_ADDR、ADMIN_USERNAME、JWT_SECRET 等
  - ServiceAccount（需要 K8S API 读写 agent CR、Pod 日志）

---

## 验收标准

- [ ] `go test ./internal/backend/...` 全部通过（覆盖率 ≥ 80%）
- [ ] 完整 API 集成测试：
  - 登录 → 获取 Token → 创建 Agent CR → 查询列表 → 删除
  - WebSocket 连接后写入 Redis 事件 → 前端收到推送
  - 审计查询 → ClickHouse 有数据
  - 训练数据导出 → JSONL 格式正确，messages/response 字段存在
- [ ] ClickHouse Writer 写入 100 条事件，全部出现在 audit.traces
- [ ] 模拟心跳超时，CR status 自动更新为 Error
