# Event Bus 设计文档

> 关联总览：[系统设计总览](system-design-overview.md)
> 关联：[Agent Gateway 设计](agent-gateway.md) | [Control Panel Backend 设计](control-panel-backend.md)

## 一、概述

Event Bus 基于 **Redis Stream** 实现，是平台所有异步事件的统一传输通道。

**核心数据流**：
```
生产者                    Redis Stream            消费者
──────────────────────────────────────────────────────────
Agent Gateway    →   events:gateway   →   ClickHouse Writer
Event Collector  →   events:gitea     →   ClickHouse Writer
Event Collector  →   events:k8s       →   ClickHouse Writer
                                      →   Backend WebSocket → Frontend
```

## 二、Stream 配置

### 2.1 三个 Stream

| Stream | 生产者 | 事件内容 | 预估日写入量 |
|--------|--------|---------|------------|
| `events:gateway` | Agent Gateway | Agent 所有出站请求（LLM/Gitea API/Git/心跳） | ~10,000 条/天（5 Agent，每个 Agent ~2000 次请求） |
| `events:gitea` | Event Collector | Gitea Webhook 事件（push/issue/PR/CI） | ~500 条/天 |
| `events:k8s` | Event Collector，Operator | K8S 事件 + Operator 告警 | ~200 条/天 |

### 2.2 Stream 保留策略

```
XADD events:gateway MAXLEN ~ 100000 * ...
XADD events:gitea   MAXLEN ~ 10000  * ...
XADD events:k8s     MAXLEN ~ 10000  * ...
```

- `~` 表示近似裁剪（性能更好，允许略超上限）
- 超出上限的旧消息自动删除（ClickHouse 是持久存储，Redis 不需要保留历史）

## 三、Consumer Group 设计

### 3.1 消费组列表

| Consumer Group | 消费的 Stream | 消费者（实例） | 作用 |
|---------------|-------------|-------------|------|
| `ch-writer` | `events:gateway`, `events:gitea`, `events:k8s` | Backend (1 个实例) | 批量写入 ClickHouse |
| `ws-push` | `events:gateway`, `events:gitea`, `events:k8s` | Backend (1 个实例) | 过滤后推送 WebSocket |

### 3.2 Consumer Group 操作

```bash
# 创建 Consumer Group（首次部署时由 Backend 自动创建）
XGROUP CREATE events:gateway ch-writer $ MKSTREAM
XGROUP CREATE events:gitea   ch-writer $ MKSTREAM
XGROUP CREATE events:k8s     ch-writer $ MKSTREAM

XGROUP CREATE events:gateway ws-push $ MKSTREAM
XGROUP CREATE events:gitea   ws-push $ MKSTREAM
XGROUP CREATE events:k8s     ws-push $ MKSTREAM
```

> `$` 表示从当前最新消息开始消费（历史消息不处理，只处理新增消息）。

### 3.3 消费流程

```
XREADGROUP GROUP ch-writer backend-1 COUNT 100 BLOCK 5000 STREAMS events:gateway >
  │
  ├─ 获取到 messages
  │   └─ 缓冲到本地 buffer
  │   └─ buffer 满 100 条 OR 等待超 5 秒 → 批量 INSERT INTO ClickHouse
  │   └─ 写入成功 → XACK events:gateway ch-writer {id1} {id2} ...
  │   └─ 写入失败 → 不 ACK，记录 error log，下次轮询重试
  │
  └─ 超时（5 秒无新消息）→ 触发已有 buffer 的写入（避免数据积压）
```

### 3.4 Pending 消息处理

Backend 重启后，未 ACK 的消息仍在 Pending 列表中，重启后自动重新消费：

```bash
# 查看 pending 消息
XPENDING events:gateway ch-writer - + 10

# 重新消费 pending 消息（Backend 启动时执行）
XAUTOCLAIM events:gateway ch-writer backend-1 0 0-0 COUNT 100
```

## 四、完整事件格式规范

所有事件遵循统一外层结构：

```json
{
  "event_id": "uuid-v4",
  "event_type": "<source>.<action>",
  "timestamp": "2026-03-11T13:00:00.000Z",
  "agent_id": "agent-developer-1",
  "agent_role": "developer",
  "payload": { ... }
}
```

> `agent_id` 和 `agent_role` 在非 Agent 产生的事件（如 Gitea Webhook）中为空字符串或省略。

### 4.1 events:gateway 事件

#### LLM 推理（`gateway.llm_inference`）
```json
{
  "payload": {
    "trace_id": "uuid",
    "method": "POST",
    "path": "/v1/chat/completions",
    "status_code": 200,
    "model": "gpt-4o",
    "tokens_in": 1200,
    "tokens_out": 350,
    "latency_ms": 2100
  }
}
```

#### Gitea API 调用（`gateway.gitea_api`）
```json
{
  "payload": {
    "trace_id": "uuid",
    "method": "POST",
    "path": "/api/v1/repos/org/repo/issues",
    "status_code": 201,
    "latency_ms": 45
  }
}
```

#### 心跳（`gateway.heartbeat`）
```json
{
  "payload": {
    "trace_id": "uuid",
    "status": "ok"
  }
}
```

### 4.2 events:gitea 事件

#### Issue 创建（`gitea.issue_open`）
```json
{
  "agent_id": "",
  "agent_role": "",
  "payload": {
    "repo": "org/platform-demo",
    "issue_number": 42,
    "title": "Fix memory leak in worker pool",
    "labels": ["type/bug", "priority/high", "role/developer"],
    "creator": "agent-observer-1"
  }
}
```

#### PR 合并（`gitea.pr_merge`）
```json
{
  "payload": {
    "repo": "org/platform-demo",
    "pr_number": 15,
    "title": "Fix memory leak in worker pool",
    "merged_by": "agent-reviewer-1",
    "closes_issues": [42]
  }
}
```

#### CI 状态（`gitea.ci_status`）
```json
{
  "payload": {
    "repo": "org/platform-demo",
    "workflow": "CI",
    "status": "success",
    "commit": "abc123",
    "pr_number": 15
  }
}
```

### 4.3 events:k8s 事件

#### Agent 告警（`operator.agent_alert`）
```json
{
  "payload": {
    "alert_type": "agent.crash_loop",
    "restart_count": 5,
    "message": "Pod has restarted 5 times"
  }
}
```

#### Pod 状态变化（`k8s.pod_status`）
```json
{
  "payload": {
    "pod_name": "agent-developer-1-xxx",
    "namespace": "agents",
    "status": "Running",
    "previous_status": "Pending"
  }
}
```

## 五、Redis 资源使用规划

Redis 实例同时被以下用途使用：

| 用途 | Key 前缀 | 内存估算 |
|------|---------|---------|
| Event Bus Streams | `events:*` | ~50 MB（基于 maxlen 设置） |
| LiteLLM 推理缓存 | `litellm:*` | ~200 MB（依实际缓存命中率） |
| Consumer Group 元数据 | 内部 | ~5 MB |

**建议**：使用单 Redis 实例，配置 `maxmemory 512mb`，策略 `allkeys-lru`（LiteLLM 缓存可接受 LRU 淘汰，Event Stream 有 MAXLEN 自行控制）。

## 六、Event Collector 设计

Event Collector 是独立的轻量 HTTP 服务，负责接收外部推送事件并写入 Redis Stream。

### 接口

| 端点 | 说明 |
|------|------|
| `POST /webhook/gitea` | 接收 Gitea Webhook，验证 HMAC 签名后写入 `events:gitea` |
| `GET /healthz` | 健康检查 |

### K8S Event Watch

Event Collector 同时作为 K8S Event Watcher（使用 `client-go` Informer）：
- Watch `agents` namespace 的 Pod Events（Scheduled/Started/Failed/OOMKilled）
- 将 K8S Events 格式化后写入 `events:k8s` Stream

## 七、告警与降级

| 场景 | 处理策略 |
|------|---------|
| Redis 不可达（Gateway 写入失败） | 降级为本地 structured log，**不影响请求转发** |
| Redis 不可达（Event Collector） | Webhook 返回 `503`，依赖 Gitea 自身的 Webhook 重试（默认重试 3 次） |
| ClickHouse 写入失败 | 不 ACK，保留在 Pending 列表，Backend 重启后重试 |
| Consumer Group 消息积压超过 10000 条 | 告警（写入 `events:k8s`），可能需要手动 XTRIM 处理 |

## 八、补充设计

### 8.1 事件 Schema 版本管理

MVP 阶段采用**字段宽容策略**：消费者只读自己关心的字段，忽略未知字段，不做严格 schema 校验。

具体规则：

```
生产者（Gateway / Event Collector）：
  - 只允许新增字段，不允许删除或重命名已有字段
  - 新增可选字段时，消费者用 omitempty 处理（字段不存在时用零值）

消费者（ClickHouse Writer / WebSocket）：
  - 使用结构体解析，未知字段自动忽略（Go json.Unmarshal 默认行为）
  - ClickHouse 表新增列时设置 DEFAULT 值，历史数据自动补 NULL/零值
```

事件外层结构（`event_id`、`event_type`、`timestamp`、`agent_id`、`agent_role`、`payload`）视为稳定契约，不得变更字段名。`payload` 内部字段可按上述规则演进。

不引入 schema registry（Kafka Schema Registry 等），复杂度与当前规模不匹配。

### 8.2 超大消息截断策略

针对 `git push` 大 diff 等可能产生大 payload 的场景：

| 事件类型 | 最大 payload 大小 | 超出时的处理 |
|---------|----------------|------------|
| `gateway.git_http` | 1 KB | 只记录元数据（method、path、status_code、size_bytes），不记录 body |
| `gitea.push` | 4 KB | 截断 `commits[].diff` 字段，保留 `commits[].id`、`message`、`author` |
| 其他事件 | 16 KB | 整体截断，追加 `"_truncated": true` 标记 |

截断在生产者侧执行（Gateway 和 Event Collector），写入 Redis 前完成，避免大消息占用 Redis 内存。

### 8.3 多 Backend 副本时的 Consumer Group 竞争

当前设计 Backend 单副本运行，Consumer Group 无竞争问题。

若未来扩展为多副本：

- Redis Consumer Group 天然支持多消费者并发读取（`XREADGROUP` 不同副本读到不同消息，自动分片）
- 无需额外协调，每条消息只会被一个副本消费一次
- ClickHouse 批量写入保持幂等（`INSERT` 操作本身幂等，重复写入同一 `trace_id` 时 ClickHouse 会去重）
- 唯一需要注意：WebSocket 推送端，多副本时每个 Backend 只推送自己消费到的事件子集，前端会收到所有事件（因为每个连接只连接一个 Backend 副本，由 K8S Service 负载均衡决定）

MVP 阶段单副本，此问题暂不需处理。

## 九、待细化

（所有待细化项已解决）
