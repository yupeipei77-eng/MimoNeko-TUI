# MimoNeko v0.1.x Roadmap

**目标**: 从"工程原型"进入"真实用户验证"
**原则**: 不新增大功能，专注用户体验

---

## P0: 发布准备

### 0.1 用户级配置系统

**当前问题**:
- API Key 存储在项目目录 `.mimoneko/models.yaml`
- 用户需要手动编辑配置文件
- 没有交互式引导

**目标设计**:

```
用户级配置（不进仓库）:
  Windows: %USERPROFILE%\.mimoneko\config.yaml
  macOS/Linux: ~/.mimoneko/config.yaml

项目级配置（进仓库）:
  .mimoneko/models.yaml  (模型配置，不含密钥)
  .mimoneko/tools.yaml   (工具配置)
  .mimoneko/security.yaml (安全配置)
```

**config.yaml 结构**:

```yaml
# 用户级配置 - 不进仓库
auth:
  providers:
    mimo:
      api_key: "sk-****abcd"  # 脱敏存储
      base_url: "https://token-plan-cn.xiaomimimo.com/v1"
    openai:
      api_key: "sk-****efgh"
      base_url: "https://api.openai.com/v1"
  default_provider: mimo

preferences:
  default_model: mimo-v2.5-pro
  dry_run: true
  worktree: true
```

### 0.2 auth 命令设计

**新增命令**:

```powershell
# 交互式登录
mimoneko auth login

# 输出示例：
# ? Select provider: MiMo / OpenAI-compatible / Local
# > MiMo
# ? API Key: ****
# ? Base URL [https://token-plan-cn.xiaomimimo.com/v1]:
# ? Model [mimo-v2.5-pro]:
# 
# ✓ Configuration saved to ~/.mimoneko/config.yaml
# ✓ Testing connection...
# ✓ Model test successful (latency: 1.2s)
# 
# Next steps:
#   mimoneko run "your first task"

# 查看配置状态
mimoneko auth status

# 输出示例：
# Provider: mimo
# Base URL: https://token-plan-cn.xiaomimimo.com/v1
# API Key: sk-****abcd (configured)
# Model: mimo-v2.5-pro
# Status: connected

# 登出
mimoneko auth logout

# 查看配置（脱敏）
mimoneko config show

# 输出示例：
# auth:
#   providers:
#     mimo:
#       api_key: "sk-****abcd"
#       base_url: "https://token-plan-cn.xiaomimimo.com/v1"
#   default_provider: mimo
# preferences:
#   default_model: mimo-v2.5-pro
```

### 0.3 交互式引导流程

**首次启动逻辑**:

```powershell
mimoneko
```

**输出**:

```
╔══════════════════════════════════════════════════════════════╗
║                    MimoNeko v0.1.0-alpha                      ║
║            专为 MiMo 大模型而生的 Agent AI 编程工具           ║
╚══════════════════════════════════════════════════════════════╝

未检测到配置，是否现在配置？ [Y/n]: Y

? 选择 Provider:
  > MiMo (推荐)
    OpenAI-compatible
    Local

? 输入 MiMo API Key: ****
? Base URL [https://token-plan-cn.xiaomimimo.com/v1]: 
? 选择模型:
  > mimo-v2.5-pro (推荐)
    mimo-v2.5-flash

✓ 配置已保存到 C:\Users\Huya\.mimoneko\config.yaml
✓ 正在测试连接...
✓ 连接成功 (延迟: 1.2s)

下一步：
  mimoneko run "你的第一个任务"
  mimoneko model test  # 测试模型连接
  mimoneko help        # 查看所有命令
```

### 0.4 环境变量优先级

```powershell
# 优先级从高到低：
1. MIMO_API_KEY           # MiMo 专用
2. MIMONEKO_API_KEY       # 通用
3. OPENAI_API_KEY         # OpenAI 兼容
4. ~/.mimoneko/config.yaml # 用户配置
5. .mimoneko/models.yaml  # 项目配置
```

### 0.5 README 快速开始更新

**新用户 3 分钟体验**:

```markdown
## 快速开始

### 安装

```bash
go install github.com/yupeipei77-eng/MimoNeko-TUI/cmd/mimoneko@main
```

### 首次配置

```powershell
mimoneko auth login
```

交互式引导会自动：
- 选择 Provider (MiMo / OpenAI / Local)
- 输入 API Key
- 确认 Base URL
- 选择模型
- 测试连接

### 验证模型

```powershell
mimoneko model test
```

### 开始使用

```powershell
mimoneko run "修改 README 增加项目说明"
```
```

---

## P1: 发行版构建

### 1.1 构建脚本

**文件**: `scripts/build-release.sh`

```bash
#!/bin/bash
set -e

VERSION=${1:-"v0.1.1"}
OUTPUT_DIR="dist"

mkdir -p $OUTPUT_DIR

# 构建矩阵
platforms=(
    "windows/amd64"
    "windows/arm64"
    "linux/amd64"
    "linux/arm64"
    "darwin/arm64"
)

for platform in "${platforms[@]}"; do
    IFS='/' read -r os arch <<< "$platform"
    
    output_name="mimoneko-${os}-${arch}"
    if [ "$os" = "windows" ]; then
        output_name="${output_name}.exe"
    fi
    
    echo "Building ${output_name}..."
    GOOS=$os GOARCH=$arch go build -o "${OUTPUT_DIR}/${output_name}" ./cmd/mimoneko
    
    # 打包
    if [ "$os" = "windows" ]; then
        cd $OUTPUT_DIR && zip "mimoneko-${os}-${arch}.zip" "${output_name}" && rm "${output_name}" && cd ..
    else
        cd $OUTPUT_DIR && tar -czf "mimoneko-${os}-${arch}.tar.gz" "${output_name}" && rm "${output_name}" && cd ..
    fi
done

# 生成 SHA256
cd $OUTPUT_DIR && sha256sum * > SHA256SUMS && cd ..

echo "Build complete! Files in ${OUTPUT_DIR}/"
ls -la $OUTPUT_DIR/
```

### 1.2 GitHub Actions

**文件**: `.github/workflows/release.yml`

```yaml
name: Release

on:
  push:
    tags:
      - 'v*'

permissions:
  contents: write

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - uses: actions/setup-go@v5
        with:
          go-version: '1.22'
      
      - name: Build release artifacts
        run: |
          chmod +x scripts/build-release.sh
          ./scripts/build-release.sh ${{ github.ref_name }}
      
      - name: Create Release
        uses: softprops/action-gh-release@v1
        with:
          files: |
            dist/*.zip
            dist/*.tar.gz
            dist/SHA256SUMS
          generate_release_notes: true
```

### 1.3 输出文件

```
dist/
├── mimoneko-windows-amd64.zip
├── mimoneko-windows-arm64.zip
├── mimoneko-linux-amd64.tar.gz
├── mimoneko-linux-arm64.tar.gz
├── mimoneko-darwin-arm64.tar.gz
└── SHA256SUMS
```

---

## P2: 用户验证

### 2.1 用户测试计划

**文件**: `docs/USER_TEST_PLAN.md`

```markdown
# MimoNeko 用户测试计划

## 测试目标

验证新用户能否在 5 分钟内完成首次体验。

## 测试环境

- Windows 11 (amd64)
- macOS Sonoma (Apple Silicon)
- Ubuntu 22.04 (amd64)

## 测试流程

### Step 1: 安装 (1分钟)

```bash
go install github.com/yupeipei77-eng/MimoNeko-TUI/cmd/mimoneko@main
```

**检查点**:
- [ ] 下载速度
- [ ] 安装成功
- [ ] `mimoneko version` 输出正确

### Step 2: 首次配置 (2分钟)

```powershell
mimoneko auth login
```

**检查点**:
- [ ] 引导流程清晰
- [ ] API Key 输入安全（不回显）
- [ ] Base URL 默认值正确
- [ ] 模型选择正确
- [ ] 配置保存成功
- [ ] 连接测试成功

### Step 3: 首次运行 (1分钟)

```powershell
mimoneko run "Reply with OK"
```

**检查点**:
- [ ] 任务执行成功
- [ ] 输出清晰
- [ ] 无错误

### Step 4: 补丁预览 (30秒)

```powershell
mimoneko patch list
```

**检查点**:
- [ ] 列表显示正确
- [ ] 无异常

### Step 5: 缓存报告 (30秒)

```powershell
mimoneko cache-report
```

**检查点**:
- [ ] 报告显示正确
- [ ] 指标合理

## 记录模板

| 测试者 | 日期 | 平台 | 卡住步骤 | 错误信息 | 建议 |
|--------|------|------|----------|----------|------|
|        |      |      |          |          |      |

## 常见问题

1. **Q: go install 失败**
   A: 检查 Go 版本 >= 1.22，检查 GOPATH 配置

2. **Q: API Key 无效**
   A: 确认 Key 格式正确，检查网络连接

3. **Q: 模型测试失败**
   A: 检查 Base URL，确认 API Key 有效
```

---

## P3: Benchmark

### 3.1 对比方案

**文件**: `docs/BENCHMARK_COMPARISON.md`

```markdown
# MimoNeko Benchmark 对比

## 对比工具

| 工具 | 版本 | 说明 |
|------|------|------|
| MimoNeko | v0.1.0-alpha | 本项目 |
| OpenCode | latest | 开源 AI 编程工具 |
| Aider | latest | AI 结对编程 |

## 统一测试任务

### 任务 1: 修改 README

**输入**: "在 README 中增加项目说明章节"
**预期**: 在 README.md 中新增 ## 项目说明 章节

### 任务 2: 修复简单 Bug

**输入**: "修复 README 中的拼写错误"
**预期**: 修正 README.md 中的拼写错误

### 任务 3: 新增配置项

**输入**: "在 config.yaml 中增加 timeout 配置项"
**预期**: 在配置文件中新增 timeout 字段

## 统计指标

| 指标 | 说明 | 计算方式 |
|------|------|----------|
| Token 使用量 | 输入 + 输出 token | API 返回 |
| 响应时间 | 从请求到响应 | 本地计时 |
| Cache Hit Rate | 缓存命中率 | cached_tokens / total_tokens |
| 成功率 | 任务完成率 | 成功次数 / 总次数 |

## 测试脚本

```bash
#!/bin/bash
# benchmark-run.sh

TASKS=("修改 README 增加项目说明" "修复 README 拼写错误" "增加 timeout 配置")
TOOLS=("mimoneko" "opencode" "aider")

for tool in "${TOOLS[@]}"; do
    for task in "${TASKS[@]}"; do
        echo "Testing ${tool}: ${task}"
        # 记录开始时间
        start_time=$(date +%s%N)
        
        # 执行任务
        ${tool} run --goal "${task}" --dry-run
        
        # 记录结束时间
        end_time=$(date +%s%N)
        duration=$(( (end_time - start_time) / 1000000 ))
        
        echo "Duration: ${duration}ms"
    done
done
```

## 结果记录

| 工具 | 任务 | Token | 时间 | Cache | 成功 |
|------|------|-------|------|-------|------|
| MimoNeko | 修改 README | | | | |
| OpenCode | 修改 README | | | | |
| Aider | 修改 README | | | | |
```

---

## P4: 安装体验优化

### 4.1 目标体验

**用户 3 分钟流程**:

```powershell
# 1. 安装 (30秒)
go install github.com/yupeipei77-eng/MimoNeko-TUI/cmd/mimoneko@main

# 2. 配置 (1分钟)
mimoneko auth login

# 3. 验证 (30秒)
mimoneko model test

# 4. 使用 (1分钟)
mimoneko run "修改 README 增加项目说明"
```

### 4.2 实现计划

**Phase 1: auth 命令 (v0.1.1)**

新增文件:
- `internal/cli/cmd_auth.go` - auth 命令实现
- `internal/auth/auth.go` - 认证逻辑
- `internal/auth/config.go` - 用户配置管理

修改文件:
- `internal/cli/help.go` - 添加 auth 命令帮助
- `internal/cli/command.go` - 注册 auth 命令

**Phase 2: 交互式引导 (v0.1.2)**

新增文件:
- `internal/prompt/prompt.go` - 交互式提示
- `internal/prompt/wizard.go` - 配置向导

修改文件:
- `internal/cli/cmd_init.go` - 集成交互式引导
- `internal/cli/cli.go` - 首次启动检测

**Phase 3: 配置迁移 (v0.1.3)**

修改文件:
- `internal/config/config.go` - 支持用户级配置
- `internal/config/migrate.go` - 配置迁移工具

---

## v0.1.1 版本目标

### 核心功能

1. **auth 命令**
   - `mimoneko auth login` - 交互式登录
   - `mimoneko auth status` - 查看状态
   - `mimoneko auth logout` - 登出
   - `mimoneko config show` - 查看配置（脱敏）

2. **用户级配置**
   - 配置存储在 `~/.mimoneko/config.yaml`
   - API Key 不进仓库
   - 支持环境变量优先

3. **交互式引导**
   - 首次启动自动引导
   - Provider 选择
   - API Key 输入
   - 模型选择
   - 连接测试

4. **发行版构建**
   - 自动构建脚本
   - GitHub Actions
   - 多平台支持

### 文档更新

1. **README**
   - 快速开始更新
   - 安装步骤简化
   - FAQ 补充

2. **用户测试计划**
   - 测试流程
   - 记录模板
   - 常见问题

3. **Benchmark 对比**
   - 对比方案
   - 统一测试
   - 结果记录

### 验收标准

- [ ] 新用户 3 分钟完成首次体验
- [ ] API Key 不进仓库
- [ ] 环境变量优先级正确
- [ ] 配置展示脱敏
- [ ] 发行版自动构建
- [ ] 测试计划完整
- [ ] Benchmark 方案可行

---

## 发现的问题

### 当前问题

1. **配置存储**: API Key 存储在项目目录，不安全
2. **首次体验**: 没有引导流程，用户需要手动编辑配置
3. **命令缺失**: 没有 auth 命令
4. **构建手动**: 没有自动构建脚本
5. **文档不全**: 缺少 FAQ 和错误排查

### 解决方案

1. **用户级配置**: 新增 `~/.mimoneko/config.yaml`
2. **交互式引导**: 新增首次启动向导
3. **auth 命令**: 新增认证管理命令
4. **自动构建**: 新增 GitHub Actions
5. **文档补充**: 新增 FAQ 和错误排查

---

## 建议的 v0.1.1 版本目标

### 核心目标

**让新用户 3 分钟跑起来**

### 功能列表

| 优先级 | 功能 | 说明 |
|--------|------|------|
| P0 | auth login | 交互式登录 |
| P0 | auth status | 查看状态 |
| P0 | config show | 查看配置（脱敏） |
| P1 | 用户级配置 | ~/.mimoneko/config.yaml |
| P1 | 环境变量优先 | MIMO_API_KEY > config |
| P2 | 交互式引导 | 首次启动向导 |
| P2 | 发行版构建 | 自动构建脚本 |
| P3 | 文档更新 | FAQ、测试计划 |

### 验收标准

```powershell
# 新用户流程
go install github.com/yupeipei77-eng/MimoNeko-TUI/cmd/mimoneko@main
mimoneko auth login
mimoneko run "Reply with OK"

# 预期时间: 3 分钟
# 预期结果: 成功执行任务
```

### 发布计划

- **v0.1.1-alpha**: auth 命令 + 用户级配置
- **v0.1.2-alpha**: 交互式引导 + 发行版构建
- **v0.1.3-alpha**: 文档完善 + Benchmark

---

## 限制

- 不新增大功能
- 不开发 Web UI
- 不开发插件市场
- 不开发向量数据库
- 不开发 RAG
- 不开发团队协作
- 不重构核心架构
- 不修改项目定位
- 不自动 push
- 不创建新分支
- 不提交未经验证的代码
