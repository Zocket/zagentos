// Package skill 实现 Skill Registry。
// P10: 管理 Agent 技能的注册、发现和匹配。
package skill

import "context"

// TriggerType 定义技能触发的方式
type TriggerType string

const (
	TriggerKeyword  TriggerType = "keyword"   // 关键词匹配
	TriggerSemantic TriggerType = "semantic"  // 语义相似度匹配
	TriggerExplicit TriggerType = "explicit"  // 显式调用
)

// Trigger 定义技能的触发条件
type Trigger struct {
	Type     TriggerType `json:"type"`
	Keywords []string    `json:"keywords,omitempty"` // keyword 模式下的关键词列表
	Pattern  string      `json:"pattern,omitempty"`  // 正则模式
}

// Skill 表示一个 Agent 技能
type Skill struct {
	Name        string      `json:"name"`
	Description string      `json:"description"` // 简短描述（始终暴露给 LLM）
	Detail      string      `json:"detail"`      // 详细说明（仅在被激活时注入）
	Version     string      `json:"version"`
	Triggers    []Trigger   `json:"triggers"`
	Tools       []string    `json:"tools,omitempty"`       // 该技能需要用到的工具
	SubSkills   []string    `json:"sub_skills,omitempty"`  // 组合技能
}

// MatchResult 是技能匹配的结果
type MatchResult struct {
	Skill      *Skill  `json:"skill"`
	Score      float32 `json:"score"`     // 匹配得分 0-1
	MatchType  string  `json:"match_type"` // 何种方式匹配上的
}

// Registry 管理所有已注册的技能
type Registry interface {
	// Register 注册一个技能
	Register(skill *Skill) error

	// Unregister 移除技能
	Unregister(name string) error

	// Get 获取指定技能
	Get(name string) (*Skill, bool)

	// List 列出所有技能（仅摘要，用于渐进式披露）
	List() []*Skill

	// Match 根据用户输入匹配相关技能
	Match(ctx context.Context, input string) ([]MatchResult, error)

	// Activate 激活技能，返回完整详情（用于注入 context）
	Activate(ctx context.Context, name string) (*Skill, error)
}
