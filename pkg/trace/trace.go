// Package trace 实现 Trace & Observability。
// P15: 记录 Agent 执行的完整轨迹，支持结构化日志。
package trace

import (
	"context"
	"time"
)

// SpanKind 标识 span 类型
type SpanKind string

const (
	SpanKindLLMCall   SpanKind = "llm_call"
	SpanKindToolCall  SpanKind = "tool_call"
	SpanKindPlanning  SpanKind = "planning"
	SpanKindRetrieval SpanKind = "retrieval"
	SpanKindAgent     SpanKind = "agent"
)

// Span 表示执行轨迹中的一个片段
type Span struct {
	ID        string            `json:"id"`
	ParentID  string            `json:"parent_id,omitempty"`
	Kind      SpanKind          `json:"kind"`
	Name      string            `json:"name"`
	StartTime time.Time         `json:"start_time"`
	EndTime   time.Time         `json:"end_time"`
	Duration  time.Duration     `json:"duration"`
	Attrs     map[string]string `json:"attrs,omitempty"`
	Status    string            `json:"status"` // ok, error
	Error     string            `json:"error,omitempty"`
}

// Trace 表示一次完整的 agent 执行轨迹
type Trace struct {
	ID        string  `json:"id"`
	SessionID string  `json:"session_id"`
	Spans     []*Span `json:"spans"`
	StartTime time.Time `json:"start_time"`
	EndTime   time.Time `json:"end_time"`
}

// Metrics 是一组聚合指标
type Metrics struct {
	TotalTokensIn    int           `json:"total_tokens_in"`
	TotalTokensOut   int           `json:"total_tokens_out"`
	TotalLatency     time.Duration `json:"total_latency"`
	LLMCallCount     int           `json:"llm_call_count"`
	ToolCallCount    int           `json:"tool_call_count"`
	ErrorCount       int           `json:"error_count"`
}

// Tracer 负责记录执行轨迹
type Tracer interface {
	// StartSpan 开始一个新的 span
	StartSpan(ctx context.Context, kind SpanKind, name string) (context.Context, *Span)

	// EndSpan 结束 span
	EndSpan(span *Span, err error)

	// CurrentTrace 返回当前 trace
	CurrentTrace(ctx context.Context) *Trace

	// Metrics 返回当前 trace 的聚合指标
	Metrics(ctx context.Context) *Metrics
}

// Exporter 导出 trace 数据
type Exporter interface {
	// Export 导出一个完整的 trace
	Export(ctx context.Context, trace *Trace) error

	// Flush 刷出所有缓冲的 trace
	Flush(ctx context.Context) error
}
