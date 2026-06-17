package llm

import (
	"context"
	"math"
	"math/rand"
	"sync"
	"time"
)

// RetryConfig 是重试策略配置
// MaxRetries = -1 表示禁用重试（不使用默认值）
type RetryConfig struct {
	MaxRetries  int           // 最大重试次数，-1 表示不重试
	InitialWait time.Duration // 初始等待时间
	MaxWait     time.Duration // 最大等待时间
	Multiplier  float64       // 退避乘数
}

// NoRetry 返回一个禁用重试的配置
func NoRetry() RetryConfig {
	return RetryConfig{MaxRetries: -1}
}

// DefaultRetryConfig 返回默认重试配置
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:  3,
		InitialWait: 1 * time.Second,
		MaxWait:     30 * time.Second,
		Multiplier:  2.0,
	}
}

// DoWithRetry 带重试执行函数
func DoWithRetry(ctx context.Context, cfg RetryConfig, fn func() error) error {
	maxRetries := cfg.MaxRetries
	if maxRetries < 0 {
		maxRetries = 0
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		lastErr = fn()
		if lastErr == nil {
			return nil
		}

		// 不可重试的错误直接返回
		if !IsRetryable(lastErr) {
			return lastErr
		}

		// 最后一次尝试失败，不再等待
		if attempt == maxRetries {
			break
		}

		// 计算退避时间（指数退避 + jitter）
		wait := float64(cfg.InitialWait) * math.Pow(cfg.Multiplier, float64(attempt))
		if wait > float64(cfg.MaxWait) {
			wait = float64(cfg.MaxWait)
		}
		// 添加 jitter（±25%）
		jitter := wait * 0.25 * (rand.Float64()*2 - 1)
		sleepDuration := time.Duration(wait + jitter)

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(sleepDuration):
		}
	}
	return lastErr
}

// RateLimiter 是简单的令牌桶限流器
type RateLimiter struct {
	mu         sync.Mutex
	tokens     float64
	maxTokens  float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

// NewRateLimiter 创建限流器
// maxTokens: 桶容量, refillRate: 每秒补充的 token 数
func NewRateLimiter(maxTokens float64, refillRate float64) *RateLimiter {
	return &RateLimiter{
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: refillRate,
		lastRefill: time.Now(),
	}
}

// Wait 等待直到获得一个 token，如果 context 取消则返回错误
func (rl *RateLimiter) Wait(ctx context.Context) error {
	for {
		rl.mu.Lock()
		rl.refill()
		if rl.tokens >= 1 {
			rl.tokens--
			rl.mu.Unlock()
			return nil
		}
		// 计算需要等待多久才能获得一个 token
		waitTime := time.Duration(float64(time.Second) / rl.refillRate)
		rl.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitTime):
		}
	}
}

// Allow 非阻塞检查是否可以获得一个 token
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.refill()
	if rl.tokens >= 1 {
		rl.tokens--
		return true
	}
	return false
}

func (rl *RateLimiter) refill() {
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill).Seconds()
	rl.tokens += elapsed * rl.refillRate
	if rl.tokens > rl.maxTokens {
		rl.tokens = rl.maxTokens
	}
	rl.lastRefill = now
}

// ProviderConfig 是创建 Provider 时的通用配置
type ProviderConfig struct {
	APIKey      string
	BaseURL     string
	Model       string
	Retry       RetryConfig
	RateLimit   *RateLimiter // nil 表示不限流
	HTTPTimeout time.Duration
}
