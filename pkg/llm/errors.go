package llm

import (
	"errors"
	"fmt"
)

// 标准错误
var (
	ErrProviderNotFound = errors.New("llm: provider not found")
	ErrNoDefault        = errors.New("llm: no default provider set")
	ErrRateLimited      = errors.New("llm: rate limited")
	ErrContextCancelled = errors.New("llm: context cancelled")
	ErrAllProvidersFailed = errors.New("llm: all providers failed")
)

// APIError 表示来自 LLM API 的错误
type APIError struct {
	Provider   string `json:"provider"`
	StatusCode int    `json:"status_code"`
	Type       string `json:"type"`
	Message    string `json:"message"`
	Retryable  bool   `json:"retryable"`
}

func (e *APIError) Error() string {
	return fmt.Sprintf("llm: %s API error (status=%d, type=%s): %s",
		e.Provider, e.StatusCode, e.Type, e.Message)
}

// IsRetryable 判断错误是否可重试
func IsRetryable(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.Retryable
	}
	return false
}

// IsRateLimited 判断是否是限流错误
func IsRateLimited(err error) bool {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == 429
	}
	return errors.Is(err, ErrRateLimited)
}
