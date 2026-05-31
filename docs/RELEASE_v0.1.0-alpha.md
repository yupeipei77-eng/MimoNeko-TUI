# Release v0.1.0-alpha

**发布日期**: 2026-05-31
**状态**: alpha / experimental

## 概述

MioNeko v0.1.0-alpha 是首个公开发布版本，专为 MiMo 大模型设计的 Agent AI 编程工具。

本版本实现了完整的本地 AI 编码代理工作流，包括模型接入、任务执行、补丁管理和缓存优化。

## MiMo 接入验证

### model test 结果

```
model=mimo-v2.5-pro
provider=mimo
base_url=https://token-plan-cn.xiaomimimo.com/v1
api_key_env=MIMO_API_KEY
api_key_status=configured
status=ok
latency_ms=1740
```

**验证项目**:
- ✅ MiMo API 连接正常
- ✅ base_url 配置正确
- ✅ api_key 通过环境变量读取
- ✅ api_key 脱敏输出（不暴露真实密钥）
- ✅ model 和 provider 信息正确

## 缓存命中率测试

### 测试环境

- MiMo API: https://token-plan-cn.xiaomimimo.com/v1
- Model: mimo-v2.5-pro
- 测试任务: "Reply with OK" (dry-run)
- 测试轮次: 50 轮

### 测试结果

| 轮次 | cached_tokens | hit_rate | 说明 |
|------|---------------|----------|------|
| 1 | 0 | 0% | 首次运行，无缓存 |
| 5 | 320 | 75.12% | 缓存快速积累 |
| 10 | 640 | 81.95% | 命中率稳步提升 |
| 20 | 1088 | 85.13% | 接近目标 |
| 30 | 1856 | 87.14% | 持续优化 |
| 50 | 2560 | **87.94%** | 稳定在 87%+ |

### 结论

- **最终缓存命中率**: 87.94% (50 轮后)
- **前缀稳定性**: 100% (fingerprint 始终一致)
- **服务端缓存**: MiMo API 正确返回 `cached_tokens`
- **目标达成**: 接近 90% 目标

## 最小演示链路

### 验证命令

```powershell
# 1. 初始化配置
mimoneko init
# 输出: MimoNeko already initialized, skipped files...

# 2. 验证 MiMo 接入
mimoneko model test --prompt "只回OK"
# 输出: status=ok, latency_ms=1740

# 3. 运行任务
mimoneko run --goal "Reply with OK" --dry-run --max-steps 1
# 输出: state=succeeded, message=OK

# 4. 查看补丁
mimoneko patch list
# 输出: Worktree list

# 5. 缓存报告
mimoneko cache-report
# 输出: hit_rate=87.94%
```

### 验证结果

| 命令 | 状态 | 说明 |
|------|------|------|
| `mimoneko init` | ✅ | 初始化配置成功 |
| `mimoneko model test` | ✅ | MiMo API 连接正常 |
| `mimoneko run --dry-run` | ✅ | 任务执行成功 |
| `mimoneko patch list` | ✅ | 补丁管理正常 |
| `mimoneko cache-report` | ✅ | 缓存统计正常 |
| `mimoneko doctor` | ✅ | 配置诊断正常 |
| `mimoneko model list` | ✅ | 模型列表正常 |

## 安全确认

### .env 文件

- ✅ `.env` 已删除
- ✅ 只保留 `.env.example` 作为模板
- ✅ `.gitignore` 已包含 `.env`

### 密钥检查

- ✅ 代码中无真实 API Key
- ✅ 代码中无真实 Token
- ✅ 代码中无真实 Secret
- ✅ 所有测试用密钥均为占位符

### 脱敏输出

- ✅ `model test` 输出脱敏 api_key
- ✅ `model list` 只显示 api_key_status
- ✅ 错误信息自动脱敏
- ✅ 日志记录脱敏

### .gitignore

```
# Environment and secrets
.env
.env.local
.env.*.local
*.key
*.pem
id_rsa
id_ed25519

# MioNeko local state
.mimoneko/
.neko/
logs/
cache/
```

## 当前限制

### 功能限制

- **单模型支持**: 当前主要支持 MiMo v2.5 Pro
- **无 Web UI**: 仅支持命令行交互
- **本地运行**: 不支持远程部署
- **无多模型切换**: 需手动修改配置

### 缓存限制

- **依赖服务端**: 缓存命中率依赖 MiMo API 返回 `cached_tokens`
- **前缀大小**: 大前缀可能影响缓存效率
- **动态内容**: 动态内容会降低缓存命中率

### 稳定性限制

- **alpha 版本**: 可能存在未知 bug
- **错误处理**: 部分边界情况处理不完善
- **性能优化**: 未进行大规模性能优化

## 已知问题

1. **neko 命令**: 在某些情况下可能无法正确找到项目根目录
2. **补丁预览**: 大文件 diff 可能显示不完整
3. **多代理模式**: 复杂任务可能需要多次迭代

## 后续计划

### v0.2.0 (计划)

- 支持更多 OpenAI-compatible 模型
- 优化缓存命中率到 90%+
- 增加更多工具支持
- 改进错误处理

### v0.3.0 (计划)

- Web UI 界面
- 远程部署支持
- 多模型自动切换
- 性能优化

## 贡献者

- MioNeko Team

## 许可证

MIT License
