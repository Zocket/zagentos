// Package conversation 实现对话短期记忆。
// P7: 管理当前对话的 message list，支持窗口压缩。
package conversation

import (
	"context"

	"github.com/Zocket/zagentos/pkg/llm"
)

// Store 是对话历史的存储接口
type Store interface {
	// Append 添加一条消息
	Append(msg llm.Message) error

	// Messages 返回完整对话历史
	Messages() []llm.Message

	// Len 返回消息数量
	Len() int

	// Clear 清空对话
	Clear()

	// Trim 裁剪到指定 token 预算内
	Trim(maxTokens int) error
}

// Compactor 负责在 context window 满时压缩对话历史
type Compactor interface {
	// Compact 将超出预算的历史压缩成摘要
	// 保留最近的 keepRecent 条消息原样，其余压缩
	Compact(ctx context.Context, messages []llm.Message, keepRecent int) ([]llm.Message, error)
}

// Session 表示一个对话会话
type Session struct {
	ID        string        `json:"id"`
	Messages  []llm.Message `json:"messages"`
	CreatedAt int64         `json:"created_at"`
	UpdatedAt int64         `json:"updated_at"`
}

// SessionStore 管理多个会话的持久化
type SessionStore interface {
	// Save 保存会话
	Save(ctx context.Context, session *Session) error

	// Load 加载会话
	Load(ctx context.Context, id string) (*Session, error)

	// List 列出所有会话
	List(ctx context.Context) ([]*Session, error)

	// Delete 删除会话
	Delete(ctx context.Context, id string) error
}
