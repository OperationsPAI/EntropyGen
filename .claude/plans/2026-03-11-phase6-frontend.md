# Phase 6: Control Panel Frontend — React 管理界面

> 关联设计：[Control Panel Frontend 设计](../designs/control-panel-frontend.md) | [Control Panel Backend 设计](../designs/control-panel-backend.md)
> 依赖 Phase：Phase 5（Backend API 已就绪）
> 可并行：Frontend 开发时可 mock Backend API

## 目标

实现基于 React + TypeScript + Semi Design 的管理控制台，包含 7 个主要页面，通过 WebSocket 接收实时事件推送。

---

## 技术栈

| 层次 | 选型 |
|------|------|
| 框架 | React 18 + TypeScript |
| 组件库 | Semi Design |
| 状态管理 | Zustand |
| 路由 | React Router v6 |
| 代码编辑器 | Monaco Editor（SOUL.md 编辑） |
| 图表 | ECharts（via echarts-for-react） |
| HTTP 客户端 | Axios |
| WebSocket | 原生 WebSocket + 自定义 hook |
| 构建工具 | Vite |

---

## 任务拆解

### 6.1 项目初始化

- [ ] `cd frontend && npm create vite@latest . -- --template react-ts`
- [ ] 安装依赖：`@douyinfe/semi-ui`、`@douyinfe/semi-icons`、`zustand`、`react-router-dom`、`axios`、`@monaco-editor/react`、`echarts`、`echarts-for-react`
- [ ] 配置 Vite 代理（开发时代理 `/api/*` 到 Backend）
- [ ] 配置 Semi Design 主题（Vite plugin）
- [ ] 配置 TypeScript strict 模式

### 6.2 类型定义（`src/types/`）

- [ ] `agent.ts`：
  ```typescript
  interface Agent {
    name: string
    spec: { role: AgentRole; soul: string; llm: LLMConfig; cron: CronConfig; ... }
    status: { phase: AgentPhase; conditions: Condition[]; lastAction: LastAction; tokenUsage: TokenUsage; ... }
  }
  type AgentPhase = 'Pending' | 'Initializing' | 'Running' | 'Paused' | 'Error'
  ```
- [ ] `trace.ts`：AuditTrace（trace_id、agent_id、request_type、model、tokens、latency 等）
- [ ] `event.ts`：RealtimeEvent（event_type、agent_id、payload）

### 6.3 API 服务层（`src/services/`）

- [ ] `api.ts`：Axios 实例，JWT Token 自动注入，401 自动跳转登录
- [ ] `agents.ts`：getAgents、getAgent、createAgent、updateAgent、deleteAgent、pauseAgent、resumeAgent、getAgentLogs
- [ ] `llm.ts`：getModels、createModel、updateModel、deleteModel、checkHealth
- [ ] `audit.ts`：getTraces、getTrace、getTokenUsage、getAgentActivity、getOperations、exportTraces
- [ ] `auth.ts`：login、logout、getMe

### 6.4 全局状态（`src/stores/`）

- [ ] `agentStore.ts`：agents 列表，WebSocket 事件触发 token 数更新
- [ ] `eventStore.ts`：实时事件队列（最近 50 条），新事件入队
- [ ] `alertStore.ts`：告警通知（含去重逻辑，60s 内同类型只展示一次）

### 6.5 WebSocket Hook（`src/hooks/useWebSocket.ts`）

- [ ] 连接 `WS /api/ws/events`（JWT 附在 query 参数）
- [ ] 断连自动重连（指数退避，3s/6s/12s/24s/48s，最多 5 次）
- [ ] 收到事件 → 分发到对应 store：
  - `gateway.llm_inference` → 更新 agentStore 中对应 Agent 的 tokenUsage.today
  - `operator.agent_alert` → alertStore 添加告警，agentStore 更新 phase
  - `gitea.*` → eventStore 添加事件

### 6.6 登录页（`src/pages/Login/`）

- [ ] Semi Design Form 表单（用户名 + 密码）
- [ ] 登录成功 → 存 JWT 到 localStorage → 跳转 `/dashboard`
- [ ] 已登录 → 自动跳转

### 6.7 概览仪表盘（`src/pages/Dashboard/`）

- [ ] 顶部指标卡（4 个 Statistic 组件）：运行中 Agent 数、今日 Token 消耗、今日 Gitea 操作次数、告警数
- [ ] Agent 状态列表（Table，行内显示 Phase 标签、Last Action、Tokens Today）
- [ ] 实时事件流（虚拟滚动列表，最近 50 条，WebSocket 推送）
- [ ] Token 消耗趋势图（ECharts Line，最近 7 天，按 Agent 分组）

### 6.8 Agent 管理（`src/pages/Agents/`）

#### 列表页

- [ ] Table 展示所有 Agent（列：Name、Role、Phase 标签、Model、Last Action、Tokens Today、Age、操作）
- [ ] Phase 标签颜色：Running=绿、Paused=橙、Error=红、Pending/Initializing=蓝
- [ ] 按 Role/Phase 过滤（Select 组件）
- [ ] **新建 Agent** 按钮 → 打开 Drawer 表单（见 6.8.3）
- [ ] 行操作：暂停/恢复（确认 Modal）、查看详情、删除（二次确认）

#### 详情页（`src/pages/Agents/Detail.tsx`）

- [ ] 5 个 Tab：
  1. **概览**：基本信息 + CR status.conditions 列表
  2. **配置**：SOUL.md Monaco Editor（Markdown 高亮）+ LLM 配置表单 + Cron 配置
     - 保存时提示"修改 SOUL.md 将触发 Pod 重启"
  3. **审计日志**：嵌入 Audit Trace 表格（按 agent_id 过滤）
  4. **实时日志**：WebSocket 推送该 Agent 的事件流
  5. **Pod 日志**：调用 `/api/agents/:name/logs`，代码块展示

#### 新建 Agent Drawer 表单

所有字段来自设计文档 §6.1：

| 字段 | 组件 |
|------|------|
| Name | Input（创建后不可改） |
| Role | Select（observer/developer/reviewer/sre） |
| LLM Model | Select（动态加载 `/api/llm/models`） |
| LLM Temperature | Slider (0-2, step 0.1) |
| LLM Max Tokens | InputNumber |
| Cron Schedule | Input + 右侧"下次触发时间"预览 |
| Cron Prompt | Textarea |
| CPU Request/Limit | Input |
| Memory Request/Limit | Input |
| Workspace Size | Input |
| Gitea Repo | Input（org/repo 格式） |
| Gitea Permissions | CheckboxGroup（read/write/review/merge） |

### 6.9 LLM 配置页（`src/pages/LLM/`）

- [ ] 模型列表 Table（Model Name、Provider、RPM/TPM、Status、操作）
- [ ] 新增模型 Modal（API Key、Base URL、RPM/TPM 等字段）
- [ ] 测试连通性按钮（调用 `/api/llm/health`）

### 6.10 审计日志（`src/pages/Audit/`）

- [ ] 多维过滤（Agent 下拉多选、Request Type、时间范围、Status Code）
- [ ] 分页 Table
- [ ] Trace 详情抽屉（request_body/response_body JSON 预览，Monaco JSON viewer）

### 6.11 监控图表（`src/pages/Monitor/`）

5 个 ECharts 图表：
- [ ] Token 消耗趋势（30 天折线，按 agent 分组）
- [ ] Agent 操作频率热力图（按小时）
- [ ] 模型使用分布饼图
- [ ] 平均延迟趋势折线图
- [ ] Agent 活跃度排行柱状图

### 6.12 训练数据导出（`src/pages/Export/`）

- [ ] 过滤表单（时间范围、Agent、Request Type）
- [ ] 预估数据量展示（COUNT 查询结果）
- [ ] 导出按钮 → 调用 `/api/audit/export` 流式下载

### 6.13 告警通知（全局组件）

**文件**：`src/components/AlertToast/`

- [ ] 右上角 Toast 展示（Semi Toast 组件）
- [ ] 告警类型对应展示规则（来自设计文档 §6.2）：
  - `crash_loop` → 红色 Toast + 顶部横幅（手动关闭）
  - `oom_expanded` → 橙色 Toast（10s 消失）
  - `heartbeat_timeout` → 橙色 Toast（10s）+ Agent 列表 Phase 变红
- [ ] 聚合去重：同 Agent 同类型 60s 内只展示一次

### 6.14 Frontend Dockerfile + Helm Template

- [ ] `frontend/Dockerfile`：多阶段构建（node:20 build + nginx:alpine serve）
- [ ] nginx.conf：SPA 路由支持（所有路径返回 index.html）
- [ ] 填充 `k8s/helm/templates/frontend-deployment.yaml` + Ingress

---

## 验收标准

- [ ] `npm run build` 无类型错误，无 lint 警告
- [ ] 登录 → Dashboard → 创建 Agent → 看到状态变化全流程可用
- [ ] WebSocket 实时事件在 Dashboard 事件流中出现
- [ ] SOUL.md 编辑器可保存（Monaco 语法高亮正确）
- [ ] 监控图表数据加载正确（ECharts 渲染无报错）
- [ ] 训练数据导出：点击后浏览器下载 `.jsonl` 文件，文件格式正确
- [ ] 告警 Toast 展示正确，60s 去重生效
