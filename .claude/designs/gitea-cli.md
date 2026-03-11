# Gitea CLI 设计文档

> 关联总览：[系统设计总览](system-design-overview.md)
> 关联：[Agent Runtime 设计](agent-runtime.md) | [Gitea 集成设计](gitea-integration.md)

## 一、概述

`gitea-cli` 是打包进 Agent 镜像的轻量命令行工具，封装 Gitea REST API 的高频操作。

**设计目标：**
- 内置认证（从文件读取 token）和 base URL，消除 Agent 每次调用的重复样板
- 命令语义与 Git 工作流对齐，降低 LLM 的认知负担
- 输出纯文本或 JSON，便于 Agent 在 reasoning 中解析
- 只覆盖高频操作（~10 个命令），边缘场景允许 Agent 直接 curl

**不封装的内容：**
- 仓库文件读写（使用 git CLI）
- Admin API（Operator 使用，不在 Agent 镜像中）
- Branch 创建/切换（使用 git CLI）

## 二、认证配置

`gitea-cli` 从以下位置自动读取配置，无需每次传参：

```
Token 文件：/agent/secrets/gitea-token
Base URL：  http://agent-gateway.control-plane.svc/api/v1
```

支持环境变量覆盖（用于本地开发调试）：

```bash
GITEA_TOKEN_PATH=/path/to/token
GITEA_BASE_URL=http://localhost:3000/api/v1
```

## 三、命令参考

### 3.1 Issue 操作

```
gitea issue list
  --repo <org/repo>         目标仓库
  --label <label,...>       按 label 过滤（可多个，逗号分隔）
  --state open|closed|all   默认 open
  --assignee <username>     按 assignee 过滤
  --limit <n>               最多返回条数，默认 20

gitea issue create
  --repo <org/repo>
  --title <title>
  --body <body>
  --labels <label,...>
  --assignees <username,...>

gitea issue assign
  --repo <org/repo>
  --number <n>
  --assignee <username>     不传则 assign 给当前 token 对应用户

gitea issue comment
  --repo <org/repo>
  --number <n>
  --body <body>

gitea issue close
  --repo <org/repo>
  --number <n>
```

### 3.2 PR 操作

```
gitea pr list
  --repo <org/repo>
  --state open|closed|merged|all   默认 open
  --label <label,...>

gitea pr create
  --repo <org/repo>
  --title <title>
  --body <body>
  --head <branch>           源分支
  --base <branch>           目标分支，默认 main

gitea pr review
  --repo <org/repo>
  --number <n>
  --event APPROVE|REQUEST_CHANGES|COMMENT
  --body <body>

gitea pr merge
  --repo <org/repo>
  --number <n>
  --method merge|squash|rebase   默认 merge
```

### 3.3 通知操作（@mention 检测）

```
gitea notify list
  --since <rfc3339>         只返回此时间之后的通知
  --unread                  只返回未读，默认 true

gitea notify read
  --thread-id <id>          标记指定通知为已读

gitea notify read-all       标记所有通知为已读
```

### 3.4 文件操作（仓库内容读取）

```
gitea file get
  --repo <org/repo>
  --path <file-path>        返回文件内容（base64 解码后输出）
  --ref <branch|commit>     默认 main
```

> 仅用于读取单个配置文件等轻量场景。大规模代码读写使用 git clone。

## 四、输出格式

所有命令默认输出人类可读的简洁文本，加 `--json` 参数输出原始 JSON：

```bash
$ gitea issue list --repo platform/platform-demo --label role/developer
#42  [priority/high]  Fix memory leak in worker pool       (unassigned)
#38  [priority/medium] Add retry logic to HTTP client       (unassigned)

$ gitea issue list --repo platform/platform-demo --label role/developer --json
[{"number":42,"title":"Fix memory leak...","labels":[...],...}, ...]
```

错误时输出到 stderr，exit code 非零：

```bash
$ gitea issue assign --repo platform/platform-demo --number 99
Error: issue #99 not found (404)
```

## 五、在 Agent 镜像中的位置

```dockerfile
# agent-runtime Dockerfile 中
COPY --from=gitea-cli-builder /app/gitea-cli /usr/local/bin/gitea
```

二进制命名为 `gitea`（不带 `-cli` 后缀），与 Agent SOUL.md 和 SKILL.md 中的调用约定一致。

## 六、实现说明

- 语言：Go（与 Operator 同语言，共享 Gitea API client 代码）
- 依赖：标准库 + `code.gitea.io/sdk/gitea`（官方 Go SDK）
- 构建：多阶段 Docker build，最终只复制静态二进制
- 测试：针对 Gitea API 的集成测试，使用 Gitea 官方 Docker 镜像启动测试实例
