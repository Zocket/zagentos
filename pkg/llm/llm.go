// Package llm 提供统一的 LLM 调用抽象层。
// P1: LLM Gateway - 封装多个 provider 的 API。
package llm

import "context"

// Role 表示消息角色
type Role string

const (
	RoleSystem    Role = "system"
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleTool      Role = "tool"
)

// Message 表示对话中的一条消息
type Message struct {
	Role       Role        `json:"role"`
	Content    string      `json:"content"`
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
}

// ToolCall 表示 LLM 发起的工具调用请求
type ToolCall struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Arguments string `json:"arguments"` // JSON string
}

// ToolDefinition 描述一个可供 LLM 调用的工具
type ToolDefinition struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"input_schema"` // JSON Schema
}

// CompletionRequest 是调用 LLM 的请求
type CompletionRequest struct {
	Model       string           `json:"model"`
	Messages    []Message        `json:"messages"`
	Tools       []ToolDefinition `json:"tools,omitempty"`
	MaxTokens   int              `json:"max_tokens,omitempty"`
	Temperature float64          `json:"temperature,omitempty"`
	Stream      bool             `json:"stream,omitempty"`
}

// CompletionResponse 是 LLM 返回的完整响应
type CompletionResponse struct {
	Message    Message `json:"message"`
	Usage      Usage   `json:"usage"`
	StopReason string  `json:"stop_reason"`
}

// Usage 记录 token 消耗
type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// StreamChunk 是流式输出的一个片段
type StreamChunk struct {
	Delta      string    `json:"delta,omitempty"`
	ToolCall   *ToolCall `json:"tool_call,omitempty"`
	Done       bool      `json:"done"`
	Usage      *Usage    `json:"usage,omitempty"`
}

// Provider 是 LLM 提供商的统一接口
type Provider interface {
	// Complete 执行一次完整的对话补全
	Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error)

	// Stream 执行流式对话补全
	Stream(ctx context.Context, req *CompletionRequest) (<-chan StreamChunk, error)

	// CountTokens 估算消息的 token 数量
	CountTokens(messages []Message) (int, error)

	// Name 返回 provider 名称
	Name() string
}

// Gateway 管理多个 Provider，提供路由和 fallback
type Gateway interface {
	Provider

	// RegisterProvider 注册一个 LLM provider
	RegisterProvider(name string, provider Provider)

	// SetDefault 设置默认 provider
	SetDefault(name string) error
}
