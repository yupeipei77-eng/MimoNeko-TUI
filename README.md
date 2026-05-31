# MioNeko

> **专为 MiMo 大模型而生的 Agent AI 编程工具 | v0.1.0-beta**

MioNeko 是一个本地优先的 AI 编码代理运行时，深度适配 MiMo 大模型，通过智能前缀缓存优化 API 调用成本。

**项目状态**: v0.1.0-beta / 公开测试

**实测缓存命中率**: 87.94% (50 轮)，详见 [benchmark](docs/BENCHMARK.md)

---

## 快速开始（5 分钟）

### 1. 安装（无需 Go）

**下载发行版**（推荐）:

访问 [Releases](https://github.com/yupeipei77-eng/MioNeko/releases) 下载对应平台：

| 平台 | 下载文件 |
|------|----------|
| Windows x64 | `mimoneko-windows-amd64.zip` |
| Windows ARM | `mimoneko-windows-arm64.zip` |
| Linux x64 | `mimoneko-linux-amd64.tar.gz` |
| Linux ARM | `mimoneko-linux-arm64.tar.gz` |
| macOS Apple Silicon | `mimoneko-darwin-arm64.tar.gz` |

**Windows 安装**:
```powershell
# 1. 下载 mimoneko-windows-amd64.zip
# 2. 解压到任意目录
# 3. 运行
mimoneko.exe auth login
```

**macOS 安装**:
```bash
# 1. 下载 mimoneko-darwin-arm64.tar.gz
# 2. 解压
tar -xzf mimoneko-darwin-arm64.tar.gz
# 3. 运行
./mimoneko auth login
```

**Linux 安装**:
```bash
# 1. 下载 mimoneko-linux-amd64.tar.gz
# 2. 解压
tar -xzf mimoneko-linux-amd64.tar.gz
# 3. 运行
./mimoneko auth login
```

**从源码构建**（需要 Go 1.22+）:
```bash
git clone https://github.com/yupeipei77-eng/MioNeko.git
cd MioNeko
go build -o mimoneko ./cmd/mimoneko
```

### 2. 登录

```bash
mimoneko auth login
```

交互式引导：
- 选择 Provider (MiMo / OpenAI-compatible)
- 输入 API Key
- 确认 Base URL
- 选择模型
- 自动测试连接

### 3. 测试

```bash
mimoneko model test
```

预期输出：
```
✓ Model: mimo-v2.5-pro
✓ Provider: mimo
✓ Base URL: https://token-plan-cn.xiaomimimo.com/v1
✓ API Key: sk-****abcd (configured)
✓ Status: connected
✓ Latency: 1.2s
```

### 4. 运行

```bash
mimoneko run "修改 README 增加项目说明"
```

---

## 命令速查

```bash
# 认证
mimoneko auth login        # 交互式登录
mimoneko auth status       # 查看配置状态
mimoneko auth logout       # 清除配置
mimoneko config show       # 查看配置（脱敏）

# 模型
mimoneko model test        # 测试模型连接
mimoneko model list        # 列出模型配置

# 任务
mimoneko run "目标"        # 运行任务
mimoneko multi-run "目标"  # 运行多代理任务
mimoneko runs              # 查看运行历史

# 补丁
mimoneko patch list        # 列出补丁
mimoneko patch preview <id> # 预览变更
mimoneko patch apply <id>  # 应用补丁

# 缓存
mimoneko cache-report      # 查看缓存统计

# 其他
mimoneko init              # 初始化项目配置
mimoneko doctor            # 诊断配置
mimoneko help              # 查看帮助
```

---

## 特性

- **MiMo 深度适配** - 原生支持 MiMo v2.5 Pro
- **智能缓存** - 实测缓存命中率 87%+
- **本地优先** - 所有数据本地存储，隐私安全
- **多代理协作** - 规划器→编码器→审查器工作流
- **工作树隔离** - Git Worktree 隔离，主分支永远安全
- **零自动提交** - 默认 dry-run，所有变更需人工确认

---

## 安全说明

- **API Key 安全**: 存储在用户目录 `~/.mimoneko/config.yaml`，不进仓库
- **脱敏输出**: 所有命令输出都会脱敏 API Key
- **本地存储**: 所有数据存储在本地，不上传云端
- **工作树隔离**: 代码修改在 Git Worktree 中进行，主分支安全
- **默认 dry-run**: 所有任务默认不修改文件，需显式确认

---

## 当前限制

- **v0.1.0-beta**: 公开测试版本
- **单模型支持**: 当前主要支持 MiMo v2.5 Pro
- **无 Web UI**: 仅支持命令行交互
- **本地运行**: 不支持远程部署

---

## 文档

- [Benchmark](docs/BENCHMARK.md)
- [Release Notes](docs/RELEASE_v0.1.0-beta.md)
- [用户体验审计](docs/FIRST_TIME_USER_AUDIT.md)

---

## 许可证

MIT License

---

## 链接

- GitHub: https://github.com/yupeipei77-eng/MioNeko
- Issues: https://github.com/yupeipei77-eng/MioNeko/issues
- Release: https://github.com/yupeipei77-eng/MioNeko/releases/tag/v0.1.0-beta
