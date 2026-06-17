package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const ollamaDefaultBaseURL = "http://localhost:11434"

// OllamaProvider 实现 Ollama 本地模型 API
type OllamaProvider struct {
	config ProviderConfig
	client *http.Client
}

// NewOllamaProvider 创建 Ollama provider
func NewOllamaProvider(config ProviderConfig) *OllamaProvider {
	if config.BaseURL == "" {
		config.BaseURL = ollamaDefaultBaseURL
	}
	if config.HTTPTimeout == 0 {
		config.HTTPTimeout = 300 * time.Second // 本地模型可能较慢
	}
	if config.Retry == (RetryConfig{}) {
		config.Retry = RetryConfig{
			MaxRetries:  2,
			InitialWait: 500 * time.Millisecond,
			MaxWait:     5 * time.Second,
			Multiplier:  2.0,
		}
	}

	return &OllamaProvider{
		config: config,
		client: &http.Client{Timeout: config.HTTPTimeout},
	}
}

func (p *OllamaProvider) Name() string {
	return "ollama"
}

func (p *OllamaProvider) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	if p.config.RateLimit != nil {
		if err := p.config.RateLimit.Wait(ctx); err != nil {
			return nil, err
		}
	}

	var resp *CompletionResponse
	err := DoWithRetry(ctx, p.config.Retry, func() error {
		var innerErr error
		resp, innerErr = p.doComplete(ctx, req)
		return innerErr
	})
	return resp, err
}

func (p *OllamaProvider) doComplete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	model := req.Model
	if model == "" {
		model = p.config.Model
	}

	ollamaReq := p.buildRequest(req, model)

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("llm: failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.config.BaseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("llm: failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, &APIError{
			Provider:  "ollama",
			Message:   err.Error(),
			Retryable: true,
		}
	}
	defer httpResp.Body.Close()

	respBody, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("llm: failed to read response: %w", err)
	}

	if httpResp.StatusCode != http.StatusOK {
		return nil, p.parseError(httpResp.StatusCode, respBody)
	}

	return p.parseResponse(respBody)
}

func (p *OllamaProvider) Stream(ctx context.Context, req *CompletionRequest) (<-chan StreamChunk, error) {
	if p.config.RateLimit != nil {
		if err := p.config.RateLimit.Wait(ctx); err != nil {
			return nil, err
		}
	}

	model := req.Model
	if model == "" {
		model = p.config.Model
	}

	ollamaReq := p.buildRequest(req, model)
	ollamaReq.Stream = true

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return nil, fmt.Errorf("llm: failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.config.BaseURL+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("llm: failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, &APIError{
			Provider:  "ollama",
			Message:   err.Error(),
			Retryable: true,
		}
	}

	if httpResp.StatusCode != http.StatusOK {
		defer httpResp.Body.Close()
		respBody, _ := io.ReadAll(httpResp.Body)
		return nil, p.parseError(httpResp.StatusCode, respBody)
	}

	ch := make(chan StreamChunk, 32)
	go p.readNDJSONStream(ctx, httpResp.Body, ch)
	return ch, nil
}

func (p *OllamaProvider) CountTokens(messages []Message) (int, error) {
	// 本地模型的 token 估算，相比 API 模型更粗糙
	total := 0
	for _, msg := range messages {
		total += 4
		total += len(msg.Content) / 4
	}
	return total, nil
}

// --- Ollama API 数据结构 ---

type ollamaRequest struct {
	Model    string           `json:"model"`
	Messages []ollamaMessage  `json:"messages"`
	Stream   bool             `json:"stream"`
	Tools    []ollamaTool     `json:"tools,omitempty"`
	Options  *ollamaOptions   `json:"options,omitempty"`
}

type ollamaMessage struct {
	Role      string         `json:"role"`
	Content   string         `json:"content"`
	ToolCalls []ollamaToolCall `json:"tool_calls,omitempty"`
}

type ollamaToolCall struct {
	Function ollamaToolFunction `json:"function"`
}

type ollamaToolFunction struct {
	Name      string      `json:"name"`
	Arguments interface{} `json:"arguments"`
}

type ollamaTool struct {
	Type     string          `json:"type"`
	Function ollamaToolDef   `json:"function"`
}

type ollamaToolDef struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

type ollamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
}

type ollamaResponse struct {
	Message          ollamaMessage `json:"message"`
	Done             bool          `json:"done"`
	TotalDuration    int64         `json:"total_duration,omitempty"`
	PromptEvalCount  int           `json:"prompt_eval_count,omitempty"`
	EvalCount        int           `json:"eval_count,omitempty"`
}

func (p *OllamaProvider) buildRequest(req *CompletionRequest, model string) ollamaRequest {
	ollamaReq := ollamaRequest{
		Model:  model,
		Stream: false,
	}

	// Options
	if req.Temperature > 0 || req.MaxTokens > 0 {
		opts := &ollamaOptions{}
		if req.Temperature > 0 {
			opts.Temperature = req.Temperature
		}
		if req.MaxTokens > 0 {
			opts.NumPredict = req.MaxTokens
		}
		ollamaReq.Options = opts
	}

	// 转换 messages
	for _, msg := range req.Messages {
		om := ollamaMessage{
			Role:    string(msg.Role),
			Content: msg.Content,
		}
		for _, tc := range msg.ToolCalls {
			var args interface{}
			json.Unmarshal([]byte(tc.Arguments), &args)
			om.ToolCalls = append(om.ToolCalls, ollamaToolCall{
				Function: ollamaToolFunction{
					Name:      tc.Name,
					Arguments: args,
				},
			})
		}
		ollamaReq.Messages = append(ollamaReq.Messages, om)
	}

	// 转换 tools
	for _, tool := range req.Tools {
		ollamaReq.Tools = append(ollamaReq.Tools, ollamaTool{
			Type: "function",
			Function: ollamaToolDef{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			},
		})
	}

	return ollamaReq
}

func (p *OllamaProvider) parseResponse(body []byte) (*CompletionResponse, error) {
	var ollamaResp ollamaResponse
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return nil, fmt.Errorf("llm: failed to parse response: %w", err)
	}

	msg := Message{
		Role:    RoleAssistant,
		Content: ollamaResp.Message.Content,
	}

	for _, tc := range ollamaResp.Message.ToolCalls {
		argsJSON, _ := json.Marshal(tc.Function.Arguments)
		msg.ToolCalls = append(msg.ToolCalls, ToolCall{
			ID:        fmt.Sprintf("call_%d", len(msg.ToolCalls)),
			Name:      tc.Function.Name,
			Arguments: string(argsJSON),
		})
	}

	stopReason := "end_turn"
	if len(msg.ToolCalls) > 0 {
		stopReason = "tool_use"
	}

	return &CompletionResponse{
		Message: msg,
		Usage: Usage{
			InputTokens:  ollamaResp.PromptEvalCount,
			OutputTokens: ollamaResp.EvalCount,
		},
		StopReason: stopReason,
	}, nil
}

func (p *OllamaProvider) parseError(statusCode int, body []byte) *APIError {
	return &APIError{
		Provider:   "ollama",
		StatusCode: statusCode,
		Message:    string(body),
		Retryable:  statusCode >= 500,
	}
}

func (p *OllamaProvider) readNDJSONStream(ctx context.Context, body io.ReadCloser, ch chan<- StreamChunk) {
	defer close(ch)
	defer body.Close()

	decoder := json.NewDecoder(body)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		var ollamaResp ollamaResponse
		if err := decoder.Decode(&ollamaResp); err != nil {
			if err != io.EOF {
				ch <- StreamChunk{Done: true}
			}
			return
		}

		if ollamaResp.Done {
			// 最终响应包含 usage 信息
			ch <- StreamChunk{
				Done: true,
				Usage: &Usage{
					InputTokens:  ollamaResp.PromptEvalCount,
					OutputTokens: ollamaResp.EvalCount,
				},
			}
			return
		}

		sc := StreamChunk{Delta: ollamaResp.Message.Content}
		if len(ollamaResp.Message.ToolCalls) > 0 {
			tc := ollamaResp.Message.ToolCalls[0]
			argsJSON, _ := json.Marshal(tc.Function.Arguments)
			sc.ToolCall = &ToolCall{
				Name:      tc.Function.Name,
				Arguments: string(argsJSON),
			}
		}
		ch <- sc
	}
}
