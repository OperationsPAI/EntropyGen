# Phase 4: Event Collector — Gitea Webhook + K8S Event Watch

> 关联设计：[Event Bus 设计](../designs/event-bus.md) | [Gitea 集成设计](../designs/gitea-integration.md)
> 依赖 Phase：Phase 1（common 包、Redis client、Event model）
> 可并行：Phase 2 (Operator)、Phase 3 (Gateway) 可同步进行

## 目标

实现 Event Collector：轻量 HTTP 服务，负责：
1. 接收 Gitea Webhook（验证 HMAC-SHA256 签名），写入 `events:gitea` Stream
2. Watch K8S `agents` namespace 的 Pod Events，写入 `events:k8s` Stream

---

## 依赖的外部服务

| 服务 | 用途 |
|------|------|
| Gitea (Webhook push) | 发送 POST 到 `/webhook/gitea` |
| K8S API | Watch `agents` namespace Pod Events |
| Redis (`events:gitea`, `events:k8s`) | 写入格式化后的事件 |
| `gitea-webhook-secret` | 验证 Gitea Webhook HMAC-SHA256 签名 |

---

## 任务拆解

### 4.1 Event Collector 主程序

**文件**：`cmd/event-collector/main.go`

- [ ] 从环境变量读取配置：
  ```go
  type Config struct {
      ListenAddr     string  // :8080
      RedisAddr      string
      WebhookSecret  string  // 从 gitea-webhook-secret 读取
      AgentNamespace string  // "agents"
  }
  ```
- [ ] 初始化 Redis client、K8S client（in-cluster）
- [ ] 启动 HTTP server
- [ ] 启动 K8S Pod Event Watcher（goroutine）

### 4.2 Gitea Webhook 处理

**文件**：`internal/event-collector/webhook/gitea_handler.go`

- [ ] `POST /webhook/gitea` 处理器：
  1. 读取 `X-Gitea-Event` 请求头，识别事件类型
  2. 读取 `X-Gitea-Signature` 请求头，验证 HMAC-SHA256 签名
     ```go
     mac := hmac.New(sha256.New, []byte(secret))
     mac.Write(body)
     expected := hex.EncodeToString(mac.Sum(nil))
     // 与 "sha256=" + X-Gitea-Signature 比较
     ```
  3. 解析 Webhook payload（按事件类型反序列化）
  4. 转换为统一 `common/models.Event` 格式
  5. 写入 `events:gitea` Redis Stream（MAXLEN ~ 10000）
  6. 返回 `200 OK`（尽快返回，避免 Gitea 超时重试）

**Gitea 事件类型映射**：

| X-Gitea-Event | 触发条件 | `event_type` |
|--------------|---------|-------------|
| `push` | 代码推送 | `gitea.push` |
| `issues` / action=opened | Issue 创建 | `gitea.issue_open` |
| `issues` / action=closed | Issue 关闭 | `gitea.issue_close` |
| `issue_comment` | Issue/PR 评论 | `gitea.issue_comment` |
| `pull_request` / action=opened | PR 创建 | `gitea.pr_open` |
| `pull_request` / action=closed+merged | PR 合并 | `gitea.pr_merge` |
| `pull_request_review_comment` | PR Review | `gitea.pr_review` |
| `workflow_run` | CI 状态 | `gitea.ci_status` |

**payload 截断**（来自设计文档 §8.2）：

| 事件类型 | 最大 payload | 超出处理 |
|---------|------------|---------|
| `gitea.push` | 4 KB | 截断 `commits[].diff`，保留 id/message/author |
| 其他 | 16 KB | 整体截断，追加 `"_truncated": true` |

**测试（TDD）**：
- 正确 HMAC 签名 → 200，事件写入 Redis
- 错误签名 → 403，不写 Redis
- PR merge 事件（action=closed + merged=true）→ event_type=gitea.pr_merge
- push 事件 payload 超 4KB → commits.diff 被截断

### 4.3 K8S Pod Event Watcher

**文件**：`internal/event-collector/k8swatch/pod_watcher.go`

- [ ] 使用 `client-go` Informer Watch `agents` namespace 的 `v1.Event`
- [ ] 过滤关注的事件类型：
  - `reason=Scheduled/Started` → `k8s.pod_status`（status=Running）
  - `reason=Failed/BackOff` → `k8s.pod_status`（status=Failed）
  - `reason=OOMKilling` → `k8s.pod_status`（status=OOMKilled）
  - `reason=Killing` → `k8s.pod_status`（status=Terminating）
- [ ] 提取 `involvedObject.name`（Pod 名称）→ 解析出 agent_id（格式：`agent-{name}-{hash}`）
- [ ] 转换为 `common/models.Event`（`event_type: k8s.pod_status`）
- [ ] 写入 `events:k8s` Redis Stream（MAXLEN ~ 10000）
- [ ] 处理 ListWatch 重连（client-go Informer 自动处理）

**测试**：
- Mock K8S Event（reason=OOMKilling）→ 验证事件写入 Redis，agent_id 解析正确
- Informer 断线重连后，不重复写入历史事件（resync 过滤）

### 4.4 健康检查端点

- [ ] `GET /healthz` → `200 OK`（进程存活）
- [ ] `GET /readyz` → 检查 Redis 连通性

### 4.5 Event Collector Helm Template

- [ ] 填充 `k8s/helm/templates/event-collector-deployment.yaml`
  - ServiceAccount（需要 Watch `agents` namespace Events 权限）
  - 挂载 `gitea-webhook-secret`
  - RBAC Role（`agents` namespace Events get/list/watch）

---

## 验收标准

- [ ] `go test ./internal/event-collector/...` 全部通过（覆盖率 ≥ 80%）
- [ ] 本地启动，使用 curl 模拟 Gitea Webhook（含正确 HMAC 签名）
  - Redis `events:gitea` 中出现对应事件，payload 格式正确
- [ ] 错误签名返回 403，Redis 中无新增事件
- [ ] 模拟 K8S Pod OOMKilling 事件 → `events:k8s` 中出现 `k8s.pod_status` 事件
