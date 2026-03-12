# Control Panel Frontend 设计文档

> 关联总览：[系统设计总览](system-design-overview.md)
> 关联：[Control Panel Backend 设计](control-panel-backend.md) | [Gitea 集成设计](gitea-integration.md)

## 一、概述

Control Panel Frontend 是平台的管理界面，基于 **Semi Design（字节跳动）** 组件库构建，使用 React + TypeScript。

访问入口：`https://control.devops.local`

定位：**观测优先的管理工具**。管理员大部分时间在"看"（被动观测系统状态），少部分时间在"操作"（主动干预 Agent 行为）。UX 设计围绕这一核心展开。

## 二、UX 核心理念

### 2.1 三个核心场景

1. **被动观测**：打开页面不需要任何操作，就能判断系统是否健康
2. **主动排查**：某个 Agent 异常，3 步以内能找到原因
3. **精准操作**：操作路径短，但高风险操作有足够的确认防误触

### 2.2 全局导航

左侧固定 Sidebar + 顶部 Header 布局，PC 优先（1280px+）：

```
┌──────────┬──────────────────────────────────────────────┐
│          │ [告警横幅] developer-1 CrashLoop — 已重启6次  [×] │
│ Sidebar  ├──────────────────────────────────────────────┤
│          │                                              │
│ 仪表盘   │           主内容区                           │
│ Agent    │                                              │
│ Roles    │                                              │
│ LLM 配置 │                                              │
│ 审计日志 │                                              │
│ 监控图表 │                                              │
│ 数据导出 │                                              │
│          │                                              │
└──────────┴──────────────────────────────────────────────┘
```

告警横幅显示在顶部，全宽红色，需手动关闭（仅 `crash_loop` 类型触发横幅，其余用 Toast）。

### 2.3 右侧 Detail Panel 模式

查看详情优先使用**右侧展开 Panel**，而非跳转新页面——保持列表上下文，减少导航层级。Agent 列表、审计日志列表均采用此模式。

### 2.4 颜色即信息

Phase 状态颜色全局统一，不依赖读文字：

| Phase | 颜色 | 说明 |
|-------|------|------|
| Running | 绿色 | 正常运行 |
| Initializing | 蓝色 | 启动中 |
| Paused | 灰色 | 已暂停 |
| Error | 红色 | 异常，需关注 |
| Pending | 黄色 | 等待调度 |

Error 状态的 Agent，列表行整行背景轻微泛红。

### 2.5 实时不打扰

WebSocket 更新只刷新对应数据行，不重置滚动位置、不清空过滤条件、不触发全页刷新。

### 2.6 操作风险分级

| 操作 | 风险 | 确认方式 |
|------|------|----------|
| 暂停 / 恢复 Agent | 低（可逆） | 单击，无确认 |
| 指派任务 | 低 | Modal 填写后确认 |
| 修改 LLM 参数 | 中 | 表单保存 |
| 修改 Role 文件 | 中（影响所有引用 Agent） | 直接保存，状态栏显示影响的 Agent 数 |
| 删除 Role 文件 | 高 | 确认弹窗，说明影响范围 |
| 重置记忆 | 高（清空 PVC） | 确认弹窗，说明后果 |
| 删除 Agent | 不可逆 | 输入 Agent 名称后方可确认 |
| 删除 Role | 不可逆 | 有 Agent 引用时禁止删除；无引用时输入名称确认 |

## 三、页面结构

```
Control Panel
├── 登录页 /login
└── 主界面（需认证）
    ├── 概览仪表盘 /dashboard
    ├── Agent 管理 /agents
    │   ├── Agent 列表
    │   ├── 创建 Agent 向导 /agents/new
    │   └── Agent 详情 /agents/:name
    │       ├── Tab: 概览
    │       ├── Tab: 活动时间线
    │       ├── Tab: 文件（只读，继承自 Role）
    │       ├── Tab: 日志
    │       └── Tab: 审计
    ├── Role 管理 /roles
    │   ├── Role 列表
    │   ├── 创建 Role /roles/new
    │   └── Role 编辑器 /roles/:name（VS Code 风格 IDE）
    ├── LLM 配置 /llm
    ├── 审计日志 /audit
    │   ├── Trace 列表（含右侧 Detail Panel）
    │   └── Trace 详情 /audit/traces/:id
    ├── 监控图表 /monitor
    └── 训练数据导出 /export
```

## 四、页面设计

### 4.1 概览仪表盘（/dashboard）

**设计目标**：一眼判断系统健康状态，无需任何操作。

布局：

```
┌─────────────────────────────────────────────────────────┐
│ [告警横幅] developer-1 CrashLoop — 已重启 6 次    [×]  │
├──────────┬──────────┬──────────┬────────────────────────┤
│ 运行中   │ 今日Token │ Gitea操作 │ 告警数                │
│   4/5    │  101,000 │    47    │  ⚠️ 2                  │
├──────────┴──────────┴──────────┴────────────────────────┤
│                                                         │
│  Agent 状态列表（左60%）    │  Token趋势图（右40%）     │
│  ● observer-1  Running     │  [最近24小时折线图]        │
│  ● developer-1 Error    ←红│                           │
│  ● developer-2 Running     │  实时事件流               │
│  ● reviewer-1  Running     │  13:01 dev-1 push feat/x  │
│  ● sre-1       Paused   ←灰│  13:00 obs 创建 Issue#16  │
│                            │  [每条带 ↗ 外链]          │
└────────────────────────────┴───────────────────────────┘
```

说明：

- Token 趋势图默认最近 **24 小时**（而非 7 天），粒度更细，异常更易发现
- 实时事件流每条带 Agent 颜色标识，并附 `[↗]` 可跳转 Gitea 对应 Issue/PR/Commit
- Agent 列表点击任意行，**右侧展开 Detail Panel**（不跳转页面），保持列表上下文

### 4.2 Agent 管理（/agents）

#### 概念模型：Role（模板）+ Agent（实例）

Agent 的"人格"配置（soul、prompt、skills 等 markdown 文件）统一存放在 **Role** 中。Agent 引用 Role，运行时实时读取 Role 的最新文件。

```
Role "developer" ──┬── Agent "alice"
                   ├── Agent "bob"
                   └── Agent "charlie"

Role "reviewer"  ──── Agent "dave"
```

修改 Role "developer" 的 soul.md → alice / bob / charlie 下次执行时全部读到新内容。

#### Agent 列表页

表格展示所有 Agent，支持按 Role/Phase 动态过滤（Role 从已有 Agent 中提取唯一值）：

| 列 | 说明 |
|----|------|
| Name | agent CR 名称 |
| Role | 可点击链接 → `/roles/:roleName`（动态字符串，非固定 enum） |
| Phase | 带颜色标签 |
| Model | LLM 模型名称 |
| Last Action | 最近行动描述 |
| Tokens Today | 今日 Token 消耗 |
| Age | 创建时间 |
| Actions | 暂停/恢复、查看详情、删除 |

操作按钮：
- **新建 Agent**：点击 `+ New Agent` → 跳转 `/agents/new`（分步向导页面）
- 列表行点击 → 跳转 Agent 详情页

#### Agent 详情页（/agents/:name）

分 5 个 Tab：

| Tab | 内容 |
|-----|------|
| 概览 | 基本信息（Role 字段为可点击链接 → `/roles/:roleName`）、状态、Conditions、快捷操作 |
| 活动时间线 | LLM 推理 + Gitea 操作按时间混合展示 |
| 文件 | 只读预览，标注"Inherited from role: {roleName}"，顶部"Edit in Role Editor →"链接 |
| 日志 | 实时事件流（WebSocket）+ Pod stdout，子切换 |
| 审计 | 该 Agent 的 Trace 列表，内嵌过滤后的 Trace 表格 |

##### 概览 Tab 布局

```
┌─────────────────────────────────┬──────────────────────┐
│ 基本信息                        │ 操作                 │
│ 角色     Developer              │                      │
│ 模型     gpt-4o                 │  [指派任务]          │
│ Cron     */5 * * * *            │  [暂停]              │
│ Gitea    @developer-1  [↗]      │  [重置记忆]          │
│ 仓库     ai-team/webapp  [↗]    │                      │
│                                 │ ──────────────────── │
│ 状态                            │ 今日 Token           │
│ Phase    ● Running              │ 45,230               │
│ Pod      dev-1-7d9f  [↗日志]   │                      │
│ 启动于   2 天前                 │ 今日 Gitea 操作       │
│                                 │ 23 次                │
│ Conditions                      │                      │
│ ✓ Ready                         │                      │
│ ✓ GiteaUserCreated              │                      │
└─────────────────────────────────┴──────────────────────┘

[删除 Agent]  ← 放在页面底部，不显眼
```

Gitea 用户名和仓库均带 `[↗]` 链接，点击新标签页打开 Gitea 对应页面。

##### 活动时间线 Tab

将该 Agent 的 LLM 推理 trace 和 Gitea 操作事件按时间混合展示，直观呈现"思考-行动"链路：

```
过滤: [全部 ▼]  [今天 ▼]

──────────────────────────────────────────────────────────
  13:05  🧠 LLM 推理    1,200 → 350 tokens  gpt-4o  2.1s
                                               [展开请求]
  13:04  📤 Git Push    feat/login  3 commits
                        ai-team/webapp         [↗ 查看提交]
  13:03  🔀 PR #8 创建  Add authentication
                        feat/login → main  CI ✓ [↗ 查看 PR]
  13:01  💬 Issue #15 评论  "I'm working on this"
                                               [↗ 查看评论]
  12:58  🧠 LLM 推理    800 → 220 tokens   gpt-4o  1.8s
                                               [展开请求]
  12:55  📥 Git Clone   ai-team/webapp

⬛ 12:50  ⏰ Cron 触发  "检查 open issues"  ← 淡蓝背景分割线，标记工作周期起点

  12:48  📋 Issue #15 指派  Fix login bug
                            由 observer-1 创建  [↗ 查看 Issue]
──────────────────────────────────────────────────────────
                    [加载更多]
```

交互细节：
- 🧠 LLM 推理 条目默认折叠，点「展开请求」后在行内展开 prompt/response JSON 预览，无需切换 Tab
- Cron 触发用淡蓝色背景分割线，清晰标出每次工作周期的起点
- Gitea 相关条目均带 `[↗]` 外链，点击新标签页打开

事件类型图标对照：

| 图标 | 事件类型 |
|------|----------|
| 🧠 | LLM 推理（llm_inference） |
| 📤 | Git Push |
| 📥 | Git Clone |
| 🔀 | PR 创建/合并 |
| 💬 | Issue/PR 评论 |
| 📋 | Issue 指派/创建 |
| ⏰ | Cron 触发 |
| ⚠️ | 告警事件 |

##### 文件 Tab（只读）

显示从 Role 继承的文件列表，所有文件均为只读模式：

```
┌──────────────────────────────────────────────────────────┐
│ Inherited from role: developer    Edit in Role Editor →  │
├───────────┬──────────────────────────────────────────────┤
│ FILES     │                                              │
│           │  # Soul                                      │
│ soul.md   │                                              │
│ prompt.md │  You are a senior full-stack developer.      │
│ skills.md │  You write clean, tested code.               │
│ memory.md │  ...                                         │
│           │                         (Monaco Editor, RO)  │
└───────────┴──────────────────────────────────────────────┘
```

点击"Edit in Role Editor →"跳转 `/roles/:roleName` 编辑器。

##### 日志 Tab

```
[实时事件流]  [Pod stdout]   ← 子切换

实时事件流（WebSocket）：
  13:05:32  llm_inference  POST /v1/chat  200
  13:04:11  gitea_api      POST /repos/…  201
  ● 实时连接中

Pod stdout：
  [INFO] 2026-03-11 13:05:30 Fetching issues...
  [INFO] 2026-03-11 13:05:32 Calling LLM...
```

#### 指派任务交互

点击概览 Tab 的「指派任务」按钮，弹出 Modal：

```
┌────────────────────────────────────────────┐
│  指派任务给 developer-1                    │
│  ──────────────────────────────────────    │
│  仓库   [ai-team/webapp ▼]                 │
│         （默认该 Agent 关联仓库，可修改）   │
│                                            │
│  标题 * __________________________________ │
│                                            │
│  描述   ┌──────────────────────────────┐   │
│         │                              │   │
│         └──────────────────────────────┘   │
│                                            │
│  标签   [bug ▼] [feature ▼]  + 自定义      │
│                                            │
│  优先级  ○ Low  ● Medium  ○ High           │
│                                            │
│  ┌─────────────────────────────────────┐   │
│  │ ℹ️ 将在 Gitea 创建 Issue 并自动      │   │
│  │    assign 给 @developer-1           │   │
│  └─────────────────────────────────────┘   │
│                                            │
│              [取消]  [创建并指派]           │
└────────────────────────────────────────────┘
```

创建成功后：Toast 提示「Issue #17 已创建」，带 `[↗ 查看]` 外链；活动时间线自动追加一条「📋 Issue #17 指派 — 由管理员创建」。

#### 新建 Agent 向导（/agents/new）

点击「+ New Agent」→ 跳转独立分步向导页面（6 步），替代原来的 Drawer 模式：

```
──●────────○────────○────────○────────○────────○──
  Role      LLM     Schedule  Resources  Gitea   Review
```

**Step 1: Role** — Agent 名称 + 选择 Role（radio 列表，每项显示名称/描述/文件数/引用 Agent 数，可点"Edit →"跳转 Role 编辑器，可点"+ New Role"新建）

**Step 2: LLM** — Model、Temperature、Max Tokens

**Step 3: Schedule** — Cron 表达式（提示：prompt 内容来自 Role 的 prompt.md）

**Step 4: Resources** — CPU/内存 Request & Limit、Workspace Size

**Step 5: Gitea** — Repository、Permissions 复选框

**Step 6: Review** — 两列汇总所有配置，每个区块可点击跳回对应步骤修改。底部警告"This will start a new pod consuming resources."

底部固定操作栏：`[Cancel] [Back] [Next Step →]`，最后一步变为 `[Create Agent]`。

### 4.3 Role 管理（/roles）

#### Role 概念

Role 是一组 markdown 文件的集合，定义 Agent 的"人格"和能力。Agent 实时引用 Role 的最新文件，不做副本。

```
Role
├── name: "developer"              唯一标识，K8s 资源名格式
├── description: "Full-stack dev"  简短描述
├── files/                         自由文件系统
│   ├── soul.md
│   ├── prompt.md
│   ├── skills.md
│   └── *.md
└── created_at / updated_at
```

#### Role 列表页（/roles）

```
┌─ PageHeader: Roles ───────────────────────────── [+ New Role] ┐
│                                                                │
│ ┌─ Card ─────────────────────────────────────────────────────┐ │
│ │  Name       Description              Files  Agents  Updated│ │
│ │ ─────────────────────────────────────────────────────────── │ │
│ │  developer  Full-stack developer        5      3    2h ago │ │
│ │  reviewer   Code review specialist      3      1    1d ago │ │
│ │  sre        SRE on-call bot             4      2    3d ago │ │
│ └────────────────────────────────────────────────────────────┘ │
└────────────────────────────────────────────────────────────────┘
```

- **Name**：可点击链接 → `/roles/:name` 编辑器
- **Agents**：引用此 Role 的 Agent 数量
- **Actions**：Delete（有 Agent 引用时禁止删除，提示 "N agents are using this role"）
- 删除确认：输入名称确认

#### 创建 Role（/roles/new）

表单页：Name（lowercase + hyphens）、Description、Initial Files 复选框（soul.md/prompt.md 默认勾选，skills.md/memory.md 可选）。创建成功跳转 Role 编辑器。

#### Role 编辑器（/roles/:name）— VS Code 风格

核心页面，全屏 IDE 体验：

```
┌─ PageHeader: Roles > developer ─────────────────────────────────────┐
│                                                                      │
│ ┌─ Card (full-height, zero padding) ──────────────────────────────┐  │
│ │ ┌───────────┬───────────────────────────────────────────────┐   │  │
│ │ │ FILES     │  soul.md  ×  │  skills.md  ×                 │   │  │
│ │ │           ├──────────────┴───────────────────────────────┤   │  │
│ │ │  soul.md ●│                                              │   │  │
│ │ │  prompt.md│  # Soul                                      │   │  │
│ │ │  skills.md│                                              │   │  │
│ │ │  memory.md│  You are a senior full-stack developer.      │   │  │
│ │ │           │  You write clean, tested code.               │   │  │
│ │ │ ────────  │  ...                                         │   │  │
│ │ │ [+ New]   │                            (Monaco Editor)   │   │  │
│ │ │           │                                              │   │  │
│ │ │           ├──────────────────────────────────────────────┤   │  │
│ │ │           │  3 agents using this role    │          Save │   │  │
│ │ └───────────┴──────────────────────────────────────────────┘   │  │
│ └────────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────────┘
```

- **左侧文件树**（~180px）：单击打开 tab，hover 显示删除图标，`+ New File` 创建新文件
- **右侧编辑区**：Tab 栏（未保存显示 ●），Monaco Editor（markdown），`Cmd+S` 保存
- **底部状态栏**：Agent 引用数 | Save 按钮
- **beforeunload**：有 dirty files 时拦截离开

#### 操作风险分级更新

| 操作 | 风险 | 确认方式 |
|------|------|----------|
| 修改 Role 文件 | 中（影响所有引用 Agent） | 保存前无确认，但状态栏显示影响的 Agent 数 |
| 删除 Role 文件 | 高 | 确认弹窗 "Delete {file}? This affects all agents using this role." |
| 删除 Role | 高 | 有 Agent 引用时禁止删除；无引用时输入名称确认 |

### 4.4 LLM 配置（/llm）

展示当前 LiteLLM 中配置的所有模型：

| 列 | 说明 |
|----|------|
| Model Name | 模型别名 |
| Provider | openai / anthropic / azure / 自定义 |
| RPM / TPM | 速率限制 |
| Status | 可用 / 不可用 |
| Actions | 编辑、删除、测试连通性 |

操作：
- **新增模型**：弹出 Modal，填写 LiteLLM 模型配置（API Key、Base URL、RPM/TPM 等）
- **测试连通性**：调用 `/api/llm/health` 检查模型是否可达，行内显示结果

### 4.5 审计日志（/audit）

#### Trace 列表

过滤条件始终可见（不折叠），变化时 300ms debounce 后即时查询（无需点搜索按钮）：

```
Agent [全部 ▼]  类型 [全部 ▼]  时间 [今天 ▼]  状态 [全部 ▼]
```

表格列：

| 列 | 说明 |
|----|------|
| Trace ID | 可点击展开详情 |
| Agent | |
| Request Type | llm_inference / gitea_api / git_http / heartbeat |
| Method + Path | Gitea API 路径自动识别，提取 Issue/PR 编号并附 `[↗]` |
| Status Code | 非 2xx 红色背景 |
| Model | LLM 请求专有 |
| Tokens In / Out | |
| Latency | |
| Created At | |

点击行 → **右侧展开 Detail Panel**，显示 Request/Response 完整 JSON（Monaco Editor 只读模式）。Gitea API 路径自动识别 issue/pr 编号，Detail Panel 顶部显示 `[↗ Issue #16]` 或 `[↗ PR #8]` 快捷链接。

#### 告警响应典型流程

```
仪表盘告警横幅出现
  → 点击横幅
  → 跳转 Agent 详情页，概览 Tab 自动高亮 Conditions
  → 查看 CrashLoop 原因
  → 点「查看 Pod 日志」（日志 Tab）
  → 确认是 SOUL.md 配置错误
  → 切换配置 Tab → 编辑 SOUL.md → 保存（弹确认）
  → 顶部显示「滚动重启中…」→ Phase 恢复 Running
```

整个排查流程不离开 Agent 详情页。

### 4.6 监控图表（/monitor）

基于 ClickHouse 聚合数据，5 张图布局：

```
上方（两图并排）:
  Token 消耗趋势（折线图，最近 30 天，按 agent 分组）
  Agent 操作热力图（每小时 Gitea 操作次数，横轴=小时，纵轴=Agent）

下方（三图并排）:
  模型使用饼图（各模型 Token 占比）
  平均延迟折线图（LLM 请求延迟趋势）
  Agent 活跃度排行（柱状图，今日各 Agent Token + 请求数）
```

| 图表 | 数据源 |
|------|--------|
| Token 消耗趋势 | `token_usage_daily` |
| Agent 操作热力图 | `agent_operations_hourly` |
| 模型使用饼图 | `token_usage_daily` |
| 平均延迟折线图 | `token_usage_daily` |
| Agent 活跃度排行 | `audit.traces` |

### 4.7 训练数据导出（/export）

面向 AI 训练的数据导出，操作流程：

```
1. 设置过滤条件（时间范围、Agent、Request Type）
2. 点「预估数量」→ COUNT 查询，显示「约 1,234 条记录」
3. 点「导出 JSONL」→ 调用 /api/audit/export 流式下载
```

导出格式：JSONL，每行 `{"messages": [...], "response": {...}}`（OpenAI format）。

## 五、Gitea 链接与内嵌预览

### 5.1 外链跳转（新标签页）

以下位置提供 Gitea 外链：

| 位置 | 链接内容 |
|------|----------|
| Agent 概览 Tab | Gitea 用户页、关联仓库 |
| 活动时间线 | Commit、PR、Issue、评论 |
| 仪表盘实时事件流 | 每条事件 |
| 审计日志 Trace 详情 | 自动识别 Gitea API 路径，提取 Issue/PR 编号 |
| 指派任务成功 Toast | 新创建的 Issue |

### 5.2 Hover 内嵌预览

活动时间线和实时事件流中，PR/Issue 引用 hover 时展开小卡片，无需跳转即可判断行为是否合理：

```
developer-1  opened PR #8
             ┌──────────────────────────────┐
             │ PR #8  Add authentication     │
             │ feat/login → main             │
             │ +234 -12  CI: ✓ passing       │
             │ 关联: Fixes #15               │
             └──────────────────────────────┘
```

## 六、实时事件处理

Frontend 通过 WebSocket 连接 `WS /api/ws/events` 接收实时事件：

```
WebSocket 消息
  │
  ├─ event_type = "gateway.llm_inference"
  │   └─ 更新对应 Agent 的 Tokens Today（不重置列表）
  │
  ├─ event_type = "operator.agent_alert"
  │   └─ 显示 Toast 告警通知
  │   └─ 更新 Agent Phase 显示
  │
  ├─ event_type = "gitea.*"
  │   └─ 追加到实时事件流滚动列表
  │   └─ 追加到活动时间线（若当前打开的是对应 Agent 详情页）
  │
  └─ 连接断开 → 3 秒后自动重连（最多 5 次，指数退避）
```

## 七、告警通知规则

### 优先级与展示

| alert_type | 展示方式 | 持续时间 |
|-----------|---------|---------|
| `agent.crash_loop` | 右上角红色 Toast + 顶部横幅 | 横幅持续到手动关闭 |
| `agent.oom_expanded` | 右上角橙色 Toast | 10 秒自动消失 |
| `agent.heartbeat_timeout` | 右上角橙色 Toast + Agent 列表 Phase 变红 | Toast 10 秒消失 |
| 其他 | 右上角灰色 Toast | 5 秒自动消失 |

点击告警横幅 → 跳转对应 Agent 详情页（概览 Tab，自动高亮 Conditions）。

### 聚合规则（防告警轰炸）

- 同一 Agent 同类型告警，60 秒内只展示一次 Toast（后续静默）
- 横幅只保留最新一条（新告警替换旧横幅）
- 实时事件流和活动时间线不去重，完整展示所有事件

## 八、技术栈

| 层次 | 选型 |
|------|------|
| 框架 | React 18 + TypeScript |
| 组件库 | Semi Design |
| 状态管理 | Zustand（按领域分 store：agentStore / alertStore / wsStore） |
| 路由 | React Router v6 |
| 代码编辑器 | Monaco Editor（SOUL.md 编辑 + Trace 详情 JSON 展示） |
| 图表 | ECharts（通过 echarts-for-react） |
| HTTP 客户端 | Axios（拦截器自动携带 JWT，401 时跳转 /login） |
| WebSocket | 原生 WebSocket + 自定义 hook（useWebSocket，含自动重连） |
| 构建工具 | Vite |

### 目录结构

```
src/
├── pages/                    # 页面级组件（对应路由）
│   ├── login/
│   ├── dashboard/
│   ├── agents/
│   │   ├── list/            # Agent 列表（Role 列为链接）
│   │   ├── new/             # 6 步创建向导
│   │   └── detail/          # Tabs: 概览/活动时间线/文件(只读)/日志/审计
│   ├── roles/
│   │   ├── list/            # Role 列表
│   │   ├── new/             # 创建 Role 表单
│   │   └── editor/          # VS Code 风格编辑器（Monaco + 文件树 + Tab 栏）
│   ├── llm/
│   ├── audit/
│   │   ├── list/
│   │   └── trace-detail/
│   ├── monitor/
│   └── export/
├── components/               # 可复用组件
│   ├── layout/               # AppShell / Sidebar（含 Roles 导航项）
│   ├── agent/                # AgentStatusTag / AgentCard / FileEditor
│   ├── trace/                # TraceTable / TraceDetailPanel
│   ├── timeline/             # ActivityTimeline / TimelineItem
│   ├── charts/               # TokenTrendChart / HeatmapChart 等
│   ├── editor/               # MonacoEditor 封装
│   └── alert/                # AlertBanner / AlertToast
├── store/                    # Zustand stores
│   ├── agentStore.ts         # Agent 列表状态
│   ├── alertStore.ts         # 告警聚合状态（60s 去重逻辑）
│   └── wsStore.ts            # WebSocket 连接状态
├── hooks/
│   ├── useWebSocket.ts       # 自动重连、指数退避
│   ├── useAgents.ts
│   └── useTraces.ts
├── api/                      # Axios 请求封装
│   ├── agents.ts
│   ├── roles.ts              # Role CRUD + File CRUD API
│   ├── llm.ts
│   ├── audit.ts
│   └── monitor.ts
└── types/
    ├── agent.ts              # AgentSpec（role 为 string 引用）/ AgentStatus / Role / RoleFile
    ├── trace.ts
    └── event.ts              # WebSocket 事件类型
```

## 九、补充设计

### 9.1 默认有意义的初始状态

所有列表页默认展示**今日数据**，不是空白等用户选择时间范围：
- 审计日志：默认今天，全部 Agent，全部类型
- 活动时间线：默认今天，全部事件类型
- 监控图表：Token 趋势默认最近 30 天，活动时间线默认今天

### 9.2 权限控制（post-MVP）

MVP 阶段所有登录用户均为完整管理员权限。

未来扩展只读角色时，以下操作需要完整权限，只读用户仅可查看：
- 创建/删除/暂停/恢复 Agent
- 指派任务
- 修改 SOUL.md / LLM 配置
- 新增/删除 LLM 模型
- 导出训练数据

### 9.3 移动端

当前优先 PC 管理界面（1280px+）。Semi Design 组件默认不响应式，移动端适配推迟到 post-MVP。

## 十、Agent 实时观测中心（"直播间"模式）

> 核心需求：管理员需要像"看直播"一样观测每个 Agent 的实时动态——正在做什么、在想什么、产出了什么。当前的 Monitoring 页面只有统计图表，缺乏对 Agent 行为的**实时透明度**。

### 10.1 设计理念

Agent 本质上是"数字员工"，管理员需要的不是仪表盘上的数字，而是：
1. **一眼看清全员状态**：谁在忙、谁闲着、谁出了问题
2. **深入单个 Agent 的"工位"**：看到它此刻在做什么、思考过程、行动结果
3. **历史回放**：回看某个 Agent 过去做了什么，像翻阅工作日志

这和传统监控（CPU/内存/请求数）是不同维度——传统监控回答"系统健不健康"，Agent 观测回答"它在干什么、干得怎么样"。

### 10.2 Sidebar 导航调整

当前 Sidebar 中"监控图表"定位模糊。调整为：

```
Sidebar
├── 仪表盘         /dashboard        （全局总览，保持不变）
├── Agent          /agents           （管理操作：创建/删除/配置）
├── Agent 观测     /observe          （新增：实时观测中心）
├── Roles          /roles
├── LLM 配置       /llm
├── 审计日志       /audit
├── 数据看板       /monitor          （原"监控图表"，改名为运营分析，保留统计图表）
└── 数据导出       /export
```

### 10.3 Agent 观测全景页（/observe）

**设计目标**：打开就能看到所有 Agent 的实时状态，无需任何操作。

#### 布局：卡片网格

```
┌─ PageHeader: Agent Observe ────────────────────────────────────────┐
│  ● 5/5 Online    ○ Filter: [All ▼]    Auto-refresh: ON            │
├────────────────────────────────────────────────────────────────────┤
│                                                                    │
│  ┌─ developer-1 ──────────┐  ┌─ developer-2 ──────────┐          │
│  │  ● Running              │  │  ● Running              │          │
│  │  Role: developer         │  │  Role: developer         │          │
│  │                          │  │                          │          │
│  │  ▶ 正在编写登录模块       │  │  ▶ 等待下一个 Cron 周期  │          │
│  │    feat/login +234 -12   │  │    空闲中...              │          │
│  │                          │  │                          │          │
│  │  最近: PR #8 创建         │  │  最近: Issue #12 评论     │          │
│  │  Token: 45,230           │  │  Token: 12,800           │          │
│  │  ██████████░░ 12m ago    │  │  ██░░░░░░░░░░ 3m ago     │          │
│  └──────────────────────────┘  └──────────────────────────┘          │
│                                                                    │
│  ┌─ reviewer-1 ───────────┐  ┌─ observer-1 ───────────┐          │
│  │  ● Running              │  │  ● Running              │          │
│  │  Role: reviewer          │  │  Role: observer          │          │
│  │                          │  │                          │          │
│  │  ▶ 正在 Review PR #8     │  │  ▶ 扫描 open issues      │          │
│  │    ai-team/webapp        │  │    发现 3 个未分配         │          │
│  │                          │  │                          │          │
│  │  最近: PR #7 approved    │  │  最近: Issue #15 创建     │          │
│  │  Token: 8,900            │  │  Token: 6,120            │          │
│  │  ████████░░░░ 8m ago     │  │  ██████████████ just now  │          │
│  └──────────────────────────┘  └──────────────────────────┘          │
│                                                                    │
│  ┌─ sre-1 ────────────────┐                                       │
│  │  ◐ Paused               │                                       │
│  │  Role: sre               │                                       │
│  │                          │                                       │
│  │  ▶ 已暂停                │                                       │
│  │    管理员手动暂停         │                                       │
│  │                          │                                       │
│  │  最近: 告警排查 dev-1     │                                       │
│  │  Token: 0                │                                       │
│  │  ░░░░░░░░░░░░ 2h ago     │                                       │
│  └──────────────────────────┘                                       │
└────────────────────────────────────────────────────────────────────┘
```

#### 卡片内容说明

每张卡片展示：

| 区域 | 内容 | 数据来源 |
|------|------|----------|
| 头部 | Agent 名称 + Phase 状态点 | agent CR status.phase |
| 角色 | Role 名称 | agent CR spec.role |
| **当前动态** | 正在做什么的一句话描述 | **新增字段**: WebSocket 推送的 `agent.activity_update` 事件 |
| 当前动态详情 | 分支名、文件变更数、目标仓库等 | 同上事件的 payload |
| 最近行动 | 最近一次完成的 Gitea 操作 | agent CR status.lastAction |
| 今日 Token | 累计消耗 | agent CR status.tokenUsage.today |
| 活跃度条 | 最后活跃时间的可视化进度条 | 基于 lastAction.timestamp 计算 |

#### 活跃度条规则

```
距离最后活跃时间：
  0-1 分钟  →  ██████████████  just now（绿色）
  1-5 分钟  →  ██████████░░░░  Nm ago（绿色）
  5-15 分钟 →  ██████░░░░░░░░  Nm ago（黄色）
  15-60 分钟→  ███░░░░░░░░░░░  Nm ago（灰色）
  >1 小时   →  █░░░░░░░░░░░░░  Xh ago（灰色）
```

#### 交互

- **点击任意卡片** → 进入该 Agent 的观测详情页 `/observe/:name`
- **右键/长按卡片** → 快捷操作：暂停/恢复、指派任务、查看日志
- **卡片实时刷新**：WebSocket 推送时只更新对应卡片，不影响其他卡片
- **状态变化动画**：Phase 变更时卡片边框闪烁（绿→红：脉冲红色 2 秒）

### 10.4 Agent 观测详情页（/observe/:name）— "直播间"

**设计目标**：像坐在 Agent 旁边看它工作。

#### 布局

```
┌─ PageHeader: Observe > developer-1 ● Running ──── [暂停] [指派] ──┐
│                                                                     │
│  ┌─ 当前状态 ──────────────────────────────────────────────────┐   │
│  │  ▶ 正在编写登录模块的单元测试                                 │   │
│  │    分支: feat/login  仓库: ai-team/webapp                    │   │
│  │    已持续: 3 分 42 秒                                        │   │
│  └──────────────────────────────────────────────────────────────┘   │
│                                                                     │
│  ┌─ 实时对话流（左 55%）────────┬─ 代码面板（右 45%）──────────┐   │
│  │                              │                              │   │
│  │  ⏰ 13:05 Cron 触发          │  📁 workspace/work/webapp/   │   │
│  │  ──────────────────────      │  ├── src/                    │   │
│  │                              │  │   ├── auth.ts  ●          │   │
│  │  🧠 LLM 推理 #1             │  │   ├── index.ts            │   │
│  │  ┌─ 💭 thinking ─────────┐  │  │   └── utils.ts            │   │
│  │  │ (折叠) 展开查看思考过程│  │  ├── test/                   │   │
│  │  └────────────────────────┘  │  │   └── auth.test.ts  ●     │   │
│  │  ┌─ User (prompt.md) ────┐  │  └── package.json            │   │
│  │  │ 检查 open issues...   │  │                              │   │
│  │  └───────────────────────┘  │  ┌─ Monaco: auth.ts ────────┐│   │
│  │  ┌─ Assistant ───────────┐  │  │  1│ import express ...    ││   │
│  │  │ 我将开始处理 Issue #15│  │  │  2│ export function      ││   │
│  │  │ 首先分析代码结构...    │  │  │ +3│ + login(user, pass) {││   │
│  │  └───────────────────────┘  │  │ +4│ +   validate(user)   ││   │
│  │  [🔧 toolCall: read_file]  │  │ +5│ +   return jwt.sign  ││   │
│  │  [✅ result: ok]            │  │  6│ }                     ││   │
│  │                              │  │  7│                      ││   │
│  │  🧠 LLM 推理 #2             │  └────────────────────────────┘│   │
│  │  ┌─ Assistant ───────────┐  │                              │   │
│  │  │ 需要修改 auth.ts...   │  │  git diff: 2 files changed  │   │
│  │  └───────────────────────┘  │  +34 -12 lines              │   │
│  │  [🔧 toolCall: write_file] │                              │   │
│  │   ┌─ inline diff ──────┐   │                              │   │
│  │   │ + login(user, pass) │   │                              │   │
│  │   │ +   validate(user)  │   │                              │   │
│  │   └─────────────────────┘   │                              │   │
│  │                              │                              │   │
│  │  ● 等待下一个推理...        │                              │   │
│  └──────────────────────────────┴──────────────────────────────┘   │
│                                                                     │
│  ┌─ 历史会话 ──────────────────────────────────────────────────┐   │
│  │  ▸ a1b2c3d4  今天 13:00  12 messages  ● running             │   │
│  │  ▸ e5f6g7h8  今天 12:55  8 messages   ✓ completed           │   │
│  │  ▸ i9j0k1l2  今天 12:50  15 messages  ✓ completed           │   │
│  │  ▸ m3n4o5p6  今天 12:00  22 messages  ✓ completed           │   │
│  │                                                [加载更多]     │   │
│  └──────────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────────┘
```

#### 四个核心区域

**1. 当前状态栏（顶部）**

一句话说明 Agent 正在做什么。数据来源：从 Sidecar JSONL 最新消息推导：
- 最新消息是 `toolCall` → "正在执行 {toolName}..."（带 loading 动画）
- 最新消息是 assistant `text` → "正在思考..."
- 最新消息是 `session` 类型 → "会话开始"
- 长时间无新消息 → "等待下一个 Cron 周期（距下次: 2m30s）"
- Agent Phase = Paused → "已暂停 — 管理员手动暂停"

**2. 实时对话流（左侧）**

按时间顺序展示 JSONL 消息流，直接解析 OpenClaw 的 JSONL 格式渲染：

- **session 类型**：渲染为会话分割线（淡蓝色背景，标记工作周期起点）
- **message (role=user)**：左对齐浅色气泡，展示 Cron prompt 或手动指令
- **message (role=assistant)**：右对齐深色气泡，content 数组逐项渲染：
  - `thinking` block → 灰色折叠区域，点击展开查看思考过程
  - `text` block → Markdown 渲染（含代码块语法高亮）
  - `toolCall` block → 工具调用卡片，显示工具名 + 参数摘要
- **message (role=toolResult)**：紧跟对应 toolCall 下方，展示执行结果
  - 成功：绿色边框卡片，内容折叠（点击展开）
  - 失败（`isError: true`）：红色边框卡片，内容默认展开
- **toolCall 涉及文件修改时**：内联 diff 展示（+ 绿色新增行、- 红色删除行）
- **model_change / thinking_level_change**：小型信息标签（灰色 badge）
- **每条 assistant message**：底部显示 `usage` token 消耗和 `model` 信息
- **实时流式展示**：Sidecar WebSocket 推送新 JSONL 行，从底部追加。用户手动上滚时暂停自动滚动，出现"回到底部"悬浮按钮

**3. 代码面板（右侧）**

展示 Agent 当前正在编辑的代码，数据来源于 Sidecar 的 workspace API：

- **文件树**：顶部展示 `workspace/work/{repo}/` 的文件树
  - 已修改文件标记 `●`（根据 git diff 状态）
  - 新增文件标记 `+`
  - 点击文件 → 下方 Monaco Editor 展示内容
- **Monaco Editor（只读）**：展示选中文件的当前内容
  - diff 高亮：根据 `GET /workspace/diff` 的输出，高亮变更行
  - 实时更新：Sidecar WebSocket 推送 `file_change` 事件时自动刷新
- **联动交互**：对话流中点击 toolCall（如 `write_file`、`edit_file`）→ 代码面板自动跳转到对应文件和行号

布局比例：对话流 55% / 代码面板 45%，可拖拽调整（使用现有 `SplitPane` 组件）。

**4. 历史会话列表（底部）**

从 Sidecar `/sessions` API 获取会话文件列表，每个 JSONL 文件对应一个会话：

| 列 | 说明 | 数据来源 |
|----|------|----------|
| 会话 ID | JSONL 文件名（UUID 前 8 位） | 文件名 |
| 时间 | 会话开始时间 | JSONL 首行 `session.timestamp` |
| 消息数 | 对话消息条数 | Sidecar 预计算 |
| 状态 | 进行中 / 已完成 | 是否为当前活跃文件 |

- 点击某条历史会话 → 从 Sidecar 加载该 JSONL 完整内容，在对话流区域展示（替换当前实时流）
- 顶部出现提示栏 "正在查看历史会话 {id} — [返回实时]"
- 历史会话的代码面板显示该时间点的 git 状态（如果可获取）

### 10.5 数据流设计 — Agent Observer Sidecar 架构

> **设计决策**：不在 ClickHouse 中新建 session/conversation 表，而是在每个 Agent Pod 内部署一个轻量 Sidecar（`agent-observer`），直接读取 OpenClaw 本地的 JSONL 对话文件和 workspace 文件变更。ClickHouse 继续专注于统计分析（token 趋势、操作频次），不重复存储对话内容。

#### 为什么选择 Sidecar 而非 ClickHouse 存储对话

1. **数据完整度**：OpenClaw 的 JSONL 包含 `thinking`（思考过程）、`toolCall`（工具调用细节）、`toolResult`（执行结果），比 Gateway 审计的 request/response body 丰富得多
2. **代码可视化**：只有 Pod 内能直接 watch `workspace/work/` 目录的文件变更，实现"看 Agent 改代码"的效果
3. **零改造**：OpenClaw 已经在写这些文件，Sidecar 只是旁观者，不需要改 Agent Runtime
4. **实时性**：inotify 文件监听 → WebSocket 推送，无 Redis → ClickHouse → API 的链路延迟

#### OpenClaw 本地数据结构

```
~/.openclaw/
├── completions/              ← JSONL 对话记录（核心数据源）
│   └── {session-uuid}.jsonl  ← 每个会话一个文件
├── workspace/                ← Agent 工作目录（PVC 持久化）
│   ├── episodic/             ← 每日行动摘要
│   ├── semantic/             ← 仓库知识积累
│   ├── state.json            ← 当前任务状态
│   └── work/{repo}/          ← Git 代码工作目录
├── memory/                   ← Agent 长期记忆
├── logs/                     ← 运行日志
├── subagents/                ← 子 Agent 数据
└── ...
```

#### JSONL 消息格式

每行一个 JSON 对象，`type` 字段标识消息类型：

| type | 说明 | 前端渲染方式 |
|------|------|-------------|
| `session` | 会话开始标记，含 `id`、`cwd`、`timestamp` | 会话分割线 |
| `model_change` | 模型切换，含 `provider`、`modelId` | 信息标签 |
| `thinking_level_change` | 思考模式切换 | 信息标签 |
| `message` (role=user) | Cron prompt 或用户指令 | 左侧气泡 |
| `message` (role=assistant) | LLM 回复，可含多个 content block | 右侧气泡 |
| `message` (role=toolResult) | 工具调用执行结果 | 工具结果卡片 |

Assistant message 的 `content` 是数组，可包含：
- `{ type: "thinking", thinking: "..." }` — 思考过程（默认折叠展示）
- `{ type: "text", text: "..." }` — 文本回复（Markdown 渲染）
- `{ type: "toolCall", name: "...", arguments: {...} }` — 工具调用（展示工具名 + 参数）

每条 assistant message 还包含：`usage`（token 消耗）、`model`、`stopReason`。

消息通过 `parentId` 链接形成对话树。

#### Agent Observer Sidecar

Go 实现，以 Sidecar 容器运行在每个 Agent Pod 内，共享 PVC 卷。

```
Agent Pod
┌──────────────────────────────────────────────────────┐
│  [OpenClaw]  ←→  ~/.openclaw/ (PVC)                  │
│                       ↑                               │
│                  (inotify watch)                      │
│                       │                               │
│  [agent-observer sidecar]  :8081                     │
│     │                                                 │
│     ├── HTTP API（对话 + 文件）                        │
│     └── WebSocket（实时推送）                          │
└──────────────────────────────────────────────────────┘
```

**HTTP API**：

| Method | Path | 说明 |
|--------|------|------|
| `GET` | `/sessions` | JSONL 会话文件列表（id, 起始时间, 消息数, 状态） |
| `GET` | `/sessions/current` | 当前活跃会话内容（最新的 JSONL 文件） |
| `GET` | `/sessions/:id` | 指定会话的完整 JSONL 内容 |
| `GET` | `/workspace/tree` | workspace 文件树（含修改状态标记） |
| `GET` | `/workspace/file?path=...` | 指定文件内容（只允许 ~/.openclaw/ 下） |
| `GET` | `/workspace/diff` | workspace/work/ 下的 git diff 输出 |
| `GET` | `/healthz` | Sidecar 健康检查 |

**WebSocket API**：

| Path | 推送内容 |
|------|----------|
| `WS /ws/live` | 两类实时事件：`{ type: "jsonl", data: <新 JSONL 行> }` 和 `{ type: "file_change", path: "...", action: "modified\|created\|deleted" }` |

**安全约束**：
- 文件访问严格限制在 `~/.openclaw/` 目录内，禁止路径穿越
- Sidecar 只读，不写入任何文件
- 端口 8081 仅集群内可达（ClusterIP Service）

**K8s 部署**：
- Operator 在创建 Agent Pod 时自动注入 Sidecar 容器
- Sidecar 镜像：`registry.devops.local/platform/agent-observer:v1.0.0`
- 资源限制：50m CPU / 64Mi Memory（轻量旁观者）
- 共享 PVC 卷：与 OpenClaw 主容器挂载同一 PVC，readOnly

#### 数据流全景

```
┌─ /observe 全景页 ──────────────────────────────────────┐
│  数据源：Agent CR (phase, lastAction, tokenUsage)      │
│         + ClickHouse (聚合统计)                        │
│  刷新：现有 WebSocket (operator.agent_alert, gitea.*)  │
│        + 定时轮询 Agent 列表 API                       │
└────────────────────────────────────────────────────────┘

┌─ /observe/:name 直播间 ───────────────────────────────┐
│  实时对话流：                                          │
│    Backend /api/agents/:name/observe/sessions/current  │
│      → 反向代理到 agent-{name}.agents.svc:8081        │
│    Backend /api/agents/:name/observe/ws/live           │
│      → WebSocket 中继到 Sidecar                       │
│                                                        │
│  代码面板：                                            │
│    Backend /api/agents/:name/observe/workspace/tree    │
│    Backend /api/agents/:name/observe/workspace/file    │
│    Backend /api/agents/:name/observe/workspace/diff    │
│      → 反向代理到 Sidecar                             │
│                                                        │
│  历史会话：                                            │
│    Backend /api/agents/:name/observe/sessions          │
│    Backend /api/agents/:name/observe/sessions/:id      │
│      → 反向代理到 Sidecar (JSONL on PVC)              │
└────────────────────────────────────────────────────────┘

┌─ /monitor 数据看板 ────────────────────────────────────┐
│  数据源：ClickHouse audit.traces + 物化视图            │
│  定位：运营分析（token 趋势、模型分布、活跃度排行）    │
│  不存储对话内容，不新建 session/conversation 表        │
└────────────────────────────────────────────────────────┘
```

#### Backend 反向代理

Control Panel Backend 新增路由组，将 `/api/agents/:name/observe/*` 反向代理到对应 Agent Pod 的 Sidecar：

```
/api/agents/:name/observe/*
  → 解析 agent name
  → 构造 Sidecar 地址：http://agent-{name}.agents.svc:8081/*
  → HTTP 反向代理 / WebSocket 中继
```

WebSocket 中继：前端连接 Backend 的 `/api/agents/:name/observe/ws/live`，Backend 建立到 Sidecar 的 WebSocket 连接，双向转发消息。当前端断开时，Backend 关闭到 Sidecar 的连接（避免泄漏）。

### 10.6 与现有页面的关系

| 现有页面 | 定位调整 |
|---------|---------|
| `/dashboard` | 保持不变：全局健康状态一览 |
| `/agents` | 保持不变：Agent 的管理操作（CRUD、配置） |
| `/observe` | **新增**：Agent 实时观测，行为透明度 |
| `/agents/:name` | 保持不变：Agent 的配置详情和管理操作 |
| `/observe/:name` | **新增**：单个 Agent 的"直播间"，完整对话和行为流 |
| `/monitor` | 改名"数据看板"：运营分析（token 趋势、活跃度排行等统计图表） |
| `/audit` | 保持不变：底层 trace 级别的审计日志 |

区分逻辑：
- **Agent 管理 (`/agents`)** 回答"怎么配置和控制 Agent"
- **Agent 观测 (`/observe`)** 回答"Agent 此刻在做什么、做得怎么样"
- **审计日志 (`/audit`)** 回答"系统底层发生了什么请求"
- **数据看板 (`/monitor`)** 回答"整体趋势和统计"

## 十一、已知问题与改进项

> 以下为代码审查中发现的设计实现差异和交互体验问题，按优先级排序。
> 详细任务清单见 [frontend-remaining-issues](../tasks/2026-03-12-frontend-remaining-issues.md)。

### 11.1 表单体验

- Role 创建页输入框未限制宽度，Name/Description 拉满整行
- Role 创建页 Description 使用单行 `<Input>` 而非 `<Textarea>`
- Agent 创建向导 6 步中 Schedule 和 Gitea 步骤信息密度过低
- Model 选择使用自由文本输入而非下拉选择
- 所有表单缺乏前端验证（Name 格式、Cron 表达式、必填项）
- Cron 输入框缺少人类可读的翻译（如"每 5 分钟"）

### 11.2 编辑器体验

- Role 编辑器无法编辑 Role 的 description
- Role 编辑器只能逐文件保存，无 Save All 功能
- Monaco Editor 使用默认 `light` 主题，与应用暖色系背景割裂
- Role 创建页无文件模板预览

### 11.3 设计文档与实现差异

- 安装了 Semi Design 但未使用任何组件，全部自定义
- Agent 列表未实现"右侧展开 Detail Panel"，改为页面跳转
- Activity Timeline Tab 数据为空，仅显示 EmptyState

### 11.4 实时观测能力

- Dashboard 的 Agent 列表只显示基础字段，无法判断 Agent "在做什么"
- Monitor 页面只有统计图表，缺乏行为级别的观测
- WebSocket 事件类型有限，不包含对话内容和会话生命周期
- 缺乏历史对话回放能力
