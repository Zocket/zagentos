// Package subagent 实现 Sub-Agent Framework。
// P13: 支持多 Agent 协作，父 Agent 委托任务给子 Agent。
package subagent

import "context"

// AgentConfig 定义一个 Agent 的配置
type AgentConfig struct {
	Name         string   `json:"name"`
	SystemPrompt string   `json:"system_prompt"`
	Tools        []string `json:"tools"`        // 可用工具列表
	Model        string   `json:"model"`        // 使用的模型
	MaxTokens    int      `json:"max_tokens"`
	Constraints  []string `json:"constraints"`  // 行为约束
}

// TaskResult 是子 Agent 执行的结果
type TaskResult struct {
	AgentName string `json:"agent_name"`
	Output    string `json:"output"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
}

// Agent 是一个可独立执行的 Agent 实例
type Agent interface {
	// Name 返回 agent 名称
	Name() string

	// Execute 执行一个任务
	Execute(ctx context.Context, prompt string) (*TaskResult, error)

	// Config 返回 agent 配置
	Config() AgentConfig
}

// Manager 管理子 Agent 的生命周期
type Manager interface {
	// Create 根据配置创建一个子 Agent
	Create(ctx context.Context, config AgentConfig) (Agent, error)

	// Delegate 委托任务给指定的子 Agent
	Delegate(ctx context.Context, agentName string, prompt string) (*TaskResult, error)

	// DelegateParallel 并行委托多个任务
	DelegateParallel(ctx context.Context, tasks map[string]string) (map[string]*TaskResult, error)

	// List 列出所有可用的子 Agent
	List() []AgentConfig

	// Destroy 销毁一个子 Agent
	Destroy(ctx context.Context, agentName string) error
}
