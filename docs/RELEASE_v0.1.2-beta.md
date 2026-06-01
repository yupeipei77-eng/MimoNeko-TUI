# Release v0.1.2-beta

**发布日期**: 2026-06-01
**状态**: beta / release candidate

## 概述

MioNeko v0.1.2-beta 聚焦 CLI/TUI 终端体验与真实 MiMo API benchmark 记录。本版本不新增 Agent Runtime 能力，不修改模型调用协议，也不修改缓存算法。

## 重点更新

### CLI/TUI 输出优化

- 统一主要命令的终端输出风格。
- 为成功、警告、错误、信息、模型、缓存、补丁和密钥状态提供一致视觉标记。
- 输出结构从机器日志风格调整为更适合普通用户阅读的分层展示。

覆盖命令包括：

```powershell
mimoneko
mimoneko auth login
mimoneko auth status
mimoneko config show
mimoneko model test
mimoneko run
mimoneko cache-report
mimoneko patch list
```

### 首次启动欢迎页优化

首次运行 `mimoneko` 且未检测到配置时，现在会显示更清晰的欢迎页，并说明 3 步配置流程：

1. 选择模型服务
2. 输入 API Key
3. 测试连接

默认推荐仍为：

| Field | Value |
|-------|-------|
| Provider | MiMo |
| Model | mimo-v2.5-pro |
| Base URL | https://token-plan-cn.xiaomimimo.com/v1 |

### auth / config / model / run / cache / patch 输出分层

- `auth status` 清晰展示用户级配置、项目级配置和连接状态。
- `config show` 区分用户级配置、项目级配置和环境变量。
- `model test` 展示 Provider、Model、Base URL、API Key 脱敏状态、Status、Latency 和 Response。
- `run` 展示 Goal、Model、Running/Completed 生命周期、Result、Run ID 和 token/cache 指标。
- `cache-report` 展示 Total Requests、Input Tokens、Cached Tokens、Hit Rate，并补充缓存说明。
- `patch list` 使用更清晰的补丁/worktree 列表格式。

### API Key 脱敏增强

- `auth status`、`config show`、`model test` 和错误输出继续避免打印真实 API Key。
- 项目级配置仍只保存 API Key 环境变量名，不保存密钥明文。
- 占位 API Key 在环境变量展示中按 missing 处理，避免误导。

### 无 emoji / NO_COLOR 降级

新增轻量 CLI UI helper，支持在以下环境中降级到 ASCII 标记：

```powershell
$env:MIMONEKO_NO_EMOJI = "1"
$env:NO_COLOR = "1"
```

`TERM=dumb` 环境也会关闭 emoji/颜色相关输出。

### 友好错误提示

模型连接错误会以更适合用户排查的格式展示：

- 401: API Key 可能无效，建议运行 `mimoneko auth login`
- 404: Base URL 或模型名可能错误
- 429: 额度不足或请求过快
- timeout: 网络或服务端超时

### model test 输出预算调整

`model test` 的轻量测试输出预算从 16 调整为 128。

原因：MiMo 推理模型可能会把过小的输出预算消耗在 reasoning 上，导致连接成功但 `Response` 为空。该调整只影响 `model test` 的连通性检查，不改变 Agent Runtime，不改变模型调用协议，不改变缓存算法。

## Real-world Benchmark 文档

新增：

- `docs/REAL_WORLD_BENCHMARK.md`

该文档记录了一次真实 MiMo API 测试结果：

| Field | Value |
|-------|-------|
| Provider | MiMo |
| Model | mimo-v2.5-pro |
| Base URL | https://token-plan-cn.xiaomimimo.com/v1 |
| Command | `mimoneko "Reply OK"` |
| InputTokens | 70 |
| CachedTokens | 64 |
| HitRate | 91.43% |

说明：

- 该数据来自真实 MiMo API。
- 不是 Mock。
- 不是模拟 benchmark。
- 使用真实 API Key 和真实网络环境。
- 该命中率只代表该次 `mimoneko "Reply OK"` 测试场景的观测值，不代表所有任务都会达到 91.43%。

## 测试与安全

新增或更新测试覆盖：

- API Key 脱敏
- `NO_COLOR` 模式
- `MIMONEKO_NO_EMOJI` 模式
- `model test` 成功输出
- 401 错误友好提示
- `run` 成功输出
- `config show` 不泄露真实 Key

发布前检查项：

- `go test ./...`
- 项目目录密钥扫描
- 确认 `.mimoneko/`、`.nekonomimo/`、`dist/`、`logs/` 未进入本次提交范围

## 兼容性说明

- 不新增 Web UI。
- 不新增 Agent Runtime 功能。
- 不修改模型调用协议。
- 不修改缓存算法。
- 不破坏现有命令兼容性。

## 是否建议升级

建议所有 beta 用户升级到 v0.1.2-beta，以获得更清晰的首次启动、认证、配置、模型测试、运行和缓存报告体验。
