// Package steering 实现 Steering & Guardrails + Hooks。
// P12: 加载 steering 规则，提供 pre/post hook 机制，约束 agent 行为。
package steering

import "context"

// RuleInclusion 定义规则的包含时机
type RuleInclusion string

const (
	InclusionAlways    RuleInclusion = "always"     // 始终包含
	InclusionFileMatch RuleInclusion = "file_match" // 匹配文件时包含
	InclusionManual    RuleInclusion = "manual"     // 手动触发
)

// Rule 表示一条 steering 规则
type Rule struct {
	Name             string        `json:"name"`
	Content          string        `json:"content"`
	Inclusion        RuleInclusion `json:"inclusion"`
	FileMatchPattern string        `json:"file_match_pattern,omitempty"`
}

// HookEvent 定义 hook 的触发事件
type HookEvent string

const (
	EventPreToolUse   HookEvent = "pre_tool_use"
	EventPostToolUse  HookEvent = "post_tool_use"
	EventPreTask      HookEvent = "pre_task"
	EventPostTask     HookEvent = "post_task"
	EventPromptSubmit HookEvent = "prompt_submit"
	EventAgentStop    HookEvent = "agent_stop"
)

// HookAction 定义 hook 触发后的行为
type HookAction string

const (
	ActionAskAgent  HookAction = "ask_agent"
	ActionRunCmd    HookAction = "run_command"
)

// Hook 表示一个事件钩子
type Hook struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Event       HookEvent  `json:"event"`
	Action      HookAction `json:"action"`
	Prompt      string     `json:"prompt,omitempty"`   // ask_agent 时用
	Command     string     `json:"command,omitempty"`  // run_command 时用
	ToolTypes   []string   `json:"tool_types,omitempty"` // 工具过滤
	FilePatterns []string  `json:"file_patterns,omitempty"`
}

// HookResult 是 hook 执行的结果
type HookResult struct {
	HookID  string `json:"hook_id"`
	Allowed bool   `json:"allowed"` // 是否允许继续
	Message string `json:"message,omitempty"`
}

// Engine 管理 steering rules 和 hooks
type Engine interface {
	// LoadRules 从目录加载所有 steering 规则
	LoadRules(ctx context.Context, dir string) error

	// GetActiveRules 获取当前上下文下应该生效的规则
	GetActiveRules(ctx context.Context, currentFile string) []Rule

	// RegisterHook 注册一个 hook
	RegisterHook(hook Hook) error

	// TriggerHooks 触发指定事件的所有 hook
	TriggerHooks(ctx context.Context, event HookEvent, payload interface{}) ([]HookResult, error)

	// CheckPermission 权限检查（用于 safety guardrails）
	CheckPermission(ctx context.Context, action string, risk string) (bool, error)
}
