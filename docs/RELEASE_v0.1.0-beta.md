# Release v0.1.0-beta

**发布日期**: 2026-05-31
**状态**: beta / 公开测试

## 概述

MioNeko v0.1.0-beta 是首个公开测试版本，重点优化了用户体验，让普通用户无需安装 Go 也能下载运行。

## 新增功能

### auth 命令

```bash
mimoneko auth login   # 交互式登录
mimoneko auth status  # 查看配置状态
mimoneko auth logout  # 清除配置
```

### 用户级配置

- API Key 存储在用户目录 `~/.mimoneko/config.yaml`
- 不进仓库，更安全
- 支持环境变量优先

### 首次启动引导

- 运行 `mimoneko` 或 `mimoneko run` 时
- 如果未配置 API Key，自动提示配置
- 引导用户完成 Provider、API Key、Model 配置

### 友好错误提示

- 401: API Key 无效，请执行 `mimoneko auth login`
- 403: 访问被拒绝，请检查 API Key 权限
- 404: Base URL 配置错误
- 429: 额度不足或请求过快
- 超时: 检查网络连接

### 无需 Go 安装

新用户可直接下载预编译二进制运行：

- Windows: `mimoneko-windows-amd64.zip`
- Linux: `mimoneko-linux-amd64.tar.gz`
- macOS: `mimoneko-darwin-arm64.tar.gz`

## 安装方式

### 方式一：下载发行版（推荐）

1. 访问 [Releases](https://github.com/yupeipei77-eng/MioNeko/releases)
2. 下载对应平台的压缩包
3. 解压并运行

**Windows**:
```powershell
mimoneko.exe auth login
```

**macOS/Linux**:
```bash
./mimoneko auth login
```

### 方式二：从源码构建

```bash
git clone https://github.com/yupeipei77-eng/MioNeko.git
cd MioNeko
go build -o mimoneko ./cmd/mimoneko
```

## 用户体验改进

### 之前 (v0.1.0-alpha)

```bash
# 需要安装 Go
# 需要手动设置环境变量
setx MIMO_API_KEY "your-key"
# 需要重启终端
mimoneko model test
```

### 现在 (v0.1.0-beta)

```bash
# 下载即用，无需 Go
# 交互式登录
mimoneko auth login
# 自动测试连接
```

## 支持平台

| 平台 | 架构 | 文件 |
|------|------|------|
| Windows | amd64 | `mimoneko-windows-amd64.zip` |
| Windows | arm64 | `mimoneko-windows-arm64.zip` |
| Linux | amd64 | `mimoneko-linux-amd64.tar.gz` |
| Linux | arm64 | `mimoneko-linux-arm64.tar.gz` |
| macOS | arm64 | `mimoneko-darwin-arm64.tar.gz` |

## 验证校验和

```bash
# 下载 SHA256SUMS 文件
# 验证文件完整性
sha256sum -c SHA256SUMS
```

## 测试结果

- ✅ go test ./... 全部通过
- ✅ 本地构建验证成功
- ✅ auth login/status/logout 命令可用
- ✅ config show 命令可用
- ✅ 首次启动引导可用
- ✅ 错误提示友好

## 当前限制

- **单模型支持**: 当前主要支持 MiMo v2.5 Pro
- **无 Web UI**: 仅支持命令行交互
- **本地运行**: 不支持远程部署

## 后续计划

- v0.1.1: 优化交互体验
- v0.2.0: 支持更多模型

## 贡献者

- MioNeko Team

## 许可证

MIT License
