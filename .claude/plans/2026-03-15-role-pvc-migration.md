# Plan: Role 存储从 ConfigMap 迁移到共享 PVC

> 日期: 2026-03-15
> 设计文档: [Operator](../designs/operator.md)

## 背景

Role 配置存储从 K8S ConfigMap (`role-{name}`) 迁移到共享 RWX PVC (`roles-data`)。
解决 ConfigMap 1MB 大小限制、不支持真实目录结构、不支持二进制文件的问题。

## 实现内容

### Phase 1: Helm — PVC 创建和挂载 ✅
- 新增 `roles-data` PVC (ReadWriteMany, juicefs-sc, 5Gi)
- Backend 挂载到 `/data/roles` (读写)
- Operator 挂载到 `/data/roles` (只读)
- 两者均新增 `ROLES_DATA_PATH` 环境变量

### Phase 2: Backend — 文件系统 RoleClient ✅
- 删除 `role_cm.go`，新建 `role_fs.go`
- 所有 CRUD 操作改为 `os` 文件系统调用
- 每个 role 目录包含 `.metadata.json`（description, created_at, updated_at）
- 移除 `__` 路径编码，文件路径保持原样（`skills/gitea-api/SKILL.md`）
- `BuiltinContentProvider` 接口移除 `SkillKey()` 方法
- API 路由从 `:filename` 改为 `*filepath`（支持嵌套路径）
- Rename 端点改为 `POST /roles/:name/rename-file`（body 中传 old_name, new_name）
- 所有路径参数做 `filepath.Clean` + 路径遍历防护

### Phase 3: Operator — 从 PVC 读取 + 变更检测 ✅
- `parseRoleData()` → `readRoleDataFromDir()`（遍历目录树）
- `fetchRoleData()` 改为读文件系统
- `buildSkillsData()` 内部将 `/` 路径转为 `__` ConfigMap key
- 删除 ConfigMap Watch，新增 `roleWatcher` goroutine（每 10 秒扫描）
- 通过 `source.Channel` + `GenericEvent` 触发 reconcile

### Phase 4: 前端适配 ✅
- `RoleFile.name` 从 `skills__gitea-api__SKILL.md` 变为 `skills/gitea-api/SKILL.md`
- `renameRoleFile()` API 签名变更（传 old_name + new_name）

### Phase 5: 测试 ✅
- `role_fs_test.go`: 11 个测试覆盖所有 CRUD 操作 + 路径安全
- `config_test.go`: 更新为使用 `readRoleDataFromDir()` 和 `/` 路径

### Phase 6: 迁移脚本 ✅
- `scripts/migrate-roles-to-pvc.sh`: 从 ConfigMap 读取数据并写入 PVC 目录

## 架构变更

```
roles-data PVC (ReadWriteMany)
├── developer/
│   ├── .metadata.json
│   ├── SOUL.md
│   ├── PROMPT.md
│   ├── AGENTS.md
│   └── skills/
│       ├── gitea-api/SKILL.md
│       └── git-ops/SKILL.md
└── reviewer/
    ├── .metadata.json
    ├── SOUL.md
    └── ...
```

Agent Pod 不直接挂载 roles PVC。Operator 仍然创建 per-agent ConfigMap，只是数据来源从 Role ConfigMap 变为 PVC 文件系统。
