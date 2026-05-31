# MimoNeko 快速开始指南

## 前提条件

- Go 1.22 或更高版本
- Git
- 有效的 API 密钥（如 MIMO_API_KEY）

## 安装

### 1. 克隆仓库
```bash
git clone <repository-url>
cd MimoNeko
```

### 2. 构建项目
```powershell
# Windows
.\build.ps1

# 或手动构建
go build -o neko.exe ./cmd/neko
go build -o reasonforge.exe ./cmd/reasonforge
```

### 3. 设置环境变量

#### Windows (Powerhell)
```powershell
# 临时设置（当前会话）
$env:MIMO_API_KEY = "your-api-key-here"

# 永久设置
setx MIMO_API_KEY "your-api-key-here"
```

#### Windows (CMD)
```cmd
set MIMO_API_KEY=your-api-key-here
```

#### Linux/macOS
```bash
export MIMO_API_KEY="your-api-key-here"
```

### 4. 初始化配置
```bash
.\reasonforge.exe init
```

### 5. 配置模型
```bash
.\reasonforge.exe model setup --preset mimo --provider mimo --model mimo-v2.5-pro --set-default
```

### 6. 测试连接
```bash
.\reasonforge.exe model test --prompt "只回?OK"
```

## 基本使用

### 运行单次任务
```bash
# 干运行（不实际修改文件）
.\reasonforge.exe run --goal "读取 README 文件" --dry-run

# 实际运行
.\reasonforge.exe run --goal "修复 README 中的拼写错误"
```

### 运行多代理任务
```bash
# 多代理模式（规划器->编码器->审查器）
.\reasonforge.exe multi-run --goal "重构配置模块" --dry-run
```

### 启动终端控制台
```bash
.\neko.exe
```

在控制台中：
- 输入 `/` 查看命令面板
- 输入 `/model` 查看当前模型
- 输入 `/run <目标>` 运行任务
- 输入 `/exit` 退出

### 查看运行状态
```bash
# 列出所有运行
.\reasonforge.exe runs

# 查看特定运行状态
.\reasonforge.exe run-status <run-id>

# 查看运行事件
.\reasonforge.exe run-events <run-id>
```

### 启动 Web 仪表板
```bash
.\reasonforge.exe serve --port 9000 --open
```

## 配置说明

### 环境变量
项目使用以下环境变量：

| 变量名 | 说明 | 示例 |
|--------|------|------|
| `MIMO_API_KEY` | MIMO 模型 API 密钥 | `sk-xxx...` |
| `OPENAI_API_KEY` | OpenAI API 密钥 | `sk-xxx...` |
| `DEEPSEEK_API_KEY` | DeepSeek API 密钥 | `sk-xxx...` |

### 配置文件
配置文件位于 `.reasonforge/` 目录：

- `models.yaml` - 模型提供商配置
- `tools.yaml` - 工具配置
- `security.yaml` - 安全配置

### 修改配置
```bash
# 查看当前配置
.\reasonforge.exe doctor

# 重新初始化配置
.\reasonforge.exe init --repair
```

## 常见问题

### 1. API 密钥错误
```
错误：API key not found in environment variable MIMO_API_KEY
```
**解决方案**：确保已正确设置环境变量。

### 2. 配置文件缺失
```
错误：models.yaml missing or invalid
```
**解决方案**：运行 `.\reasonforge.exe init --repair`

### 3. 权限问题
```
错误：permission denied
```
**解决方案**：以管理员身份运行或检查文件权限。

### 4. 模型连接失败
```
错误：connection refused
```
**解决方案**：
1. 检查 API 密钥是否正确
2. 检查网络连接
3. 检查模型提供商状态

## 高级用法

### 自定义工具
在 `tools.yaml` 中添加自定义工具：
```yaml
tools:
  - name: my_tool
    kind: custom
    enabled: true
    risk_level: medium
    command: ["python", "my_script.py"]
```

### 多模型配置
在 `models.yaml` 中配置多个提供商：
```yaml
providers:
  - name: mimo
    type: openai-compatible
    base_url: https://api.mimo.com/v1
    api_key_env: MIMO_API_KEY
    models:
      - name: mimo-v2.5-pro
        purpose: coding
  - name: openai
    type: openai-compatible
    base_url: https://api.openai.com/v1
    api_key_env: OPENAI_API_KEY
    models:
      - name: gpt-4
        purpose: general
```

### 工作树隔离
启用 Git 工作树隔离：
```yaml
# worktree.yaml
enabled: true
root: .reasonforge/worktrees
branch_prefix: MimoNeko
max_active: 10
```

## 获取帮助

```bash
# 查看帮助
.\reasonforge.exe help

# 查看版本
.\reasonforge.exe version

# 诊断配置
.\reasonforge.exe doctor
```

## 下一步

1. 阅读完整的 [README.md](../README.md)
2. 查看 [项目结构](STRUCTURE.md)
3. 探索 [配置选项](../.reasonforge/)
4. 尝试运行示例任务
