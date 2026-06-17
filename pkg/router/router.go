// Package router 实现 Agent Router。
// P14: 根据任务类型智能路由到合适的 Agent。
package router

import "context"

// Capability 描述 Agent 的能力
type Capability struct {
	AgentName   string   `json:"agent_name"`
	Description string   `json:"description"`
	Domains     []string `json:"domains"`     // 擅长的领域
	Priority    int      `json:"priority"`    // 优先级（越小越优先）
	MaxLoad     int      `json:"max_load"`    // 最大并发负载
	CurrentLoad int      `json:"current_load"`
}

// RouteResult 是路由决策的结果
type RouteResult struct {
	AgentName string  `json:"agent_name"`
	Score     float32 `json:"score"`     // 匹配得分
	Reason    string  `json:"reason"`    // 为什么选择这个 agent
}

// Strategy 定义路由策略
type Strategy string

const (
	StrategyBestMatch  Strategy = "best_match"  // 选择最匹配的
	StrategyRoundRobin Strategy = "round_robin" // 轮询
	StrategyLeastLoad  Strategy = "least_load"  // 最小负载
)

// Router 负责任务路由
type Router interface {
	// Register 注册一个 agent 的能力描述
	Register(cap Capability) error

	// Unregister 移除 agent
	Unregister(agentName string) error

	// Route 根据任务描述选择合适的 agent
	Route(ctx context.Context, task string) (*RouteResult, error)

	// RouteWithStrategy 使用指定策略路由
	RouteWithStrategy(ctx context.Context, task string, strategy Strategy) (*RouteResult, error)

	// Failover 当 primary 失败时选择备选 agent
	Failover(ctx context.Context, task string, failedAgent string) (*RouteResult, error)
}
