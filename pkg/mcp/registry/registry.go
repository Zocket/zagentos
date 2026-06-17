// Package registry 实现 MCP Server 注册中心。
// P5: 管理多个 MCP Server 的注册、发现、健康检查。
package registry

import (
	"context"
	"time"

	"github.com/Zocket/zagentos/pkg/mcp/client"
)

// ServerStatus 描述注册的 server 状态
type ServerStatus struct {
	Name       string                 `json:"name"`
	State      client.ConnectionState `json:"state"`
	LastCheck  time.Time              `json:"last_check"`
	ToolCount  int                    `json:"tool_count"`
	Error      string                 `json:"error,omitempty"`
}

// Registry 管理多个 MCP Server 的生命周期
type Registry interface {
	// Register 注册一个新的 MCP Server
	Register(ctx context.Context, config client.ServerConfig) error

	// Unregister 移除一个 server
	Unregister(ctx context.Context, name string) error

	// Get 获取指定 server 的 client
	Get(name string) (client.Client, bool)

	// ListServers 列出所有已注册的 server 及其状态
	ListServers(ctx context.Context) []ServerStatus

	// AllTools 聚合所有 server 暴露的 tools（带命名空间前缀）
	AllTools(ctx context.Context) ([]NamespacedTool, error)

	// CallTool 按工具全名调用（自动路由到正确的 server）
	CallTool(ctx context.Context, fullName string, args map[string]interface{}) (interface{}, error)

	// HealthCheck 对所有 server 执行健康检查
	HealthCheck(ctx context.Context) []ServerStatus

	// Close 关闭所有连接
	Close(ctx context.Context) error
}

// NamespacedTool 是带命名空间的工具
type NamespacedTool struct {
	Namespace   string      `json:"namespace"`   // server name
	Name        string      `json:"name"`        // tool name
	FullName    string      `json:"full_name"`   // namespace.name
	Description string      `json:"description"`
	InputSchema interface{} `json:"input_schema"`
}
