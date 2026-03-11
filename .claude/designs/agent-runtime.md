# Agent Runtime 设计文档

> 关联总览：[系统设计总览](system-design-overview.md)
> 关联：[Operator 设计](operator.md) | [Agent Gateway 设计](agent-gateway.md) | [Gitea 集成设计](gitea-integration.md)

## 一、概述

Agent Runtime 基于 **[OpenClaw](https://github.com/openclaw/openclaw)** 实现。OpenClaw 是一个开源自托管 AI Agent 框架（MIT 协议），提供：

- 文件驱动的 Agent 人格配置（SOUL.md / SKILL.md）
- 内置 Cron 任务调度和 Webhook 事件触发
- 持久化 Workspace（Markdown 文件作为记忆存储）
- 多 LLM 后端支持（通过 `ANTHROPIC_BASE_URL` 等环境变量指向 LiteLLM）
- Skills 插件系统（扩展 Agent 工具能力）

**核心思想**：每个 Agent Pod 就是一个配置好的 OpenClaw 实例。Operator 负责把正确的配置文件写进去，OpenClaw 负责驱动感知→推理→行动循环。

## 二、Pod 内文件布局

OpenClaw 以 `/home/node/.openclaw/` 为 Workspace 根目录，我们把各配置来源映射如下：

```
/home/node/.openclaw/               ← PVC 挂载（持久化）
├── openclaw.json                   ← 主配置（Model、Gateway URL 等）
├── SOUL.md                         ← Agent 人格/角色定义（ConfigMap 挂载）
├── AGENTS.md                       ← Agent 行为约束（ConfigMap 挂载）
├── skills/                         ← OpenClaw Skills（ConfigMap 挂载）
│   ├── gitea-api/
│   │   └── SKILL.md                ← Gitea 操作技能
│   ├── git-ops/
│   │   └── SKILL.md                ← Git 操作技能
│   └── kubectl-ops/                ← 仅 sre 角色注入
│       └── SKILL.md
├── cron/                           ← Cron Job 配置（OpenClaw 内部）
│   └── main-loop.json
└── workspace/                      ← 记忆和工作目录（PVC 持久化）
    ├── episodic/                   ← 行动历史摘要
    ├── semantic/                   ← 仓库知识积累
    └── work/                       ← Git clone 工作目录
```

**挂载策略：**

| 路径 | 来源 | 生命周期 |
|------|------|---------|
| `openclaw.json` | ConfigMap `agent-{name}-config` | 随 CR spec 变更更新 |
| `SOUL.md` | ConfigMap `agent-{name}-config` | SOUL 变更时热更新（无需重启） |
| `AGENTS.md` | ConfigMap `agent-{name}-config` | 同上 |
| `skills/` | ConfigMap `agent-{name}-skills` | Skills 变更时热更新 |
| `/agent/secrets/gitea-token` | Secret `agent-{name}-gitea-token` | 只读挂载 |
| `workspace/` | PVC `agent-{name}-workspace` | 跨重启持久化 |

## 三、核心配置文件

### 3.1 openclaw.json

```json
{
  "agent": {
    "model": "anthropic/claude-opus-4-6"
  },
  "providers": {
    "anthropic": {
      "baseURL": "http://agent-gateway.control-plane.svc/v1",
      "apiKey": "__JWT_PLACEHOLDER__"
    }
  },
  "skills": {
    "load": {
      "extraDirs": []
    }
  },
  "automation": {
    "webhook": {
      "enabled": true,
      "port": 9090
    }
  }
}
```

> `apiKey` 的实际 JWT 由启动脚本从 `/agent/secrets/jwt-token` 读取后注入，不硬编码在 ConfigMap 中。

### 3.2 SOUL.md（各角色示例）

SOUL.md 是注入 OpenClaw 系统 prompt 的核心文件，定义 Agent 的身份、职责和约束。

**Developer Agent 示例：**
```markdown
# Identity
You are Developer Agent "{{AGENT_ID}}", a software engineer working on the platform-demo repository.
Your Gitea username is agent-developer-1. You work autonomously — no human will review your decisions in real-time.

# Responsibilities
- Pick up unassigned Issues labeled `role/developer` ordered by priority
- Write clean, well-tested code following the repository's existing patterns
- Create focused PRs (one issue per PR), with `Fixes #<issue_number>` in the description
- Respond promptly to Reviewer's change requests

# Constraints
- Never push directly to `main` branch — always use feature branches (`feat/issue-{id}-{slug}`)
- Keep changes minimal and focused — do not refactor unrelated code
- All requests go through the Gateway at http://agent-gateway.control-plane.svc
```

**Observer Agent 示例：**
```markdown
# Identity
You are Observer Agent "{{AGENT_ID}}", a code quality analyst for the platform-demo repository.

# Responsibilities
- Periodically inspect the codebase for bugs, security issues, and tech debt
- Create well-described Issues with appropriate labels (type/*, priority/*, role/developer)
- Avoid duplicate issues — search existing issues before creating new ones

# Constraints
- You have read-only access; never push code or merge PRs
- Maximum 3 new Issues per inspection cycle to avoid noise
```

### 3.3 AGENTS.md（行为约束，所有角色通用部分）

```markdown
# Behavior Rules

1. All HTTP requests to external services MUST go through the Gateway.
   - LLM: http://agent-gateway.control-plane.svc/v1/*
   - Gitea: http://agent-gateway.control-plane.svc/api/v1/*
   - Git (HTTP): http://agent-gateway.control-plane.svc/git/*

2. Read the Gitea token from file: /agent/secrets/gitea-token
   Use it as HTTP header: Authorization: token <content>

3. After completing each task, write a brief summary to workspace/episodic/<date>.md

4. If a tool call fails, retry once. If it fails again, log the error and skip to the next task.

# @mention Handling Rules

At the START of every Cron cycle, before doing anything else:

5. Check Gitea notifications:
   GET /api/v1/notifications?all=false&since={last_check_time}
   Save last_check_time to workspace/state.json after each check.

6. For each unread notification:
   - Read the linked comment content
   - If comment contains "@{AGENT_ID}": analyze intent and respond
   - Always mark as read: PATCH /api/v1/notifications/threads/{id}

7. De-duplication: Before responding to any @mention comment, check
   workspace/mention-handled.json for the comment_id.
   If already handled, skip silently. If not, respond then append the
   comment_id to mention-handled.json.

8. No response loops: When replying to a @mention, do NOT @mention
   other agents unless you genuinely need them to take action.
   Acknowledgement replies must not contain @mentions.
```

### 3.4 Skills（SKILL.md 格式）

每个 Skill 是一个独立目录，包含 `SKILL.md` 文件，遵循 OpenClaw AgentSkills 格式。

**`gitea-api` Skill** 通过 `gitea` CLI（打包进镜像）操作 Gitea，而非直接调用 REST API。命令详情见 [Gitea CLI 设计](gitea-cli.md)。

```markdown
---
name: gitea-api
description: Interact with Gitea using the gitea CLI for issue and PR management
---

# Gitea API Skill

The `gitea` binary is pre-installed in this image. It handles authentication
and base URL automatically — no need to pass tokens or URLs manually.

## Common Commands

# List open issues assigned to this agent's role
gitea issue list --repo <org/repo> --label role/developer --state open

# Assign an issue to yourself
gitea issue assign --repo <org/repo> --number <n>

# Comment on an issue or PR
gitea issue comment --repo <org/repo> --number <n> --body "..."

# Create a pull request
gitea pr create --repo <org/repo> --title "..." --body "Fixes #<n>" --head <branch>

# Submit a review
gitea pr review --repo <org/repo> --number <n> --event APPROVE --body "..."

# Check unread @mention notifications
gitea notify list --unread --since <last-check-time>

# Mark notification as read
gitea notify read --thread-id <id>

See: gitea --help, gitea issue --help, gitea pr --help
```

## 四、触发机制

### 4.1 Cron 触发（主要方式）

OpenClaw 内置 Cron 调度，按 `spec.cron.schedule` 触发主循环：

```
Cron 触发（例：每 2 分钟）
  │
  └─ OpenClaw 向自身 main session 发送 cron prompt：
     "Check for new unassigned issues labeled role/developer.
      If found, pick the highest priority one and start working on it."
     │
     └─ Agent 开始 Perception → Reasoning → Action 循环
```

Cron Job 配置由 Operator 写入 `~/.openclaw/cron/main-loop.json`：

```json
{
  "name": "main-loop",
  "cron": "*/2 * * * *",
  "session": "isolated",
  "message": "{{CRON_PROMPT}}",
  "lightContext": false
}
```

### 4.2 Webhook 触发（事件驱动，补充方式）

OpenClaw 暴露 Webhook 端点（`:9090`），Event Collector 收到 Gitea 事件后可直接通知 Agent：

```
Gitea 事件（PR 创建）
  → Event Collector
  → POST http://agent-reviewer-1.agents.svc:9090/webhook/gitea-pr
     Body: { "pr_number": 15, "repo": "org/platform-demo" }
  → OpenClaw 触发 prompt：
     "A new PR #{{pr_number}} in {{repo}} needs review."
```

> **注**：Webhook 是加速响应的补充机制，Cron 是保底兜底。即使 Webhook 未送达，Cron 也会在下次轮询时处理。

### 4.3 Heartbeat

OpenClaw 的 Cron 中单独配置心跳任务，每 5 分钟向 Gateway 发送 heartbeat：

```json
{
  "name": "heartbeat",
  "cron": "*/5 * * * *",
  "session": "isolated",
  "message": "Send a heartbeat: POST http://agent-gateway.control-plane.svc/heartbeat with body {\"status\": \"ok\"}"
}
```

## 五、Docker 镜像策略

### 5.1 基础镜像

使用官方镜像为基础，预装平台所需工具：

```dockerfile
FROM ghcr.io/openclaw/openclaw:v2026.3.1

USER root

# 安装平台工具
RUN apt-get update && apt-get install -y \
    git \
    curl \
    kubectl \
    && rm -rf /var/lib/apt/lists/*

# 预装平台专用 Skills（减少冷启动时间）
COPY skills/ /home/node/.openclaw/skills/

# 启动脚本（注入 JWT Token 后再启动 OpenClaw）
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

USER node

ENTRYPOINT ["/entrypoint.sh"]
```

镜像名：`registry.devops.local/platform/agent-runtime:v1.0.0`

### 5.2 启动脚本（entrypoint.sh）

```bash
#!/bin/bash
set -e

# 从 Secret 文件读取 JWT Token，写入 openclaw.json
JWT_TOKEN=$(cat /agent/secrets/jwt-token)
sed -i "s/__JWT_PLACEHOLDER__/${JWT_TOKEN}/" ~/.openclaw/openclaw.json

# 替换 SOUL.md 中的模板变量
sed -i "s/{{AGENT_ID}}/${AGENT_ID}/g" ~/.openclaw/SOUL.md

# 启动 OpenClaw（前台运行）
exec openclaw gateway --headless --port 18789
```

### 5.3 角色差异化

不同 Agent 角色使用相同基础镜像，通过挂载不同 ConfigMap 实现差异化：

| 角色 | SOUL.md | Skills | kubectl |
|------|---------|--------|---------|
| observer | 观察者人格 | gitea-api | ✗ |
| developer | 开发者人格 | gitea-api, git-ops | ✗ |
| reviewer | 审查者人格 | gitea-api | ✗ |
| sre | 运维人格 | gitea-api, git-ops, kubectl-ops | ✓（挂载 kubeconfig） |

## 六、Git Workspace 管理

Developer/Reviewer/SRE 需要在本地 clone 代码仓库进行操作：

```
/home/node/.openclaw/workspace/work/
└── {repo-name}/                    ← git clone 目标目录
    └── ...                         ← 代码文件
```

**策略：**
- 同一时间 Agent 只处理一个 Issue/PR（顺序执行，不并发）
- 每次任务开始前检查目录是否已存在：
  - 存在 → `git fetch --all && git checkout main && git pull`
  - 不存在 → `git clone http://agent-gateway.../git/{org}/{repo}.git`
- Git 认证：HTTP Basic Auth，用 Gitea Token（`agent-{name}:token`），通过 Gateway 代理

## 七、记忆结构

OpenClaw 原生使用 Markdown 文件作为持久记忆，我们约定以下结构：

```
workspace/
├── episodic/
│   └── 2026-03-11.md        ← 当日行动日志（OpenClaw 自动追加）
├── semantic/
│   └── repo-knowledge.md    ← Agent 对仓库的积累认知（定期更新）
├── state.json               ← 运行时状态（last_check_time 等）
├── mention-handled.json     ← 已处理的 @mention comment_id 去重表
└── work/
    └── {repo}/              ← Git 工作目录
```

**state.json 格式：**

```json
{
  "last_notification_check": "2026-03-11T13:00:00Z",
  "current_task": {
    "type": "issue",
    "id": 42,
    "started_at": "2026-03-11T12:00:00Z"
  }
}
```

**mention-handled.json 格式：**

```json
{
  "handled_comment_ids": [128, 134, 156],
  "last_updated": "2026-03-11T13:00:00Z"
}
```

> `mention-handled.json` 只保留最近 7 天的 comment_id，防止文件无限增长。OpenClaw 的内置记忆检索机制（语义相似度）会在每次任务开始时自动检索相关历史上下文，无需手动管理注入策略。

## 八、健康检查

OpenClaw headless 模式（`--headless`）不暴露 Web UI，但我们在 Sidecar 或 OpenClaw 自身提供探针端点：

| 端点 | 逻辑 | 探针类型 |
|------|------|---------|
| `GET :8080/healthz` | OpenClaw 进程存活 | Liveness |
| `GET :8080/readyz` | Gateway 可达（HTTP GET /healthz 返回 200） | Readiness |

> 通过 `openclaw gateway --health-port 8080` 暴露健康端点，或通过 Sidecar 代理实现。

## 九、环境变量

| 环境变量 | 来源 | 用途 |
|---------|------|------|
| `AGENT_ID` | Operator 注入 | Agent 唯一标识（agent-developer-1） |
| `AGENT_ROLE` | `spec.role` | observer / developer / reviewer / sre |
| `CRON_SCHEDULE` | `spec.cron.schedule` | 主循环 Cron 表达式 |
| `CRON_PROMPT` | `spec.cron.prompt` | 每次 Cron 触发时发送的 prompt |
| `GITEA_REPO` | `spec.gitea.repo` | 目标仓库（org/repo 格式） |
| `GATEWAY_URL` | Operator 注入 | `http://agent-gateway.control-plane.svc` |

Secrets（文件挂载，非环境变量）：

| 文件路径 | 内容 |
|---------|------|
| `/agent/secrets/gitea-token` | Gitea API Token |
| `/agent/secrets/jwt-token` | Gateway JWT Token |

## 十、与 Control Panel 的交互

| 操作 | 机制 |
|------|------|
| **暂停 Agent** | Operator 将 Deployment replicas 设为 0，Pod 终止 |
| **恢复 Agent** | Operator 将 replicas 恢复为 1，Pod 重建，OpenClaw 重新加载 Workspace |
| **查看实时日志** | `kubectl logs`（Pod stdout 是 OpenClaw 的结构化日志） |
| **修改 SOUL.md** | Control Panel 更新 ConfigMap → K8S 自动 rolling update（OpenClaw 热加载） |
| **修改 Cron 频率** | 更新 ConfigMap 中 `CRON_SCHEDULE` → Pod 重启 |

## 十一、补充设计

### 11.1 Tool Call 错误重试策略

OpenClaw 本身不内置 tool call 级别的重试（它是 LLM 层，不感知工具执行结果）。重试约定在 `AGENTS.md` 中描述，由 LLM 在 reasoning 过程中执行：

```markdown
# Error Handling

When a tool call returns an error:
- HTTP 4xx: Do not retry. Log the error in your reasoning and skip this action.
  (4xx means your request is wrong, retrying won't help)
- HTTP 5xx / network error: Retry once after 5 seconds. If it fails again, skip and
  note the failure in workspace/episodic/<date>.md
- Timeout (no response in 30s): Same as 5xx — retry once, then skip.

After 3 consecutive tool failures in one session, stop the current task and write
a status comment on the related Issue/PR explaining what went wrong.
```

### 11.2 openclaw.json model 字段与 LiteLLM alias 映射

`openclaw.json` 中的 `model` 字段值需要与 LiteLLM 中配置的 model alias 完全一致。

Operator 在生成 `openclaw.json` 时，直接使用 `spec.llm.model` 的值：

```json
{
  "agent": {
    "model": "anthropic/claude-opus-4-6"   ← 与 spec.llm.model 相同
  }
}
```

LiteLLM 侧需要预先配置同名 alias：

```yaml
# LiteLLM config.yaml
model_list:
  - model_name: "anthropic/claude-opus-4-6"   ← 与 Agent spec 中一致
    litellm_params:
      model: "claude-opus-4-6-20251101"
      api_key: "os.environ/ANTHROPIC_API_KEY"
```

如果 LiteLLM 中不存在对应 alias，Gateway 转发的请求会返回 404，Agent 按 §11.1 的错误处理策略处理。

### 11.3 SRE Agent kubeconfig 权限

SRE Agent 通过 K8S ServiceAccount 凭证操作 `app-staging`（不挂载 kubeconfig 文件），使用 in-cluster config：

- SRE Agent Pod 的 ServiceAccount 为 `agent-sre-{name}`（Operator 创建）
- Operator 在 `app-staging` namespace 创建对应 RoleBinding（见 `operator.md §7.5`）
- OpenClaw 的 `kubectl-ops` Skill 中约定使用默认 in-cluster 认证（不指定 `--kubeconfig`）

```markdown
# kubectl-ops/SKILL.md

## Authentication
Use in-cluster ServiceAccount credentials (default kubectl behavior inside a Pod).
Do NOT use --kubeconfig or --context flags.

## Allowed Namespace
Only operate in: app-staging
Always pass --namespace app-staging (or -n app-staging) to every kubectl command.
```

### 11.4 OpenClaw 版本锁定策略

镜像 tag 固定到具体版本号，不使用 `latest`：

```dockerfile
FROM ghcr.io/openclaw/openclaw:v2026.3.1   # 固定版本
```

版本升级流程：
1. 在测试环境验证新版本兼容性（SOUL.md / SKILL.md 格式是否有 breaking change）
2. 更新 `agent-runtime` 镜像 tag
3. 滚动更新所有 Agent Pod（Operator 检测到镜像变更后触发 rolling update）

## 十二、待细化

（所有待细化项已解决）
