# Operator 设计文档

> 关联总览：[系统设计总览](system-design-overview.md)

## 一、职责边界

Operator (`devops-operator`) 是平台的 K8S 控制面核心，负责将 `agent` CR 的声明式期望状态转化为实际的 K8S 底层资源。

**该做（✅）**
- Watch `agent` CR，执行 Reconcile 循环
- 创建/更新/删除底层资源：Deployment、ConfigMap、Secret、PVC、ServiceAccount、RoleBinding
- 调用 Gitea API 创建/删除 Agent 对应的 Gitea 用户和 Token
- 更新 `agent` CR 的 `status`（phase、conditions、giteaUser、podName 等）
- 向 `events:k8s` Stream 写入告警事件（CrashLoop、OOMKilled 等）

**不该做（❌）**
- 不直接查询或写入 ClickHouse（数据层访问统一由 Backend 负责）
- 不处理 Gitea Webhook
- 不做场景调度或任务分发
- 不管理 LiteLLM 模型配置

## 二、Reconcile 流程

完整流程见 [系统设计总览 §6.1](system-design-overview.md#61-reconcile-流程)。

### 关键设计决策

| 变更类型 | Reconcile 行为 | 是否重启 Pod |
|----------|---------------|-------------|
| `spec.soul` / Role ConfigMap 变更 | 更新 ConfigMap（SOUL.md / SKILL.md），触发 Pod 滚动重启 | **是**（OpenClaw 在启动时加载，需重启生效） |
| `spec.cron.schedule` | 更新 ConfigMap（cron 配置），触发 Pod 滚动重启 | **是**（Cron 在 OpenClaw 启动时注册） |
| `spec.llm.model` | 更新 ConfigMap（openclaw.json 中 model 字段），触发 Pod 滚动重启 | **是**（模型配置在启动时加载） |
| `spec.resources` | 更新 Deployment 资源 limits/requests | 否（K8S 原地更新） |
| `spec.paused = true` | Scale Deployment replicas = 0 | 否（优雅停机） |
| `spec.paused = false` | Scale Deployment replicas = 1 | 否（重建 Pod，OpenClaw 从 PVC Workspace 恢复） |
| agent 删除 | 清理所有关联资源 + 删除 Gitea 用户 | — |

### Finalizer 机制

Operator 在 agent CR 上设置 Finalizer `aidevops.io/cleanup`，确保删除时能完成 Gitea 用户清理：

```
CR 删除请求
  │
  ├─ Finalizer 未清除 → Operator 执行清理逻辑
  │   ├─ 删除 Deployment / ConfigMap / Secret / PVC / RBAC
  │   └─ 调用 Gitea API 删除用户
  │
  └─ 移除 Finalizer → CR 真正删除
```

## 三、Agent 初始化资源创建顺序

Operator Reconcile 新 Agent 时，按以下顺序创建资源（每步失败则更新 status.conditions 并 requeue）：

```
步骤 1: 创建 Gitea 用户
  POST /api/v1/admin/users  →  存入 Secret: agent-{name}-gitea-token

步骤 2: 签发 Gateway JWT Token
  生成 JWT（HS256，使用 agent-gateway-jwt-secret）
  →  存入 Secret: agent-{name}-jwt-token
  （步骤 1 完成后立即执行，不依赖其他资源）

步骤 3: 读取 Role ConfigMap
  读取 role-{spec.role} ConfigMap，解析 well-known 文件：
  - SOUL.md / soul.md → roleData.Soul
  - PROMPT.md / prompt.md → roleData.Prompt
  - AGENTS.md / agents.md → roleData.AgentsMD
  - skills__* → roleData.Skills
  - 其他文件 → roleData.ExtraFiles
  ConfigMap 不存在时优雅降级（继续使用 spec 字段和模板）

步骤 4: 创建主 ConfigMap（agent-{name}-config）
  写入 openclaw.json / SOUL.md / AGENTS.md / cron-config.json
  优先级链：
  - SOUL.md: roleData > spec.soul > 空
  - AGENTS.md: roleData > buildAgentsMD() 模板
  - cron prompt: spec.cron.prompt > roleData.Prompt > 空

步骤 5: 创建 Skills ConfigMap（agent-{name}-skills）
  内置 skills（按 spec.role 选择）+ Role ConfigMap 中的自定义 skills
  合并规则：内置 skill 不被覆盖

步骤 6: 创建 Role Files ConfigMap（agent-{name}-role-files）
  仅当 roleData.ExtraFiles 非空时创建
  挂载到 /agent/role/，entrypoint.sh 复制到 ~/.openclaw/

步骤 7: 创建 PVC
  agent-{name}-workspace（spec.memory.storageSize，默认 1Gi）

步骤 8: 创建 ServiceAccount + RoleBinding
  observer/developer/reviewer → agent-readonly-role
  sre → app-staging RoleBinding（额外步骤）

步骤 9: 创建 Deployment
  引用以上所有 Secret / ConfigMap / PVC
  如果 roleData.ExtraFiles 非空，额外挂载 role-files volume

步骤 10: 更新 status.phase = Running（Pod Ready 后）
```

### JWT Token 签发细节

```go
// Operator 签发 JWT 的逻辑（伪代码）
claims := jwt.MapClaims{
    "sub":        agentID,          // "agent-developer-1"
    "agent_id":   agentID,
    "agent_role": agentRole,        // "developer"
    "iat":        time.Now().Unix(),
    "exp":        0,                // 永不过期
}
token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
signed, _ := token.SignedString([]byte(jwtSigningSecret))

// 存入 Secret
secret := &corev1.Secret{
    ObjectMeta: metav1.ObjectMeta{
        Name:      fmt.Sprintf("agent-%s-jwt-token", agentName),
        Namespace: "agents",
        OwnerReferences: []metav1.OwnerReference{agentCROwnerRef},
    },
    StringData: map[string]string{
        "token": signed,
    },
}
```

> `jwtSigningSecret` 从 `agent-gateway-jwt-secret` Secret 读取（Operator 启动时加载，不每次 Reconcile 重新读取）。

### Gitea 用户管理

1. Operator 调用 Gitea REST API (`POST /api/v1/admin/users`) 创建用户
   - 用户名：`agent-{spec.gitea.username}` 或默认 `agent-{metadata.name}`
   - Email：`{username}@agents.devops.local`
2. 调用 Gitea API (`POST /api/v1/users/{username}/tokens`) 生成 Token
3. 将 Token 存入 K8S Secret（`agent-{name}-gitea-token`），同一 namespace
4. 更新 `status.giteaUser.created = true`，`status.giteaUser.tokenSecretRef = {secret-name}`

### Gitea 权限映射

根据 `spec.gitea.permissions` 配置对应的 Gitea 权限：

| 权限值 | Gitea 对应权限 | 适用角色 |
|--------|--------------|---------|
| `read` | 只读仓库 | Observer |
| `write` | 推送代码、创建 Issue/PR | Developer |
| `review` | PR Code Review | Reviewer |
| `merge` | 合并 PR | Reviewer |
| `admin` | 仓库管理员 | （特殊场景） |

### Secret 挂载路径（两个 Token）

| Secret | 挂载路径 | 用途 |
|--------|---------|------|
| `agent-{name}-gitea-token` | `/agent/secrets/gitea-token` | Gitea API 认证 |
| `agent-{name}-jwt-token` | `/agent/secrets/jwt-token` | Gateway 请求认证 |

两个 Secret 均以文件形式挂载（`subPath`），不通过环境变量传递，避免 `kubectl describe pod` 泄漏。

## 四、生成的 K8S 资源清单

每个 agent CR 对应以下 K8S 资源（所有资源均带 `ownerReference` 指向 agent CR）：

```
agent CR: developer-1
├── Deployment: agent-developer-1
│   └── Pod template 使用以下 volumes
│
├── ConfigMap: agent-developer-1-config
│   ├── openclaw.json   ← OpenClaw 主配置（model、gateway URL 等）
│   ├── SOUL.md         ← Role ConfigMap > spec.soul > 空
│   ├── AGENTS.md       ← Role ConfigMap > 角色模板
│   └── cron-config.json ← spec.cron + Role PROMPT.md fallback
│
├── ConfigMap: agent-developer-1-skills
│   ├── gitea-api/SKILL.md    ← 所有角色均有（内置）
│   ├── git-ops/SKILL.md      ← developer / sre 角色（内置）
│   ├── kubectl-ops/SKILL.md  ← 仅 sre 角色（内置）
│   └── {custom}/SKILL.md     ← Role ConfigMap 中的自定义 skills（不覆盖内置）
│
├── ConfigMap: agent-developer-1-role-files（可选）
│   └── {filename}     ← Role ConfigMap 中的非 well-known 文件
│                        挂载到 /agent/role/，entrypoint.sh 复制到 ~/.openclaw/
│
├── Secret: agent-developer-1-gitea-token
│   └── token: <gitea-token>
│
├── Secret: agent-developer-1-jwt-token
│   └── token: <gateway-jwt-token>  ← Operator 签发，OpenClaw 启动时注入
│
├── PersistentVolumeClaim: agent-developer-1-workspace
│   └── 挂载到 /home/node/.openclaw/workspace/（记忆 + 代码工作目录）
│   └── spec.memory.storageSize（默认 1Gi）
│   └── accessModes: [ReadWriteOnce]
│
├── ServiceAccount: agent-developer-1
└── RoleBinding: agent-developer-1
    └── 绑定到 ClusterRole: agent-{role}-role
        （按 spec.role 选择对应预设 ClusterRole）
```

**ConfigMap 挂载到 Pod 的路径映射：**

| ConfigMap Key | 挂载路径 |
|--------------|---------|
| `openclaw.json` | `/agent/config/openclaw.json` → `~/.openclaw/openclaw.json` |
| `SOUL.md` | `/agent/config/SOUL.md` → `~/.openclaw/SOUL.md` |
| `AGENTS.md` | `/agent/config/AGENTS.md` → `~/.openclaw/AGENTS.md` |
| `cron-config.json` | `/agent/config/cron-config.json` → `~/.openclaw/cron-config.json` |
| `gitea-api/SKILL.md` | `/agent/skills/gitea-api/SKILL.md` → `~/.openclaw/skills/gitea-api/SKILL.md` |
| `git-ops/SKILL.md` | `/agent/skills/git-ops/SKILL.md` → `~/.openclaw/skills/git-ops/SKILL.md` |
| `kubectl-ops/SKILL.md` | `/agent/skills/kubectl-ops/SKILL.md` → `~/.openclaw/skills/kubectl-ops/SKILL.md` |
| Role extra files | `/agent/role/*` → `~/.openclaw/*`（由 entrypoint.sh 复制） |

> 注：ConfigMap 挂载到 `/agent/config` 和 `/agent/skills` 为只读卷，entrypoint.sh 在启动时复制到可写的 `~/.openclaw/` 目录。

### RBAC ClusterRole 预设

| Agent Role | ClusterRole | 权限范围 |
|-----------|-------------|---------|
| observer | `agent-observer-role` | 只读（不操作任何资源） |
| developer | `agent-developer-role` | 只读（不操作任何资源） |
| reviewer | `agent-reviewer-role` | 只读（不操作任何资源） |
| sre | `agent-sre-role` | app-staging namespace 内的 Deployment/Service CRUD |
| custom | 自定义（`spec.kubernetes.rbacRole`） | 用户指定 |

> Observer/Developer/Reviewer 无需 K8S 权限（通过 Gateway 操作 Gitea），SRE 需要操作 app-staging。

## 五、告警事件格式

Operator 在异常情况下向 `events:k8s` Redis Stream 写入事件：

```json
{
  "event_id": "uuid-v4",
  "event_type": "operator.agent_alert",
  "timestamp": "2026-03-11T13:00:00.000Z",
  "agent_id": "agent-developer-1",
  "agent_role": "developer",
  "payload": {
    "alert_type": "agent.crash_loop",
    "restart_count": 5,
    "message": "Pod has restarted 5 times, marking as Error"
  }
}
```

告警类型见 [系统设计总览 §10](system-design-overview.md#十agent-健康检查与异常恢复)。

## 六、与其他组件的接口

| 调用方向 | 接口 | 说明 |
|---------|------|------|
| Operator → K8S API | `client-go` Watch/Apply | 管理所有底层资源 |
| Operator → Gitea API | `POST /api/v1/admin/users` 等 | 用 admin token（存于 Operator Deployment 的 Secret） |
| Operator → Redis | `XADD events:k8s` | 写入告警事件 |
| Backend → K8S API | agent CR CRUD | Backend 通过 K8S API 操作 CR，Operator 响应 CR 变化 |

## 七、Operator RBAC 权限

### 7.1 Operator ServiceAccount

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  name: devops-operator
  namespace: control-plane
```

### 7.2 Operator ClusterRole

Operator 是 cluster-scoped controller（需要跨 namespace watch/manage 资源），使用 ClusterRole：

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: devops-operator-role
rules:

# ① 管理 agent CRD（核心权限）
- apiGroups: ["aidevops.io"]
  resources: ["agents"]
  verbs: ["get", "list", "watch", "update", "patch"]
- apiGroups: ["aidevops.io"]
  resources: ["agents/status"]
  verbs: ["get", "update", "patch"]
- apiGroups: ["aidevops.io"]
  resources: ["agents/finalizers"]
  verbs: ["update"]

# ② 管理 agents namespace 内的核心资源
- apiGroups: ["apps"]
  resources: ["deployments"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
- apiGroups: [""]
  resources: ["configmaps", "secrets", "serviceaccounts", "persistentvolumeclaims"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]

# ③ 管理 agents namespace 内的 RBAC（为 Agent Pod 绑定角色）
- apiGroups: ["rbac.authorization.k8s.io"]
  resources: ["rolebindings"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]

# ④ 读取 Pod 状态（用于心跳检测和 CrashLoop 告警）
- apiGroups: [""]
  resources: ["pods", "pods/log"]
  verbs: ["get", "list", "watch"]

# ⑤ 写 K8S Events（controller 标准做法，便于 kubectl describe 查看）
- apiGroups: [""]
  resources: ["events"]
  verbs: ["create", "patch"]

# ⑥ Leader Election（多副本部署时协调主副本）
- apiGroups: ["coordination.k8s.io"]
  resources: ["leases"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
```

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: devops-operator-rolebinding
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: devops-operator-role
subjects:
- kind: ServiceAccount
  name: devops-operator
  namespace: control-plane
```

### 7.3 权限设计说明

**为什么用 ClusterRole 而非 Role：**

Agent CRD 是 Namespaced scope（在 `agents` namespace 下），但 Operator 需要 Watch 整个集群内的 CR 变化（`list` + `watch` 需要 cluster-level 权限才能高效工作），因此用 ClusterRole + ClusterRoleBinding。

**Operator 不需要的权限（明确排除）：**

| 资源 | 原因 |
|------|------|
| `deployments` in `app-staging` | SRE Agent 通过自身 kubeconfig 操作，Operator 不介入 |
| `nodes`, `namespaces` | Operator 不做集群级别运维 |
| `clusterroles`, `clusterrolebindings` | Agent 的 ClusterRole 预先部署，Operator 只创建 RoleBinding |
| ClickHouse / Redis 外部服务 | 通过应用层连接，不涉及 K8S RBAC |

### 7.4 Agent ClusterRole 预设（只读版本）

Observer/Developer/Reviewer 的 ClusterRole（实际只需要获取自身状态，无额外 K8S 权限）：

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: agent-readonly-role
rules:
# Agent 只需要读取自身所在 namespace 的基本信息（用于自诊断）
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get"]
  # ResourceNames 限制只能读取自身 Pod（通过 Downward API 注入 POD_NAME）
```

### 7.5 SRE Agent Role（app-staging 操作权限）

SRE Agent 的权限通过独立 Role + RoleBinding 授权，**限定在 `app-staging` namespace**：

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: sre-agent-role
  namespace: app-staging   # ← 仅限 app-staging，不用 ClusterRole
rules:
- apiGroups: ["apps"]
  resources: ["deployments", "replicasets", "statefulsets", "daemonsets"]
  verbs: ["get", "list", "watch", "create", "update", "patch"]
- apiGroups: [""]
  resources: ["services", "configmaps", "pods", "pods/log"]
  verbs: ["get", "list", "watch"]
- apiGroups: [""]
  resources: ["pods/exec"]
  verbs: ["create"]   # 允许 kubectl exec 排查问题
- apiGroups: ["networking.k8s.io"]
  resources: ["ingresses"]
  verbs: ["get", "list", "watch", "update", "patch"]
```

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: agent-sre-1-app-staging
  namespace: app-staging
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: sre-agent-role
subjects:
- kind: ServiceAccount
  name: agent-sre-1        # ← Operator 在 agents namespace 创建的 SA
  namespace: agents
```

> Operator Reconcile SRE Agent 时，额外在 `app-staging` namespace 创建此 RoleBinding（需要 ③ 中的 rolebindings create 权限，但当前 ClusterRole 只授权了 `agents` namespace，需补充）。

### 7.6 补丁：跨 Namespace RoleBinding 创建

Operator 需要在 `app-staging` namespace 创建 SRE Agent 的 RoleBinding，需要额外 Role：

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: devops-operator-app-staging
  namespace: app-staging
rules:
- apiGroups: ["rbac.authorization.k8s.io"]
  resources: ["rolebindings"]
  verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
---
apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: devops-operator-app-staging
  namespace: app-staging
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: devops-operator-app-staging
subjects:
- kind: ServiceAccount
  name: devops-operator
  namespace: control-plane
```

### 7.7 Gitea Admin Token 注入

Operator Pod 通过 Secret 引用获取 Gitea Admin Token：

```yaml
# 手动创建（首次部署时执行一次）
kubectl create secret generic gitea-admin-token \
  --from-literal=token=<gitea-admin-token> \
  -n control-plane

# Operator Deployment 中引用
env:
- name: GITEA_ADMIN_TOKEN
  valueFrom:
    secretKeyRef:
      name: gitea-admin-token
      key: token
```

> **安全边界**：此 Secret 只存在于 `control-plane` namespace，且只有 Operator Pod 的 ServiceAccount 可以读取（通过 NetworkPolicy + RBAC 双重限制）。

## 八、补充设计

### 8.1 OOMKilled 自动扩容策略

Operator Watch Pod Events，检测到 `OOMKilled` 时自动调整 memory limit：

```
OOMKilled 事件
  │
  ├─ 读取当前 spec.resources.limits.memory（如 "512Mi"）
  ├─ 扩容至 1.5x（512Mi → 768Mi），向上取整到 128Mi 边界 → 896Mi → 取 768Mi
  ├─ 上限：max(spec.resources.limits.memory × 2, 2Gi)，不超过节点可用内存
  ├─ 更新 Deployment resources（触发 Pod 重建）
  ├─ 向 events:k8s 写入 operator.agent_alert（alert_type: agent.oom_expanded）
  └─ 连续 OOMKilled 3 次仍失败 → 不再扩容，设置 status.phase = Error，写入告警
```

扩容倍数：每次 **1.5x**，上限 **2x 初始值或 2Gi**（取较大值）。超过上限后不再扩容，改为报警。

### 8.2 Leader Election 配置

多副本 Operator 使用 `controller-runtime` 内置 Leader Election，基于 K8S Lease 资源：

```go
mgr, err := ctrl.NewManager(cfg, ctrl.Options{
    LeaderElection:          true,
    LeaderElectionID:        "devops-operator-leader",
    LeaderElectionNamespace: "control-plane",    // Lease 资源存放位置
    LeaseDuration:           15 * time.Second,
    RenewDeadline:           10 * time.Second,
    RetryPeriod:             2 * time.Second,
})
```

仅 Leader 副本执行 Reconcile，Follower 副本处于热备状态。Leader 失联后 15 秒内新 Leader 选出。

MVP 阶段单副本部署，Leader Election 代码预留但 `replicas: 1`。

## 九、待细化

（所有待细化项已解决）
