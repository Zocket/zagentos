package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

// --- RateLimiter 测试 ---

func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter(2, 10) // 容量2, 每秒补充10个

	// 前两次应该立即通过
	if !rl.Allow() {
		t.Fatal("expected first Allow to succeed")
	}
	if !rl.Allow() {
		t.Fatal("expected second Allow to succeed")
	}
	// 第三次应该失败（桶空了）
	if rl.Allow() {
		t.Fatal("expected third Allow to fail")
	}
}

func TestRateLimiter_Wait(t *testing.T) {
	rl := NewRateLimiter(1, 100) // 容量1, 每秒补充100个

	ctx := context.Background()
	// 第一次立即通过
	if err := rl.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	// 第二次需要等待一小段时间
	start := time.Now()
	if err := rl.Wait(ctx); err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)
	if elapsed > 50*time.Millisecond {
		t.Fatalf("wait too long: %v", elapsed)
	}
}

func TestRateLimiter_WaitCancelled(t *testing.T) {
	rl := NewRateLimiter(0, 1) // 桶空，每秒才补充1个

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err := rl.Wait(ctx)
	if err == nil {
		t.Fatal("expected error due to context cancellation")
	}
}

// --- DoWithRetry 测试 ---

func TestDoWithRetry_Success(t *testing.T) {
	var attempts int
	err := DoWithRetry(context.Background(), RetryConfig{
		MaxRetries:  3,
		InitialWait: time.Millisecond,
		MaxWait:     10 * time.Millisecond,
		Multiplier:  2,
	}, func() error {
		attempts++
		if attempts < 3 {
			return &APIError{Provider: "test", StatusCode: 500, Retryable: true}
		}
		return nil
	})

	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestDoWithRetry_NonRetryable(t *testing.T) {
	var attempts int
	err := DoWithRetry(context.Background(), RetryConfig{
		MaxRetries:  3,
		InitialWait: time.Millisecond,
		MaxWait:     10 * time.Millisecond,
		Multiplier:  2,
	}, func() error {
		attempts++
		return &APIError{Provider: "test", StatusCode: 400, Retryable: false}
	})

	if err == nil {
		t.Fatal("expected error")
	}
	if attempts != 1 {
		t.Fatalf("expected 1 attempt (non-retryable), got %d", attempts)
	}
}

// --- OpenAI Provider 测试（使用 mock server）---

func TestOpenAIProvider_Complete(t *testing.T) {
	// Mock OpenAI API
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证请求
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("expected auth header, got: %s", r.Header.Get("Authorization"))
		}

		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Hello! How can I help you?",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     10,
				"completion_tokens": 8,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewOpenAIProvider(ProviderConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
		Model:   "gpt-4o",
	})

	resp, err := provider.Complete(context.Background(), &CompletionRequest{
		Messages: []Message{
			{Role: RoleUser, Content: "Hello"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Message.Content != "Hello! How can I help you?" {
		t.Errorf("unexpected content: %s", resp.Message.Content)
	}
	if resp.Usage.InputTokens != 10 {
		t.Errorf("unexpected input tokens: %d", resp.Usage.InputTokens)
	}
	if resp.Usage.OutputTokens != 8 {
		t.Errorf("unexpected output tokens: %d", resp.Usage.OutputTokens)
	}
}

func TestOpenAIProvider_ToolCall(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "",
						"tool_calls": []map[string]interface{}{
							{
								"id":   "call_123",
								"type": "function",
								"function": map[string]interface{}{
									"name":      "calculator",
									"arguments": `{"expression":"2+2"}`,
								},
							},
						},
					},
					"finish_reason": "tool_calls",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     15,
				"completion_tokens": 20,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewOpenAIProvider(ProviderConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
		Model:   "gpt-4o",
	})

	resp, err := provider.Complete(context.Background(), &CompletionRequest{
		Messages: []Message{
			{Role: RoleUser, Content: "What is 2+2?"},
		},
		Tools: []ToolDefinition{
			{
				Name:        "calculator",
				Description: "Perform calculations",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"expression": map[string]interface{}{"type": "string"},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(resp.Message.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.Message.ToolCalls))
	}
	tc := resp.Message.ToolCalls[0]
	if tc.ID != "call_123" {
		t.Errorf("unexpected tool call id: %s", tc.ID)
	}
	if tc.Name != "calculator" {
		t.Errorf("unexpected tool name: %s", tc.Name)
	}
	if tc.Arguments != `{"expression":"2+2"}` {
		t.Errorf("unexpected arguments: %s", tc.Arguments)
	}
	if resp.StopReason != "tool_use" {
		t.Errorf("unexpected stop reason: %s", resp.StopReason)
	}
}

func TestOpenAIProvider_APIError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"type":    "authentication_error",
				"message": "Invalid API key",
			},
		})
	}))
	defer server.Close()

	provider := NewOpenAIProvider(ProviderConfig{
		APIKey:  "bad-key",
		BaseURL: server.URL,
		Model:   "gpt-4o",
		Retry:   NoRetry(),
	})

	_, err := provider.Complete(context.Background(), &CompletionRequest{
		Messages: []Message{{Role: RoleUser, Content: "test"}},
	})
	if err == nil {
		t.Fatal("expected error")
	}

	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected APIError, got: %T", err)
	}
	if apiErr.StatusCode != 401 {
		t.Errorf("expected status 401, got %d", apiErr.StatusCode)
	}
	if apiErr.Retryable {
		t.Error("401 should not be retryable")
	}
}

// --- Anthropic Provider 测试 ---

func TestAnthropicProvider_Complete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("x-api-key") != "test-key" {
			t.Errorf("expected x-api-key header")
		}
		if r.Header.Get("anthropic-version") != anthropicAPIVersion {
			t.Errorf("expected anthropic-version header")
		}

		resp := map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": "Hello from Claude!"},
			},
			"stop_reason": "end_turn",
			"usage": map[string]interface{}{
				"input_tokens":  12,
				"output_tokens": 5,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewAnthropicProvider(ProviderConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
		Model:   "claude-3-5-sonnet-20241022",
	})

	resp, err := provider.Complete(context.Background(), &CompletionRequest{
		Messages: []Message{
			{Role: RoleUser, Content: "Hello"},
		},
		MaxTokens: 1024,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Message.Content != "Hello from Claude!" {
		t.Errorf("unexpected content: %s", resp.Message.Content)
	}
	if resp.Usage.InputTokens != 12 {
		t.Errorf("unexpected input tokens: %d", resp.Usage.InputTokens)
	}
}

func TestAnthropicProvider_ToolUse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"content": []map[string]interface{}{
				{"type": "text", "text": "Let me calculate that."},
				{
					"type":  "tool_use",
					"id":    "toolu_123",
					"name":  "calculator",
					"input": map[string]interface{}{"expression": "2+2"},
				},
			},
			"stop_reason": "tool_use",
			"usage": map[string]interface{}{
				"input_tokens":  20,
				"output_tokens": 30,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewAnthropicProvider(ProviderConfig{
		APIKey:  "test-key",
		BaseURL: server.URL,
		Model:   "claude-3-5-sonnet-20241022",
	})

	resp, err := provider.Complete(context.Background(), &CompletionRequest{
		Messages: []Message{
			{Role: RoleUser, Content: "What is 2+2?"},
		},
		Tools: []ToolDefinition{
			{Name: "calculator", Description: "calc", InputSchema: map[string]interface{}{}},
		},
		MaxTokens: 1024,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Message.Content != "Let me calculate that." {
		t.Errorf("unexpected content: %s", resp.Message.Content)
	}
	if len(resp.Message.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.Message.ToolCalls))
	}
	if resp.Message.ToolCalls[0].Name != "calculator" {
		t.Errorf("unexpected tool name: %s", resp.Message.ToolCalls[0].Name)
	}
	if resp.StopReason != "tool_use" {
		t.Errorf("unexpected stop reason: %s", resp.StopReason)
	}
}

// --- Ollama Provider 测试 ---

func TestOllamaProvider_Complete(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"message": map[string]interface{}{
				"role":    "assistant",
				"content": "Hello from Ollama!",
			},
			"done":              true,
			"prompt_eval_count": 8,
			"eval_count":        5,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewOllamaProvider(ProviderConfig{
		BaseURL: server.URL,
		Model:   "llama3",
	})

	resp, err := provider.Complete(context.Background(), &CompletionRequest{
		Messages: []Message{
			{Role: RoleUser, Content: "Hello"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.Message.Content != "Hello from Ollama!" {
		t.Errorf("unexpected content: %s", resp.Message.Content)
	}
	if resp.Usage.InputTokens != 8 {
		t.Errorf("unexpected input tokens: %d", resp.Usage.InputTokens)
	}
}

// --- Gateway 测试 ---

func TestGateway_FallbackOnError(t *testing.T) {
	var primaryCalls, backupCalls int32

	// Primary: 总是失败
	primaryServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&primaryCalls, 1)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprint(w, `{"error":{"type":"server_error","message":"internal error"}}`)
	}))
	defer primaryServer.Close()

	// Backup: 总是成功
	backupServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&backupCalls, 1)
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message":       map[string]interface{}{"role": "assistant", "content": "from backup"},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]interface{}{"prompt_tokens": 5, "completion_tokens": 3},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer backupServer.Close()

	gw := NewGateway()
	gw.RegisterProvider("primary", NewOpenAIProvider(ProviderConfig{
		APIKey:  "key",
		BaseURL: primaryServer.URL,
		Model:   "gpt-4o",
		Retry:   NoRetry(),
	}))
	gw.RegisterProvider("backup", NewOpenAIProvider(ProviderConfig{
		APIKey:  "key",
		BaseURL: backupServer.URL,
		Model:   "gpt-4o-mini",
		Retry:   NoRetry(),
	}))
	gw.SetDefault("primary")

	resp, err := gw.Complete(context.Background(), &CompletionRequest{
		Messages: []Message{{Role: RoleUser, Content: "test"}},
	})
	if err != nil {
		t.Fatalf("expected fallback to succeed, got: %v", err)
	}
	if resp.Message.Content != "from backup" {
		t.Errorf("unexpected content: %s", resp.Message.Content)
	}
	if atomic.LoadInt32(&primaryCalls) == 0 {
		t.Error("expected primary to be called")
	}
	if atomic.LoadInt32(&backupCalls) == 0 {
		t.Error("expected backup to be called")
	}
}

func TestGateway_NoDefault(t *testing.T) {
	gw := NewGateway()
	_, err := gw.Complete(context.Background(), &CompletionRequest{
		Messages: []Message{{Role: RoleUser, Content: "test"}},
	})
	if err != ErrNoDefault {
		t.Fatalf("expected ErrNoDefault, got: %v", err)
	}
}

// --- Token 计数测试 ---

func TestOpenAI_CountTokens(t *testing.T) {
	provider := NewOpenAIProvider(ProviderConfig{Model: "gpt-4o"})
	count, err := provider.CountTokens([]Message{
		{Role: RoleUser, Content: "Hello, how are you?"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if count <= 0 {
		t.Errorf("expected positive count, got %d", count)
	}
}
