package loop

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/Zocket/zagentos/pkg/llm"
	"github.com/Zocket/zagentos/pkg/tool"
)

// reactLoop 是 Loop 接口的默认实现。
type reactLoop struct {
	provider llm.Provider
	registry tool.Registry
	executor *tool.Executor
	config   Config
	hooks    Hooks
	logger   *log.Logger
}

// NewLoop 创建一个 ReAct 循环。
func NewLoop(provider llm.Provider, registry tool.Registry, config Config) Loop {
	return &reactLoop{
		provider: provider,
		registry: registry,
		executor: tool.NewExecutor(registry, 0),
		config:   config,
	}
}

// NewLoopWithHooks 创建带 Hooks 的 ReAct 循环。
func NewLoopWithHooks(provider llm.Provider, registry tool.Registry, config Config, hooks Hooks) Loop {
	return &reactLoop{
		provider: provider,
		registry: registry,
		executor: tool.NewExecutor(registry, 0),
		config:   config,
		hooks:    hooks,
	}
}

// Run 执行一次完整的 agent 循环。
func (l *reactLoop) Run(ctx context.Context, messages []llm.Message) (*RunResult, error) {
	result := &RunResult{
		Steps: []Step{},
	}

	maxIter := l.config.MaxIterations
	if maxIter <= 0 {
		maxIter = 10 // 默认最大迭代
	}

	// 构建工具定义列表（从 registry 获取）
	tools := l.buildToolDefinitions()

	for iteration := 1; iteration <= maxIter; iteration++ {
		// 检查 context 是否取消
		if err := ctx.Err(); err != nil {
			result.StopReason = StopReasonCancelled
			return result, nil
		}

		// 构建 LLM 请求
		req := &llm.CompletionRequest{
			Messages:  messages,
			Tools:     tools,
			MaxTokens: l.config.MaxTokens,
		}

		// BeforeCompletion hook
		if l.hooks.BeforeCompletion != nil {
			if err := l.hooks.BeforeCompletion(ctx, req); err != nil {
				result.StopReason = StopReasonError
				return result, fmt.Errorf("loop: BeforeCompletion hook error: %w", err)
			}
		}

		if l.config.Verbose {
			l.logf("迭代 %d: 调用 LLM（消息数=%d）", iteration, len(messages))
		}

		// 调用 LLM
		resp, err := l.provider.Complete(ctx, req)
		if err != nil {
			result.StopReason = StopReasonError
			return result, fmt.Errorf("loop: LLM 调用失败（迭代 %d）: %w", iteration, err)
		}

		// AfterCompletion hook
		if l.hooks.AfterCompletion != nil {
			if err := l.hooks.AfterCompletion(ctx, resp); err != nil {
				result.StopReason = StopReasonError
				return result, fmt.Errorf("loop: AfterCompletion hook error: %w", err)
			}
		}

		// 累计 token 用量
		result.TotalUsage.InputTokens += resp.Usage.InputTokens
		result.TotalUsage.OutputTokens += resp.Usage.OutputTokens

		// 记录步骤
		step := Step{
			Iteration: iteration,
			Request:   req,
			Response:  resp,
		}

		if l.config.Verbose {
			l.logf("迭代 %d: LLM 返回（stop_reason=%s, tool_calls=%d）",
				iteration, resp.StopReason, len(resp.Message.ToolCalls))
		}

		// 如果没有工具调用，循环结束
		if len(resp.Message.ToolCalls) == 0 {
			result.FinalMessage = resp.Message.Content
			result.StopReason = StopReasonComplete
			result.Steps = append(result.Steps, step)
			return result, nil
		}

		// 将 assistant 消息（含 tool_calls）加入对话
		messages = append(messages, resp.Message)

		// 执行工具调用
		toolExecs := l.executeToolCalls(ctx, resp.Message.ToolCalls)
		step.ToolCalls = toolExecs
		result.Steps = append(result.Steps, step)

		// 将工具结果作为 tool 角色消息回填到对话
		for _, te := range toolExecs {
			content := ""
			if te.Result != nil {
				content = te.Result.Content
			}
			if te.Error != nil {
				content = fmt.Sprintf("Error: %v", te.Error)
			}
			messages = append(messages, llm.Message{
				Role:       llm.RoleTool,
				Content:    content,
				ToolCallID: te.CallID,
			})
		}
	}

	// 达到最大迭代次数
	result.StopReason = StopReasonMaxIter
	if len(result.Steps) > 0 {
		lastResp := result.Steps[len(result.Steps)-1].Response
		if lastResp != nil {
			result.FinalMessage = lastResp.Message.Content
		}
	}
	return result, nil
}

// executeToolCalls 批量执行工具调用（并行）。
func (l *reactLoop) executeToolCalls(ctx context.Context, calls []llm.ToolCall) []ToolExecution {
	results := make([]ToolExecution, len(calls))
	var wg sync.WaitGroup

	for i, call := range calls {
		wg.Add(1)
		go func(idx int, c llm.ToolCall) {
			defer wg.Done()
			results[idx] = l.executeSingleToolCall(ctx, c)
		}(i, call)
	}

	wg.Wait()
	return results
}

// executeSingleToolCall 执行单个工具调用。
func (l *reactLoop) executeSingleToolCall(ctx context.Context, call llm.ToolCall) ToolExecution {
	te := ToolExecution{
		CallID:   call.ID,
		ToolName: call.Name,
	}

	// 解析参数
	var input map[string]interface{}
	if call.Arguments != "" {
		if err := json.Unmarshal([]byte(call.Arguments), &input); err != nil {
			te.Error = fmt.Errorf("解析工具参数失败: %w", err)
			return te
		}
	}
	te.Input = input

	// BeforeToolCall hook
	if l.hooks.BeforeToolCall != nil {
		if err := l.hooks.BeforeToolCall(ctx, call); err != nil {
			te.Error = fmt.Errorf("BeforeToolCall hook 拒绝: %w", err)
			return te
		}
	}

	// 执行工具
	result := l.executor.Execute(ctx, tool.CallRequest{
		Name:  call.Name,
		Input: input,
	})
	te.Result = result.Result
	te.Error = result.Error

	// AfterToolCall hook
	if l.hooks.AfterToolCall != nil {
		if hookErr := l.hooks.AfterToolCall(ctx, call, te.Result); hookErr != nil {
			// hook 错误不覆盖工具结果，仅记录日志
			l.logf("AfterToolCall hook error: %v", hookErr)
		}
	}

	return te
}

// buildToolDefinitions 从 registry 构建工具定义列表。
func (l *reactLoop) buildToolDefinitions() []llm.ToolDefinition {
	tools := l.registry.List()
	defs := make([]llm.ToolDefinition, 0, len(tools))
	for _, t := range tools {
		defs = append(defs, llm.ToolDefinition{
			Name:        t.Name(),
			Description: t.Description(),
			InputSchema: t.InputSchema(),
		})
	}
	return defs
}

// logf 在 Verbose 模式下打印日志。
func (l *reactLoop) logf(format string, args ...interface{}) {
	if l.logger != nil {
		l.logger.Printf("[loop] "+format, args...)
	} else {
		log.Printf("[loop] "+format, args...)
	}
}

// SetLogger 设置日志记录器。
func (l *reactLoop) SetLogger(logger *log.Logger) {
	l.logger = logger
}

// RunWithTimeout 带超时的 Run。
func RunWithTimeout(ctx context.Context, loop Loop, messages []llm.Message, timeout time.Duration) (*RunResult, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return loop.Run(ctx, messages)
}
