# Control Panel Frontend 设计文档

> 关联总览：[系统设计总览](system-design-overview.md)
> 关联：[Control Panel Backend 设计](control-panel-backend.md)

## 一、概述

Control Panel Frontend 是平台的管理界面，基于 **Semi Design（字节跳动）** 组件库构建，使用 React + TypeScript。

访问入口：`https://control.devops.local`

## 二、页面结构

```
Control Panel
├── 登录页 /login
└── 主界面（需认证）
    ├── 概览仪表盘 /dashboard
    ├── Agent 管理 /agents
    │   ├── Agent 列表
    │   └── Agent 详情 /agents/:name
    ├── LLM 配置 /llm
    ├── 审计日志 /audit
    │   ├── Trace 列表
    │   └── Trace 详情 /audit/traces/:id
    ├── 监控图表 /monitor
    └── 训练数据导出 /export
```

## 三、页面设计

### 3.1 概览仪表盘（/dashboard）

实时展示平台整体状态：

| 区域 | 内容 |
|------|------|
| 顶部指标卡 | 运行中 Agent 数、今日 Token 消耗、今日 Gitea 操作次数、告警数量 |
| Agent 状态列表 | 每个 Agent 的当前状态（角色、Phase、最后行动、今日 Token） |
| 实时事件流 | WebSocket 推送的最新事件（滚动展示，最近 50 条） |
| Token 消耗趋势图 | 最近 7 天各 Agent 的 Token 消耗折线图 |

### 3.2 Agent 管理（/agents）

#### Agent 列表页

表格展示所有 Agent，支持按 Role/Phase 过滤：

| 列 | 说明 |
|----|------|
| Name | agent CR 名称 |
| Role | observer / developer / reviewer / sre |
| Phase | Pending / Initializing / Running / Paused / Error（带颜色标签） |
| Model | LLM 模型名称 |
| Last Action | 最近行动描述 |
| Tokens Today | 今日 Token 消耗 |
| Age | 创建时间 |
| Actions | 暂停/恢复、查看详情、删除 |

操作按钮：
- **新建 Agent**：弹出抽屉（Drawer），填写 Agent 配置表单
- **批量暂停/恢复**：选中多个 Agent 批量操作

#### Agent 详情页（/agents/:name）

分 Tab 展示：

| Tab | 内容 |
|-----|------|
| 概览 | 基本信息、当前状态、Conditions 列表 |
| 配置 | SOUL.md 编辑器（Monaco Editor）、SKILL 配置、LLM 配置、Cron 配置 |
| 审计日志 | 该 Agent 的最近 Trace 列表（嵌入过滤后的 Trace 表格） |
| 实时日志 | WebSocket 推送该 Agent 的实时事件流 |
| Pod 日志 | 调用 `/api/agents/:name/logs` 展示 Pod stdout |

### 3.3 LLM 配置（/llm）

展示当前 LiteLLM 中配置的所有模型：

| 列 | 说明 |
|----|------|
| Model Name | 模型别名 |
| Provider | openai / anthropic / azure / 自定义 |
| RPM / TPM | 速率限制 |
| Status | 可用 / 不可用 |
| Actions | 编辑、删除 |

操作：
- **新增模型**：弹出 Modal，填写 LiteLLM 模型配置（API Key、Base URL、RPM/TPM 等）
- **测试连通性**：调用 `/api/llm/health` 检查模型是否可达

### 3.4 审计日志（/audit）

#### Trace 列表

支持多维度过滤 + 分页：

过滤条件：
- Agent（下拉多选）
- Request Type（llm_inference / gitea_api / git_http / heartbeat）
- 时间范围（日期选择器）
- Status Code（200 / 非 2xx）

表格列：
- Trace ID（可点击查看详情）
- Agent
- Request Type
- Method + Path
- Status Code
- Model（LLM 请求专有）
- Tokens（In / Out）
- Latency
- Created At

#### Trace 详情

展示单条 trace 的完整信息（Request/Response 的 JSON 预览）。

### 3.5 监控图表（/monitor）

基于 ClickHouse 聚合数据的分析图表：

| 图表 | 数据源 | 说明 |
|------|--------|------|
| Token 消耗趋势 | `token_usage_daily` | 折线图，按 agent 分组，最近 30 天 |
| Agent 操作频率 | `agent_operations_hourly` | 热力图，展示每小时 Gitea 操作次数 |
| 模型使用分布 | `token_usage_daily` | 饼图，各模型 Token 占比 |
| 平均延迟趋势 | `token_usage_daily` | 折线图，LLM 请求平均延迟 |
| Agent 活跃度排行 | `audit.traces` | 柱状图，今日各 Agent Token + 请求数 |

### 3.6 训练数据导出（/export）

面向 AI 训练的数据导出功能：

- 过滤条件：时间范围、Agent、Request Type
- 导出格式：JSONL（每行一条 `{"messages":..., "response":..., "tools":...}`）
- 导出方式：调用 `/api/audit/export` 流式下载
- 预估数据量展示（导出前先 COUNT 查询）

## 四、实时事件处理

Frontend 通过 WebSocket 连接 `WS /api/ws/events` 接收实时事件，处理逻辑：

```
WebSocket 消息
  │
  ├─ event_type = "gateway.llm_inference"
  │   └─ 更新 Agent 列表中对应 Agent 的 Tokens Today
  │
  ├─ event_type = "operator.agent_alert"
  │   └─ 显示 Toast 告警通知
  │   └─ 更新 Agent Phase 显示
  │
  ├─ event_type = "gitea.*"
  │   └─ 更新实时事件流滚动列表
  │
  └─ 连接断开 → 3 秒后自动重连（最多 5 次，指数退避）
```

## 五、技术栈

| 层次 | 选型 |
|------|------|
| 框架 | React 18 + TypeScript |
| 组件库 | Semi Design |
| 状态管理 | Zustand |
| 路由 | React Router v6 |
| 代码编辑器 | Monaco Editor（SOUL.md / SKILL.md 编辑） |
| 图表 | ECharts（通过 echarts-for-react） |
| HTTP 客户端 | Axios |
| WebSocket | 原生 WebSocket + 自定义 hook |
| 构建工具 | Vite |

## 六、补充设计

### 6.1 Agent 配置表单字段设计

新建/编辑 Agent 的 Drawer 表单，字段如下：

| 字段 | 组件 | 说明 |
|------|------|------|
| Name | Input | agent CR 名称，创建后不可修改 |
| Role | Select | observer / developer / reviewer / sre |
| LLM Model | Select | 从 `/api/llm/models` 动态加载可用模型列表 |
| LLM Temperature | Slider (0-2, step 0.1) | 默认 0.7 |
| LLM Max Tokens | InputNumber | 默认 4096 |
| Cron Schedule | Input | cron 表达式，右侧显示"下次触发时间"预览 |
| Cron Prompt | Textarea | 每次 Cron 触发时的 prompt |
| CPU Request / Limit | Input | 如 "100m" / "500m" |
| Memory Request / Limit | Input | 如 "256Mi" / "1Gi" |
| Workspace Size | Input | PVC 大小，如 "5Gi" |
| Gitea Repo | Input | org/repo 格式 |
| Gitea Permissions | CheckboxGroup | read / write / review / merge |

**SOUL.md 编辑**：独立 Tab，使用 Monaco Editor（markdown 语法高亮）。编辑后点击"保存"触发 `PUT /api/agents/:name`，Operator 自动滚动更新 Pod。顶部显示提示："修改 SOUL.md 将触发 Agent Pod 重启"。

### 6.2 告警通知规则

**优先级与展示：**

| alert_type | 展示方式 | 持续时间 |
|-----------|---------|---------|
| `agent.crash_loop` | 右上角红色 Toast + 顶部横幅 | 横幅持续到手动关闭 |
| `agent.oom_expanded` | 右上角橙色 Toast | 10 秒自动消失 |
| `agent.heartbeat_timeout` | 右上角橙色 Toast + Agent 列表 Phase 变红 | Toast 10 秒消失 |
| 其他 | 右上角灰色 Toast | 5 秒自动消失 |

**聚合规则（防告警轰炸）：**
- 同一 Agent 同类型告警，60 秒内只展示一次 Toast（后续静默）
- 横幅只保留最新一条（新告警替换旧横幅）
- 实时事件流滚动列表不去重，完整展示所有事件

### 6.3 权限控制（post-MVP）

MVP 阶段所有登录用户均为完整管理员权限。

未来扩展只读角色时，以下操作需要完整权限，只读用户仅可查看：
- 创建/删除/暂停/恢复 Agent
- 修改 SOUL.md / LLM 配置
- 新增/删除 LLM 模型
- 导出训练数据

### 6.4 移动端

当前优先 PC 管理界面（1280px 以上宽度）。Semi Design 组件默认不响应式，移动端适配推迟到 post-MVP。

## 七、待细化

（所有待细化项已解决）
