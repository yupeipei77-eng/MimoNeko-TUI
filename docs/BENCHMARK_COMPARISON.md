# MimoNeko Benchmark 对比方案

**版本**: v0.1.0-alpha
**目标**: 客观对比 MimoNeko 与其他 AI 编程工具

---

## 对比工具

| 工具 | 版本 | 说明 | 安装方式 |
|------|------|------|----------|
| MimoNeko | v0.1.0-alpha | 本项目 | go install |
| OpenCode | latest | 开源 AI 编程工具 | npm install |
| Aider | latest | AI 结对编程 | pip install |

---

## 统一测试任务

### 任务 1: 修改 README

**输入**: "在 README 中增加项目说明章节"

**预期结果**:
- 在 README.md 中新增 `## 项目说明` 章节
- 章节内容合理
- 不破坏现有内容

**评分标准**:
- 5分: 完美完成，内容丰富
- 4分: 基本完成，内容合理
- 3分: 完成但内容简单
- 2分: 部分完成
- 1分: 未完成或破坏现有内容

---

### 任务 2: 修复简单 Bug

**输入**: "修复 README 中的拼写错误"

**预期结果**:
- 识别并修复拼写错误
- 不引入新错误
- 保持格式一致

**评分标准**:
- 5分: 识别并修复所有错误
- 4分: 识别并修复大部分错误
- 3分: 识别部分错误
- 2分: 识别但未修复
- 1分: 未识别或引入新错误

---

### 任务 3: 新增配置项

**输入**: "在配置文件中增加 timeout 配置项"

**预期结果**:
- 在配置文件中新增 timeout 字段
- 字段类型正确（整数）
- 有合理的默认值
- 有注释说明

**评分标准**:
- 5分: 完美完成，有默认值和注释
- 4分: 基本完成，有默认值
- 3分: 完成但无默认值
- 2分: 部分完成
- 1分: 未完成或格式错误

---

## 统计指标

### Token 使用量

**说明**: API 调用消耗的 token 数量

**计算方式**:
- 输入 token: 发送给 API 的 token
- 输出 token: API 返回的 token
- 总 token: 输入 + 输出

**数据来源**: API 响应中的 `usage` 字段

---

### 响应时间

**说明**: 从请求到响应的时间

**计算方式**:
- 开始时间: 发送请求前
- 结束时间: 收到响应后
- 响应时间 = 结束时间 - 开始时间

**数据来源**: 本地计时

---

### Cache Hit Rate

**说明**: 缓存命中率

**计算方式**:
- cached_tokens: API 返回的缓存 token
- total_tokens: 总 token
- hit_rate = cached_tokens / total_tokens

**数据来源**: API 响应中的 `usage.prompt_tokens_details.cached_tokens`

**注意**: 需要 API 支持此字段

---

### 成功率

**说明**: 任务完成率

**计算方式**:
- 成功次数: 任务完成且结果正确
- 总次数: 所有尝试
- 成功率 = 成功次数 / 总次数

**数据来源**: 本地统计

---

## 测试脚本

### PowerShell 版本

```powershell
# benchmark-run.ps1

$Tasks = @(
    "在 README 中增加项目说明章节",
    "修复 README 中的拼写错误",
    "在配置文件中增加 timeout 配置项"
)

$Tools = @("mimoneko", "opencode", "aider")

$Results = @()

foreach ($tool in $Tools) {
    foreach ($task in $Tasks) {
        Write-Host "Testing ${tool}: ${task}" -ForegroundColor Cyan
        
        # Record start time
        $startTime = Get-Date
        
        # Execute task (dry-run)
        $output = & $tool run --goal $task --dry-run 2>&1
        
        # Record end time
        $endTime = Get-Date
        $duration = ($endTime - $startTime).TotalMilliseconds
        
        # Parse output for token usage (if available)
        $tokens = 0
        if ($output -match "total_tokens=(\d+)") {
            $tokens = [int]$Matches[1]
        }
        
        # Check success
        $success = $output -match "state=succeeded"
        
        $Results += [PSCustomObject]@{
            Tool = $tool
            Task = $task
            Tokens = $tokens
            Duration = $duration
            Success = $success
        }
    }
}

# Display results
$Results | Format-Table -AutoSize

# Export to CSV
$Results | Export-Csv -Path "benchmark-results.csv" -NoTypeInformation
```

### Bash 版本

```bash
#!/bin/bash
# benchmark-run.sh

TASKS=(
    "在 README 中增加项目说明章节"
    "修复 README 中的拼写错误"
    "在配置文件中增加 timeout 配置项"
)

TOOLS=("mimoneko" "opencode" "aider")

echo "Tool,Task,Tokens,Duration,Success" > benchmark-results.csv

for tool in "${TOOLS[@]}"; do
    for task in "${TASKS[@]}"; do
        echo "Testing ${tool}: ${task}"
        
        # Record start time
        start_time=$(date +%s%N)
        
        # Execute task (dry-run)
        output=$(${tool} run --goal "${task}" --dry-run 2>&1)
        
        # Record end time
        end_time=$(date +%s%N)
        duration=$(( (end_time - start_time) / 1000000 ))
        
        # Parse output for token usage (if available)
        tokens=0
        if [[ $output =~ total_tokens=([0-9]+) ]]; then
            tokens=${BASH_REMATCH[1]}
        fi
        
        # Check success
        success="false"
        if [[ $output =~ state=succeeded ]]; then
            success="true"
        fi
        
        echo "${tool},${task},${tokens},${duration},${success}" >> benchmark-results.csv
    done
done

echo "Results saved to benchmark-results.csv"
cat benchmark-results.csv
```

---

## 结果记录表

### Token 使用量

| 工具 | 任务 | 输入 Token | 输出 Token | 总 Token | 说明 |
|------|------|------------|------------|----------|------|
| MimoNeko | 修改 README | | | | |
| OpenCode | 修改 README | | | | |
| Aider | 修改 README | | | | |
| MimoNeko | 修复 Bug | | | | |
| OpenCode | 修复 Bug | | | | |
| Aider | 修复 Bug | | | | |
| MimoNeko | 新增配置 | | | | |
| OpenCode | 新增配置 | | | | |
| Aider | 新增配置 | | | | |

### 响应时间

| 工具 | 任务 | 响应时间 (ms) | 说明 |
|------|------|---------------|------|
| MimoNeko | 修改 README | | |
| OpenCode | 修改 README | | |
| Aider | 修改 README | | |
| MimoNeko | 修复 Bug | | |
| OpenCode | 修复 Bug | | |
| Aider | 修复 Bug | | |
| MimoNeko | 新增配置 | | |
| OpenCode | 新增配置 | | |
| Aider | 新增配置 | | |

### Cache Hit Rate

| 工具 | 任务 | Cached Tokens | Total Tokens | Hit Rate | 说明 |
|------|------|---------------|--------------|----------|------|
| MimoNeko | 修改 README | | | | |
| OpenCode | 修改 README | | | | |
| Aider | 修改 README | | | | |
| MimoNeko | 修复 Bug | | | | |
| OpenCode | 修复 Bug | | | | |
| Aider | 修复 Bug | | | | |
| MimoNeko | 新增配置 | | | | |
| OpenCode | 新增配置 | | | | |
| Aider | 新增配置 | | | | |

### 成功率

| 工具 | 任务 | 成功 | 失败 | 成功率 | 说明 |
|------|------|------|------|--------|------|
| MimoNeko | 修改 README | | | | |
| OpenCode | 修改 README | | | | |
| Aider | 修改 README | | | | |
| MimoNeko | 修复 Bug | | | | |
| OpenCode | 修复 Bug | | | | |
| Aider | 修复 Bug | | | | |
| MimoNeko | 新增配置 | | | | |
| OpenCode | 新增配置 | | | | |
| Aider | 新增配置 | | | | |

---

## 汇总对比

### 总体评分

| 工具 | Token 效率 | 响应速度 | 缓存命中 | 成功率 | 总分 |
|------|------------|----------|----------|--------|------|
| MimoNeko | /5 | /5 | /5 | /5 | /20 |
| OpenCode | /5 | /5 | /5 | /5 | /20 |
| Aider | /5 | /5 | /5 | /5 | /20 |

### 优势分析

**MimoNeko 优势**:
- 
- 
- 

**OpenCode 优势**:
- 
- 
- 

**Aider 优势**:
- 
- 
- 

### 改进建议

**MimoNeko 改进方向**:
- 
- 
- 

---

## 测试注意事项

1. **公平性**: 所有工具使用相同的任务和环境
2. **可重复性**: 记录详细的测试步骤
3. **客观性**: 不伪造数据，如实记录
4. **完整性**: 记录所有指标，包括失败情况
5. **时效性**: 注明测试日期和版本

---

## 后续计划

1. **定期测试**: 每个版本发布后进行对比
2. **扩展任务**: 增加更多测试场景
3. **自动化**: 完善测试脚本
4. **公开结果**: 在 README 中公布对比结果
