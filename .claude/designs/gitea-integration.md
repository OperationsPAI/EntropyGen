# Gitea 集成设计文档

> 关联总览：[系统设计总览](system-design-overview.md)
> 关联：[Operator 设计](operator.md) | [Agent Runtime 设计](agent-runtime.md) | [Event Bus 设计](event-bus.md)

## 一、概述

Gitea 是平台的代码托管和任务协作中心。Agent 通过 Gitea 的 Issue/PR/Code Review 机制协作，与人类开发模式完全一致。

本文档描述：
- Gitea 用户和权限体系（Agent 账户管理）
- Agent 任务感知机制（Issue Board 协作流程）
- Gitea Webhook 事件收集
- Agent 与 Gitea 交互的安全边界

## 二、用户和权限体系

### 2.1 账户类型

| 账户 | 创建方式 | 用途 |
|------|---------|------|
| `admin` | 初始化时手动创建 | Gitea 管理员，Operator 使用此账户调用 Admin API |
| `agent-{name}` | Operator 通过 Admin API 自动创建 | 每个 Agent 的独立 Gitea 身份 |
| 人类开发者（可选） | 手动注册 | 直接登录 Gitea 查看/干预 Agent 行为 |

### 2.2 Operator Admin Token

Operator 使用一个全局 Gitea Admin Token 来创建和管理 Agent 用户，该 Token 存储于：

```
K8S Secret: gitea-admin-token
  Namespace: control-plane
  Key: token
```

**注意**：此 Token 不挂载到任何 Agent Pod，仅 Operator Pod 可访问。

### 2.3 Agent 用户创建流程

```
Operator Reconcile（新 Agent）
  │
  ├─ POST /api/v1/admin/users
  │   body: {
  │     "username": "agent-{name}",
  │     "email": "agent-{name}@agents.devops.local",
  │     "password": "<random 32 char>",   ← 仅用于账户创建，不存储
  │     "must_change_password": false
  │   }
  │
  ├─ POST /api/v1/users/agent-{name}/tokens
  │   body: { "name": "agent-runtime-token" }
  │   response: { "sha1": "<token>" }
  │
  ├─ 将 token 写入 K8S Secret: agent-{name}-gitea-token
  │
  └─ 按 spec.gitea.permissions 添加仓库协作者权限
      POST /api/v1/repos/{owner}/{repo}/collaborators/agent-{name}
      body: { "permission": "write" }   ← 根据角色映射
```

### 2.4 权限映射

| Agent Role | Gitea 仓库权限 | 说明 |
|-----------|--------------|------|
| observer | `read` | 只读，创建 Issue |
| developer | `write` | 推送代码，创建 PR |
| reviewer | `read` | 只读 + PR Review（Gitea read 权限包含 Review） |
| sre | `write` | 推送部署配置 |

> Gitea 的 `write` 权限包含 `read`，且允许创建 Issue、Review PR。`read` 权限也允许创建 Issue 和提交 Review，因此 reviewer 使用 `read` 权限即可。

## 三、任务协作：Issue Board 工作流

### 3.1 Label 约定

平台使用以下标准 Label（初始化 Gitea 仓库时自动创建）：

**Priority**
- `priority/critical` — 红色
- `priority/high` — 橙色
- `priority/medium` — 黄色
- `priority/low` — 灰色

**Type**
- `type/bug` — 红色
- `type/feature` — 蓝色
- `type/refactor` — 紫色
- `type/chore` — 灰色
- `type/security` — 深红色

**Agent 角色**
- `role/observer` — 绿色（Observer 创建的 Issue）
- `role/developer` — 蓝色（需要 Developer 处理的 Issue）
- `role/reviewer` — 橙色（需要 Reviewer 处理的 PR）
- `role/sre` — 紫色（需要 SRE 处理的部署任务）

### 3.2 Agent 轮询策略

各 Agent 通过 Cron 定时轮询 Gitea，每次 Cron 触发时**同时**检查两类任务：

| Agent | 主任务轮询 | @mention 轮询 | 默认频率 |
|-------|-----------|-------------|---------|
| Observer | 代码仓库（文件、提交记录）| 自身 Issue 评论中的 @mention | 每 30 分钟 |
| Developer | Open Issue（`role/developer`，未 assigned）| 自身 PR 的 Review + 评论中的 @mention | 每 2 分钟 |
| Reviewer | Open PR（未 reviewed，CI 通过）| 自身提交 Review 后的 PR 评论中的 @mention | 每 5 分钟 |
| SRE | Open Issue（`role/sre`）+ 部署状态 | Issue/PR 评论中的 @mention | 每 5 分钟 |

**@mention 检查优先于主任务**：每次 Cron 触发时先查 @mention，有待响应的 @mention 则先处理，处理完再执行主任务（或本轮跳过主任务，下次 Cron 再捡）。

### 3.3 @mention 协作机制

#### 检测逻辑

Agent 在每次 Cron 触发时，调用 Gitea 的 "通知" 接口查询未读通知：

```
GET /api/v1/notifications?all=false&since={last_check_time}
```

Gitea 原生支持 @mention 通知（在 Issue/PR 评论中 @username 会产生通知），返回示例：

```json
[
  {
    "id": 42,
    "subject": {
      "type": "Pull",
      "title": "Fix memory leak in worker pool",
      "url": "https://gitea.../pulls/15",
      "latest_comment_url": "https://gitea.../issues/comments/128"
    },
    "unread": true,
    "updated_at": "2026-03-11T13:00:00Z"
  }
]
```

Agent 过滤出 `unread=true` 的通知，读取对应评论内容，判断是否包含对自身的 @mention（`@agent-{name}`）。

#### 响应决策流程

```
Cron 触发
  │
  ├─ 1. GET /api/v1/notifications?all=false&since={last_check_time}
  │
  ├─ 2. 过滤 unread 通知，读取评论内容
  │      评论包含 @agent-{self}？
  │        否 → 标记已读，跳过
  │        是 → 进入响应决策
  │
  ├─ 3. LLM 分析评论内容，判断需要什么行动：
  │      - REQUEST_CHANGES → 修改代码，push，回复说明
  │      - 问题/讨论 → 在评论中回复
  │      - 请求帮助 → 评论中说明当前状态或分配
  │      - 无需行动（如纯通知）→ 仅标记已读
  │
  ├─ 4. 执行行动
  │
  └─ 5. 标记通知已读
         PATCH /api/v1/notifications/threads/{id}
```

#### 典型场景

**场景 A：Reviewer REQUEST_CHANGES + @mention Developer**

```
Reviewer 提交 Review：
  event: "REQUEST_CHANGES"
  body: "@agent-developer-1 第 42 行的错误处理不够健壮，
         请增加 retry 逻辑，并补充对应单元测试"

Developer 下次 Cron（最多 2 分钟后）：
  → 检测到未读通知（PR #15 有新 review comment）
  → 读取 review，识别 REQUEST_CHANGES + @mention self
  → LLM 分析：需要修改代码 + 补测试
  → git checkout feat/issue-42-xxx → 修改代码 → push
  → POST /api/v1/repos/{owner}/{repo}/issues/{pr_id}/comments
     body: "@agent-reviewer-1 已修改，请重新 review。
            修改说明：在第 42 行增加了最多 3 次 retry..."
  → 标记通知已读
```

**场景 B：Developer @mention Reviewer 请求提前 Review**

```
Developer 在 PR 评论中写：
  "@agent-reviewer-1 这个 PR 修改比较紧急，能否优先 review？"

Reviewer 下次 Cron（最多 5 分钟后）：
  → 检测到未读通知（PR #15 有新评论，包含 @self）
  → LLM 分析：被请求优先 review
  → 将 PR #15 加入本次优先处理队列（跳过按创建时间排序）
  → 执行 Review 流程
```

**场景 C：Observer @mention SRE 请求关注部署状态**

```
Observer 在 Issue 评论中写：
  "@agent-sre-1 发现 app-staging 的 worker pod 频繁重启，
   已创建 Issue #67，请优先排查"

SRE 下次 Cron（最多 5 分钟后）：
  → 检测到通知，读取 Issue #67
  → LLM 分析：被要求排查 pod 重启
  → kubectl get pods -n app-staging
  → kubectl logs agent-worker-xxx --previous
  → 在 Issue #67 评论中回复排查结论
```

#### 防止循环触发

Agent 回复评论时，避免触发其他 Agent 的 @mention 响应引发循环：

- 回复时**不主动 @mention 其他 Agent**（除非确实需要对方行动）
- 每个 Agent 维护 `workspace/mention-handled.json`，记录已处理的 comment_id，避免重复响应同一条评论
- LLM 在 AGENTS.md 中被约束：同一个 comment 只响应一次

```json
// workspace/mention-handled.json
{
  "handled_comment_ids": [128, 134, 156],
  "last_updated": "2026-03-11T13:00:00Z"
}
```

### 3.3 完整任务生命周期

```
[Issue 创建]
  Observer Cron 巡检 → 创建 Issue（label: type/bug, priority/high, role/developer）
  OR 管理员手动创建 Issue

        ↓

[Issue 领取 - Developer]
  Developer Cron 轮询 GET /api/v1/repos/{owner}/{repo}/issues?state=open&type=issues&labels=role/developer
  → 按 priority 排序，选未 assigned 的 Issue
  → PATCH /api/v1/repos/{owner}/{repo}/issues/{id}  body: {assignees: ["agent-developer-1"]}
  → POST /api/v1/repos/{owner}/{repo}/issues/{id}/comments  body: {body: "I'm working on this"}

        ↓

[Issue 开发 - Developer]
  git clone → 创建分支（feat/issue-{id}-{slug}）→ 编码 → 提交
  → POST /api/v1/repos/{owner}/{repo}/pulls
     body: {title, body: "Fixes #{id}", head, base: "main"}

        ↓

[CI 自动运行 - Gitea Actions]
  PR 创建 → 触发 CI workflow → Runner 执行测试/构建
  → Gitea Webhook 发送 pull_request / check_run 事件

        ↓

[PR 审查 - Reviewer]
  Reviewer Cron 轮询 GET /api/v1/repos/{owner}/{repo}/pulls?state=open
  → 过滤：未 reviewed，CI 通过的 PR
  → 读取 diff，提交 Review
  POST /api/v1/repos/{owner}/{repo}/pulls/{id}/reviews
    body: {event: "APPROVE" | "REQUEST_CHANGES", body, comments}

        ↓

[PR 合并]
  APPROVE 后，Developer 或 Reviewer 合并
  → Gitea "Fixes #{id}" 语法自动关闭关联 Issue
  → Webhook 发送 pull_request merged 事件

        ↓

[部署 - SRE]
  SRE Cron 发现新 merged PR → 检查是否有部署相关变更
  → kubectl apply 到 app-staging
  → 在 Issue/PR 中评论部署结果
```

## 四、Webhook 配置

Gitea 推送事件到 Event Collector，配置以下 Webhook（仓库级别，初始化时自动创建）：

| 事件类型 | Webhook 触发条件 | Event Bus `event_type` |
|---------|----------------|----------------------|
| `push` | 代码推送 | `gitea.push` |
| `issues` | Issue 创建/关闭/label 变更 | `gitea.issue_open`, `gitea.issue_close` |
| `issue_comment` | Issue/PR 评论 | `gitea.issue_comment` |
| `pull_request` | PR 创建/合并/关闭 | `gitea.pr_open`, `gitea.pr_merge` |
| `pull_request_review` | PR Review 提交 | `gitea.pr_review` |
| `workflow_run` | CI Actions 运行状态 | `gitea.ci_status` |

**Webhook 目标 URL**：`http://event-collector.control-plane.svc:8080/webhook/gitea`

**Secret**：Gitea Webhook 使用 HMAC-SHA256 签名，Event Collector 验证签名。Secret 与 K8S Secret `gitea-webhook-secret` 共享同一个值（见 §五.2）。

## 五、仓库初始化

### 5.1 概述

平台首次部署时，通过一个一次性 K8S Job（`gitea-init-job`）完成所有 Gitea 配置。Job 幂等执行：每步操作前先查询是否已存在，已存在则跳过，不报错。

Job 由 Helm `post-install` hook 触发，在 Backend、Operator 等组件就绪后执行。

**初始化步骤顺序：**

```
1. 等待 Gitea 就绪（健康检查）
2. 创建 Organization
3. 创建 Repository
4. 创建标准 Labels（12 个）
5. 创建 Webhook（含 HMAC Secret）
6. 配置 Branch 保护规则
```

### 5.2 K8S 资源清单

**Secret（手动创建，在 Helm install 之前）：**

```bash
# 生成 Webhook HMAC Secret，同时供 Gitea Webhook 配置和 Event Collector 使用
kubectl create secret generic gitea-webhook-secret \
  --from-literal=secret=$(openssl rand -hex 32) \
  -n control-plane
```

**Helm post-install Job：**

```yaml
apiVersion: batch/v1
kind: Job
metadata:
  name: gitea-init-job
  namespace: control-plane
  annotations:
    "helm.sh/hook": post-install
    "helm.sh/hook-weight": "10"          # 在其他 post-install hook 后执行
    "helm.sh/hook-delete-policy": hook-succeeded  # 成功后自动清理
spec:
  ttlSecondsAfterFinished: 300           # 5 分钟后自动删除（失败时保留用于排查）
  backoffLimit: 3
  template:
    spec:
      restartPolicy: OnFailure
      serviceAccountName: gitea-init     # 只需读取 Secrets
      containers:
      - name: init
        image: registry.devops.local/platform/gitea-init:v1.0.0
        env:
        - name: GITEA_URL
          value: "http://gitea.devops-infra.svc:3000"
        - name: GITEA_ADMIN_TOKEN
          valueFrom:
            secretKeyRef:
              name: gitea-admin-token
              key: token
        - name: WEBHOOK_SECRET
          valueFrom:
            secretKeyRef:
              name: gitea-webhook-secret
              key: secret
        - name: GITEA_ORG
          value: "platform"
        - name: GITEA_REPO
          value: "platform-demo"
        - name: EVENT_COLLECTOR_URL
          value: "http://event-collector.control-plane.svc:8080/webhook/gitea"
```

### 5.3 初始化内容详解

#### 创建 Organization 和 Repository

```
GET /api/v1/orgs/{org}
  → 200: 已存在，跳过
  → 404: POST /api/v1/orgs
         body: { "username": "platform", "visibility": "private" }

GET /api/v1/repos/{org}/{repo}
  → 200: 已存在，跳过
  → 404: POST /api/v1/org/{org}/repos
         body: {
           "name": "platform-demo",
           "description": "AI DevOps platform demo repository",
           "private": false,
           "auto_init": true,      ← 自动创建 main 分支和初始 commit
           "default_branch": "main"
         }
```

#### 创建标准 Labels

共 12 个 Label，已存在则跳过（按 name 匹配）：

```
GET /api/v1/repos/{org}/{repo}/labels  → 获取现有 labels 列表

待创建列表：
Priority:
  priority/critical  #FF0000  紧急，需立即处理
  priority/high      #FF6600  高优先级
  priority/medium    #FFAA00  中优先级
  priority/low       #999999  低优先级，可延后

Type:
  type/bug           #EE0701  缺陷
  type/feature       #0075CA  新功能
  type/refactor      #7057FF  重构
  type/chore         #CCCCCC  日常维护
  type/security      #8B0000  安全问题

Role:
  role/observer      #2EA44F  Observer 创建的 Issue
  role/developer     #0052CC  需要 Developer 处理
  role/reviewer      #E4A000  需要 Reviewer 处理
  role/sre           #6F42C1  需要 SRE 处理
```

> 共 13 个（priority×4 + type×5 + role×4），`gitea-integration.md §3.1` 中已定义。

#### 创建 Webhook

```
GET /api/v1/repos/{org}/{repo}/hooks
  → 检查是否已存在指向 event-collector 的 hook
  → 已存在: 跳过
  → 不存在: POST /api/v1/repos/{org}/{repo}/hooks

body:
{
  "type": "gitea",
  "config": {
    "url": "http://event-collector.control-plane.svc:8080/webhook/gitea",
    "content_type": "json",
    "secret": "<WEBHOOK_SECRET>"    ← 从环境变量读取
  },
  "events": [
    "push",
    "issues",
    "issue_comment",
    "pull_request",
    "pull_request_review_comment",
    "workflow_run"
  ],
  "active": true
}
```

#### 配置 Branch 保护规则

```
GET /api/v1/repos/{org}/{repo}/branches/main
  → 检查 protected 字段

POST /api/v1/repos/{org}/{repo}/branches/{branch}/protection
   （Gitea 使用 PUT /api/v1/repos/{org}/{repo}/branch_protections）

body:
{
  "branch_name": "main",
  "enable_push": false,             ← 禁止直接 push 到 main
  "enable_push_whitelist": false,
  "require_signed_commits": false,
  "required_approvals": 1,          ← 至少 1 个 Reviewer Approve
  "enable_approvals_whitelist": false,
  "block_on_official_review_requests": true,   ← REQUEST_CHANGES 时阻止合并
  "block_on_rejected_reviews": true,
  "block_on_outdated_branch": false,           ← 不要求分支最新（避免频繁 rebase）
  "dismiss_stale_approvals": true,             ← push 新 commit 后 Approve 失效
  "require_signed_commits": false,
  "protected_file_patterns": "",
  "enable_status_check": true,
  "status_check_contexts": ["CI"]   ← 要求 CI workflow 通过
}
```

### 5.4 Job 镜像

`gitea-init` 是一个轻量 Go 程序，封装上述所有 API 调用：

```dockerfile
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o gitea-init ./cmd/gitea-init

FROM alpine:3.19
COPY --from=builder /app/gitea-init /gitea-init
ENTRYPOINT ["/gitea-init"]
```

主要逻辑：`init.go` 按顺序执行 6 个步骤，每步失败则整个 Job 失败（触发 backoffLimit 重试）。

### 5.5 Webhook Secret 共享

Webhook HMAC Secret 需要在两处使用：
- **Gitea 侧**：Init Job 在创建 Webhook 时传入
- **Event Collector 侧**：启动时从 `gitea-webhook-secret` Secret 读取，用于验证签名

```yaml
# Event Collector Deployment 中引用
env:
- name: WEBHOOK_SECRET
  valueFrom:
    secretKeyRef:
      name: gitea-webhook-secret
      key: secret
```

这解决了 `event-bus.md §八` 中提到的 "Event Collector 的 Gitea Webhook HMAC Secret 管理" 待细化项。

## 六、安全边界

- Agent 只能访问被授权的仓库（Operator 显式添加为 collaborator）
- Agent 不能访问 Gitea Admin API（Token 权限仅限普通用户操作）
- 所有 Agent 的 Gitea 请求经过 Gateway，Gateway 记录完整审计日志
- Agent 不能直接访问 Gitea（NetworkPolicy 仅允许 agents namespace → Agent Gateway）

## 七、待细化

- [ ] 多仓库支持（当前假设单仓库，多仓库时的权限管理）
- [ ] Gitea Actions Runner 的资源限制配置
