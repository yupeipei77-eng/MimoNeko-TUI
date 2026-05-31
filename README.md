# MioNeko

> **专为 Mimo 大模型而生的 Agent AI 编程工具 | 缓存率 90%+**

MioNeko 是一个本地优先的 AI 编码代理运行时，深度适配 Mimo 大模型，通过智能缓存和上下文优化，实现 90% 以上的缓存命中率，大幅降低 API 调用成本。

## 为什么选择 MioNeko

| 特性 | 说明 |
|------|------|
| **Mimo 深度适配** | 原生支持 Mimo v2.5 Pro，针对 Mimo API 优化 |
| **90%+ 缓存率** | 智能前缀缓存，重复上下文不重复计费 |
| **本地优先** | 所有数据本地存储，代码不上传，隐私安全 |
| **多代理协作** | 规划器→编码器→审查器，自动迭代优化 |
| **工作树隔离** | Git Worktree 隔离，主分支永远安全 |
| **零自动提交** | 默认 dry-run，所有变更需人工确认 |

## 快速开始

### 安装

```bash
# 克隆仓库
git clone https://github.com/yupeipei77-eng/MioNeko.git
cd MioNeko

# 构建
go build -o neko.exe ./cmd/neko

# 设置环境变量
setx MIMO_API_KEY "your-api-key"
setx MimoNeko_NEKO_ROOT "path/to/your/project"
```

### 使用

```bash
# 初始化配置
neko init

# 设置模型
neko model setup --preset mimo --provider mimo --model mimo-v2.5-pro --set-default

# 测试连接
neko model test

# 启动交互式控制台
neko

# 运行任务
neko run --goal "修复 README 中的拼写错误"
```

## 命令速查

```bash
# 基础
neko version              # 查看版本
neko init                 # 初始化配置
neko doctor               # 诊断配置

# 模型
neko models               # 列出模型
neko model test           # 测试连接
neko model use mimo-v2.5-pro  # 切换模型

# 任务
neko run --goal "目标"    # 运行单任务
neko multi-run "目标"     # 运行多代理任务
neko runs                 # 查看运行历史

# 补丁
neko patch list           # 列出补丁
neko patch preview <id>   # 预览变更
neko patch apply <id>     # 应用补丁

# 仪表板
neko serve                # 启动 Web 界面
neko dashboard            # 终端仪表板
```

## 配置

### 环境变量

| 变量 | 说明 | 必需 |
|------|------|------|
| `MIMO_API_KEY` | Mimo API 密钥 | 是 |
| `MimoNeko_NEKO_ROOT` | 默认项目目录 | 否 |
| `MimoNeko_CONFIG_DIR` | 自定义配置目录 | 否 |

### 配置文件

配置存储在 `.mimoneko/` 目录：

```
.mimoneko/
├── models.yaml      # 模型配置
├── tools.yaml       # 工具配置
├── security.yaml    # 安全策略
├── prefix.yaml      # 缓存配置
├── worktree.yaml    # 工作树配置
└── events.yaml      # 事件配置
```

## 架构

```
MioNeko/
├── cmd/              # 命令行入口
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

## 缓存优化

MioNeko 通过以下方式实现 90%+ 缓存率：

1. **字节稳定前缀** - 系统提示和工具定义保持不变
2. **智能上下文管理** - 动态内容放入 volatile 区域
3. **增量更新** - 只发送变化的部分
4. **自动缓存检测** - 自动识别可缓存内容

## 安全

- 工具执行需要审批
- 文件访问路径白名单
- Git 工作树隔离
- 敏感信息自动脱敏
- 默认 dry-run 模式

## 开发

```bash
# 构建
go build -o neko.exe ./cmd/neko

# 测试
go test ./...

# 代码检查
golangci-lint run
```

## 许可证

MIT License

## 链接

- [快速开始](docs/QUICKSTART.md)
- [项目结构](docs/STRUCTURE.md)
- [API 文档](docs/)
