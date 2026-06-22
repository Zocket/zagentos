package llm

import (
	"context"
	"time"
)

const gtaiDefaultBaseURL = "https://gtaiapi.gtcloud.cn/v1"

// GTAIProvider 实现绿城 GTAI API（兼容 OpenAI 接口）
type GTAIProvider struct {
	inner *OpenAIProvider
}

// NewGTAIProvider 创建 GTAI provider
func NewGTAIProvider(config ProviderConfig) *GTAIProvider {
	if config.BaseURL == "" {
		config.BaseURL = gtaiDefaultBaseURL
	}
	if config.HTTPTimeout == 0 {
		config.HTTPTimeout = 60 * time.Second
	}
	if config.Retry == (RetryConfig{}) {
		config.Retry = DefaultRetryConfig()
	}

	return &GTAIProvider{
		inner: NewOpenAIProvider(config),
	}
}

func (p *GTAIProvider) Name() string {
	return "GreenTownAI"
}

func (p *GTAIProvider) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	return p.inner.Complete(ctx, req)
}

func (p *GTAIProvider) Stream(ctx context.Context, req *CompletionRequest) (<-chan StreamChunk, error) {
	return p.inner.Stream(ctx, req)
}

func (p *GTAIProvider) CountTokens(messages []Message) (int, error) {
	return p.inner.CountTokens(messages)
}
