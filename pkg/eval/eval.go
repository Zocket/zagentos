// Package eval 实现 Evaluation Framework。
// P16: Agent 质量评估，自动化回归测试。
package eval

import "context"

// Case 是一个评估用例
type Case struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Input       string            `json:"input"`        // 用户输入
	Expected    ExpectedBehavior  `json:"expected"`     // 期望行为
	Tags        []string          `json:"tags,omitempty"`
}

// ExpectedBehavior 定义期望的 agent 行为
type ExpectedBehavior struct {
	ContainsText    []string `json:"contains_text,omitempty"`     // 输出应包含的文本
	NotContainsText []string `json:"not_contains_text,omitempty"` // 输出不应包含的文本
	ToolsCalled     []string `json:"tools_called,omitempty"`      // 应该被调用的工具
	MaxIterations   int      `json:"max_iterations,omitempty"`    // 最大步数
	MaxTokens       int      `json:"max_tokens,omitempty"`        // 最大 token 消耗
	CustomCheck     string   `json:"custom_check,omitempty"`      // 自定义检查函数名
}

// Score 是评估分数
type Score struct {
	Pass     bool    `json:"pass"`
	Value    float64 `json:"value"`    // 0-1 分数
	Details  string  `json:"details"`  // 评分细节
}

// Result 是单个用例的评估结果
type Result struct {
	CaseID   string `json:"case_id"`
	CaseName string `json:"case_name"`
	Score    Score  `json:"score"`
	Output   string `json:"output"`    // agent 实际输出
	Duration int64  `json:"duration_ms"`
	Error    string `json:"error,omitempty"`
}

// Report 是一批评估的汇总报告
type Report struct {
	TotalCases int       `json:"total_cases"`
	Passed     int       `json:"passed"`
	Failed     int       `json:"failed"`
	Errors     int       `json:"errors"`
	PassRate   float64   `json:"pass_rate"`
	Results    []Result  `json:"results"`
}

// Runner 执行评估
type Runner interface {
	// Run 执行一组评估用例
	Run(ctx context.Context, cases []Case) (*Report, error)

	// RunSingle 执行单个用例
	RunSingle(ctx context.Context, c Case) (*Result, error)
}

// Scorer 负责评分
type Scorer interface {
	// Score 根据期望和实际输出计算分数
	Score(ctx context.Context, expected ExpectedBehavior, actual string) (*Score, error)
}

// CaseLoader 加载评估用例
type CaseLoader interface {
	// Load 从目录加载所有评估用例
	Load(ctx context.Context, dir string) ([]Case, error)
}
