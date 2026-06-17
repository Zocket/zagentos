// Package loop 实现 Agent 的 Tool Use Loop (ReAct Loop)。
// P3: LLM → Tool Call → Result → LLM 的自主循环。
package loop

import (
	"context"

	"github.com/Zocket/zagentos/pkg/llm"
	"github.com/Zocket/zagentos/pkg/tool"
)

// StopReason 表示循环终止的原因
type StopReason string

const (
	StopReasonComplete  StopReason = "complete"  // LLM 自行结束
	StopReasonMaxIter   StopReason = "max_iter"  // 达到最大迭代次数
	StopReasonError     StopReason = "error"     // 发生错误
	StopReasonCancelled StopReason = "cancelled" // 被外部取消
)

// Config 是循环的配置
type Config struct {
	MaxIterations int  // 最大迭代次数
	MaxTokens     int  // 单次调用最大 token
	Verbose       bool // 是否输出详细日志
}

// Step 记录循环中的一步
type Step struct {
	Iteration int                      `json:"iteration"`
	Request   *llm.CompletionRequest   `json:"request,omitempty"`
	Response  *llm.CompletionResponse  `json:"response,omitempty"`
	ToolCalls []ToolExecution          `json:"tool_calls,omitempty"`
}

// ToolExecution 记录一次工具调用的执行
type ToolExecution struct {
	CallID   string       `json:"call_id"`
	ToolName string       `json:"tool_name"`
	Input    interface{}  `json:"input"`
	Result   *tool.Result `json:"result"`
	Error    error        `json:"error,omitempty"`
}

// RunResult 是一次完整循环的结果
type RunResult struct {
	FinalMessage string     `json:"final_message"`
	Steps        []Step     `json:"steps"`
	StopReason   StopReason `json:"stop_reason"`
	TotalUsage   llm.Usage  `json:"total_usage"`
}

// Loop 是 ReAct 循环的接口
type Loop interface {
	// Run 执行一次完整的 agent 循环
	Run(ctx context.Context, messages []llm.Message) (*RunResult, error)
}

// Hooks 允许在循环的各个阶段注入逻辑
type Hooks struct {
	// BeforeCompletion 在每次 LLM 调用前触发
	BeforeCompletion func(ctx context.Context, req *llm.CompletionRequest) error

	// AfterCompletion 在每次 LLM 响应后触发
	AfterCompletion func(ctx context.Context, resp *llm.CompletionResponse) error

	// BeforeToolCall 在工具调用前触发
	BeforeToolCall func(ctx context.Context, call llm.ToolCall) error

	// AfterToolCall 在工具调用后触发
	AfterToolCall func(ctx context.Context, call llm.ToolCall, result *tool.Result) error
}
