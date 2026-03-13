# Phase 7: Agent Tooling — gitea-cli + gitea-init + agent-runtime 镜像

> 关联设计：[Gitea CLI 设计](../designs/gitea-cli.md) | [Gitea 集成设计](../designs/gitea-integration.md) | [Agent Runtime 设计](../designs/agent-runtime.md)
> 依赖 Phase：Phase 1（common/giteaclient 包）
> 可并行：可与 Phase 2-5 同步进行（无强依赖）

## 目标

实现三个独立工具：
1. **gitea-cli**：打包进 Agent 镜像的 CLI 工具，封装 Gitea 高频操作
2. **gitea-init-job**：Helm post-install 初始化 Job，幂等建立 Gitea 组织/仓库/标签/Webhook
3. **agent-runtime 镜像**：基于 OpenClaw，预装工具和 Skills

---

## 7.1 gitea-cli

**文件**：`cmd/gitea-cli/main.go` + `internal/gitea-cli/`

### 7.1.1 CLI 框架初始化

- [ ] 使用 `github.com/spf13/cobra` 构建 CLI
- [ ] 根命令：`gitea [--json] <subcommand>`
- [ ] 自动读取配置（无需每次传参）：
  - Token 文件：`/agent/secrets/gitea-token`（默认）
  - Base URL：`http://agent-gateway.control-plane.svc/api/v1`（默认）
  - 环境变量覆盖：`GITEA_TOKEN_PATH`、`GITEA_BASE_URL`

### 7.1.2 Issue 命令（`internal/gitea-cli/commands/issue.go`）

- [ ] `gitea issue list --repo <org/repo> [--label <l,...>] [--state open] [--assignee <u>] [--limit 20]`
- [ ] `gitea issue create --repo <org/repo> --title <t> --body <b> [--labels <l,...>] [--assignees <u,...>]`
- [ ] `gitea issue assign --repo <org/repo> --number <n> [--assignee <u>]`（默认 assign 给 token 对应用户）
- [ ] `gitea issue comment --repo <org/repo> --number <n> --body <b>`
- [ ] `gitea issue close --repo <org/repo> --number <n>`

### 7.1.3 PR 命令（`internal/gitea-cli/commands/pr.go`）

- [ ] `gitea pr list --repo <org/repo> [--state open] [--label <l,...>]`
- [ ] `gitea pr create --repo <org/repo> --title <t> --body <b> --head <branch> [--base main]`
- [ ] `gitea pr review --repo <org/repo> --number <n> --event APPROVE|REQUEST_CHANGES|COMMENT --body <b>`
- [ ] `gitea pr merge --repo <org/repo> --number <n> [--method merge|squash|rebase]`

### 7.1.4 通知命令（`internal/gitea-cli/commands/notify.go`）

- [ ] `gitea notify list [--unread] [--since <rfc3339>]`
- [ ] `gitea notify read --thread-id <id>`
- [ ] `gitea notify read-all`

### 7.1.5 文件命令（`internal/gitea-cli/commands/file.go`）

- [ ] `gitea file get --repo <org/repo> --path <p> [--ref main]`（输出 base64 解码后内容）

### 7.1.6 输出格式（`internal/gitea-cli/output/`）

- [ ] `text.go`：默认人类可读格式
  - issue list：`#42  [priority/high]  Fix memory leak  (unassigned)`
- [ ] `json.go`：`--json` 时输出原始 JSON
- [ ] 错误输出到 stderr，exit code 非零

**测试（TDD）**：

- [ ] 集成测试：启动 Gitea Docker 容器，验证全部命令正常执行
- [ ] 单元测试：Mock Gitea SDK，验证 `--label` 多值过滤、`--assignee` 默认行为
- [ ] `--json` 输出可被 `jq` 正确解析

### 7.1.7 gitea-cli 构建

- [ ] `cmd/gitea-cli/main.go`：Cobra root command 注册所有子命令
- [ ] 静态二进制：`CGO_ENABLED=0 go build -o gitea ./cmd/gitea-cli/`
- [ ] 文件名编译为 `gitea`（不带 `-cli` 后缀，与 SKILL.md 约定一致）

---

## 7.2 gitea-init-job

**文件**：`cmd/gitea-init/main.go` + `internal/gitea-init/init/steps.go`

### 7.2.1 初始化步骤（幂等，6 步）

**`internal/gitea-init/init/steps.go`**

每步执行前先查询是否已存在，已存在跳过，不报错。

- [ ] **Step 1**：等待 Gitea 就绪（`GET /api/v1/version`，最多等待 5 分钟，指数退避）
- [ ] **Step 2**：创建 Organization `platform`（`POST /api/v1/orgs`，409 时跳过）
- [ ] **Step 3**：创建 Repository `platform-demo`（auto_init=true，default_branch=main）
- [ ] **Step 4**：创建 13 个标准 Labels（priority×4 + type×5 + role×4，按 name 查重）
  ```go
  var standardLabels = []Label{
      {Name: "priority/critical", Color: "#FF0000", Description: "紧急，需立即处理"},
      {Name: "priority/high",     Color: "#FF6600", Description: "高优先级"},
      // ... 完整 13 个
  }
  ```
- [ ] **Step 5**：创建 Webhook（指向 Event Collector，含 HMAC Secret，事件类型列表）
- [ ] **Step 6**：配置 Branch 保护规则（main 分支：禁直接 push，需 1 Approve，需 CI 通过）

### 7.2.2 Runner Registration Token 获取

- [ ] **Step 7（附加）**：获取 Runner Registration Token
  - `POST /api/v1/admin/runners/registration-token`
  - 将 Token 写入 K8S Secret `gitea-runner-token`（供 act_runner Deployment 使用）

### 7.2.3 主程序

- [ ] `cmd/gitea-init/main.go`：顺序执行 7 步，每步失败 → 整个 Job 失败（触发 K8S backoffLimit 重试）
- [ ] 环境变量：`GITEA_URL`、`GITEA_ADMIN_TOKEN`、`WEBHOOK_SECRET`、`GITEA_ORG`、`GITEA_REPO`、`EVENT_COLLECTOR_URL`

### 7.2.4 Dockerfile

```dockerfile
FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -o gitea-init ./cmd/gitea-init/

FROM alpine:3.21
COPY --from=builder /app/gitea-init /gitea-init
ENTRYPOINT ["/gitea-init"]
```

### 7.2.5 Helm Template

- [ ] 填充 `k8s/helm/templates/gitea-init-job.yaml`：
  - `helm.sh/hook: post-install`
  - `helm.sh/hook-delete-policy: hook-succeeded`
  - `ttlSecondsAfterFinished: 300`
  - `backoffLimit: 3`
  - 挂载 `gitea-admin-token`、`gitea-webhook-secret`

**测试**：

- [ ] 空 Gitea 实例运行 init-job → 验证 org/repo/labels/webhook 均被创建
- [ ] 再次运行 → 幂等，不重复创建，不报错

---

## 7.3 agent-runtime 镜像

**文件**：`agent-runtime/Dockerfile` + `agent-runtime/entrypoint.sh` + `agent-runtime/skills/`

### 7.3.1 Dockerfile

```dockerfile
# 第一阶段：构建 gitea-cli
FROM golang:1.25-alpine AS gitea-cli-builder
WORKDIR /app
COPY . .
RUN CGO_ENABLED=0 go build -o gitea ./cmd/gitea-cli/

# 第二阶段：Agent Runtime 镜像
FROM ghcr.io/openclaw/openclaw:v2026.3.1

USER root

# 安装平台工具
RUN apt-get update && apt-get install -y \
    git \
    curl \
    kubectl \
    && rm -rf /var/lib/apt/lists/*

# 复制 gitea-cli 二进制
COPY --from=gitea-cli-builder /app/gitea /usr/local/bin/gitea

# 预装 Skills（减少冷启动时间）
COPY agent-runtime/skills/ /home/node/.openclaw/skills/

# 启动脚本
COPY agent-runtime/entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

USER node
ENTRYPOINT ["/entrypoint.sh"]
```

### 7.3.2 entrypoint.sh

```bash
#!/bin/bash
set -e

# 从 Secret 文件读取 JWT Token，写入 openclaw.json
JWT_TOKEN=$(cat /agent/secrets/jwt-token)
sed -i "s/__JWT_PLACEHOLDER__/${JWT_TOKEN}/" ~/.openclaw/openclaw.json

# 替换 SOUL.md 中的模板变量
sed -i "s/{{AGENT_ID}}/${AGENT_ID}/g" ~/.openclaw/SOUL.md
sed -i "s/{{AGENT_ROLE}}/${AGENT_ROLE}/g" ~/.openclaw/SOUL.md

# 启动 OpenClaw
exec openclaw gateway --headless --port 18789 --health-port 8080
```

### 7.3.3 Skills 内容（`agent-runtime/skills/`）

- [ ] **`gitea-api/SKILL.md`**：gitea CLI 完整命令参考（来自 gitea-cli 设计文档 §三）
  - 说明 gitea 二进制已预装，无需传 token/URL
  - 列出所有子命令示例

- [ ] **`git-ops/SKILL.md`**：Git 工作流操作说明
  - clone 策略：`git clone --depth=1 http://agent-gateway.../git/{org}/{repo}.git`
  - 分支命名规范：`feat/issue-{id}-{slug}`
  - Commit message 格式
  - push 和 PR 创建流程

- [ ] **`kubectl-ops/SKILL.md`**（SRE Only）：
  - 说明使用 in-cluster ServiceAccount，不指定 --kubeconfig
  - 只操作 `app-staging` namespace（`-n app-staging` 必须加）
  - 常用命令：get pods/logs、apply、rollout status

### 7.3.4 验证镜像

- [ ] `docker build -t agent-runtime:test -f agent-runtime/Dockerfile .`
- [ ] `docker run --rm agent-runtime:test gitea --help`（验证 gitea-cli 可用）
- [ ] `docker run --rm agent-runtime:test git --version`（验证 git 可用）
- [ ] `docker run --rm agent-runtime:test kubectl version --client`（验证 kubectl 可用）

---

## 验收标准

- [ ] `go test ./internal/gitea-cli/...` 全部通过
- [ ] `go test ./internal/gitea-init/...` 全部通过（使用 Gitea Docker 集成测试）
- [ ] `gitea issue list --repo platform/platform-demo` 返回正确结果
- [ ] `gitea issue create ... && gitea issue list` 新 issue 出现
- [ ] gitea-init-job 在空 Gitea 上完整运行成功（7 步全通过）
- [ ] agent-runtime 镜像构建成功，gitea/git/kubectl 均可用
- [ ] Skills SKILL.md 文件内容完整（Agent 可以根据它正确调用命令）
