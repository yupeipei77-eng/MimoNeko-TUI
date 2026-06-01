# MioNeko Security

## 概述

MioNeko 实现了多层安全机制来保护用户数据和代码安全。

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
import "github.com/mimoneko/mimoneko/internal/security"

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
import "github.com/mimoneko/mimoneko/internal/security"

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
- 不会让 MioNeko 自己被误伤锁死

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
