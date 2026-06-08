# MimoNeko 首次用户体验审计报告

**审计日期**: 2026-05-31
**审计版本**: v0.1.0-alpha
**审计视角**: 第一次接触 MimoNeko 的用户
**审计环境**: 全新 Windows / macOS / Linux

---

## 审计摘要

| 项目 | 评分 | 说明 |
|------|------|------|
| 安装体验 | ⚠️ 2/5 | 需要 Go 环境，步骤繁琐 |
| 配置体验 | ⚠️ 2/5 | 需要手动设置环境变量 |
| 首次运行 | ⚠️ 3/5 | 命令可用但提示不足 |
| 文档质量 | ⚠️ 2/5 | 缺少新手引导 |
| 错误提示 | ⚠️ 2/5 | 不够友好 |
| **总体评分** | **⚠️ 2.2/5** | **需要大幅优化** |

---

## 第一部分：安装步骤审计

### Step 1: 阅读 README

**用户视角**:
```
我看到 README 说：
1. git clone <repository-url>
2. cd MimoNeko
3. go build -o mimoneko.exe ./cmd/mimoneko

问题：
- 我需要先安装 Go？
- Go 是什么？
- 我不会编程怎么办？
```

**卡点 #1: 需要 Go 环境**

| 问题 | 影响 | 建议 |
|------|------|------|
| 需要安装 Go 1.22+ | 非开发者无法使用 | 提供预编译二进制 |
| 需要安装 Git | 增加安装步骤 | 提供直接下载链接 |
| 构建命令复杂 | 新手困惑 | 提供一键安装脚本 |

**卡点 #2: 命令名困惑**

| 问题 | 用户困惑 | 建议 |
|------|----------|------|
| `mimoneko.exe` | 为什么是 .exe？ | 跨平台说明 |
| `./cmd/mimoneko` | 这是什么路径？ | 简化构建命令 |

---

### Step 2: 配置 API Key

**用户视角**:
```
README 说要设置环境变量：
setx MIMO_API_KEY "your-mimo-api-key"

问题：
- API Key 从哪里获取？
- setx 是什么命令？
- 设置后为什么没反应？
```

**卡点 #3: API Key 获取**

| 问题 | 影响 | 建议 |
|------|------|------|
| 未说明如何获取 API Key | 用户卡住 | 添加获取链接 |
| 未说明 API Key 格式 | 用户困惑 | 添加格式示例 |
| 未说明是否需要付费 | 用户犹豫 | 添加说明 |

**卡点 #4: 环境变量设置**

| 问题 | 影响 | 建议 |
|------|------|------|
| setx 需要重启终端 | 用户困惑 | 说明重启要求 |
| macOS/Linux 命令不同 | 用户混淆 | 分平台说明 |
| 未验证是否设置成功 | 用户不确定 | 添加验证命令 |

---

### Step 3: 初始化和验证

**用户视角**:
```
README 说：
mimoneko init
mimoneko model test --prompt "只回OK"

问题：
- init 是做什么的？
- model test 为什么要加 --prompt？
- 如果失败了怎么办？
```

**卡点 #5: 命令参数困惑**

| 问题 | 影响 | 建议 |
|------|------|------|
| `--prompt "只回OK"` 含义不明 | 用户困惑 | 解释参数作用 |
| `--dry-run` 含义不明 | 用户困惑 | 解释参数作用 |
| `--goal` 含义不明 | 用户困惑 | 解释参数作用 |

**卡点 #6: 错误处理**

| 问题 | 影响 | 建议 |
|------|------|------|
| 401 错误无指导 | 用户卡住 | 添加排查步骤 |
| 连接超时无指导 | 用户卡住 | 添加排查步骤 |
| 配置错误无指导 | 用户卡住 | 添加排查步骤 |

---

## 第二部分：命令体验审计

### mimoneko auth login

**当前状态**: ❌ 命令不存在

**用户期望**:
```
我希望运行：
mimoneko auth login

然后：
1. 选择 Provider (MiMo / OpenAI / Local)
2. 输入 API Key
3. 确认 Base URL
4. 选择模型
5. 自动测试连接
```

**实际体验**:
```
$ mimoneko auth login
Error: unknown command "auth"

用户困惑：auth 命令在哪里？
```

**问题**:
- [ ] 没有 auth 命令
- [ ] 没有交互式引导
- [ ] 没有 Provider 选择
- [ ] 没有自动测试

---

### mimoneko model test

**当前状态**: ✅ 命令可用

**用户期望**:
```
$ mimoneko model test
✓ Model: mimo-v2.5-pro
✓ Provider: mimo
✓ Status: connected
✓ Latency: 1.2s
```

**实际体验**:
```
$ mimoneko model test
model=mimo-v2.5-pro
provider=mimo
base_url=https://token-plan-cn.xiaomimimo.com/v1
api_key_env=MIMO_API_KEY
api_key_status=configured
status=ok
latency_ms=1740
```

**问题**:
- [ ] 输出格式不友好（纯 key=value）
- [ ] 没有成功/失败图标
- [ ] 没有下一步提示
- [ ] 术语不清晰（api_key_env 是什么？）

**建议**:
```
✓ Model: mimo-v2.5-pro
✓ Provider: mimo
✓ Base URL: https://token-plan-cn.xiaomimimo.com/v1
✓ API Key: sk-****abcd (configured)
✓ Status: connected
✓ Latency: 1.2s

下一步：
  mimoneko run "your first task"
```

---

### mimoneko run

**当前状态**: ✅ 命令可用

**用户期望**:
```
$ mimoneko run "修改 README"
✓ 任务执行成功
✓ 修改了 README.md
✓ 查看变更：mimoneko patch preview
```

**实际体验**:
```
$ mimoneko run --goal "修改 README" --dry-run
MimoNeko Agent
run_id=pending goal="修改 README" max_steps=5 dry_run=true worktree=false

run_id=run_xxx state=succeeded steps=1
message=...
```

**问题**:
- [ ] 需要 `--goal` 参数（不直观）
- [ ] 需要 `--dry-run` 参数（默认行为不明）
- [ ] 输出格式不友好
- [ ] 没有下一步提示
- [ ] `run_id` 对用户无意义

**建议**:
```
$ mimoneko run "修改 README"
✓ Task started: 修改 README
✓ Mode: dry-run (use --apply to make changes)
✓ Result: ...

下一步：
  mimoneko patch preview  # 查看变更
  mimoneko patch apply    # 应用变更
```

---

### mimoneko patch

**当前状态**: ✅ 命令可用

**用户期望**:
```
$ mimoneko patch list
工作树列表：
  wt_xxx - 修改 README - 2024-01-01
  wt_yyy - 修复 Bug - 2024-01-02

$ mimoneko patch preview wt_xxx
变更预览：
  README.md: +5 -2
```

**实际体验**:
```
$ mimoneko patch list
MimoNeko Worktrees
id=wt_xxx task_id=task_xxx state=active path=... created_at=...
```

**问题**:
- [ ] 输出格式不友好
- [ ] 术语不清晰（task_id 是什么？）
- [ ] 没有变更摘要
- [ ] 没有操作提示

**建议**:
```
$ mimoneko patch list
工作树列表：
  ID: wt_xxx
  任务: 修改 README
  状态: active
  创建时间: 2024-01-01
  变更文件: 1

  ID: wt_yyy
  任务: 修复 Bug
  状态: active
  创建时间: 2024-01-02
  变更文件: 3

使用 'mimoneko patch preview <id>' 查看变更
```

---

### mimoneko cache-report

**当前状态**: ✅ 命令可用

**用户期望**:
```
$ mimoneko cache-report
缓存统计：
  命中率: 87.94%
  缓存 Token: 2560
  总 Token: 2911
  节省成本: ~88%
```

**实际体验**:
```
$ mimoneko cache-report
total_observations=41
total_tokens=2911
cached_tokens=2560
hit_rate=0.8794
estimated_saving_percent=87.94
fingerprint_count=1
  fingerprint=acaf3892... hit_rate=0.8794 reuse_count=40 uncached_tokens=351
```

**问题**:
- [ ] 输出格式不友好
- [ ] 术语不清晰（fingerprint 是什么？）
- [ ] 数字格式不直观（0.8794 vs 87.94%）
- [ ] 没有解释说明

**建议**:
```
$ mimoneko cache-report
缓存统计：
  命中率: 87.94%
  缓存 Token: 2,560
  总 Token: 2,911
  节省成本: ~88%
  观察次数: 41
  前缀指纹: acaf3892...

说明：
  - 命中率越高，成本越低
  - 前缀指纹相同表示缓存有效
```

---

## 第三部分：术语审计

### 普通用户看不懂的术语

| 术语 | 用户困惑 | 建议解释 |
|------|----------|----------|
| API Key | 这是什么？ | "用于访问 AI 模型的密钥" |
| Provider | 什么提供者？ | "AI 模型服务商" |
| Base URL | 这是什么？ | "API 服务地址" |
| dry-run | 什么意思？ | "试运行，不实际修改" |
| worktree | 什么树？ | "独立的工作目录" |
| fingerprint | 指纹？ | "缓存标识符" |
| token | 代币？ | "AI 模型的计费单位" |
| prefix | 前缀？ | "缓存的固定部分" |
| cache hit | 缓存命中？ | "复用之前的计算结果" |
| run_id | 运行ID？ | "任务的唯一标识" |
| task_id | 任务ID？ | "任务的唯一标识" |
| state=active | 什么状态？ | "进行中" |
| state=succeeded | 什么状态？ | "成功完成" |

---

## 第四部分：错误提示审计

### 当前错误提示

**场景 1: API Key 未设置**
```
$ mimoneko model test
error=API key not found in environment variable MIMO_API_KEY

用户困惑：
- 我应该在哪里设置？
- 如何获取 API Key？
- 设置后需要重启吗？
```

**建议**:
```
✗ API Key 未设置

请按以下步骤设置：
1. 获取 API Key: https://mimo.xiaomi.com
2. 设置环境变量：
   Windows: setx MIMO_API_KEY "your-key"
   macOS/Linux: export MIMO_API_KEY="your-key"
3. 重启终端

或运行 'mimoneko auth login' 进行交互式配置
```

**场景 2: 连接失败**
```
$ mimoneko model test
status=failed
error=API returned status 401

用户困惑：
- 401 是什么意思？
- 是 Key 错误还是网络问题？
- 如何排查？
```

**建议**:
```
✗ 连接失败 (401 Unauthorized)

可能原因：
1. API Key 无效或过期
2. API Key 格式错误
3. 网络连接问题

排查步骤：
1. 检查 API Key 是否正确
2. 检查 API Key 格式（应以 sk- 开头）
3. 检查网络连接
4. 运行 'mimoneko auth status' 查看配置
```

**场景 3: 命令不存在**
```
$ mimoneko auth login
Error: unknown command "auth"

用户困惑：
- auth 命令在哪里？
- 我应该用什么命令？
```

**建议**:
```
✗ 未知命令 'auth'

可用命令：
  mimoneko init      - 初始化配置
  mimoneko model     - 管理模型
  mimoneko run       - 运行任务
  mimoneko help      - 查看帮助

提示：运行 'mimoneko' 查看所有命令
```

---

## 第五部分：安装步骤遗漏检查

### README 遗漏的内容

| 遗漏 | 影响 | 建议 |
|------|------|------|
| Go 安装说明 | 非开发者无法使用 | 添加 Go 安装链接 |
| Git 安装说明 | 无 Git 用户卡住 | 添加 Git 安装链接 |
| API Key 获取方式 | 用户不知道如何获取 | 添加获取链接 |
| API Key 格式说明 | 用户格式错误 | 添加格式示例 |
| 首次运行示例 | 用户不知道做什么 | 添加完整示例 |
| 常见错误排查 | 用户卡住无解 | 添加 FAQ |
| 下一步指引 | 用户迷茫 | 添加下一步 |

---

## 第六部分：最容易失败的步骤

### 失败率排名

| 排名 | 步骤 | 失败原因 | 失败率估计 |
|------|------|----------|------------|
| 1 | 安装 Go | 非开发者不会安装 | 60% |
| 2 | 设置环境变量 | 命令不同、需要重启 | 40% |
| 3 | 获取 API Key | 不知道在哪里获取 | 30% |
| 4 | 运行 model test | 401 错误、网络问题 | 20% |
| 5 | 运行 run 命令 | 参数不理解 | 15% |

---

## 第七部分：需要自动化的地方

### 优先级排序

| 优先级 | 功能 | 说明 |
|--------|------|------|
| P0 | 预编译二进制 | 免去安装 Go |
| P0 | 交互式配置 | `mimoneko auth login` |
| P0 | 错误引导 | 失败时提示解决方案 |
| P1 | 一键安装脚本 | 自动下载和配置 |
| P1 | 状态检查 | `mimoneko auth status` |
| P2 | 友好输出 | 格式化输出 |
| P2 | 术语解释 | 帮助文档 |

---

## 第八部分：命令名合理性审计

### 当前命令名

| 命令 | 问题 | 建议 |
|------|------|------|
| `mimoneko` | 主命令名太长 | 保留，提供别名 `neko` |
| `mimoneko init` | 含义清晰 | 保留 |
| `mimoneko model test` | 含义清晰 | 保留 |
| `mimoneko run --goal` | `--goal` 不直观 | 支持 `mimoneko run "目标"` |
| `mimoneko patch list` | 含义清晰 | 保留 |
| `mimoneko cache-report` | 连字符不统一 | 改为 `mimoneko cache` 或 `mimoneko cache-report` |
| `mimoneko neko` | 重复 | 改为 `mimoneko console` 或 `mimoneko shell` |
| `mimoneko multi-run` | 连字符不统一 | 改为 `mimoneko multi` 或 `mimoneko multi-run` |

### 建议的命令结构

```
mimoneko
├── auth
│   ├── login
│   ├── status
│   └── logout
├── init
├── config
│   ├── show
│   ├── set
│   └── get
├── model
│   ├── test
│   ├── list
│   └── use
├── run
├── patch
│   ├── list
│   ├── preview
│   ├── apply
│   └── discard
├── cache
├── doctor
└── help
```

---

## 第九部分：改进建议总结

### 立即改进（v0.1.0-alpha）

| 改进 | 工作量 | 影响 |
|------|--------|------|
| 添加 Go 安装说明 | 小 | 高 |
| 添加 API Key 获取链接 | 小 | 高 |
| 添加常见错误排查 | 小 | 高 |
| 添加下一步指引 | 小 | 中 |
| 格式化输出 | 中 | 中 |

### 短期改进（v0.1.1）

| 改进 | 工作量 | 影响 |
|------|--------|------|
| 预编译二进制 | 中 | 高 |
| auth 命令 | 中 | 高 |
| 交互式引导 | 中 | 高 |
| 错误引导 | 中 | 高 |

### 中期改进（v0.2.0）

| 改进 | 工作量 | 影响 |
|------|--------|------|
| 一键安装脚本 | 中 | 高 |
| 友好输出格式 | 中 | 中 |
| 术语解释 | 小 | 中 |
| 命令别名 | 小 | 低 |

---

## 第十部分：完整用户旅程

### 理想用户旅程（5分钟）

```
0:00 - 用户访问 GitHub 仓库
0:30 - 下载预编译二进制
1:00 - 运行 'mimoneko auth login'
1:30 - 选择 Provider，输入 API Key
2:00 - 自动测试连接成功
2:30 - 运行 'mimoneko run "修改 README"'
3:00 - 任务成功
3:30 - 运行 'mimoneko patch preview'
4:00 - 查看变更
4:30 - 运行 'mimoneko patch apply'
5:00 - 完成首次体验
```

### 当前用户旅程（15+ 分钟）

```
0:00 - 用户访问 GitHub 仓库
1:00 - 阅读 README，困惑
2:00 - 搜索如何安装 Go
3:00 - 安装 Go
4:00 - 克隆仓库
5:00 - 构建项目
6:00 - 搜索如何获取 API Key
7:00 - 获取 API Key
8:00 - 设置环境变量
9:00 - 运行 mimoneko init
10:00 - 运行 mimoneko model test
11:00 - 遇到 401 错误
12:00 - 排查问题
13:00 - 重新设置环境变量
14:00 - 再次运行 model test
15:00 - 终于成功
```

---

## 结论

### 主要问题

1. **安装门槛高**: 需要 Go 环境，非开发者无法使用
2. **配置复杂**: 需要手动设置环境变量
3. **引导缺失**: 没有交互式引导
4. **错误不友好**: 错误提示无指导
5. **术语不清晰**: 普通用户看不懂

### 优先改进

1. **P0**: 提供预编译二进制
2. **P0**: 实现 `mimoneko auth login`
3. **P0**: 添加错误引导
4. **P1**: 添加 API Key 获取说明
5. **P1**: 格式化输出

### 评分理由

- **安装体验 2/5**: 需要 Go 环境，步骤繁琐
- **配置体验 2/5**: 需要手动设置环境变量
- **首次运行 3/5**: 命令可用但提示不足
- **文档质量 2/5**: 缺少新手引导
- **错误提示 2/5**: 不够友好
- **总体评分 2.2/5**: 需要大幅优化

---

**审计结论**: v0.1.0-alpha 可以工作，但不适合普通用户。需要在 v0.1.1 中重点优化安装和配置体验。
