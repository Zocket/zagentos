// Package server 实现 MCP Server SDK。
// P4: 基于 Tool Protocol Runtime 实现完整的 MCP Server。
package server

import "context"

// ServerInfo 描述 MCP Server 的基本信息
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Capability 描述 server 支持的能力
type Capability struct {
	Tools     bool `json:"tools,omitempty"`
	Resources bool `json:"resources,omitempty"`
	Prompts   bool `json:"prompts,omitempty"`
}

// State 表示 server 生命周期状态
type State string

const (
	StateCreated      State = "created"
	StateInitializing State = "initializing"
	StateReady        State = "ready"
	StateShutdown     State = "shutdown"
)

// Resource 描述 server 暴露的资源
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// Server 是 MCP Server 的接口
type Server interface {
	// Info 返回 server 信息
	Info() ServerInfo

	// Capabilities 返回 server 支持的能力
	Capabilities() Capability

	// State 返回当前状态
	State() State

	// Initialize 初始化 server
	Initialize(ctx context.Context) error

	// Shutdown 关闭 server
	Shutdown(ctx context.Context) error

	// Serve 启动 server 监听（阻塞）
	Serve(ctx context.Context) error
}
