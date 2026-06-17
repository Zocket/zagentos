package llm

import (
	"context"
	"fmt"
	"sync"
)

// defaultGateway 是 Gateway 接口的默认实现
type defaultGateway struct {
	mu            sync.RWMutex
	providers     map[string]Provider
	defaultName   string
	fallbackOrder []string // fallback 顺序
}

// NewGateway 创建一个新的 Gateway
func NewGateway() Gateway {
	return &defaultGateway{
		providers: make(map[string]Provider),
	}
}

func (g *defaultGateway) Name() string {
	return "gateway"
}

func (g *defaultGateway) RegisterProvider(name string, provider Provider) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.providers[name] = provider
	g.fallbackOrder = append(g.fallbackOrder, name)

	// 第一个注册的 provider 自动成为默认
	if g.defaultName == "" {
		g.defaultName = name
	}
}

func (g *defaultGateway) SetDefault(name string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if _, ok := g.providers[name]; !ok {
		return fmt.Errorf("%w: %s", ErrProviderNotFound, name)
	}
	g.defaultName = name
	return nil
}

// SetFallbackOrder 设置 fallback 的尝试顺序
func (g *defaultGateway) SetFallbackOrder(names []string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.fallbackOrder = names
}

func (g *defaultGateway) Complete(ctx context.Context, req *CompletionRequest) (*CompletionResponse, error) {
	g.mu.RLock()
	defaultName := g.defaultName
	fallbackOrder := g.fallbackOrder
	providers := g.providers
	g.mu.RUnlock()

	if defaultName == "" {
		return nil, ErrNoDefault
	}

	// 先尝试默认 provider
	provider := providers[defaultName]
	resp, err := provider.Complete(ctx, req)
	if err == nil {
		return resp, nil
	}

	// 默认 provider 失败，尝试 fallback
	var lastErr error = err
	for _, name := range fallbackOrder {
		if name == defaultName {
			continue // 跳过已失败的默认 provider
		}
		p, ok := providers[name]
		if !ok {
			continue
		}

		resp, err = p.Complete(ctx, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
	}

	return nil, fmt.Errorf("%w: last error: %v", ErrAllProvidersFailed, lastErr)
}

func (g *defaultGateway) Stream(ctx context.Context, req *CompletionRequest) (<-chan StreamChunk, error) {
	g.mu.RLock()
	defaultName := g.defaultName
	fallbackOrder := g.fallbackOrder
	providers := g.providers
	g.mu.RUnlock()

	if defaultName == "" {
		return nil, ErrNoDefault
	}

	// 先尝试默认 provider
	provider := providers[defaultName]
	ch, err := provider.Stream(ctx, req)
	if err == nil {
		return ch, nil
	}

	// fallback
	var lastErr error = err
	for _, name := range fallbackOrder {
		if name == defaultName {
			continue
		}
		p, ok := providers[name]
		if !ok {
			continue
		}

		ch, err = p.Stream(ctx, req)
		if err == nil {
			return ch, nil
		}
		lastErr = err
	}

	return nil, fmt.Errorf("%w: last error: %v", ErrAllProvidersFailed, lastErr)
}

func (g *defaultGateway) CountTokens(messages []Message) (int, error) {
	g.mu.RLock()
	defaultName := g.defaultName
	providers := g.providers
	g.mu.RUnlock()

	if defaultName == "" {
		return 0, ErrNoDefault
	}
	return providers[defaultName].CountTokens(messages)
}
