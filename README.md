# MioNeko

> **专为 MiMo 大模型而生的 Agent AI 编程工具 | v0.1.0-alpha**

MioNeko 是一个本地优先的 AI 编码代理运行时，深度适配 MiMo 大模型，通过智能前缀缓存优化 API 调用成本。

**项目状态**: v0.1.0-alpha / experimental

**实测缓存命中率**: 87.94% (50 轮)，详见 [benchmark](docs/BENCHMARK.md)

## 特性

- **MiMo 深度适配** - 原生支持 MiMo v2.5 Pro
- **智能缓存** - 实测缓存命中率 87%+
- **本地优先** - 所有数据本地存储，隐私安全
- **多代理协作** - 规划器→编码器→审查器工作流
- **工作树隔离** - Git Worktree 隔离，主分支永远安全
- **零自动提交** - 默认 dry-run，所有变更需人工确认

## 快速开始

### 1. 安装

**从源码构建**:

```bash
git clone https://github.com/yupeipei77-eng/MioNeko.git
cd MioNeko
go build -o mimoneko.exe ./cmd/mimoneko
```

**系统要求**:
- Go 1.22+
- Git
- Windows / macOS / Linux

### 2. 配置 MiMo API

```powershell
# Windows (永久设置)
setx MIMO_API_KEY "your-mimo-api-key"

# Windows (当前会话)
$env:MIMO_API_KEY = "your-mimo-api-key"

# macOS / Linux
export MIMO_API_KEY="your-mimo-api-key"
```

### 3. 初始化和验证

```powershell
# 初始化配置
mimoneko init

# 验证 MiMo 接入
mimoneko model test --prompt "只回OK"

# 查看配置状态
mimoneko doctor
```

**预期输出**:
```
model=mimo-v2.5-pro
provider=mimo
base_url=https://token-plan-cn.xiaomimimo.com/v1
api_key_env=MIMO_API_KEY
api_key_status=configured
status=ok
latency_ms=1740
```

### 4. 运行任务

```powershell
# 干运行（不修改文件）
mimoneko run --goal "读取 README 文件" --dry-run

# 实际运行
mimoneko run --goal "修复 README 中的拼写错误"

# 启动交互式控制台
mimoneko neko
```

## MiMo 配置示例

配置文件位于 `.mimoneko/models.yaml`：

```yaml
providers:
  - name: mimo
    type: openai-compatible
    base_url: https://token-plan-cn.xiaomimimo.com/v1
    api_key_env: MIMO_API_KEY
    models:
      - name: mimo-v2.5-pro
        purpose: coding
        max_output_tokens: 4096
        supports_prefix_cache: false

routing:
  default_model: mimo-v2.5-pro
```

## 命令速查

```powershell
# 基础
mimoneko version              # 查看版本
mimoneko init                 # 初始化配置
mimoneko doctor               # 诊断配置

# 模型
mimoneko models               # 列出模型
mimoneko model test           # 测试连接
mimoneko model list           # 查看配置
mimoneko model use mimo-v2.5-pro  # 切换模型

# 任务
mimoneko run --goal "目标"    # 运行单任务
mimoneko multi-run "目标"     # 运行多代理任务
mimoneko runs                 # 查看运行历史

# 补丁
mimoneko patch list           # 列出补丁
mimoneko patch preview <id>   # 预览变更
mimoneko patch apply <id>     # 应用补丁

# 缓存
mimoneko cache-report         # 查看缓存统计

# 仪表板
mimoneko serve                # 启动 Web 界面
mimoneko dashboard            # 终端仪表板
```

## 安全说明

- **API Key 安全**: API Key 仅存储在环境变量中，不写入配置文件
- **脱敏输出**: 所有命令输出都会脱敏 API Key
- **本地存储**: 所有数据存储在 `.mimoneko/` 目录，不上传云端
- **工作树隔离**: 代码修改在 Git Worktree 中进行，主分支安全
- **默认 dry-run**: 所有任务默认不修改文件，需显式确认

## 当前限制

- **v0.1.0-alpha**: 实验版本，可能存在 bug
- **单模型支持**: 当前主要支持 MiMo v2.5 Pro
- **无 Web UI**: 仅支持命令行交互
- **本地运行**: 不支持远程部署

## 项目结构

```
MioNeko/
├── cmd/              # 命令行入口
│   ├── mimoneko/     # 主命令
│   └── neko/         # 终端控制台
├── internal/         # 核心实现
│   ├── agent/        # 代理运行时
│   ├── cache/        # 缓存引擎
│   ├── modelrouter/  # 模型路由
│   ├── multiagent/   # 多代理协作
│   ├── neko/         # 终端控制台
│   └── tools/        # 工具系统
├── prompts/          # 提示词模板
└── schemas/          # JSON Schema
```

## 文档

- [快速开始](docs/QUICKSTART.md)
- [项目结构](docs/STRUCTURE.md)
- [Benchmark](docs/BENCHMARK.md)
- [Release Notes](docs/RELEASE_v0.1.0-alpha.md)
- [架构设计](docs/architecture.md)

## 许可证

MIT License

## 链接

- GitHub: https://github.com/yupeipei77-eng/MioNeko
- Issues: https://github.com/yupeipei77-eng/MioNeko/issues
- Release: https://github.com/yupeipei77-eng/MioNeko/releases/tag/v0.1.0-alpha
