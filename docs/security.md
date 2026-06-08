# MimoNeko Security

## 概述

MimoNeko 实现了多层安全机制来保护用户数据和代码安全。

## 安全层级

### 1. Secret Redaction Layer (Phase 4.3)

**状态**: ✅ 已实现

**功能**: 自动脱敏敏感信息，防止泄露到日志或输出中。

**覆盖范围**:
- OpenAI API Key (`sk-xxx`)
- MiMo API Key (`tp-cxxx`)
- Bearer Token
- Authorization Header
- Cookie Header
- JWT Token
- Generic Secrets (`api_key=xxx`, `token=xxx`)

**使用方式**:
```go
import "github.com/yupeipei77-eng/MimoNeko-TUI/internal/security"

// 脱敏文本
safe := security.SanitizeText("Using key sk-abcdefghijklmnopqrstuvwxyz")
// 输出: "Using key sk-****wxyz"

// 脱敏 Map（深拷贝，不修改输入）
safeMap := security.SanitizeMap(inputMap)

// 脱敏 Event Map（递归处理嵌套结构）
safeEvent := security.SanitizeEventMap(eventMap)
```

**重要说明**:
- 这是 **display-layer redaction**，不是 encryption
- 不修改真实数据，只修改显示内容
- 内部真实数据仍保留完整

---

### 2. Path Sandbox Detection (Phase 4.4)

**状态**: ✅ 已实现（仅检测）

**功能**: 检测敏感路径，用于安全审计和日志记录。

**重要说明**:
- ⚠️ 当前阶段只是 **detection**，不是 **enforcement**
- 不会阻断任何工具执行
- 不会拒绝任何文件访问
- 只记录检测结果用于审计

**检测规则**:

| 规则 | 路径示例 | 严重级别 |
|------|----------|----------|
| git-directory | `.git`, `.git/config` | critical |
| env-file | `.env`, `.env.local` | critical |
| ssh-private-key | `id_rsa`, `id_ed25519` | critical |
| credentials-file | `credentials`, `credentials.json` | warning |
| token-file | `token`, `access_token` | warning |
| secrets-file | `secrets`, `secrets.yaml` | warning |
| key-file | `*.pem`, `*.key` | warning |
| path-traversal | `../secrets` | warning |

**使用方式**:
```go
import "github.com/yupeipei77-eng/MimoNeko-TUI/internal/security"

// 检查路径
violations := security.ValidatePath(".git/config")
// 返回: [{Path:".git/config", Rule:"git-directory", Severity:"critical", Candidate:true}]

// 快速检查
isSensitive := security.IsSensitivePath(".env")  // true
isCritical := security.IsCriticalPath("id_rsa")  // true

// 获取摘要
summary := security.GetViolationSummary(violations)  // "blocked_candidate"
```

**CLI 命令**:
```bash
mimoneko sandbox check .git/config
# 输出:
# blocked_candidate
#   - rule: git-directory
#     severity: critical
#     candidate: true

mimoneko sandbox check README.md
# 输出:
# allowed
```

---

### 3. Sandbox Audit Candidate Events (Phase 4.4.5)

**状态**: ✅ 已实现（仅记录）

**功能**: 当工具参数中包含可识别的 path 字段时，自动检测敏感路径并记录审计事件。

**重要说明**:
- ⚠️ 当前阶段只是 **observe-only**，不是 **enforcement**
- 不会阻断工具执行
- 不会拒绝路径访问
- 只记录检测结果用于审计

**触发条件**:
当工具参数中包含以下 key 时触发检测：
- `path`, `file`, `filename`, `filepath`
- `dir`, `directory`, `cwd`, `target`

**事件类型**: `path.violation_candidate`

**事件字段**:
```json
{
  "type": "path.violation_candidate",
  "source": "sandbox",
  "tool_name": "file_read",
  "metadata": {
    "path": ".git/config",
    "rule": "git-directory",
    "severity": "critical",
    "candidate": "true",
    "arg_key": "path"
  }
}
```

**查看事件**:
```bash
neko events tools
# 输出包含 path.violation_candidate 事件
```

**使用场景**:
- 审计工具访问了哪些敏感路径
- 识别潜在的安全风险
- 为未来的 enforcement 策略收集数据

---

### 4. Tool Execution Security (已存在)

**功能**: 工具执行时的安全检查。

**机制**:
- 工具需要注册才能执行
- 高风险工具需要审批
- 执行结果经过脱敏处理
- 审计日志记录所有执行

---

### 4. Worktree Isolation (已存在)

**功能**: 使用 Git Worktree 隔离代码修改。

**机制**:
- 代码修改在独立的 worktree 中进行
- 主分支永远安全
- 补丁需要显式应用
- 默认 dry-run 模式

---

## 安全原则

### 最小权限原则

- 默认 dry-run，不修改文件
- 高风险操作需要显式确认
- 工具执行受路径限制

### 纵深防御

- 多层安全检查
- 每层独立运作
- 单点失败不影响整体安全

### 透明审计

- 所有操作都有审计日志
- 敏感信息自动脱敏
- 检测结果可追溯

---

### 5. Security Enforcement (Phase 4.5)

**状态**: ✅ 已实现

**功能**: 可配置的安全拦截策略。

**重要说明**:
- 默认模式是 `warn` - 检测并警告，不阻断
- 需要显式设置 `MIMONEKO_SECURITY_MODE=enforce` 才会阻断
- 不会让 MimoNeko 自己被误伤锁死

**三种模式**:

| 模式 | 说明 | 行为 |
|------|------|------|
| `off` | 关闭 | 不拦截，只记录 audit candidate |
| `warn` | 警告（默认） | 不拦截，输出 warning，emit security.warning |
| `enforce` | 强制 | 阻断 critical path，high risk tool 需要 approval |

**配置方式**:
```bash
# 环境变量
export MIMONEKO_SECURITY_MODE=warn  # 默认
export MIMONEKO_SECURITY_MODE=off   # 关闭
export MIMONEKO_SECURITY_MODE=enforce  # 强制
```

**执行策略**:

| 风险级别 | off | warn | enforce |
|----------|-----|------|---------|
| low | 允许 | 允许 | 允许 |
| medium | 允许 | 允许 | 允许（需 approval 则拒绝） |
| high | 允许 | 允许 | 需要 approval |
| critical | 允许 | 允许 | 拒绝 |

**路径策略**:

| 违规级别 | off | warn | enforce |
|----------|-----|------|---------|
| critical | 允许，记录 candidate | 允许，警告 | 阻断 |
| warning | 允许，记录 candidate | 允许，警告 | 允许，警告 |

**CLI 命令**:
```bash
# 查看安全状态
mimoneko security status

# 检查路径安全
mimoneko security check .git/config
```

**事件类型**:
- `security.warning` - 安全警告
- `path.blocked` - 路径被阻断
- `tool.denied` - 工具被拒绝
- `tool.approval_required` - 工具需要审批

**Approval 说明**:
当前阶段不实现交互式 approval 流程，只返回 `approval_required` 错误状态。后续版本将实现 `approve` 命令。

---

### Security Matrix

#### Tool Risk Level Matrix

| Risk Level | off | warn | enforce |
|------------|-----|------|---------|
| **low** | ✅ Allow | ✅ Allow | ✅ Allow |
| **medium** | ✅ Allow | ✅ Allow | ✅ Allow (❌ Deny if requires_approval) |
| **high** | ✅ Allow | ✅ Allow | ⚠️ Approval Required |
| **critical** | ✅ Allow | ✅ Allow | ❌ Deny |

#### Path Severity Matrix

| Path Severity | off | warn | enforce |
|---------------|-----|------|---------|
| **critical** (.git, .env, id_rsa) | ✅ Allow (audit only) | ✅ Allow + ⚠️ Warning | ❌ Block |
| **warning** (credentials, token, secrets) | ✅ Allow (audit only) | ✅ Allow + ⚠️ Warning | ✅ Allow + ⚠️ Warning |
| **info** (.npmrc, .pypirc) | ✅ Allow | ✅ Allow | ✅ Allow |

#### Event Types Matrix

| Event Type | off | warn | enforce |
|------------|-----|------|---------|
| `tool.called` | ✅ | ✅ | ✅ |
| `tool.completed` | ✅ | ✅ | ✅ |
| `tool.failed` | ✅ | ✅ | ✅ |
| `path.violation_candidate` | ✅ | ✅ | ✅ |
| `security.warning` | ❌ | ✅ | ✅ |
| `path.blocked` | ❌ | ❌ | ✅ |
| `tool.denied` | ❌ | ❌ | ✅ |
| `tool.approval_required` | ❌ | ❌ | ✅ |

#### Security Summary API

```go
summary := security.GetSecuritySummary(toolCount, highRisk, criticalRisk, approvalRequired)

// Returns:
// - TotalRegisteredTools: 10
// - HighRiskTools: ["file_write", "file_patch"]
// - CriticalRiskTools: ["dangerous_tool"]
// - ApprovalRequired: ["file_patch"]
// - BlockedRules: ["git-directory", "env-file", ...]
// - EnforcementMode: "warn"
// - SandboxRulesCount: 15
// - EnforcementEnabled: false
```

---

### 6. Approval Request Model (Phase 5.1)

**状态**: ✅ 已实现（数据模型）

**功能**: Approval Request 的核心数据模型。

**重要说明**:
- ⚠️ 当前只是 **data model**，不是 **interactive approval**
- 不实现交互式审批流程
- 不实现持久化
- 不修改 ToolRuntime 执行行为

**ApprovalStatus**:
| Status | 说明 |
|--------|------|
| `pending` | 等待审批 |
| `approved` | 已批准 |
| `rejected` | 已拒绝 |
| `expired` | 已过期 |

**ApprovalScope**:
| Scope | 说明 |
|-------|------|
| `tool` | 工具执行 |
| `path` | 路径访问 |
| `patch` | 补丁应用 |
| `command` | 命令执行 |

**状态转换**:
```
pending → approved
pending → rejected
pending → expired
```

**API 示例**:
```go
import "github.com/yupeipei77-eng/MimoNeko-TUI/internal/approval"

// 创建请求
req, err := approval.NewRequest(
    "run-123",
    "file_write",
    approval.ScopeTool,
    "high risk tool requires approval",
    "high",
    "",
    "",
    "",
)

// 批准
err = req.Approve("user-1")

// 拒绝
err = req.Reject("user-1")

// 过期
err = req.Expire()

// 检查状态
req.IsPending()  // true if pending
req.IsExpired(time.Now())  // true if expired
```

**使用场景**:
- 为后续 CLI approval 命令做准备
- 为持久化 approval 记录做准备
- 为恢复执行做准备

---

### 7. Approval CLI Stub (Phase 5.2)

**状态**: ✅ 已实现（Stub）

**功能**: Approval CLI 入口和只读/模拟操作。

**重要说明**:
- ⚠️ 当前是 **stub 实现**，不接 Runtime，不持久化
- 使用 in-memory demo store
- 不影响真实 ToolRuntime
- 不影响 Security Enforcement

**CLI 命令**:
```bash
# 列出待审批请求
mimoneko approvals list

# 显示审批请求详情
mimoneko approvals show <id>

# 批准请求
mimoneko approvals approve <id>

# 拒绝请求
mimoneko approvals reject <id>
```

**当前行为**:
| 命令 | 行为 |
|------|------|
| `list` | 输出 `no pending approvals`（无数据时） |
| `show` | 显示请求详情（脱敏） |
| `approve` | 验证状态转换逻辑 |
| `reject` | 验证状态转换逻辑 |

**使用场景**:
- 验证 CLI 入口
- 验证状态转换逻辑
- 为后续持久化做准备

---

### 8. Approval Persistence (Phase 5.3)

**状态**: ✅ 已实现

**功能**: Approval Request 的本地磁盘持久化。

**重要说明**:
- ⚠️ 当前 **不接 ToolRuntime**
- ⚠️ 当前 **不改变 Security Enforcement 行为**
- 真实 approval enforcement 会在后续阶段完成

**存储路径**:
```
.mimoneko/approvals.json
```

**持久化格式**:
- JSON 格式
- 人类可读（带缩进）
- 确定性排序（按 created_at 然后 id）
- 文件权限 0600（Unix）

**CLI 命令**:
```bash
# 列出待审批请求
mimoneko approvals list

# 显示审批请求详情
mimoneko approvals show <id>

# 批准请求（持久化到磁盘）
mimoneko approvals approve <id>

# 拒绝请求（持久化到磁盘）
mimoneko approvals reject <id>
```

**FileStore API**:
```go
store := approval.NewFileStore(".mimoneko/approvals.json")
store.Load()  // 从文件加载
store.Save()  // 保存到文件
store.Add(req)  // 添加并持久化
store.Update(req)  // 更新并持久化
store.Delete(id)  // 删除并持久化
```

**行为**:
- 文件不存在时自动视为空 store
- approve/reject 后自动落盘
- 不损坏已有 approval 文件
- JSON 解析失败返回明确错误

---

### 9. Approval Runtime Integration (Phase 5.4)

**状态**: ✅ 已实现（Dry Run）

**功能**: 将 Approval Request 生成接入 ToolRuntime / Security Enforcement。

**重要说明**:
- ⚠️ 当前阶段只是 **生成 pending approval**
- ⚠️ approve 后 **不会自动 resume**
- resume 留到后续阶段

**触发条件**:
当 `MIMONEKO_SECURITY_MODE=enforce` 时：
- high risk tool 需要 approval
- medium risk 且 requires_approval=true 需要 approval
- critical risk tool 仍然直接 deny，不创建 approval
- critical path 仍然直接 block，不创建 approval
- warning path 只 warning，不创建 approval

**行为**:
1. 创建 ApprovalRequest
2. 写入 `.mimoneko/approvals.json`
3. 返回错误：`approval required: <approval_id>`
4. emit `tool.approval_required`
5. 不执行工具
6. 不自动恢复执行

**去重**:
如果相同 run_id + tool_name + reason + path 已存在 pending request，不重复创建，直接返回已有 approval_id。

**CLI 查看**:
```bash
mimoneko approvals list
mimoneko approvals show <id>
mimoneko approvals approve <id>
mimoneko approvals reject <id>
```

**使用场景**:
- enforce 模式下自动创建 approval 请求
- CLI 可查看和处理 approval 请求
- 为后续 resume 执行做准备

---

### 10. Approval Resume Snapshot (Phase 5.5)

**状态**: ✅ 已实现

**功能**: 为 approval_required 的工具调用保存可恢复快照。

**重要说明**:
- ⚠️ 当前只是 **snapshot**，不是 **resume**
- ⚠️ approve 后 **仍不会自动执行**
- 自动 resume 留给后续阶段

**存储路径**:
```
.mimoneko/approval_snapshots.json
```

**ResumeSnapshot 字段**:
| 字段 | 说明 |
|------|------|
| approval_id | 关联 approval 请求 |
| run_id | 运行 ID |
| tool_name | 工具名 |
| tool_args | 原始工具参数（不显示） |
| risk_level | 风险级别 |
| reason | 原因 |
| path | 路径 |
| command | 命令 |
| created_at | 创建时间 |
| sanitized_preview | 脱敏预览 |

**CLI 命令**:
```bash
# 显示恢复快照（脱敏）
mimoneko approvals snapshot <id>
```

**行为**:
- approval_required 时自动创建 snapshot
- 重复 approval 复用同一 snapshot
- CLI 只显示 sanitized_preview
- 不显示真实 tool_args
- 不执行工具

---

### 11. Approval Resume Preview (Phase 5.5.5)

**状态**: ✅ 已实现

**功能**: 预览批准后将执行的操作。

**重要说明**:
- ⚠️ preview ≠ execution
- ⚠️ 当前阶段 **不会执行任何工具**
- 只展示未来将执行什么

**CLI 命令**:
```bash
# 预览批准后将执行的操作
mimoneko approvals preview <id>
```

**状态要求**:
| 状态 | 行为 |
|------|------|
| pending | 显示 `approval still pending` |
| approved | 允许 preview |
| rejected | 显示 `approval rejected` |
| expired | 显示 `approval expired` |

**输出内容**:
- Approval ID
- Tool Name
- Risk Level
- Status
- Reason
- Path
- Command
- Sanitized Preview

**安全要求**:
- 绝不显示真实 tool_args
- 绝不显示原始 secret
- 必须使用 security.SanitizeOutput()
- 必须使用 security.SanitizeText()

---

### 12. Approval Resume Execution (Phase 5.6)

**状态**: ✅ 已实现

**功能**: 用户手动恢复执行已批准 approval 的工具调用。

**核心原则**:
- approve 只改变状态
- resume 才执行工具
- 不会 approve 后自动执行

**CLI 命令**:
```bash
# 恢复执行已批准的请求
mimoneko approvals resume <id>
```

**执行条件**:
1. approval 存在
2. approval.status == approved
3. snapshot 存在
4. snapshot.approval_id == approval.id
5. tool 仍然存在于 registry
6. 未被 resume 过

**禁止执行**:
| 状态 | 错误 |
|------|------|
| pending | approval still pending |
| rejected | approval rejected |
| expired | approval expired |
| missing snapshot | approval snapshot missing |
| tool missing | tool not found |
| already resumed | approval already resumed |

**安全要求**:
- resume 前必须再次运行 Security Enforcement
- critical path / critical risk tool 仍然必须阻断
- rejected / expired approval 不可执行
- resume 输出必须脱敏
- resume 成功或失败都 emit event

**事件**:
- `approval.resume_started`
- `approval.resume_completed`
- `approval.resume_failed`

**防重复执行**:
- 默认拒绝再次执行
- 错误: `approval already resumed`

---

## 最佳实践

### 对于用户

1. **不要禁用安全检查**: 除非你完全理解风险
2. **定期检查审计日志**: 使用 `mimoneko runs` 查看历史
3. **使用 dry-run 模式**: 先预览再执行
4. **保护 API Key**: 使用 `mimoneko auth login` 配置

### 对于开发者

1. **不要绕过安全层**: 使用统一的 API
2. **不要硬编码密钥**: 使用环境变量或配置文件
3. **测试安全功能**: 确保脱敏和检测正常工作
4. **文档更新**: 安全相关变更需要更新文档

---

## 常见问题

### Q: 为什么检测不阻断？

A: 当前阶段是 detection only，用于收集数据和验证规则。阻断功能将在 Phase 4.5 实现，届时用户可以配置策略。

### Q: 如何查看检测结果？

A: 使用 `mimoneko sandbox check <path>` 命令，或查看审计日志中的 `path.violation_candidate` 事件。

### Q: 如何自定义检测规则？

A: 当前版本不支持自定义规则。未来版本将支持用户配置。

### Q: 脱敏会影响功能吗？

A: 不会。脱敏只影响显示内容，不影响真实数据和工具执行。

---

## 相关文档

- [Phase 4.1 Tool Metadata Registry](./PHASE_4_1_TOOL_METADATA.md)
- [Phase 4.2 Audit Event Foundation](./PHASE_4_2_AUDIT_EVENTS.md)
- [Phase 4.3 Secret Redaction Layer](./PHASE_4_3_SECRET_REDACTION.md)
- [Phase 4.4 Path Sandbox Detection](./PHASE_4_4_PATH_SANDBOX.md)
