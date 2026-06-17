// Package token 提供 token 计数工具。
// 用于估算 prompt 的 token 消耗，辅助 context 预算管理。
package token

// Counter 是 token 计数器接口
type Counter interface {
	// Count 估算文本的 token 数
	Count(text string) int

	// CountMessages 估算一组消息的 token 数
	CountMessages(messages interface{}) int
}

// SimpleCounter 是一个基于字符数的简单估算器（1 token ≈ 4 chars for English, ≈ 2 chars for CJK）
type SimpleCounter struct {
	CharsPerToken float64
}

// NewSimpleCounter 创建一个简单计数器
func NewSimpleCounter() *SimpleCounter {
	return &SimpleCounter{CharsPerToken: 3.5} // 中英文混合估算
}

// Count 简单估算
func (c *SimpleCounter) Count(text string) int {
	if len(text) == 0 {
		return 0
	}
	return int(float64(len([]rune(text))) / c.CharsPerToken)
}
