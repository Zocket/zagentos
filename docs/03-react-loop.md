# P3: Tool Use Loop (ReAct Loop)

## 目标
实现 LLM → Tool Call → Result → LLM 的自主循环。

## 核心接口
见 `pkg/loop/loop.go`

## 实现要点
- [x] 基本 ReAct 循环
- [x] 解析 LLM 输出中的 tool_use block
- [x] 工具调用分发
- [x] 结果回填到对话
- [x] 最大迭代次数限制
- [x] 错误恢复策略
- [x] Pre/Post hook 触发
- [x] 并行 tool call 支持

## 设计决策

### 1. 循环结构
```
Run() 主循环：
  for iteration 1..MaxIterations:
    1. 检查 context 取消
    2. 构建 CompletionRequest（消息 + 工具定义）
    3. BeforeCompletion hook
    4. 调用 LLM
    5. AfterCompletion hook
    6. 累计 token 用量
    7. 记录 Step
    8. 如果无 tool_calls → 返回（StopReasonComplete）
    9. 将 assistant 消息加入对话
    10. 并行执行所有 tool_calls
    11. 将工具结果作为 tool 角色消息回填
  达到 MaxIterations → 返回（StopReasonMaxIter）
```

### 2. 工具调用分发
- 使用 `tool.Executor.ExecuteBatch` 并行执行同一轮内的多个工具调用
- 每个工具调用在独立 goroutine 中执行
- 结果顺序与请求顺序一致（通过索引保序）

### 3. 结果回填
- LLM 返回的 assistant 消息（含 tool_calls）追加到 messages
- 每个工具结果作为 `RoleTool` 消息追加，包含 `ToolCallID` 关联
- 工具执行错误时，错误信息作为 content 回填给 LLM，允许其重试或换策略

### 4. Hooks 机制
| Hook | 触发时机 | 可阻止 |
|------|---------|--------|
| BeforeCompletion | 每次 LLM 调用前 | 是（返回 error 终止循环） |
| AfterCompletion | 每次 LLM 响应后 | 是 |
| BeforeToolCall | 每次工具调用前 | 是（返回 error 阻止该次调用） |
| AfterToolCall | 每次工具调用后 | 否（仅记录日志） |

### 5. 停止原因
| StopReason | 含义 |
|---|---|
| `complete` | LLM 自行结束（无 tool call） |
| `max_iter` | 达到最大迭代次数 |
| `error` | LLM 调用或 hook 出错 |
| `cancelled` | context 被取消 |

### 6. 错误恢复
- 工具执行失败不会终止循环，错误信息回填给 LLM
- LLM 可以在下一轮决定重试、换工具或给出最终回复
- LLM 调用失败直接终止循环（StopReasonError）

## 测试
```bash
go test ./pkg/loop/ -v
```

### 测试用例（11 个）
| 测试 | 验证内容 |
|------|---------|
| TestLoop_NoToolCall | 无工具调用，直接回复 |
| TestLoop_SingleToolCall | 单次工具调用 + 结果回填 |
| TestLoop_MultipleToolCalls | 同一轮多个工具调用 |
| TestLoop_MaxIterations | 达到最大迭代次数 |
| TestLoop_ToolError | 工具执行出错，LLM 收到错误信息 |
| TestLoop_Hooks | 4 种 hook 都被正确触发 |
| TestLoop_HookBeforeCompletionError | hook 返回 error 时终止循环 |
| TestLoop_ContextCancelled | context 取消时返回 cancelled |
| TestLoop_Verbose | Verbose 模式日志输出 |
| TestLoop_ParallelToolCalls | 3 个 100ms 工具并行执行，总耗时 < 250ms |
| TestLoop_StepRecording | Step 记录完整（iteration/request/response/toolcalls） |

## 文件结构
```
pkg/loop/
├── loop.go        # 接口和类型定义
├── react.go       # ReAct 循环实现
└── loop_test.go   # 单元测试
```
