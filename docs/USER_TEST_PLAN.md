# MioNeko 用户测试计划

**版本**: v0.1.0-alpha
**目标**: 验证新用户能否在 5 分钟内完成首次体验

---

## 测试环境

| 平台 | 版本 | 架构 |
|------|------|------|
| Windows 11 | 22H2+ | amd64 |
| macOS | Sonoma 14+ | Apple Silicon |
| Ubuntu | 22.04+ | amd64 |

---

## 测试流程

### Step 1: 安装 (1分钟)

**方式 A: 从源码构建**
```bash
git clone https://github.com/yupeipei77-eng/MioNeko.git
cd MioNeko
go build -o mimoneko.exe ./cmd/mimoneko
```

**方式 B: go install**
```bash
go install github.com/yupeipei77-eng/MioNeko/cmd/mimoneko@latest
```

**方式 C: 下载发行版**
1. 访问 https://github.com/yupeipei77-eng/MioNeko/releases
2. 下载对应平台的压缩包
3. 解压并添加到 PATH

**检查点**:
- [ ] 下载/构建成功
- [ ] `mimoneko version` 输出正确
- [ ] 无错误信息

**常见问题**:
| 问题 | 原因 | 解决方案 |
|------|------|----------|
| `go: command not found` | Go 未安装 | 安装 Go 1.22+ |
| `permission denied` | 权限不足 | 使用 sudo 或检查 PATH |
| `module not found` | 网络问题 | 设置 GOPROXY |

---

### Step 2: 首次配置 (2分钟)

```powershell
mimoneko auth login
```

**引导流程**:
1. 选择 Provider: MiMo / OpenAI-compatible / Local
2. 输入 API Key
3. 确认 Base URL
4. 选择模型
5. 自动测试连接

**检查点**:
- [ ] 引导流程清晰
- [ ] API Key 输入安全（不回显或部分回显）
- [ ] Base URL 默认值正确
- [ ] 模型选择正确
- [ ] 配置保存成功
- [ ] 连接测试成功

**常见问题**:
| 问题 | 原因 | 解决方案 |
|------|------|----------|
| `API Key 无效` | Key 格式错误 | 检查 Key 是否完整 |
| `连接超时` | 网络问题 | 检查网络连接 |
| `权限拒绝` | 权限不足 | 检查文件权限 |

---

### Step 3: 验证模型 (30秒)

```powershell
mimoneko model test
```

**检查点**:
- [ ] 测试成功
- [ ] 显示 provider、model、base_url
- [ ] API Key 脱敏显示
- [ ] 延迟合理 (< 5s)

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

---

### Step 4: 首次运行 (1分钟)

```powershell
mimoneko run "Reply with OK"
```

**检查点**:
- [ ] 任务执行成功
- [ ] 输出清晰
- [ ] 无错误
- [ ] 响应时间合理 (< 10s)

**预期输出**:
```
MimoNeko Agent
run_id=xxx goal="Reply with OK" max_steps=5 dry_run=true worktree=false

run_id=xxx state=succeeded steps=1
message=OK
```

---

### Step 5: 补丁预览 (30秒)

```powershell
mimoneko patch list
```

**检查点**:
- [ ] 列表显示正确
- [ ] 无异常
- [ ] 格式清晰

---

### Step 6: 缓存报告 (30秒)

```powershell
mimoneko cache-report
```

**检查点**:
- [ ] 报告显示正确
- [ ] 指标合理
- [ ] 无异常

**预期输出**:
```
total_observations=1
total_tokens=71
cached_tokens=0
hit_rate=0.0000
estimated_saving_percent=0.00
fingerprint_count=1
```

---

## 测试记录模板

| 测试者 | 日期 | 平台 | Go版本 | 卡住步骤 | 错误信息 | 建议 | 耗时 |
|--------|------|------|--------|----------|----------|------|------|
|        |      |      |        |          |          |      |      |

---

## 常见问题 FAQ

### 安装问题

**Q1: go install 失败**
```
go: github.com/yupeipei77-eng/MioNeko/cmd/mimoneko@latest: 
    github.com/yupeipei77-eng/MioNeko@v0.1.0-alpha: parsing go.mod:
    module declares its path as: github.com/mimoneko/mimoneko
```
**A**: 模块路径不匹配。使用 git clone + go build 方式安装。

**Q2: 编译失败**
```
# github.com/mimoneko/mimoneko/internal/config
internal/config/config.go:17:21: undefined: defaultConfigFiles
```
**A**: 代码版本问题。请使用最新代码。

**Q3: 命令不存在**
```
'mimoneko' is not recognized as an internal or external command
```
**A**: 未添加到 PATH。将可执行文件所在目录添加到系统 PATH。

### 配置问题

**Q4: API Key 无效**
```
error=API returned status 401
```
**A**: 检查 API Key 是否正确，是否有多余空格。

**Q5: 连接超时**
```
error=connection timeout
```
**A**: 检查网络连接，确认 Base URL 正确。

**Q6: 配置文件损坏**
```
error=parse config: yaml: line X: did not find expected key
```
**A**: 删除配置文件重新配置：
```powershell
# Windows
Remove-Item "$env:USERPROFILE\.mimoneko\config.yaml"

# macOS/Linux
rm ~/.mimoneko/config.yaml
```

### 运行问题

**Q7: 任务失败**
```
run_id=xxx state=failed steps=1
error=model call failed
```
**A**: 检查模型配置，确认 API Key 有效。

**Q8: 补丁应用失败**
```
error=patch apply failed: dirty working tree
```
**A**: 提交或暂存当前修改：
```powershell
git stash
# 或
git add . && git commit -m "WIP"
```

---

## 反馈收集

### 反馈渠道

1. **GitHub Issues**: https://github.com/yupeipei77-eng/MioNeko/issues
2. **测试记录表**: 本文档底部
3. **直接反馈**: 开发者邮箱

### 反馈内容

- 卡住的步骤
- 错误信息
- 改进建议
- 耗时记录
- 平台信息

---

## 测试通过标准

| 指标 | 目标 | 说明 |
|------|------|------|
| 安装成功率 | 100% | 所有平台都能安装成功 |
| 配置成功率 | 95%+ | 绝大多数用户能配置成功 |
| 首次运行成功率 | 90%+ | 绝大多数用户能完成首次运行 |
| 平均耗时 | < 5分钟 | 从安装到首次运行 |
| 错误率 | < 10% | 用户遇到错误的比例 |

---

## 后续改进

根据测试反馈，优先改进：

1. **错误提示**: 更清晰的错误信息
2. **引导流程**: 更友好的交互体验
3. **文档完善**: 更详细的说明
4. **自动修复**: 常见问题自动修复
