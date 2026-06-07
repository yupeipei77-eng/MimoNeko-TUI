# MimoNeko

MimoNeko 是一个本地优先的终端 AI 编程助手，面向 MiMo 和 OpenAI-compatible 模型接口设计。它提供交互式 TUI、模型/provider 配置、上下文缓存、工具运行、worktree/patch 管理以及多 agent dry-run 工作流。

当前版本：`0.1.3-beta`

> 当前 multi-agent 工作流仍以 dry-run、plan、patch preview 为主；不会自动写文件、自动提交或自动推送代码。

## 快速开始

安装后重新打开终端，直接运行：

```bash
mimoneko
```

首次运行会进入配置向导：

```text
Provider
API Key
Base URL
Model
```

API Key 保存到用户级配置，不写入项目目录。

## 常用命令

```bash
mimoneko                         # 进入交互式 TUI
mimoneko "修复测试失败"           # 直接运行一个目标
mimoneko run "修复测试失败"       # 显式运行单 agent 任务
mimoneko model setup             # 配置 provider/model
mimoneko model test              # 测试模型连接
mimoneko doctor                  # 检查本地配置
mimoneko --help                  # 查看 CLI 命令
```

TUI 内常用命令：

```text
/              打开命令面板
/models        切换模型
/connect       连接 provider
/agents        切换 agent 模式
/diff          查看 diff
/editor        打开编辑入口
/new           新会话
/help          帮助
/exit          退出
```

## Model Provider

内置 provider preset：

- `mimo`
- `openai`
- `deepseek`
- `glm`
- `custom-openai-compatible`

示例：

```bash
mimoneko model setup --preset mimo --provider mimo --model mimo-v2.5-pro --set-default
mimoneko model discover --provider mimo
mimoneko model use mimo-v2.5-pro
```

项目目录只保存 provider、base URL、model、env var 名称等配置；真实 API Key 保存在用户级配置中。

## TUI Agent 模式

`Build`：

- 面向工程任务
- 默认使用 multi-agent/worktree 思路
- 当前主要用于计划、预览和安全执行入口

`Single`：

- 面向直接聊天和轻量任务
- 不创建 multi-agent 工作流
- 更适合问答、解释代码、短反馈

## 安全策略

MimoNeko 的默认原则：

- 不自动 commit
- 不自动 push
- 不在项目目录保存真实 API Key
- 对工具调用记录审计事件
- worktree/patch 流程优先预览再应用

当前安全 enforcement 仍在完善中，建议在真实项目中优先使用 dry-run 和 patch preview。

## 本地开发

```bash
go test ./...
go vet ./...
git diff --check
```

构建：

```bash
go build ./cmd/mimoneko
go build ./cmd/neko
```

## 当前重点

- 稳定 TUI 渲染、宽字符输入和原生光标
- 收敛命令面板和 provider/model picker
- 完善真实 agent 执行链路
- 强化默认安全策略和 approval 流程
- 清理历史品牌名、乱码文档和本地产物
