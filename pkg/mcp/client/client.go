// Package client 实现 MCP Client。
// P5: 发现和连接 MCP Server，调用其暴露的 tool/resource。
package client

import "context"

// ServerConfig 描述如何连接一个 MCP Server
type ServerConfig struct {
	Name    string            `json:"name"`
	Command string            `json:"command,omitempty"`  // stdio 模式
	Args    []string          `json:"args,omitempty"`
	URL     string            `json:"url,omitempty"`      // HTTP 模式
	Env     map[string]string `json:"env,omitempty"`
}

// ConnectionState 表示与 server 的连接状态
type ConnectionState string

const (
	StateDisconnected ConnectionState = "disconnected"
	StateConnecting   ConnectionState = "connecting"
	StateConnected    ConnectionState = "connected"
	StateError        ConnectionState = "error"
)

// ToolInfo 描述一个远程 tool 的信息
type ToolInfo struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema interface{} `json:"inputSchema"`
}

// Client 是 MCP Client 的接口
type Client interface {
	// Connect 连接到 MCP Server
	Connect(ctx context.Context) error

	// Disconnect 断开连接
	Disconnect(ctx context.Context) error

	// State 返回当前连接状态
	State() ConnectionState

	// ListTools 列出 server 暴露的所有 tools
	ListTools(ctx context.Context) ([]ToolInfo, error)

	// CallTool 调用远程 tool
	CallTool(ctx context.Context, name string, args map[string]interface{}) (interface{}, error)

	// ListResources 列出 server 暴露的资源
	ListResources(ctx context.Context) ([]Resource, error)

	// ReadResource 读取指定资源
	ReadResource(ctx context.Context, uri string) ([]byte, error)
}

// Resource 描述一个远程 resource
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}
