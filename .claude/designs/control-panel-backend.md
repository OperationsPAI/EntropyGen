# Control Panel Backend 设计文档

> 关联总览：[系统设计总览](system-design-overview.md)
> 关联：[Control Panel Frontend 设计](control-panel-frontend.md) | [Event Bus 设计](event-bus.md)

## 一、职责

Control Panel Backend 是管控面的 Go HTTP 服务，提供以下能力：

1. **Agent 管理**：通过 K8S API 对 agent CR 进行 CRUD
2. **LLM 模型管理**：代理转发到 LiteLLM REST API
3. **审计数据查询**：查询 ClickHouse 中的 `audit.traces` 及物化视图
4. **实时事件推送**：订阅 Redis Stream，通过 WebSocket 推送到前端
5. **ClickHouse Writer**：内嵌消费 Redis Stream，批量写入 ClickHouse
6. **心跳超时检测**：定时查询 ClickHouse，发现超时 Agent 后通过 K8S API 更新 CR status
7. **JWT 认证**：管理员登录认证（用户名/密码），签发 JWT

## 二、完整 API 列表

### 2.1 认证

| Method | Path | 说明 |
|--------|------|------|
| `POST` | `/api/auth/login` | 管理员登录，返回 JWT |
| `POST` | `/api/auth/logout` | 注销（客户端删除 Token） |
| `GET` | `/api/auth/me` | 获取当前用户信息 |

### 2.2 Agent 管理

| Method | Path | 说明 |
|--------|------|------|
| `GET` | `/api/agents` | 列出所有 agent CR |
| `POST` | `/api/agents` | 创建 agent CR |
| `GET` | `/api/agents/:name` | 获取单个 agent CR |
| `PUT` | `/api/agents/:name` | 更新 agent CR（spec 变更） |
| `DELETE` | `/api/agents/:name` | 删除 agent CR |
| `POST` | `/api/agents/:name/pause` | 暂停 Agent（设置 spec.paused=true） |
| `POST` | `/api/agents/:name/resume` | 恢复 Agent（设置 spec.paused=false） |
| `GET` | `/api/agents/:name/logs` | 获取 Agent Pod 最近日志（kubectl logs） |

### 2.3 LLM 模型管理

| Method | Path | LiteLLM 转发目标 | 说明 |
|--------|------|----------------|------|
| `GET` | `/api/llm/models` | `GET /model/info` | 列出所有已配置模型 |
| `POST` | `/api/llm/models` | `POST /model/new` | 新增模型配置 |
| `PUT` | `/api/llm/models/:id` | `POST /model/update` | 更新模型配置 |
| `DELETE` | `/api/llm/models/:id` | `POST /model/delete` | 删除模型配置 |
| `GET` | `/api/llm/health` | `GET /health` | LiteLLM 健康检查 |

### 2.4 审计数据查询

| Method | Path | 说明 |
|--------|------|------|
| `GET` | `/api/audit/traces` | 查询审计记录（支持分页、过滤） |
| `GET` | `/api/audit/traces/:trace_id` | 获取单条完整 trace |
| `GET` | `/api/audit/stats/token-usage` | Token 消耗趋势（按天聚合） |
| `GET` | `/api/audit/stats/agent-activity` | Agent 活跃度排行 |
| `GET` | `/api/audit/stats/operations` | Agent 操作统计（按小时聚合） |
| `GET` | `/api/audit/export` | 导出训练数据（JSONL 格式，流式响应） |

#### 查询参数示例（`GET /api/audit/traces`）

```
?agent_id=agent-developer-1
&request_type=llm_inference
&start=2026-03-01T00:00:00Z
&end=2026-03-11T23:59:59Z
&page=1
&limit=50
```

### 2.5 实时事件（WebSocket）

| Path | 说明 |
|------|------|
| `WS /api/ws/events` | 实时事件推送，订阅所有 `events:*` Stream |
| `WS /api/ws/events?agent_id=xxx` | 过滤特定 Agent 的事件 |

WebSocket 消息格式与 Event Bus 事件格式一致（见 [Event Bus 设计](event-bus.md)）。

### 2.6 系统信息

| Method | Path | 说明 |
|--------|------|------|
| `GET` | `/api/health` | Backend 自身健康检查 |
| `GET` | `/api/version` | 版本信息 |

## 三、ClickHouse Writer（内嵌服务）

Backend 启动时，后台 goroutine 以 Consumer Group 方式消费三个 Redis Stream：

```
events:gateway  ─┐
events:gitea   ──┼──→ [Consumer Group: ch-writer] ──→ ClickHouse Writer ──→ audit.traces
events:k8s     ─┘
```

### 批量写入策略

```
每个 Stream 维护独立缓冲区：
- 缓冲大小：100 条
- 最大等待时间：5 秒
- 触发条件：缓冲满 100 条 OR 等待超过 5 秒

写入完成后：XACK 确认消费
写入失败：重试 3 次，失败后记录到 error log（不丢弃，等下次重启后从 pending 继续）
```

### 字段映射（Event → ClickHouse）

```
events:gateway payload → audit.traces
  trace_id        → trace_id (UUID)
  agent_id        → agent_id
  agent_role      → agent_role
  event_type      → request_type （去掉 "gateway." 前缀）
  method          → method
  path            → path
  status_code     → status_code
  model           → model
  tokens_in       → tokens_in
  tokens_out      → tokens_out
  latency_ms      → latency_ms
  request_body    → request_body  （仅 llm_inference 有值，其余为空字符串）
  response_body   → response_body （仅 llm_inference 有值，其余为空字符串）
  timestamp       → created_at
```

## 四、心跳超时检测

Backend 启动后台定时任务（每 5 分钟执行一次）：

```sql
-- 检测超过 15 分钟无心跳的 Agent
SELECT agent_id, max(created_at) AS last_heartbeat
FROM audit.traces
WHERE request_type = 'heartbeat'
  AND created_at >= now() - INTERVAL 1 HOUR
GROUP BY agent_id
HAVING last_heartbeat < now() - INTERVAL 15 MINUTE;
```

检测到超时 Agent 后：
1. 通过 K8S API 更新 agent CR `status.phase = Error`
2. 向 `events:k8s` Stream 写入 `agent.heartbeat_timeout` 告警事件
3. WebSocket 推送告警到前端

## 五、认证设计

### JWT 策略

- 算法：HS256
- 有效期：24 小时（管理员手动操作场景，不需要长期 Token）
- 存储：前端 localStorage（管控面不是高安全场景，简化处理）
- 刷新：Token 过期前 1 小时内自动刷新

### 管理员账户

初期使用静态配置（环境变量注入用户名/密码 hash），不引入数据库。未来可扩展 LDAP。

```
ADMIN_USERNAME=admin
ADMIN_PASSWORD_HASH=bcrypt($password)
JWT_SECRET=<random 256-bit key>
```

## 六、错误响应格式

所有 API 使用统一错误格式：

```json
{
  "success": false,
  "error": "agent not found",
  "code": "AGENT_NOT_FOUND",
  "detail": "agent 'developer-1' does not exist in namespace 'agents'"
}
```

## 七、与其他组件的接口

| 方向 | 接口 | 说明 |
|------|------|------|
| Backend → K8S API | `client-go` REST client | agent CR CRUD，日志查询 |
| Backend → ClickHouse | TCP 9000，native protocol | 查询审计数据，批量写入 |
| Backend → Redis | TCP 6379，XREADGROUP | 消费 Stream（Writer + WebSocket） |
| Backend → LiteLLM | HTTP 4000 | 模型配置管理代理 |
| Frontend → Backend | HTTPS + WebSocket | 所有管控操作 |

## 八、部署配置

```yaml
# Backend 环境变量
KUBECONFIG: ""  # 集群内运行，使用 in-cluster config
CLICKHOUSE_ADDR: "clickhouse.storage.svc:9000"
CLICKHOUSE_DB: "audit"
REDIS_ADDR: "redis.storage.svc:6379"
LITELLM_ADDR: "http://litellm.llm-gateway.svc:4000"
ADMIN_USERNAME: "admin"
ADMIN_PASSWORD_HASH: "<bcrypt>"
JWT_SECRET: "<secret>"
AGENT_NAMESPACE: "agents"
```

## 九、补充设计

### 9.1 管理员账户

MVP 阶段使用静态单管理员配置（env var），不引入 LDAP 或多账户体系。多管理员、RBAC 属于 post-MVP 功能。

```
ADMIN_USERNAME=admin
ADMIN_PASSWORD_HASH=bcrypt($password)
```

只读管理员角色（不能创建/删除 Agent）同样推迟到 post-MVP，当前所有登录用户均为完整权限。

### 9.2 审计数据流式导出

`GET /api/audit/export` 使用 ClickHouse 的流式查询 + HTTP chunked transfer encoding，避免大数据集 OOM：

```
请求进入
  │
  ├─ COUNT 查询预估数据量（返回给前端展示）
  ├─ 设置响应头：Content-Type: application/x-ndjson
  │              Transfer-Encoding: chunked
  │              Content-Disposition: attachment; filename="audit-export.jsonl"
  │
  └─ 流式 SELECT，每 100 行 flush 一次到客户端
     → 客户端中断时服务端检测到写错误，提前终止查询
```

**训练数据 Trajectory 格式：**

每行为一条 Agent 推理记录，格式：

```json
{"messages": [{"role": "user", "content": "..."}, ...], "response": {"role": "assistant", "content": "..."}}
```

- `messages`：LLM 请求的 `messages` 数组（OpenAI format），从 `request_body` 字段提取
- `response`：LLM 返回的 `choices[0].message`，从 `response_body` 字段提取
- 过滤条件：`request_type = 'llm_inference' AND request_body != ''`（跳过无 body 的历史记录）

限流：同时最多允许 2 个并发导出请求（超出返回 `429`），防止大查询压垮 ClickHouse。

### 9.3 ClickHouse Writer Dead Letter Queue

写入失败超过 3 次重试后的处理：

```
写入失败（重试 3 次）
  │
  ├─ 将失败的 batch 序列化写入本地文件：
  │   /var/lib/backend/dlq/events-{timestamp}-{stream}.jsonl
  │
  ├─ 记录 error log（含 stream 名、消息 ID 列表、错误原因）
  │
  ├─ 执行 XACK（从 pending 移除，避免无限重试阻塞后续消息）
  │
  └─ Backend 后台任务每 10 分钟扫描 DLQ 目录，尝试重新写入
     成功 → 删除文件
     失败 → 保留，等待下次重试（最多保留 7 天）
```

DLQ 文件挂载到独立 PVC（`backend-dlq`，1Gi），与主存储隔离。

### 9.4 Backend 高可用

多副本时 ClickHouse Writer 的 Consumer Group 行为见 `event-bus.md §8.3`：Redis Consumer Group 天然支持多消费者并发，无需额外协调。

WebSocket 推送：每个副本只推送自己消费到的事件子集（由 Consumer Group 分片决定）。前端与单个 Backend 副本保持 WebSocket 长连接，副本重启时前端自动重连（3s 退避，最多 5 次）。

DLQ 文件存于各副本独立 PVC，不共享，各自独立重试。

MVP 阶段单副本，以上为多副本扩展时的预设方案。

## 十、待细化

（所有待细化项已解决）
