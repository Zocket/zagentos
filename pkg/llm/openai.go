package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const openaiDefaultBaseURL = "https://api.openai.com/v1"

// OpenAIProvider 实现 OpenAI Chat Completions API
type OpenAIProvider struct {
	config ProviderConfig
	client *http.Client
}

// NewOpenAIProvider 创建 OpenAI provider
func NewOpenAIProvider(config ProviderConfig) *OpenAIProvider {
	if config.BaseURL == "" {
		config.BaseURL = openaiDefaultBaseURL
	}
	if config.HTTPTimeout == 0 {
		config.HTTPTimeout = 60 * time.Second
	}
	if config.Retry == (RetryConfig{}) {
		config.Retry = DefaultRetryConfig()
	}

	return &OpenAIProvider{
		config: config,
		client: &http.Client{Timeout: config.HTTPTimeout},
	}
}

func (p *OpenAIProvider) Name() string {
	return "openai"
}

func (p *OpenAIProvider) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	// Rate limiting
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

func (p *OpenAIProvider) doComplete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	model := req.Model
	if model == "" {
		model = p.config.Model
	}

	// 构造 OpenAI 请求体
	oaiReq := p.buildRequest(req, model, false)

	body, err := json.Marshal(oaiReq)
	if err != nil {
		return nil, fmt.Errorf("llm: failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.config.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("llm: failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.config.APIKey)

	httpResp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, &APIError{
			Provider:  "openai",
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

func (p *OpenAIProvider) Stream(ctx context.Context, req *CompletionRequest) (<-chan StreamChunk, error) {
	// Rate limiting
	if p.config.RateLimit != nil {
		if err := p.config.RateLimit.Wait(ctx); err != nil {
			return nil, err
		}
	}

	model := req.Model
	if model == "" {
		model = p.config.Model
	}

	oaiReq := p.buildRequest(req, model, true)

	body, err := json.Marshal(oaiReq)
	if err != nil {
		return nil, fmt.Errorf("llm: failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.config.BaseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("llm: failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+p.config.APIKey)

	httpResp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, &APIError{
			Provider:  "openai",
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
	go p.readSSEStream(ctx, httpResp.Body, ch)
	return ch, nil
}

func (p *OpenAIProvider) CountTokens(messages []Message) (int, error) {
	// 简单估算：每条消息约 4 token 开销，内容按 4 字符 1 token 估算
	total := 0
	for _, msg := range messages {
		total += 4 // message overhead
		total += len(msg.Content) / 4
		for _, tc := range msg.ToolCalls {
			total += len(tc.Name)/4 + len(tc.Arguments)/4
		}
	}
	return total, nil
}

// --- 内部辅助方法 ---

// openaiRequest 是发送给 OpenAI 的请求结构
type openaiRequest struct {
	Model       string             `json:"model"`
	Messages    []openaiMessage    `json:"messages"`
	Tools       []openaiTool       `json:"tools,omitempty"`
	MaxTokens   int                `json:"max_tokens,omitempty"`
	Temperature *float64           `json:"temperature,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
}

type openaiMessage struct {
	Role       string           `json:"role"`
	Content    string           `json:"content,omitempty"`
	ToolCalls  []openaiToolCall `json:"tool_calls,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
}

type openaiToolCall struct {
	ID       string             `json:"id"`
	Type     string             `json:"type"`
	Function openaiToolFunction `json:"function"`
}

type openaiToolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type openaiTool struct {
	Type     string             `json:"type"`
	Function openaiToolDef      `json:"function"`
}

type openaiToolDef struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Parameters  interface{} `json:"parameters"`
}

// openaiResponse 是 OpenAI 返回的响应结构
type openaiResponse struct {
	Choices []openaiChoice `json:"choices"`
	Usage   openaiUsage    `json:"usage"`
}

type openaiChoice struct {
	Message      openaiMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

type openaiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

// SSE 流相关类型
type openaiStreamChunk struct {
	Choices []openaiStreamChoice `json:"choices"`
	Usage   *openaiUsage         `json:"usage,omitempty"`
}

type openaiStreamChoice struct {
	Delta        openaiStreamDelta `json:"delta"`
	FinishReason *string           `json:"finish_reason"`
}

type openaiStreamDelta struct {
	Content   string           `json:"content,omitempty"`
	ToolCalls []openaiToolCall `json:"tool_calls,omitempty"`
}

func (p *OpenAIProvider) buildRequest(req *CompletionRequest, model string, stream bool) openaiRequest {
	oaiReq := openaiRequest{
		Model:  model,
		Stream: stream,
	}

	if req.MaxTokens > 0 {
		oaiReq.MaxTokens = req.MaxTokens
	}
	if req.Temperature > 0 {
		temp := req.Temperature
		oaiReq.Temperature = &temp
	}

	// 转换 messages
	for _, msg := range req.Messages {
		oaiMsg := openaiMessage{
			Role:       string(msg.Role),
			Content:    msg.Content,
			ToolCallID: msg.ToolCallID,
		}
		for _, tc := range msg.ToolCalls {
			oaiMsg.ToolCalls = append(oaiMsg.ToolCalls, openaiToolCall{
				ID:   tc.ID,
				Type: "function",
				Function: openaiToolFunction{
					Name:      tc.Name,
					Arguments: tc.Arguments,
				},
			})
		}
		oaiReq.Messages = append(oaiReq.Messages, oaiMsg)
	}

	// 转换 tools
	for _, tool := range req.Tools {
		oaiReq.Tools = append(oaiReq.Tools, openaiTool{
			Type: "function",
			Function: openaiToolDef{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.InputSchema,
			},
		})
	}

	return oaiReq
}

func (p *OpenAIProvider) parseResponse(body []byte) (*CompletionResponse, error) {
	var oaiResp openaiResponse
	if err := json.Unmarshal(body, &oaiResp); err != nil {
		return nil, fmt.Errorf("llm: failed to parse response: %w", err)
	}

	if len(oaiResp.Choices) == 0 {
		return nil, fmt.Errorf("llm: empty response from OpenAI")
	}

	choice := oaiResp.Choices[0]
	msg := Message{
		Role:    RoleAssistant,
		Content: choice.Message.Content,
	}

	for _, tc := range choice.Message.ToolCalls {
		msg.ToolCalls = append(msg.ToolCalls, ToolCall{
			ID:        tc.ID,
			Name:      tc.Function.Name,
			Arguments: tc.Function.Arguments,
		})
	}

	stopReason := choice.FinishReason
	if stopReason == "tool_calls" {
		stopReason = "tool_use"
	}

	return &CompletionResponse{
		Message: msg,
		Usage: Usage{
			InputTokens:  oaiResp.Usage.PromptTokens,
			OutputTokens: oaiResp.Usage.CompletionTokens,
		},
		StopReason: stopReason,
	}, nil
}

func (p *OpenAIProvider) parseError(statusCode int, body []byte) *APIError {
	apiErr := &APIError{
		Provider:   "openai",
		StatusCode: statusCode,
		Retryable:  statusCode == 429 || statusCode >= 500,
	}

	var errResp struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &errResp) == nil {
		apiErr.Type = errResp.Error.Type
		apiErr.Message = errResp.Error.Message
	} else {
		apiErr.Message = string(body)
	}

	return apiErr
}

func (p *OpenAIProvider) readSSEStream(ctx context.Context, body io.ReadCloser, ch chan<- StreamChunk) {
	defer close(ch)
	defer body.Close()

	buf := make([]byte, 4096)
	var accumulated string

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		n, err := body.Read(buf)
		if n > 0 {
			accumulated += string(buf[:n])

			// 按 SSE 事件分割处理
			for {
				idx := strings.Index(accumulated, "\n\n")
				if idx == -1 {
					break
				}
				event := accumulated[:idx]
				accumulated = accumulated[idx+2:]

				// 解析 data: 行
				for _, line := range strings.Split(event, "\n") {
					line = strings.TrimSpace(line)
					if !strings.HasPrefix(line, "data: ") {
						continue
					}
					data := strings.TrimPrefix(line, "data: ")
					if data == "[DONE]" {
						ch <- StreamChunk{Done: true}
						return
					}

					var chunk openaiStreamChunk
					if json.Unmarshal([]byte(data), &chunk) != nil {
						continue
					}

					for _, choice := range chunk.Choices {
						sc := StreamChunk{}
						if choice.Delta.Content != "" {
							sc.Delta = choice.Delta.Content
						}
						if len(choice.Delta.ToolCalls) > 0 {
							tc := choice.Delta.ToolCalls[0]
							sc.ToolCall = &ToolCall{
								ID:        tc.ID,
								Name:      tc.Function.Name,
								Arguments: tc.Function.Arguments,
							}
						}
						if chunk.Usage != nil {
							sc.Usage = &Usage{
								InputTokens:  chunk.Usage.PromptTokens,
								OutputTokens: chunk.Usage.CompletionTokens,
							}
						}
						ch <- sc
					}
				}
			}
		}
		if err != nil {
			if err != io.EOF {
				ch <- StreamChunk{Done: true}
			}
			return
		}
	}
}
