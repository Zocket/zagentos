// Package longterm 实现长期记忆存储。
// P8: 基于 embedding 的向量存储，支持 episodic 和 semantic memory。
package longterm

import "context"

// MemoryType 表示记忆类型
type MemoryType string

const (
	MemoryEpisodic MemoryType = "episodic"  // 事件记忆（做过什么）
	MemorySemantic MemoryType = "semantic"  // 知识记忆（知道什么）
)

// Entry 是一条记忆条目
type Entry struct {
	ID        string     `json:"id"`
	Content   string     `json:"content"`
	Type      MemoryType `json:"type"`
	Embedding []float32  `json:"embedding,omitempty"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt int64      `json:"created_at"`
	Score     float32    `json:"score,omitempty"` // 检索时的相关性得分
}

// SearchQuery 是记忆检索的查询
type SearchQuery struct {
	Text    string     `json:"text"`
	Type    MemoryType `json:"type,omitempty"` // 为空则搜索全部
	TopK    int        `json:"top_k"`
	MinScore float32   `json:"min_score,omitempty"`
}

// Store 是长期记忆存储的接口
type Store interface {
	// Store 存储一条记忆
	Store(ctx context.Context, entry *Entry) error

	// Search 语义搜索相关记忆
	Search(ctx context.Context, query SearchQuery) ([]*Entry, error)

	// Get 根据 ID 获取记忆
	Get(ctx context.Context, id string) (*Entry, error)

	// Delete 删除记忆
	Delete(ctx context.Context, id string) error

	// Forget 按策略遗忘旧记忆（如超过时间或低相关性）
	Forget(ctx context.Context, policy ForgetPolicy) (int, error)
}

// ForgetPolicy 定义遗忘策略
type ForgetPolicy struct {
	MaxAge    int64   `json:"max_age,omitempty"`     // 超过此时间（秒）的记忆
	MaxCount  int     `json:"max_count,omitempty"`   // 保留最多条数
	MinScore  float32 `json:"min_score,omitempty"`   // 低于此分数的记忆
}

// Embedder 是 embedding 生成器的接口
type Embedder interface {
	// Embed 将文本转换为向量
	Embed(ctx context.Context, text string) ([]float32, error)

	// EmbedBatch 批量生成 embedding
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)

	// Dimensions 返回 embedding 维度
	Dimensions() int
}
