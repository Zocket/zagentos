// Package retrieval 实现 Context Retrieval Engine (RAG)。
// P9: 从大量文档中检索相关上下文并注入 prompt。
package retrieval

import "context"

// Document 表示一个可被检索的文档
type Document struct {
	ID       string            `json:"id"`
	Content  string            `json:"content"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// Chunk 是文档被分片后的一个片段
type Chunk struct {
	ID         string            `json:"id"`
	DocumentID string            `json:"document_id"`
	Content    string            `json:"content"`
	Index      int               `json:"index"`      // 在文档中的顺序
	Metadata   map[string]string `json:"metadata,omitempty"`
	Score      float32           `json:"score,omitempty"` // 检索得分
}

// IndexConfig 是索引的配置
type IndexConfig struct {
	ChunkSize    int `json:"chunk_size"`    // 分片大小（字符数）
	ChunkOverlap int `json:"chunk_overlap"` // 分片重叠
}

// Indexer 负责文档索引
type Indexer interface {
	// Index 索引一个文档（分片 + embedding + 存储）
	Index(ctx context.Context, doc Document) error

	// IndexBatch 批量索引
	IndexBatch(ctx context.Context, docs []Document) error

	// Remove 移除文档索引
	Remove(ctx context.Context, docID string) error
}

// Retriever 负责检索相关内容
type Retriever interface {
	// Retrieve 根据查询检索最相关的 chunks
	Retrieve(ctx context.Context, query string, topK int) ([]Chunk, error)
}

// Reranker 对检索结果进行重排序
type Reranker interface {
	// Rerank 对 chunks 按与 query 的相关性重新排序
	Rerank(ctx context.Context, query string, chunks []Chunk) ([]Chunk, error)
}

// Pipeline 组合了完整的 RAG 管线
type Pipeline interface {
	Indexer
	Retriever

	// RetrieveAndFormat 检索后按 token budget 格式化成可注入 prompt 的文本
	RetrieveAndFormat(ctx context.Context, query string, tokenBudget int) (string, error)
}
