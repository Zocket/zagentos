// Package tool 提供 Tool Protocol Runtime。
// P2: 定义 Tool 的标准接口，支持 JSON-RPC 通信。
package tool

import "context"

// Tool 是一个可被 Agent 调用的工具
type Tool interface {
	// Name 返回工具名称
	Name() string

	// Description 返回工具的描述
	Description() string

	// InputSchema 返回工具输入的 JSON Schema
	InputSchema() map[string]interface{}

	// Execute 执行工具，接收 JSON 参数，返回 JSON 结果
	Execute(ctx context.Context, input map[string]interface{}) (*Result, error)
}

// Result 是工具执行的结果
type Result struct {
	Content string `json:"content"`
	IsError bool   `json:"is_error,omitempty"`
}

// Registry 管理已注册的工具
type Registry interface {
	// Register 注册一个工具
	Register(tool Tool) error

	// Get 根据名称获取工具
	Get(name string) (Tool, bool)

	// List 列出所有已注册的工具
	List() []Tool

	// Unregister 移除一个工具
	Unregister(name string) error
}

// Transport 定义工具调用的传输层
type Transport interface {
	// Serve 启动服务，监听工具调用请求
	Serve(ctx context.Context, registry Registry) error

	// Close 关闭传输层
	Close() error
}
