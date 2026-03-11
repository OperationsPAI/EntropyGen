# Phase 2: Operator — K8S Controller

> 关联设计：[Operator 设计](../designs/operator.md) | [系统设计总览](../designs/system-design-overview.md)
> 依赖 Phase：Phase 1（CRD types、common 包）
> 可并行：Phase 3 (Gateway)、Phase 4 (Event Collector) 可同步进行

## 目标

实现 K8S Operator，Watch `agent` CR，将声明式期望状态 Reconcile 为底层 K8S 资源（Deployment、ConfigMap、Secret、PVC、RBAC），并管理对应的 Gitea 用户和 JWT Token。

---

## 依赖的外部服务

| 服务 | 用途 |
|------|------|
| K8S API | Watch agent CR，CRUD Deployment/ConfigMap/Secret/PVC/RoleBinding |
| Gitea API | 创建/删除 Agent 用户和 Token（需 `gitea-admin-token` Secret） |
| Redis (`events:k8s`) | 写入告警事件（CrashLoop、OOMKilled） |
| `agent-gateway-jwt-secret` | 签发 Agent JWT Token（HS256） |

---

## 任务拆解

### 2.1 Operator 主程序框架

**文件**：`cmd/operator/main.go`

- [ ] 使用 `controller-runtime` 初始化 Manager
  ```go
  mgr, err := ctrl.NewManager(cfg, ctrl.Options{
      Scheme:                  scheme,
      LeaderElection:          true,
      LeaderElectionID:        "devops-operator-leader",
      LeaderElectionNamespace: "control-plane",
      LeaseDuration:           15 * time.Second,
      RenewDeadline:           10 * time.Second,
      RetryPeriod:             2 * time.Second,
  })
  ```
- [ ] 注册 Agent CRD scheme
- [ ] 注册 AgentReconciler
- [ ] 环境变量读取：`GITEA_URL`、`GITEA_ADMIN_TOKEN`、`JWT_SIGNING_SECRET`、`AGENT_NAMESPACE`

**测试**：Manager 启动不报错，Leader Election 正常（两个实例只有一个 Active）。

### 2.2 Agent Reconciler 主逻辑

**文件**：`internal/operator/controller/agent_controller.go`

- [ ] 实现 `Reconcile(ctx, req)` 方法
- [ ] 读取 agent CR，处理三种情况：
  1. CR 被删除（检查 DeletionTimestamp + Finalizer）→ 执行清理
  2. CR 新创建 → 按顺序创建所有资源（7 步）
  3. CR spec 变更 → 差异化更新（根据变更类型决定是否重启 Pod）
- [ ] 设置 Finalizer `aidevops.io/cleanup`
- [ ] 错误时更新 `status.conditions`，requeue

**变更类型与响应**（来自设计文档 §二）：

| spec 变更 | 行为 | 重启 Pod |
|-----------|------|---------|
| soul/skills/cron | 更新 ConfigMap | **是** |
| llm.model | 更新 ConfigMap (openclaw.json) | **是** |
| resources | 更新 Deployment limits | 否 |
| paused=true | Scale replicas=0 | 否 |
| paused=false | Scale replicas=1 | 否 |

**测试（TDD）**：
- 新 Agent CR → 验证创建了 Deployment、2 个 ConfigMap、2 个 Secret、PVC、SA、RoleBinding
- 修改 soul → 验证 ConfigMap 内容更新，Deployment annotation 变化触发 rolling update
- paused=true → 验证 replicas=0
- 删除 CR → 验证 Finalizer 清理后 CR 真正删除

### 2.3 初始化资源创建（7 步）

**文件**：`internal/operator/reconciler/`

#### 2.3.1 Gitea 用户管理（`gitea_user.go`）

- [ ] `CreateGiteaUser(agent)` → 调用 `POST /api/v1/admin/users`
- [ ] `CreateGiteaToken(username)` → 调用 `POST /api/v1/users/{username}/tokens`
- [ ] `AddRepoCollaborator(username, repo, permission)` → 按 spec.gitea.permissions 映射
- [ ] `DeleteGiteaUser(username)` → 删除时调用
- [ ] 幂等：操作前先查询是否已存在

**测试**：Mock Gitea Server，验证 Admin Token 传递正确，用户名/邮件规则符合预期。

#### 2.3.2 JWT Token 签发（`jwt.go`）

- [ ] `IssueAgentJWT(agentID, agentRole, signingSecret)` → HS256 签发
  ```go
  claims := jwt.MapClaims{
      "sub":        agentID,
      "agent_id":   agentID,
      "agent_role": agentRole,
      "iat":        time.Now().Unix(),
      "exp":        0,  // 永不过期
  }
  ```
- [ ] 将 JWT 写入 K8S Secret `agent-{name}-jwt-token`

**测试**：签发的 JWT 能被 Gateway 的验签逻辑正确验证（claims 提取正确）。

#### 2.3.3 K8S 资源创建（`k8s_resources.go`）

- [ ] `EnsureConfigMap(agent)` → 生成 `openclaw.json`、`SOUL.md`、`AGENTS.md`、`cron-config.json`
- [ ] `EnsureSkillsConfigMap(agent)` → 按 role 选择 SKILL.md 内容注入
- [ ] `EnsurePVC(agent)` → storageSize 来自 `spec.memory.storageSize`（默认 5Gi）
- [ ] `EnsureServiceAccount(agent)`
- [ ] `EnsureRoleBinding(agent)` → observer/developer/reviewer 绑 `agent-readonly-role`，SRE 额外绑 `app-staging`
- [ ] `EnsureDeployment(agent)` → 挂载 ConfigMap/Secret/PVC，注入环境变量，配置 Liveness/Readiness Probe
- [ ] `DeleteAllResources(agent)` → 清理所有资源（Finalizer 触发）

**openclaw.json 生成规则**：
```json
{
  "agent": { "model": "<spec.llm.model>" },
  "providers": {
    "anthropic": {
      "baseURL": "http://agent-gateway.control-plane.svc/v1",
      "apiKey": "__JWT_PLACEHOLDER__"
    }
  },
  "automation": { "webhook": { "enabled": true, "port": 9090 } }
}
```

**Deployment Probe 配置**：
```yaml
livenessProbe:
  httpGet: { path: /healthz, port: 8080 }
  periodSeconds: 30
  failureThreshold: 3
readinessProbe:
  httpGet: { path: /readyz, port: 8080 }
  periodSeconds: 15
  failureThreshold: 2
```

**测试**：
- ConfigMap 内容正确（SOUL.md 来自 spec.soul，openclaw.json model 字段正确）
- SRE Agent RoleBinding 在 app-staging namespace
- Deployment volume mounts 覆盖所有必要文件

### 2.4 Pod 事件监听与告警

**文件**：`internal/operator/controller/agent_controller.go`（补充）

- [ ] Watch Pod Events（`SetupWithManager` 中注册 Pod Owns）
- [ ] `restartCount >= 5` → 写入 `events:k8s` 告警（`agent.crash_loop`），更新 CR `status.phase=Error`
- [ ] `OOMKilled` → 调整 memory limit（1.5x，上限 2x 初始值或 2Gi），重建 Pod，写告警

**OOMKilled 扩容逻辑**：
```
当前 memory limit → × 1.5 → 向上取整到 128Mi 边界 → 不超过 max(2x初始值, 2Gi)
连续 3 次仍 OOMKilled → 不再扩容，设 phase=Error
```

**测试**：
- 模拟 CrashLoopBackOff（restartCount=5）→ 验证告警事件写入 Redis，CR phase=Error
- 模拟 OOMKilled → 验证 memory limit 正确扩容（512Mi → 768Mi）

### 2.5 status 更新

- [ ] `UpdatePhase(phase string)`：Running/Paused/Error/Initializing
- [ ] `UpdateCondition(condType, status, reason, message)`
- [ ] `UpdateGiteaUser(created bool, username, tokenSecretRef string)`
- [ ] `UpdateLastAction(description string)`（Backend 定时检测后调用，此处仅定义字段）

### 2.6 Operator Helm Template

- [ ] 填充 `k8s/helm/templates/operator-deployment.yaml`：
  - ServiceAccount `devops-operator`
  - ClusterRoleBinding
  - Deployment（引用 `gitea-admin-token` 和 `agent-gateway-jwt-secret` Secret）
  - Leader Election Lease 权限

---

## 验收标准

- [ ] `go test ./internal/operator/...` 全部通过（覆盖率 ≥ 80%）
- [ ] 在本地 K8S（kind/k3s）中部署 Operator，apply 一个示例 `observer.yaml`
  - Gitea 用户 `agent-observer-1` 被创建
  - K8S 中出现 Deployment、ConfigMap（含正确 SOUL.md）、Secret、PVC、SA、RoleBinding
  - Agent Pod 启动，Liveness 健康检查通过
- [ ] 修改 CR spec.soul → Pod 自动滚动重启，ConfigMap 内容更新
- [ ] 删除 CR → 所有关联资源和 Gitea 用户被清理
