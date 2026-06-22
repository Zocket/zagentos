package loop

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Zocket/zagentos/pkg/llm"
	"github.com/Zocket/zagentos/pkg/tool"
)

// --- Mock LLM Provider ---

type mockProvider struct {
	responses []*llm.CompletionResponse // 预设的响应序列
	calls     int32                     // 已调用次数
}

func (m *mockProvider) Name() string { return "mock" }

func (m *mockProvider) Complete(ctx context.Context, req *llm.CompletionRequest) (*llm.CompletionResponse, error) {
	idx := atomic.AddInt32(&m.calls, 1) - 1
	if int(idx) >= len(m.responses) {
		return nil, fmt.Errorf("mock: no more responses (call %d)", idx+1)
	}
	return m.responses[idx], nil
}

func (m *mockProvider) Stream(ctx context.Context, req *llm.CompletionRequest) (<-chan llm.StreamChunk, error) {
	return nil, fmt.Errorf("not implemented")
}

func (m *mockProvider) CountTokens(messages []llm.Message) (int, error) {
	return 0, nil
}

// --- Mock Tool ---

type mockTool struct {
	name        string
	description string
	execFn      func(ctx context.Context, input map[string]interface{}) (*tool.Result, error)
}

func (m *mockTool) Name() string                        { return m.name }
func (m *mockTool) Description() string                 { return m.description }
func (m *mockTool) InputSchema() map[string]interface{} { return nil }
func (m *mockTool) Execute(ctx context.Context, input map[string]interface{}) (*tool.Result, error) {
	return m.execFn(ctx, input)
}

// --- 测试用例 ---

func TestLoop_NoToolCall(t *testing.T) {
	// LLM 直接回复，不需要调工具
	provider := &mockProvider{
		responses: []*llm.CompletionResponse{
			{
				Message:    llm.Message{Role: llm.RoleAssistant, Content: "你好！我是助手。"},
				Usage:      llm.Usage{InputTokens: 10, OutputTokens: 8},
				StopReason: "stop",
			},
		},
	}

	registry := tool.NewRegistry()
	loop := NewLoop(provider, registry, Config{MaxIterations: 5})

	result, err := loop.Run(context.Background(), []llm.Message{
		{Role: llm.RoleUser, Content: "你好"},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if result.StopReason != StopReasonComplete {
		t.Errorf("expected StopReasonComplete, got %s", result.StopReason)
	}
	if result.FinalMessage != "你好！我是助手。" {
		t.Errorf("unexpected final message: %q", result.FinalMessage)
	}
	if len(result.Steps) != 1 {
		t.Errorf("expected 1 step, got %d", len(result.Steps))
	}
	if result.TotalUsage.InputTokens != 10 {
		t.Errorf("expected input tokens 10, got %d", result.TotalUsage.InputTokens)
	}
}

func TestLoop_SingleToolCall(t *testing.T) {
	// 第一轮：LLM 请求调用计算器
	// 第二轮：LLM 根据工具结果给出最终回复
	provider := &mockProvider{
		responses: []*llm.CompletionResponse{
			{
				Message: llm.Message{
					Role: llm.RoleAssistant,
					ToolCalls: []llm.ToolCall{
						{ID: "call_1", Name: "calculator", Arguments: `{"expr":"1+2"}`},
					},
				},
				Usage:      llm.Usage{InputTokens: 15, OutputTokens: 5},
				StopReason: "tool_use",
			},
			{
				Message:    llm.Message{Role: llm.RoleAssistant, Content: "1+2=3"},
				Usage:      llm.Usage{InputTokens: 20, OutputTokens: 3},
				StopReason: "stop",
			},
		},
	}

	registry := tool.NewRegistry()
	registry.Register(&mockTool{
		name:        "calculator",
		description: "计算器",
		execFn: func(ctx context.Context, input map[string]interface{}) (*tool.Result, error) {
			return &tool.Result{Content: "3"}, nil
		},
	})

	loop := NewLoop(provider, registry, Config{MaxIterations: 5})

	result, err := loop.Run(context.Background(), []llm.Message{
		{Role: llm.RoleUser, Content: "1+2等于几？"},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if result.StopReason != StopReasonComplete {
		t.Errorf("expected StopReasonComplete, got %s", result.StopReason)
	}
	if result.FinalMessage != "1+2=3" {
		t.Errorf("unexpected final message: %q", result.FinalMessage)
	}
	if len(result.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(result.Steps))
	}
	// 第一步应该有 tool call
	if len(result.Steps[0].ToolCalls) != 1 {
		t.Errorf("expected 1 tool call in step 1, got %d", len(result.Steps[0].ToolCalls))
	}
	if result.Steps[0].ToolCalls[0].ToolName != "calculator" {
		t.Errorf("expected tool 'calculator', got %q", result.Steps[0].ToolCalls[0].ToolName)
	}
	if result.Steps[0].ToolCalls[0].Result.Content != "3" {
		t.Errorf("expected result '3', got %q", result.Steps[0].ToolCalls[0].Result.Content)
	}
	// token 累计
	if result.TotalUsage.InputTokens != 35 {
		t.Errorf("expected total input 35, got %d", result.TotalUsage.InputTokens)
	}
}

func TestLoop_MultipleToolCalls(t *testing.T) {
	// LLM 在一轮中同时请求调用两个工具
	provider := &mockProvider{
		responses: []*llm.CompletionResponse{
			{
				Message: llm.Message{
					Role: llm.RoleAssistant,
					ToolCalls: []llm.ToolCall{
						{ID: "call_1", Name: "search", Arguments: `{"q":"天气"}`},
						{ID: "call_2", Name: "search", Arguments: `{"q":"杭州"}`},
					},
				},
				Usage:      llm.Usage{InputTokens: 15, OutputTokens: 10},
				StopReason: "tool_use",
			},
			{
				Message:    llm.Message{Role: llm.RoleAssistant, Content: "搜索完成"},
				Usage:      llm.Usage{InputTokens: 30, OutputTokens: 5},
				StopReason: "stop",
			},
		},
	}

	registry := tool.NewRegistry()
	registry.Register(&mockTool{
		name:        "search",
		description: "搜索",
		execFn: func(ctx context.Context, input map[string]interface{}) (*tool.Result, error) {
			q, _ := input["q"].(string)
			return &tool.Result{Content: "结果:" + q}, nil
		},
	})

	loop := NewLoop(provider, registry, Config{MaxIterations: 5})

	result, err := loop.Run(context.Background(), []llm.Message{
		{Role: llm.RoleUser, Content: "搜索天气和杭州"},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if len(result.Steps[0].ToolCalls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(result.Steps[0].ToolCalls))
	}
	// 两个工具调用都应该有结果
	for i, tc := range result.Steps[0].ToolCalls {
		if tc.Error != nil {
			t.Errorf("tool call %d error: %v", i, tc.Error)
		}
		if tc.Result == nil {
			t.Errorf("tool call %d has nil result", i)
		}
	}
}

func TestLoop_MaxIterations(t *testing.T) {
	// LLM 每次都请求调工具，永远不结束
	alwaysToolCall := &llm.CompletionResponse{
		Message: llm.Message{
			Role: llm.RoleAssistant,
			ToolCalls: []llm.ToolCall{
				{ID: "call_1", Name: "echo", Arguments: `{}`},
			},
		},
		Usage:      llm.Usage{InputTokens: 5, OutputTokens: 5},
		StopReason: "tool_use",
	}

	// 准备 10 个响应（足够多）
	responses := make([]*llm.CompletionResponse, 10)
	for i := range responses {
		responses[i] = alwaysToolCall
	}

	provider := &mockProvider{responses: responses}
	registry := tool.NewRegistry()
	registry.Register(&mockTool{
		name: "echo",
		execFn: func(ctx context.Context, input map[string]interface{}) (*tool.Result, error) {
			return &tool.Result{Content: "echo"}, nil
		},
	})

	loop := NewLoop(provider, registry, Config{MaxIterations: 3})

	result, err := loop.Run(context.Background(), []llm.Message{
		{Role: llm.RoleUser, Content: "loop"},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if result.StopReason != StopReasonMaxIter {
		t.Errorf("expected StopReasonMaxIter, got %s", result.StopReason)
	}
	if len(result.Steps) != 3 {
		t.Errorf("expected 3 steps, got %d", len(result.Steps))
	}
}

func TestLoop_ToolError(t *testing.T) {
	// 工具执行出错，LLM 应该收到错误信息并给出最终回复
	provider := &mockProvider{
		responses: []*llm.CompletionResponse{
			{
				Message: llm.Message{
					Role: llm.RoleAssistant,
					ToolCalls: []llm.ToolCall{
						{ID: "call_1", Name: "fail", Arguments: `{}`},
					},
				},
				Usage:      llm.Usage{InputTokens: 10, OutputTokens: 5},
				StopReason: "tool_use",
			},
			{
				Message:    llm.Message{Role: llm.RoleAssistant, Content: "工具出错了"},
				Usage:      llm.Usage{InputTokens: 15, OutputTokens: 3},
				StopReason: "stop",
			},
		},
	}

	registry := tool.NewRegistry()
	registry.Register(&mockTool{
		name: "fail",
		execFn: func(ctx context.Context, input map[string]interface{}) (*tool.Result, error) {
			return nil, fmt.Errorf("工具执行失败")
		},
	})

	loop := NewLoop(provider, registry, Config{MaxIterations: 5})

	result, err := loop.Run(context.Background(), []llm.Message{
		{Role: llm.RoleUser, Content: "调用会失败的工具"},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if result.StopReason != StopReasonComplete {
		t.Errorf("expected StopReasonComplete, got %s", result.StopReason)
	}
	// 工具调用应该记录了错误
	if result.Steps[0].ToolCalls[0].Error == nil {
		t.Error("expected tool call error")
	}
}

func TestLoop_Hooks(t *testing.T) {
	var beforeCompletion, afterCompletion, beforeTool, afterTool int32

	provider := &mockProvider{
		responses: []*llm.CompletionResponse{
			{
				Message: llm.Message{
					Role: llm.RoleAssistant,
					ToolCalls: []llm.ToolCall{
						{ID: "call_1", Name: "echo", Arguments: `{}`},
					},
				},
				Usage:      llm.Usage{InputTokens: 5, OutputTokens: 5},
				StopReason: "tool_use",
			},
			{
				Message:    llm.Message{Role: llm.RoleAssistant, Content: "done"},
				Usage:      llm.Usage{InputTokens: 10, OutputTokens: 2},
				StopReason: "stop",
			},
		},
	}

	registry := tool.NewRegistry()
	registry.Register(&mockTool{
		name: "echo",
		execFn: func(ctx context.Context, input map[string]interface{}) (*tool.Result, error) {
			return &tool.Result{Content: "echoed"}, nil
		},
	})

	hooks := Hooks{
		BeforeCompletion: func(ctx context.Context, req *llm.CompletionRequest) error {
			atomic.AddInt32(&beforeCompletion, 1)
			return nil
		},
		AfterCompletion: func(ctx context.Context, resp *llm.CompletionResponse) error {
			atomic.AddInt32(&afterCompletion, 1)
			return nil
		},
		BeforeToolCall: func(ctx context.Context, call llm.ToolCall) error {
			atomic.AddInt32(&beforeTool, 1)
			return nil
		},
		AfterToolCall: func(ctx context.Context, call llm.ToolCall, result *tool.Result) error {
			atomic.AddInt32(&afterTool, 1)
			return nil
		},
	}

	loop := NewLoopWithHooks(provider, registry, Config{MaxIterations: 5}, hooks)

	_, err := loop.Run(context.Background(), []llm.Message{
		{Role: llm.RoleUser, Content: "test"},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if beforeCompletion != 2 {
		t.Errorf("expected BeforeCompletion 2, got %d", beforeCompletion)
	}
	if afterCompletion != 2 {
		t.Errorf("expected AfterCompletion 2, got %d", afterCompletion)
	}
	if beforeTool != 1 {
		t.Errorf("expected BeforeToolCall 1, got %d", beforeTool)
	}
	if afterTool != 1 {
		t.Errorf("expected AfterToolCall 1, got %d", afterTool)
	}
}

func TestLoop_HookBeforeCompletionError(t *testing.T) {
	provider := &mockProvider{
		responses: []*llm.CompletionResponse{
			{Message: llm.Message{Role: llm.RoleAssistant, Content: "ok"}},
		},
	}

	registry := tool.NewRegistry()
	hooks := Hooks{
		BeforeCompletion: func(ctx context.Context, req *llm.CompletionRequest) error {
			return fmt.Errorf("hook blocked")
		},
	}

	loop := NewLoopWithHooks(provider, registry, Config{MaxIterations: 5}, hooks)

	result, err := loop.Run(context.Background(), []llm.Message{
		{Role: llm.RoleUser, Content: "test"},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if result.StopReason != StopReasonError {
		t.Errorf("expected StopReasonError, got %s", result.StopReason)
	}
}

func TestLoop_ContextCancelled(t *testing.T) {
	provider := &mockProvider{
		responses: []*llm.CompletionResponse{
			{Message: llm.Message{Role: llm.RoleAssistant, Content: "ok"}},
		},
	}

	registry := tool.NewRegistry()
	loop := NewLoop(provider, registry, Config{MaxIterations: 5})

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // 立即取消

	result, err := loop.Run(ctx, []llm.Message{
		{Role: llm.RoleUser, Content: "test"},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if result.StopReason != StopReasonCancelled {
		t.Errorf("expected StopReasonCancelled, got %s", result.StopReason)
	}
}

func TestLoop_Verbose(t *testing.T) {
	provider := &mockProvider{
		responses: []*llm.CompletionResponse{
			{
				Message:    llm.Message{Role: llm.RoleAssistant, Content: "verbose test"},
				Usage:      llm.Usage{InputTokens: 1, OutputTokens: 1},
				StopReason: "stop",
			},
		},
	}

	registry := tool.NewRegistry()
	loop := NewLoop(provider, registry, Config{MaxIterations: 5, Verbose: true})

	result, err := loop.Run(context.Background(), []llm.Message{
		{Role: llm.RoleUser, Content: "test"},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if result.FinalMessage != "verbose test" {
		t.Errorf("unexpected message: %q", result.FinalMessage)
	}
}

func TestLoop_ParallelToolCalls(t *testing.T) {
	// 验证并行 tool call 确实并行执行（通过时间验证）
	provider := &mockProvider{
		responses: []*llm.CompletionResponse{
			{
				Message: llm.Message{
					Role: llm.RoleAssistant,
					ToolCalls: []llm.ToolCall{
						{ID: "call_1", Name: "delay", Arguments: `{}`},
						{ID: "call_2", Name: "delay", Arguments: `{}`},
						{ID: "call_3", Name: "delay", Arguments: `{}`},
					},
				},
				Usage:      llm.Usage{InputTokens: 10, OutputTokens: 10},
				StopReason: "tool_use",
			},
			{
				Message:    llm.Message{Role: llm.RoleAssistant, Content: "all done"},
				Usage:      llm.Usage{InputTokens: 20, OutputTokens: 2},
				StopReason: "stop",
			},
		},
	}

	registry := tool.NewRegistry()
	registry.Register(&mockTool{
		name: "delay",
		execFn: func(ctx context.Context, input map[string]interface{}) (*tool.Result, error) {
			time.Sleep(100 * time.Millisecond)
			return &tool.Result{Content: "done"}, nil
		},
	})

	loop := NewLoop(provider, registry, Config{MaxIterations: 5})

	start := time.Now()
	result, err := loop.Run(context.Background(), []llm.Message{
		{Role: llm.RoleUser, Content: "parallel test"},
	})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	if result.StopReason != StopReasonComplete {
		t.Errorf("expected complete, got %s", result.StopReason)
	}
	// 3 个 100ms 的工具并行执行，总耗时应远小于 300ms
	if elapsed > 250*time.Millisecond {
		t.Errorf("parallel execution took too long: %v", elapsed)
	}
}

func TestLoop_TooICallArguments(t *testing.T) {
	// 工具参数 JSON 解析失败
	provider := &mockProvider{
		responses: []*llm.CompletionResponse{
			{
				Message: llm.Message{
					Role: llm.RoleAssistant,
					ToolCalls: []llm.ToolCall{
						{ID: "call_1", Name: "echo", Arguments: `invalid json`},
					},
				},
				Usage:      llm.Usage{InputTokens: 5, OutputTokens: 5},
				StopReason: "tool_use",
			},
			{
				Message:    llm.Message{Role: llm.RoleAssistant, Content: "参数错误已修复"},
				Usage:      llm.Usage{InputTokens: 10, OutputTokens: 3},
				StopReason: "stop",
			},
		},
	}

	registry := tool.NewRegistry()
	registry.Register(&mockTool{
		name: "echo",
		execFn: func(ctx context.Context, input map[string]interface{}) (*tool.Result, error) {
			return &tool.Result{Content: "ok"}, nil
		},
	})

	loop := NewLoop(provider, registry, Config{MaxIterations: 5})

	result, err := loop.Run(context.Background(), []llm.Message{
		{Role: llm.RoleUser, Content: "test"},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}
	// 工具调用应该记录了 JSON 解析错误
	if result.Steps[0].ToolCalls[0].Error == nil {
		t.Error("expected JSON parse error")
	}
}

func TestLoop_StepRecording(t *testing.T) {
	// 验证每一步的记录是否完整
	provider := &mockProvider{
		responses: []*llm.CompletionResponse{
			{
				Message: llm.Message{
					Role: llm.RoleAssistant,
					ToolCalls: []llm.ToolCall{
						{ID: "c1", Name: "add", Arguments: `{"a":1,"b":2}`},
					},
				},
				Usage:      llm.Usage{InputTokens: 10, OutputTokens: 5},
				StopReason: "tool_use",
			},
			{
				Message:    llm.Message{Role: llm.RoleAssistant, Content: "结果是3"},
				Usage:      llm.Usage{InputTokens: 15, OutputTokens: 3},
				StopReason: "stop",
			},
		},
	}

	registry := tool.NewRegistry()
	registry.Register(&mockTool{
		name: "add",
		execFn: func(ctx context.Context, input map[string]interface{}) (*tool.Result, error) {
			a, _ := input["a"].(float64)
			b, _ := input["b"].(float64)
			return &tool.Result{Content: fmt.Sprintf("%v", a+b)}, nil
		},
	})

	loop := NewLoop(provider, registry, Config{MaxIterations: 5, Verbose: true})

	result, err := loop.Run(context.Background(), []llm.Message{
		{Role: llm.RoleUser, Content: "1+2"},
	})
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	// 验证 step 记录
	if len(result.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d", len(result.Steps))
	}

	// 第一步：有 tool call
	s1 := result.Steps[0]
	if s1.Iteration != 1 {
		t.Errorf("expected iteration 1, got %d", s1.Iteration)
	}
	if s1.Request == nil || s1.Response == nil {
		t.Error("expected non-nil request and response")
	}
	if len(s1.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(s1.ToolCalls))
	}
	if s1.ToolCalls[0].CallID != "c1" {
		t.Errorf("expected call ID 'c1', got %q", s1.ToolCalls[0].CallID)
	}

	// 验证 input 被正确解析
	inputMap, ok := s1.ToolCalls[0].Input.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map input, got %T", s1.ToolCalls[0].Input)
	}
	if inputMap["a"] != float64(1) {
		t.Errorf("expected a=1, got %v", inputMap["a"])
	}

	// 第二步：无 tool call
	s2 := result.Steps[1]
	if s2.Iteration != 2 {
		t.Errorf("expected iteration 2, got %d", s2.Iteration)
	}
	if len(s2.ToolCalls) != 0 {
		t.Errorf("expected 0 tool calls in step 2, got %d", len(s2.ToolCalls))
	}

	// 验证 JSON 序列化
	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("failed to marshal result: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty JSON")
	}
}
