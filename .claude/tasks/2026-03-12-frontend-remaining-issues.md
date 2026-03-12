# Frontend Remaining Issues

> 关联设计：[control-panel-frontend.md](../designs/control-panel-frontend.md) 第十一节
> 创建时间：2026-03-12
> 状态：待实施

---

## P0 — 阻塞体验的问题

### 表单验证

- [ ] Role 创建页：Name 字段加正则校验 `/^[a-z][a-z0-9-]*$/`，不符合时行内提示
- [ ] Role 创建页：Name 不能为空时禁用 Create 按钮（当前已实现，但无格式校验）
- [ ] Agent 创建向导：每步 Next 按钮加 validation gate（Step1 需要 name+role，Step2 需要 model 等）
- [ ] Agent 创建向导：Name 字段加同样的正则校验
- [ ] Cron 表达式输入：加基本格式校验（5 段，合法字符），不合法时行内提示
- [ ] 指派任务 Modal：Title 为空时禁用 Create & Assign 按钮

### 表单布局

- [ ] Role 创建页 `.formBody` 加 `max-width: 480px`
- [ ] Role 创建页 Name 输入框加 `max-width: 280px`
- [ ] Role 创建页 Description 从 `<Input>` 改为 `<Textarea>`，2-3 行高度

---

## P1 — 显著影响体验的问题

### Agent 创建向导优化

- [ ] 合并 6 步为 4 步：Identity (Name+Role) → Configuration (LLM+Schedule) → Infrastructure (Resources+Gitea) → Review
- [ ] Model 字段从自由文本 `<Input>` 改为 `<Select>`，数据源从 `/api/llm/models` 获取
- [ ] Cron 输入框下方实时显示人类可读的解析（如 `cronstrue` 库："Every 5 minutes"）
- [ ] Review 步骤的 Role 区域加 "Preview files" 展开按钮，展开后用只读 Monaco 显示 soul.md 内容

### Agent 实时观测中心（新增页面）

- [ ] 新增 Sidebar 导航项 "Agent 观测" `/observe`
- [ ] 实现 Agent 观测全景页 `/observe`：卡片网格，每张卡片显示 Agent 名称、Phase、当前动态、最近行动、Token、活跃度条
- [ ] 实现 Agent 观测详情页 `/observe/:name`（"直播间"）：
  - [ ] 顶部当前状态栏：从 Sidecar JSONL 最新消息推导 Agent 正在做什么
  - [ ] 左侧实时对话流：解析 JSONL 渲染气泡（user/assistant/toolCall/toolResult/thinking）
  - [ ] 右侧代码面板：文件树 + 只读 Monaco Editor + git diff 高亮
  - [ ] 对话流 toolCall 内联 diff 展示
  - [ ] 对话流点击 toolCall → 代码面板联动跳转到对应文件
  - [ ] 底部历史会话列表：从 Sidecar /sessions 获取，可点击回放

### Agent Observer Sidecar（Go，Pod 内 Sidecar）

- [ ] 实现 Sidecar 核心：inotify watch `~/.openclaw/completions/` 和 `workspace/` 目录
- [ ] HTTP API：`GET /sessions`（会话文件列表）
- [ ] HTTP API：`GET /sessions/current`（当前活跃会话内容）
- [ ] HTTP API：`GET /sessions/:id`（历史会话完整 JSONL）
- [ ] HTTP API：`GET /workspace/tree`（文件树 + 修改状态）
- [ ] HTTP API：`GET /workspace/file?path=...`（文件内容，限制 ~/.openclaw/ 目录）
- [ ] HTTP API：`GET /workspace/diff`（workspace/work/ 下 git diff）
- [ ] WebSocket：`WS /ws/live`（推送新 JSONL 行 + 文件变更事件）
- [ ] 安全：路径穿越防护，只读模式
- [ ] Dockerfile + 镜像构建
- [ ] Operator 改造：创建 Agent Pod 时自动注入 Sidecar 容器

### Backend 反向代理（观测中心所需）

- [ ] 新增路由组 `/api/agents/:name/observe/*` → 反向代理到 `agent-{name}.agents.svc:8081`
- [ ] WebSocket 中继：`/api/agents/:name/observe/ws/live` → Sidecar WebSocket
- [ ] 连接管理：前端断开时自动关闭到 Sidecar 的连接

### 对话流交互细节

- [ ] thinking block 默认折叠，点击展开查看思考过程
- [ ] Assistant text 回复支持 Markdown 渲染（含代码块语法高亮）
- [ ] toolCall 渲染为工具卡片（工具名 + 参数摘要）
- [ ] toolResult 渲染为结果卡片（成功绿色/失败红色，可折叠）
- [ ] 新消息自动滚动到底部；用户手动上滚时暂停自动滚动，出现"回到底部"按钮
- [ ] session 类型消息用淡蓝色分割线标记会话起点
- [ ] model_change / thinking_level_change 渲染为灰色 badge
- [ ] 每条 assistant message 底部显示 token 消耗和模型信息
- [ ] 查看历史会话时顶部出现提示栏 "正在查看历史会话 {id} — [返回实时]"

---

## P2 — 体验改进

### Role 编辑器

- [ ] Role 编辑器 PageHeader 旁或下方加可编辑的 description 行（click-to-edit 模式）
- [ ] 新增 "Save All" 按钮或 `Cmd+Shift+S` 快捷键，批量保存所有 dirty 文件
- [ ] 状态栏：有多个 dirty 文件时显示 "N unsaved files"
- [ ] 保存成功后 Tab 上的 dirty dot 消失时加短暂绿色勾动画

### Monaco Editor 主题

- [ ] 定义自定义 Monaco 主题，背景色与 `--bg-surface` 对齐
- [ ] 或者给 Monaco 容器加与 Card 一致的内阴影/边框过渡

### Role 创建页文件预览

- [ ] 勾选初始文件后，下方展示只读 Monaco 预览，显示模板内容
- [ ] 让用户在创建前看到 soul.md / prompt.md 的默认模板是什么

### 观测全景页增强

- [ ] 卡片支持右键/长按快捷操作菜单：暂停/恢复、指派任务、查看日志
- [ ] Agent Phase 变更时卡片边框闪烁动画（Running→Error：脉冲红色 2 秒）
- [ ] 支持按 Role / Phase 过滤卡片

---

## P3 — 技术债务

### Semi Design 组件库

- [ ] 评估：真正使用 Semi Design 组件替换自定义 `components/ui/`，利用 Semi 的 Form 验证、Notification、Cascader 等能力
- [ ] 或者：从 `package.json` 中移除 `@douyinfe/semi-ui` 依赖，减小包体积（当前只用了 `@douyinfe/semi-icons`）

### 设计文档与实现对齐

- [ ] Agent 列表实现"右侧展开 Detail Panel"模式（设计文档核心理念：保持列表上下文）
- [ ] Activity Timeline Tab 接入实际数据（当前仅 EmptyState）
- [ ] Dashboard Agent 列表行点击后右侧展开 Detail Panel（设计文档描述），而非跳转页面

### Monitor 页面改名

- [ ] Sidebar 导航项从"监控图表"改为"数据看板"
- [ ] 明确定位为运营分析（统计趋势），与新增的"Agent 观测"（行为透明度）区分
