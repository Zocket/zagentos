// Package context 实现 Prompt Assembler / Context Engine。
// P6: 将 system prompt、tools、memory、steering 按 token 预算智能组装。
package context

import (
	"github.com/Zocket/zagentos/pkg/llm"
)

// Priority 定义各段内容的优先级
type Priority int

const (
	PriorityCritical Priority = 0 // 必须包含（system prompt）
	PriorityHigh     Priority = 1 // 高优先（tool definitions、当前对话）
	PriorityMedium   Priority = 2 // 中优先（相关记忆、steering rules）
	PriorityLow      Priority = 3 // 低优先（背景上下文）
)

// Segment 是 context window 中的一个内容段
type Segment struct {
	ID       string   `json:"id"`
	Content  string   `json:"content"`
	Role     llm.Role `json:"role"`
	Priority Priority `json:"priority"`
	Tokens   int      `json:"tokens"` // 预估 token 数
}

// Budget 是 token 预算配置
type Budget struct {
	TotalTokens       int `json:"total_tokens"`
	ReservedForOutput int `json:"reserved_for_output"` // 给 LLM 回复预留的 token
}

// AssembleResult 是组装后的结果
type AssembleResult struct {
	Messages    []llm.Message `json:"messages"`
	TotalTokens int           `json:"total_tokens"`
	Dropped     []Segment     `json:"dropped"` // 因预算限制被丢弃的段
}

// Engine 是上下文组装引擎的接口
type Engine interface {
	// SetBudget 设置 token 预算
	SetBudget(budget Budget)

	// AddSegment 添加一个内容段
	AddSegment(seg Segment)

	// Assemble 在 token 预算内组装最终的 messages
	Assemble() (*AssembleResult, error)

	// Reset 清空所有已添加的 segments
	Reset()
}

// TemplateEngine 是 prompt 模板引擎
type TemplateEngine interface {
	// Render 渲染模板，替换变量
	Render(template string, vars map[string]interface{}) (string, error)
}
