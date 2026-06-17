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

const (
	anthropicDefaultBaseURL = "https://api.anthropic.com/v1"
	anthropicAPIVersion     = "2023-06-01"
)

// AnthropicProvider 实现 Anthropic Messages API
type AnthropicProvider struct {
	config ProviderConfig
	client *http.Client
}

// NewAnthropicProvider 创建 Anthropic provider
func NewAnthropicProvider(config ProviderConfig) *AnthropicProvider {
	if config.BaseURL == "" {
		config.BaseURL = anthropicDefaultBaseURL
	}
	if config.HTTPTimeout == 0 {
		config.HTTPTimeout = 120 * time.Second
	}
	if config.Retry == (RetryConfig{}) {
		config.Retry = DefaultRetryConfig()
	}

	return &AnthropicProvider{
		config: config,
		client: &http.Client{Timeout: config.HTTPTimeout},
	}
}

func (p *AnthropicProvider) Name() string {
	return "anthropic"
}

func (p *AnthropicProvider) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
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

func (p *AnthropicProvider) doComplete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	model := req.Model
	if model == "" {
		model = p.config.Model
	}

	anthReq := p.buildRequest(req, model, false)

	body, err := json.Marshal(anthReq)
	if err != nil {
		return nil, fmt.Errorf("llm: failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.config.BaseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("llm: failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.config.APIKey)
	httpReq.Header.Set("anthropic-version", anthropicAPIVersion)

	httpResp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, &APIError{
			Provider:  "anthropic",
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

func (p *AnthropicProvider) Stream(ctx context.Context, req *CompletionRequest) (<-chan StreamChunk, error) {
	if p.config.RateLimit != nil {
		if err := p.config.RateLimit.Wait(ctx); err != nil {
			return nil, err
		}
	}

	model := req.Model
	if model == "" {
		model = p.config.Model
	}

	anthReq := p.buildRequest(req, model, true)

	body, err := json.Marshal(anthReq)
	if err != nil {
		return nil, fmt.Errorf("llm: failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.config.BaseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("llm: failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", p.config.APIKey)
	httpReq.Header.Set("anthropic-version", anthropicAPIVersion)

	httpResp, err := p.client.Do(httpReq)
	if err != nil {
		return nil, &APIError{
			Provider:  "anthropic",
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

func (p *AnthropicProvider) CountTokens(messages []Message) (int, error) {
	// Anthropic 对中文和英文的 token 比率不同
	// 简单估算：英文约 4 chars/token，中文约 2 chars/token
	total := 0
	for _, msg := range messages {
		total += 4 // message overhead
		// 混合估算
		for _, r := range msg.Content {
			if r > 127 {
				total += 1 // CJK 字符约 0.5-1 token
			}
		}
		total += len(msg.Content) / 4
	}
	return total, nil
}

// --- Anthropic API 数据结构 ---

type anthropicRequest struct {
	Model     string             `json:"model"`
	Messages  []anthropicMessage `json:"messages"`
	System    string             `json:"system,omitempty"`
	MaxTokens int                `json:"max_tokens"`
	Tools     []anthropicTool    `json:"tools,omitempty"`
	Stream    bool               `json:"stream,omitempty"`
}

type anthropicMessage struct {
	Role    string               `json:"role"`
	Content []anthropicContent   `json:"content"`
}

type anthropicContent struct {
	Type      string      `json:"type"`
	Text      string      `json:"text,omitempty"`
	ID        string      `json:"id,omitempty"`         // tool_use block
	Name      string      `json:"name,omitempty"`       // tool_use block
	Input     interface{} `json:"input,omitempty"`      // tool_use block
	ToolUseID string      `json:"tool_use_id,omitempty"` // tool_result block
	Content   string      `json:"content,omitempty"`    // tool_result 的文本内容（嵌套时用不同字段名）
}

type anthropicTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"input_schema"`
}

type anthropicResponse struct {
	Content    []anthropicContent `json:"content"`
	StopReason string             `json:"stop_reason"`
	Usage      anthropicUsage     `json:"usage"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// SSE event types
type anthropicSSEEvent struct {
	Type  string          `json:"type"`
	Index int             `json:"index,omitempty"`
	Delta json.RawMessage `json:"delta,omitempty"`
	Usage json.RawMessage `json:"usage,omitempty"`
}

type anthropicContentDelta struct {
	Type  string `json:"type"`
	Text  string `json:"text,omitempty"`
	// partial tool input
	PartialJSON string `json:"partial_json,omitempty"`
}

func (p *AnthropicProvider) buildRequest(req *CompletionRequest, model string, stream bool) anthropicRequest {
	anthReq := anthropicRequest{
		Model:     model,
		MaxTokens: req.MaxTokens,
		Stream:    stream,
	}
	if anthReq.MaxTokens == 0 {
		anthReq.MaxTokens = 4096
	}

	// 从 messages 中提取 system prompt
	var msgs []anthropicMessage
	for _, msg := range req.Messages {
		switch msg.Role {
		case RoleSystem:
			anthReq.System = msg.Content
		case RoleUser:
			msgs = append(msgs, anthropicMessage{
				Role: "user",
				Content: []anthropicContent{{Type: "text", Text: msg.Content}},
			})
		case RoleAssistant:
			am := anthropicMessage{Role: "assistant"}
			if msg.Content != "" {
				am.Content = append(am.Content, anthropicContent{Type: "text", Text: msg.Content})
			}
			for _, tc := range msg.ToolCalls {
				// 解析 arguments JSON string 为 interface{}
				var input interface{}
				json.Unmarshal([]byte(tc.Arguments), &input)
				am.Content = append(am.Content, anthropicContent{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Name,
					Input: input,
				})
			}
			msgs = append(msgs, am)
		case RoleTool:
			msgs = append(msgs, anthropicMessage{
				Role: "user",
				Content: []anthropicContent{{
					Type:      "tool_result",
					ToolUseID: msg.ToolCallID,
					Content:   msg.Content,
				}},
			})
		}
	}
	anthReq.Messages = msgs

	// 转换 tools
	for _, tool := range req.Tools {
		anthReq.Tools = append(anthReq.Tools, anthropicTool{
			Name:        tool.Name,
			Description: tool.Description,
			InputSchema: tool.InputSchema,
		})
	}

	return anthReq
}

func (p *AnthropicProvider) parseResponse(body []byte) (*CompletionResponse, error) {
	var anthResp anthropicResponse
	if err := json.Unmarshal(body, &anthResp); err != nil {
		return nil, fmt.Errorf("llm: failed to parse response: %w", err)
	}

	msg := Message{Role: RoleAssistant}

	for _, block := range anthResp.Content {
		switch block.Type {
		case "text":
			msg.Content += block.Text
		case "tool_use":
			inputJSON, _ := json.Marshal(block.Input)
			msg.ToolCalls = append(msg.ToolCalls, ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: string(inputJSON),
			})
		}
	}

	return &CompletionResponse{
		Message: msg,
		Usage: Usage{
			InputTokens:  anthResp.Usage.InputTokens,
			OutputTokens: anthResp.Usage.OutputTokens,
		},
		StopReason: anthResp.StopReason,
	}, nil
}

func (p *AnthropicProvider) parseError(statusCode int, body []byte) *APIError {
	apiErr := &APIError{
		Provider:   "anthropic",
		StatusCode: statusCode,
		Retryable:  statusCode == 429 || statusCode == 529 || statusCode >= 500,
	}

	var errResp struct {
		Type  string `json:"type"`
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

func (p *AnthropicProvider) readSSEStream(ctx context.Context, body io.ReadCloser, ch chan<- StreamChunk) {
	defer close(ch)
	defer body.Close()

	buf := make([]byte, 4096)
	var accumulated string
	var currentToolCall *ToolCall

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		n, err := body.Read(buf)
		if n > 0 {
			accumulated += string(buf[:n])

			for {
				idx := strings.Index(accumulated, "\n\n")
				if idx == -1 {
					break
				}
				event := accumulated[:idx]
				accumulated = accumulated[idx+2:]

				p.processSSEEvent(event, ch, &currentToolCall)
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

func (p *AnthropicProvider) processSSEEvent(event string, ch chan<- StreamChunk, currentToolCall **ToolCall) {
	var eventType, data string

	for _, line := range strings.Split(event, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "event: ") {
			eventType = strings.TrimPrefix(line, "event: ")
		} else if strings.HasPrefix(line, "data: ") {
			data = strings.TrimPrefix(line, "data: ")
		}
	}

	switch eventType {
	case "content_block_start":
		var block struct {
			ContentBlock anthropicContent `json:"content_block"`
		}
		if json.Unmarshal([]byte(data), &block) == nil && block.ContentBlock.Type == "tool_use" {
			*currentToolCall = &ToolCall{
				ID:   block.ContentBlock.ID,
				Name: block.ContentBlock.Name,
			}
		}

	case "content_block_delta":
		var delta struct {
			Delta struct {
				Type        string `json:"type"`
				Text        string `json:"text,omitempty"`
				PartialJSON string `json:"partial_json,omitempty"`
			} `json:"delta"`
		}
		if json.Unmarshal([]byte(data), &delta) == nil {
			switch delta.Delta.Type {
			case "text_delta":
				ch <- StreamChunk{Delta: delta.Delta.Text}
			case "input_json_delta":
				if *currentToolCall != nil {
					(*currentToolCall).Arguments += delta.Delta.PartialJSON
				}
			}
		}

	case "content_block_stop":
		if *currentToolCall != nil {
			ch <- StreamChunk{ToolCall: *currentToolCall}
			*currentToolCall = nil
		}

	case "message_delta":
		// 可能包含 usage
		var msgDelta struct {
			Usage *anthropicUsage `json:"usage,omitempty"`
		}
		if json.Unmarshal([]byte(data), &msgDelta) == nil && msgDelta.Usage != nil {
			ch <- StreamChunk{
				Usage: &Usage{
					InputTokens:  msgDelta.Usage.InputTokens,
					OutputTokens: msgDelta.Usage.OutputTokens,
				},
			}
		}

	case "message_stop":
		ch <- StreamChunk{Done: true}
	}
}
