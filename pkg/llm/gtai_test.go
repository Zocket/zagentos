package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// gtaiTestConfig 从配置文件加载测试配置
type gtaiTestConfig struct {
	APIKey string `json:"api_key"`
	Model  string `json:"model"`
}

func loadGTAITestConfig(t *testing.T) gtaiTestConfig {
	t.Helper()

	// 配置文件路径：testdata/llm/gtai_test_config.json
	configPath := filepath.Join("..", "..", "testdata", "llm", "gtai_test_config.json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("failed to read config file: %v", err)
	}

	var cfg gtaiTestConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("failed to parse config file: %v", err)
	}

	return cfg
}

func TestGTAIProvider_Name(t *testing.T) {
	cfg := loadGTAITestConfig(t)

	provider := NewGTAIProvider(ProviderConfig{
		APIKey: cfg.APIKey,
		Model:  cfg.Model,
	})

	t.Logf("Testing with service from: %s", provider.Name())
	t.Logf("Testing with model: %s", provider.inner.config.Model)
	t.Logf("Testing API Key: %s", provider.inner.config.APIKey)

	if provider.Name() != "GreenTownAI" {
		t.Errorf("expected name 'GreenTownAI', got '%s'", provider.Name())
	}
}

func TestGTAIProvider_DefaultBaseURL(t *testing.T) {
	provider := NewGTAIProvider(ProviderConfig{Model: "test"})
	if provider.inner.config.BaseURL != gtaiDefaultBaseURL {
		t.Errorf("expected base URL %s, got %s", gtaiDefaultBaseURL, provider.inner.config.BaseURL)
	}
}

func TestGTAIProvider_Complete(t *testing.T) {
	cfg := loadGTAITestConfig(t)
	t.Logf("testing with model: %s", cfg.Model)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 验证请求路径
		if r.URL.Path != "/chat/completions" {
			t.Errorf("expected path /chat/completions, got %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer "+cfg.APIKey {
			t.Errorf("expected auth header 'Bearer %s', got: %s", cfg.APIKey, r.Header.Get("Authorization"))
		}

		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "Hello from GTAI!",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     12,
				"completion_tokens": 6,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewGTAIProvider(ProviderConfig{
		APIKey:  cfg.APIKey,
		BaseURL: server.URL,
		Model:   cfg.Model,
	})

	resp, err := provider.Complete(context.Background(), &CompletionRequest{
		Messages: []Message{
			{Role: RoleUser, Content: "你好"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Message.Content != "Hello from GTAI!" {
		t.Errorf("unexpected content: %s", resp.Message.Content)
	}
	if resp.Usage.InputTokens != 12 {
		t.Errorf("unexpected input tokens: %d", resp.Usage.InputTokens)
	}
	if resp.Usage.OutputTokens != 6 {
		t.Errorf("unexpected output tokens: %d", resp.Usage.OutputTokens)
	}
}

func TestGTAIProvider_ToolCall(t *testing.T) {
	cfg := loadGTAITestConfig(t)
	t.Logf("testing with model: %s", cfg.Model)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"role":    "assistant",
						"content": "",
						"tool_calls": []map[string]interface{}{
							{
								"id":   "call_gtai_001",
								"type": "function",
								"function": map[string]interface{}{
									"name":      "get_weather",
									"arguments": `{"city":"杭州"}`,
								},
							},
						},
					},
					"finish_reason": "tool_calls",
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     20,
				"completion_tokens": 15,
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	provider := NewGTAIProvider(ProviderConfig{
		APIKey:  cfg.APIKey,
		BaseURL: server.URL,
		Model:   cfg.Model,
	})

	resp, err := provider.Complete(context.Background(), &CompletionRequest{
		Messages: []Message{
			{Role: RoleUser, Content: "杭州天气怎么样？"},
		},
		Tools: []ToolDefinition{
			{
				Name:        "get_weather",
				Description: "获取城市天气",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"city": map[string]interface{}{"type": "string"},
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
	if tc.ID != "call_gtai_001" {
		t.Errorf("unexpected tool call id: %s", tc.ID)
	}
	if tc.Name != "get_weather" {
		t.Errorf("unexpected tool name: %s", tc.Name)
	}
	if tc.Arguments != `{"city":"杭州"}` {
		t.Errorf("unexpected arguments: %s", tc.Arguments)
	}
	if resp.StopReason != "tool_use" {
		t.Errorf("unexpected stop reason: %s", resp.StopReason)
	}
}

func TestGTAIProvider_APIError(t *testing.T) {
	cfg := loadGTAITestConfig(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": map[string]interface{}{
				"type":    "rate_limit_error",
				"message": "请求过于频繁",
			},
		})
	}))
	defer server.Close()

	provider := NewGTAIProvider(ProviderConfig{
		APIKey:  cfg.APIKey,
		BaseURL: server.URL,
		Model:   cfg.Model,
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
	if apiErr.StatusCode != 429 {
		t.Errorf("expected status 429, got %d", apiErr.StatusCode)
	}
	if !apiErr.Retryable {
		t.Error("429 should be retryable")
	}
}
