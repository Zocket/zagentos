# P3: Tool Use Loop (ReAct Loop)

## 目标
实现 LLM → Tool Call → Result → LLM 的自主循环。

## 核心接口
见 `pkg/loop/loop.go`

## 实现要点
- [ ] 基本 ReAct 循环
- [ ] 解析 LLM 输出中的 tool_use block
- [ ] 工具调用分发
- [ ] 结果回填到对话
- [ ] 最大迭代次数限制
- [ ] 错误恢复策略
- [ ] Pre/Post hook 触发
- [ ] 并行 tool call 支持

## 设计决策
_待填写_
