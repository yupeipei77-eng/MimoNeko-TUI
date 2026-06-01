# MioNeko

MioNeko 是一个本地优先的 AI 编码代理运行时，深度适配 MiMo 模型。

当前版本：`v0.1.1-beta`

## 快速开始

安装完成并重新打开终端后，直接输入：

```bash
mimoneko
```

不需要进入下载目录，不需要手动设置环境变量，不需要安装 Go。

首次运行会自动进入配置向导，保存用户级配置后自动执行 `mimoneko model test`。

## Windows 安装

1. 从 [Releases](https://github.com/yupeipei77-eng/MioNeko/releases) 下载 `mimoneko-windows-amd64.zip`。
2. 解压到任意目录。
3. 在解压目录中运行 `install.ps1`。
4. 重新打开 PowerShell。
5. 输入：

```powershell
mimoneko
```

安装脚本会把 `mimoneko.exe` 复制到：

```text
%LOCALAPPDATA%\MioNeko\bin\mimoneko.exe
```

并把该目录加入用户 PATH。整个过程不需要管理员权限。

如果不熟悉终端，也可以双击 `start-mimoneko.bat` 打开 MioNeko。

## macOS / Linux 安装

1. 从 [Releases](https://github.com/yupeipei77-eng/MioNeko/releases) 下载对应平台的 `.tar.gz`。
2. 解压。
3. 在解压目录中运行：

```bash
./install.sh
```

4. 重新打开终端。
5. 输入：

```bash
mimoneko
```

安装脚本会把 `mimoneko` 复制到：

```text
~/.local/bin/mimoneko
```

如果 `~/.local/bin` 不在 PATH 中，脚本会提示你添加：

```bash
export PATH="$HOME/.local/bin:$PATH"
```

整个过程不需要 `sudo`。

## 首次启动引导

首次输入 `mimoneko` 时，如果还没有用户级模型配置，会看到：

```text
欢迎使用 MioNeko
检测到你还没有配置模型

请选择：
1. MiMo
2. OpenAI-compatible
3. Local
```

默认选择 MiMo。随后依次输入：

```text
API Key
Base URL [https://token-plan-cn.xiaomimimo.com/v1]
Model [mimo-v2.5-pro]
```

配置会保存到用户目录：

```text
Windows: %USERPROFILE%\.mimoneko\config.yaml
macOS/Linux: ~/.mimoneko/config.yaml
```

API Key 不会写入项目目录。

配置成功后可以直接运行：

```bash
mimoneko "修改 README"
```

也可以使用完整命令：

```bash
mimoneko run "修改 README"
```

## 常用命令

```bash
mimoneko                         # 首次配置；已配置时显示帮助
mimoneko "修改 README"           # 等价于 mimoneko run "修改 README"
mimoneko run "修改 README"       # 运行单代理任务
mimoneko auth login              # 重新配置用户级模型
mimoneko auth status             # 查看配置状态
mimoneko auth logout             # 清除用户级配置
mimoneko model test              # 测试模型连接
mimoneko model list              # 查看项目模型配置
mimoneko init                    # 初始化当前项目配置
```

## Agent Workflow Commands

These commands are designed for a Claude Code / OpenCode style review loop. They are safe read-only entry points in this phase.

```bash
neko status                 # Show git branch, clean state, change counts, and latest run status
neko diff                   # Show working tree diff for review
neko diff --staged          # Show staged diff for review
neko plan --goal "..."      # Print a structured plan skeleton without writing files
neko cache stats            # Show prefix fingerprint and cache observability stats
neko tools                  # List tool metadata: risk, approval flag, timeout
neko events tools           # Show recent tool audit events when available
```

Equivalent `mimoneko` form:

```bash
mimoneko neko status
mimoneko neko diff --staged
mimoneko neko plan --goal "Update README"
mimoneko neko cache stats
mimoneko neko tools
mimoneko neko events tools
```

## Multi-Agent Workflow Commands (Phase 6.1+)

These commands provide a skeleton layer for multi-agent workflows. They do NOT call LLMs or modify files.

```bash
mimoneko agents                             # List available agent roles
mimoneko agents plan --goal "修复 README"   # Create workflow skeleton
mimoneko agents plan --goal "优化 README" --llm   # Create plan with LLM (plan only)
mimoneko agents plan --goal "优化 README" --llm --json  # Output as JSON
mimoneko agents code --goal "优化 README" --plan-file plan.json  # Create patch intent skeleton
mimoneko agents code --goal "优化 README" --plan-file plan.json --llm  # Create patch intent with LLM
mimoneko agents code --goal "优化 README" --plan-file plan.json --llm --json  # Output as JSON
mimoneko agents review --intent-file intent.json  # Review patch intent skeleton
mimoneko agents review --intent-file intent.json --llm  # Review with LLM
mimoneko agents review --intent-file intent.json --llm --json  # Output as JSON
mimoneko agents validate --review-file review.json --intent-file intent.json  # Validation suggestions skeleton
mimoneko agents validate --review-file review.json --intent-file intent.json --llm  # Validation with LLM
mimoneko agents validate --review-file review.json --intent-file intent.json --llm --json  # Output as JSON
mimoneko agents run --goal "优化 README" --dry-run  # End-to-end dry run (skeleton)
mimoneko agents run --goal "优化 README" --llm --dry-run  # End-to-end dry run (LLM)
mimoneko agents run --goal "优化 README" --llm --dry-run --save-report  # Save report
mimoneko agents run --goal "优化 README" --llm --dry-run --json  # Output as JSON
mimoneko agents reports                     # List saved reports
mimoneko agents report <workflow_id>        # View specific report
mimoneko agents report <workflow_id> --json # View report as JSON
mimoneko agents patch-preview --intent-file intent.json  # Preview patch from intent
mimoneko agents patch-preview --report <workflow_id>     # Preview patch from report
mimoneko agents patch-preview --report <workflow_id> --json  # Preview as JSON
mimoneko neko events agents                 # View agent workflow events
```

The workflow skeleton includes four roles: Planner, Coder, Reviewer, and Validator. In the current skeleton phase:

- **Planner**: Produces a skeleton plan (no real LLM call) or LLM-generated plan with `--llm`
- **Coder**: Produces a skeleton patch intent (no real patch) or LLM-generated intent with `--llm`
- **Reviewer**: Produces a skeleton review (no real analysis) or LLM-generated review with `--llm`
- **Validator**: Produces skeleton validation suggestions (no real tests) or LLM-generated suggestions with `--llm`

**Important**: 
- `--dry-run` is **required** for `agents run` and must be explicitly enabled
- `--llm` only generates plans/intents/reviews/suggestions. No files are written, no patches are generated, no tests are executed, no tools are executed.
- `--plan-file` is required for `code` command and must contain a valid AgentPlan JSON.
- `--intent-file` is required for `review` command and must contain a valid CoderPatchIntent JSON.
- `--review-file` and `--intent-file` are required for `validate` command.
- `implementation_status` is always `plan_only` (Planner), `intent_only` (Coder), `review_only` (Reviewer), or `suggestions_only` (Validator).
- `approved` in Reviewer only means intent review passed, NOT permission to apply patch.
- `recommended_commands` in Validator are suggestions only, NOT executed automatically.

`neko approve <patch_id>` and `neko rollback <run_id>` are reserved for a later phase. They are not implemented in this release slice.

Tool audit events are observational only in this phase. `tool.called`, `tool.completed`, and `tool.failed` help users review tool activity, but they do not enforce approval, sandboxing, or redaction policy by themselves.

## FAQ

**Windows 输入 `mimoneko` 后提示 command not found / 无法识别命令**

重新打开 PowerShell。如果仍然失败，检查用户 PATH 是否包含：

```text
%LOCALAPPDATA%\MioNeko\bin
```

也可以直接运行：

```powershell
%LOCALAPPDATA%\MioNeko\bin\mimoneko.exe
```

**Windows 运行的仍然是旧版本 / 旧行为**

如果 `mimoneko version` 不是最新版本，或者 `mimoneko auth login` 不存在，或者执行 `mimoneko` 后仍然是旧行为，通常是 PATH 中有旧的 `mimoneko.exe` 排在新版本前面。

运行：

```powershell
where mimoneko
```

如果第一行不是：

```text
%LOCALAPPDATA%\MioNeko\bin\mimoneko.exe
```

请删除或调整第一行显示的旧路径，然后重新打开 PowerShell。安装脚本只会提示冲突，不会自动删除 `C:\Windows\System32\mimoneko.exe`，因为该位置通常需要管理员权限。

**macOS/Linux 输入 `mimoneko` 后提示 command not found**

确认 `~/.local/bin` 在 PATH 中：

```bash
echo "$PATH"
```

如果不在，添加：

```bash
export PATH="$HOME/.local/bin:$PATH"
```

然后重新打开终端。

**我还需要 `cd` 到下载目录吗？**

不需要。安装后重新打开终端，直接输入 `mimoneko`。

**是否需要安装 Go？**

不需要。Release 包里已经包含可执行文件。

## 安全说明

- API Key 保存到用户级配置：`~/.mimoneko/config.yaml`。
- 项目目录只保存 provider、base URL、model、env var 名称，不保存真实 API Key。
- 安装脚本只修改用户目录和用户 PATH，不需要管理员权限。
- MioNeko 默认 dry-run，不会自动提交或推送代码。

## 链接

- GitHub: https://github.com/yupeipei77-eng/MioNeko
- Releases: https://github.com/yupeipei77-eng/MioNeko/releases
- Issues: https://github.com/yupeipei77-eng/MioNeko/issues
