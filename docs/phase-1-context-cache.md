# Phase 1: Context Engine + Cache Engine

## 实现概览

Phase 1 实现了 ReasonForge 的上下文工程和缓存工程基础能力，将项目从"接口骨架"推进到"可运行、可测试、可观测"的核心地基。

## Context Engine 结构

Context Engine 按照 4 层结构组装上下文，顺序固定：

1. **Immutable Prefix** — 字节级稳定的不可变前缀
2. **Conversation Log** — 只追加的事件日志
3. **Volatile Scratchpad** — 临时动态上下文
4. **Current User Input** — 当前用户输入（最末）

```
┌──────────────────────────────────┐
│ Immutable Prefix                 │  ← sha256 fingerprint, cache key
├──────────────────────────────────┤
│ Conversation Log (tail)          │  ← append-only history
├──────────────────────────────────┤
│ Volatile Scratchpad              │  ← dynamic context, priority-based
├──────────────────────────────────┤
│ Current User Input               │  ← current turn input
└──────────────────────────────────┘
```

## Immutable Prefix 规则

1. 组成：system prompt + coding rules + tool schemas（按固定顺序拼接）
2. Tool schemas 按 name 字母排序
3. 所有文本 LF 规范化，去除尾随空格
4. JSON 字段按 key 排序（canonical JSON）
5. 相同配置重复构建，SHA-256 hash 必须完全一致
6. 禁止：当前时间、session_id、随机 ID、动态 RAG 结果进入 prefix
7. Source kind 白名单：`static_file`、`generated_schema`

## Append-only Log 规则

1. 事件只能 Append，不能 Update / Delete / Reorder
2. `Archived` 字段允许软归档标记，但不物理删除
3. 查询时默认排除 Archived 事件，`IncludeArchived=true` 可查看
4. 存储格式：`.reasonforge/conversations/{conversation_id}.jsonl`
5. 未来压缩只能新增 summary event，不能覆盖原始 event

## Scratchpad 使用边界

1. 纯内存实现，进程重启后丢失
2. 每个 Item 有 Priority（值越高越重要）
3. Snapshot 按 Priority 降序返回，受 Limit 和 TokenBudget 约束
4. 超出 TokenBudget 时，低优先级 Item 被跳过
5. 过期 Item（ExpiresAt 已过）不进入 Snapshot
6. **Scratchpad 内容不得写入 Immutable Prefix**

## Cache Report 字段说明

Cache Report 按 prefix_hash 分组统计，并提供全局汇总：

### PerFingerprintReport

| 字段 | 类型 | 说明 |
|------|------|------|
| PrefixHash | string | SHA-256 fingerprint |
| TotalTokens | int | 该 fingerprint 下的总 input tokens |
| CachedTokens | int | 命中缓存的 tokens |
| UncachedTokens | int | 未命中缓存的 tokens |
| HitRate | float64 | CachedTokens / TotalTokens |
| EstimatedSavingPercent | float64 | HitRate * 100 |
| ReuseCount | int | 重复使用次数（observation 数 - 1） |
| PossibleMissReasons | []MissReason | 可能的未命中原因 |

### MissReason 枚举

- `no_prior_observation` — 首次观察该 fingerprint
- `prefix_changed` — fingerprint 发生变化
- `cache_expired` — 缓存引用已过期
- `model_changed` — 模型或 Provider 发生变化

### GlobalCacheSummary

| 字段 | 类型 | 说明 |
|------|------|------|
| TotalObservations | int | 总观察次数 |
| TotalTokens | int | 全局总 tokens |
| TotalCachedTokens | int | 全局缓存命中 tokens |
| OverallHitRate | float64 | 全局命中率 |
| EstimatedSavingPercent | float64 | 全局估算节省百分比 |

## Token Budget Guard 规则

1. 从 `prefix.yaml` 的 `budget` 配置读取阈值
2. 检查结果三种状态：
   - **OK** — utilization < warn_ratio
   - **WARN** — utilization >= warn_ratio
   - **BLOCK** — utilization >= block_ratio
3. budget <= 0 时始终返回 OK
4. 默认：warn_ratio=0.8, block_ratio=1.0
5. 校验：warn_ratio 必须小于 block_ratio

## 后续 Phase 2 接入 Model Router

Phase 2 接入 Model Router 时：

1. `ContextEngine.Build()` 返回的 `Bundle` 直接传给 `ModelRouter.Complete()`
2. `Bundle.ImmutablePrefix.Bytes` 作为 `CompletionRequest.ImmutablePrefix`
3. `Bundle.Volatile` 转换为 `CompletionRequest.VolatileMessages`
4. 模型调用完成后，将 response 的 cache usage 通过 `ContextEngine.RecordModelCall()` 记录
5. `CacheFingerprint.SHA256` 用于匹配 `CompletionRequest` 的 prefix cache

## 新增配置字段

`prefix.yaml` 新增：

```yaml
budget:
  warn_ratio: 0.8
  block_ratio: 1.0
```

## 已知限制

1. Token 估算使用 `len/4` 启发式，非真实 tokenizer
2. Scratchpad 纯内存，进程重启后丢失
3. Cache Registry JSONL 无 LRU 淘汰
4. Conversation Log 全文件扫描，无索引
5. 未接入真实模型 API
6. 压缩（summary event）仅保留接口
