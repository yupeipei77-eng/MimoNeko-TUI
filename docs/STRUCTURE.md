# MimoNeko 项目结构

## 目录结构

```
MimoNeko/
├── cmd/                    # 命令行入口点
│   ├── neko/              # Neko 终端控制台
│   └── reasonforge/       # ReasonForge 主程序
├── internal/              # 内部包（不对外暴露）
│   ├── agent/            # 代理运行时
│   ├── cache/            # 缓存引擎
│   ├── cli/              # 命令行界面
│   ├── config/           # 配置管理
│   ├── contextengine/    # 上下文引擎
│   ├── conversation/     # 会话管理
│   ├── dashboard/        # 仪表板
│   ├── events/           # 事件系统
│   ├── memory/           # 记忆系统
│   ├── model/            # 模型管理
│   ├── modelprofile/     # 模型配置文件
│   ├── modelrouter/      # 模型路由器
│   ├── multiagent/       # 多代理运行时
│   ├── neko/             # Neko 终端实现
│   ├── patch/            # 补丁管理
│   ├── pathutil/         # 路径工具
│   ├── prefix/           # 前缀引擎
│   ├── repoindex/        # 仓库索引
│   ├── review/           # 代码审查
│   ├── scratchpad/       # 草稿本
│   ├── server/           # Web 服务器
│   ├── task/             # 任务管理
│   ├── toolruntime/      # 工具运行时
│   ├── tools/            # 工具实现
│   ├── validation/       # 验证系统
│   ├── version/          # 版本管理
│   └── worktree/         # 工作树管理
├── prompts/               # 提示词模板
├── schemas/               # JSON 模式定义
├── docs/                  # 文档
├── .reasonforge/          # ReasonForge 配置目录
├── .env.example           # 环境变量示例
├── build.ps1              # 构建脚本
├── go.mod                 # Go 模块定义
└── README.md              # 项目说明
```

## 配置目录

### `.reasonforge/` 目录
这是 ReasonForge 的主要配置目录，包含：

- `models.yaml` - 模型提供商配置
- `tools.yaml` - 工具配置
- `security.yaml` - 安全配置
- `prefix.yaml` - 前缀配置
- `worktree.yaml` - 工作树配置
- `patch.yaml` - 补丁配置
- `review.yaml` - 审查配置
- `validation.yaml` - 验证配置
- `multiagent.yaml` - 多代理配置
- `events.yaml` - 事件配置

### 环境变量配置
项目使用环境变量来管理敏感信息（如 API 密钥）。配置文件中只存储环境变量名称，不存储实际值。

## 主要组件

### 1. 命令行界面 (CLI)
- `cmd/reasonforge/` - 主程序入口
- `cmd/neko/` - 终端控制台入口

### 2. 核心引擎
- `internal/contextengine/` - 上下文组装引擎
- `internal/prefix/` - 字节稳定前缀引擎
- `internal/modelrouter/` - 模型路由和负载均衡

### 3. 代理系统
- `internal/agent/` - 单代理运行时
- `internal/multiagent/` - 多代理编排（规划器->编码器->审查器）

### 4. 工具系统
- `internal/tools/` - 内置工具实现
- `internal/toolruntime/` - 工具执行和安全策略

### 5. 状态管理
- `internal/events/` - 事件存储和日志
- `internal/memory/` - 记忆系统
- `internal/worktree/` - Git 工作树隔离

## 构建和运行

### 构建项目
```powershell
# Windows
.\build.ps1

# 或手动构建
go build -o neko.exe ./cmd/neko
go build -o reasonforge.exe ./cmd/reasonforge
```

### 运行项目
```powershell
# 初始化配置
.\reasonforge.exe init

# 设置模型
.\reasonforge.exe model setup --preset mimo --provider mimo --model mimo-v2.5-pro --set-default

# 设置环境变量
$env:MIMO_API_KEY = "your-api-key"
# 或使用 setx 永久设置
setx MIMO_API_KEY "your-api-key"

# 测试连接
.\reasonforge.exe model test --prompt "只回?OK"

# 运行任务
.\reasonforge.exe run --goal "读取 README 文件" --dry-run

# 启动终端控制台
.\neko.exe
```

## 开发指南

### 添加新功能
1. 在 `internal/` 下创建新的包
2. 实现功能逻辑
3. 在 `internal/cli/` 中添加命令
4. 更新配置文件模式
5. 添加测试

### 配置管理
- 所有配置存储在 `.reasonforge/` 目录
- 使用 YAML 格式
- 环境变量通过 `api_key_env` 字段引用
- 敏感信息从不存储在配置文件中

### 安全考虑
- 工具执行需要审批策略
- 文件访问受路径白名单限制
- 环境变量前缀控制敏感信息访问
- Git 工作树隔离防止主工作区污染
