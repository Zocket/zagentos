package tool

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Zocket/zagentos/internal/schema"
)

// Executor 负责执行工具调用，支持参数校验、超时和并行调用。
type Executor struct {
	registry    Registry
	defaultTimeout time.Duration
}

// NewExecutor 创建一个 Executor。
// defaultTimeout 为 0 表示不设默认超时（使用调用方传入的 context）。
func NewExecutor(r Registry, defaultTimeout time.Duration) *Executor {
	return &Executor{
		registry:    r,
		defaultTimeout: defaultTimeout,
	}
}

// CallRequest 是单次工具调用请求
type CallRequest struct {
	Name  string                 `json:"name"`
	Input map[string]interface{} `json:"input"`
}

// CallResult 是单次工具调用结果
type CallResult struct {
	Name   string    `json:"name"`
	Result *Result   `json:"result,omitempty"`
	Error  error     `json:"error,omitempty"`
}

// Execute 执行单次工具调用，包含参数校验。
func (e *Executor) Execute(ctx context.Context, req CallRequest) CallResult {
	t, ok := e.registry.Get(req.Name)
	if !ok {
		return CallResult{
			Name:  req.Name,
			Error: fmt.Errorf("tool: %q not found", req.Name),
		}
	}

	// 参数校验
	if schemaVal := t.InputSchema(); schemaVal != nil {
		// InputSchema 返回 map[string]interface{}，需要转为 *schema.Schema
		// 这里直接用 map 做基本校验
		if err := validateInput(schemaVal, req.Input); err != nil {
			return CallResult{
				Name:  req.Name,
				Error: fmt.Errorf("tool: input validation failed: %w", err),
			}
		}
	}

	// 超时控制
	callCtx := ctx
	if e.defaultTimeout > 0 {
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(ctx, e.defaultTimeout)
		defer cancel()
	}

	result, err := t.Execute(callCtx, req.Input)
	return CallResult{
		Name:   req.Name,
		Result: result,
		Error:  err,
	}
}

// ExecuteBatch 并行执行多个工具调用。
// 所有调用并行启动，各自独立超时（受 context 控制）。
// 返回结果顺序与输入顺序一致。
func (e *Executor) ExecuteBatch(ctx context.Context, reqs []CallRequest) []CallResult {
	results := make([]CallResult, len(reqs))
	var wg sync.WaitGroup

	for i, req := range reqs {
		wg.Add(1)
		go func(idx int, r CallRequest) {
			defer wg.Done()
			results[idx] = e.Execute(ctx, r)
		}(i, req)
	}

	wg.Wait()
	return results
}

// validateInput 用 schema 校验输入参数。
// schemaMap 是 tool.InputSchema() 返回的 map[string]interface{}。
func validateInput(schemaMap map[string]interface{}, input map[string]interface{}) error {
	if schemaMap == nil {
		return nil
	}

	// 从 map 构建 schema.Schema 做校验
	s := mapToSchema(schemaMap)
	return schema.Validate(s, input)
}

// mapToSchema 把 map[string]interface{} 转为 *schema.Schema
func mapToSchema(m map[string]interface{}) *schema.Schema {
	s := &schema.Schema{}
	if t, ok := m["type"].(string); ok {
		s.Type = t
	}
	if d, ok := m["description"].(string); ok {
		s.Description = d
	}
	if reqs, ok := m["required"].([]interface{}); ok {
		for _, r := range reqs {
			if rs, ok := r.(string); ok {
				s.Required = append(s.Required, rs)
			}
		}
	}
	if props, ok := m["properties"].(map[string]interface{}); ok {
		s.Properties = make(map[string]*schema.Schema)
		for k, v := range props {
			if vm, ok := v.(map[string]interface{}); ok {
				s.Properties[k] = mapToSchema(vm)
			}
		}
	}
	if items, ok := m["items"].(map[string]interface{}); ok {
		s.Items = mapToSchema(items)
	}
	if enums, ok := m["enum"].([]interface{}); ok {
		s.Enum = enums
	}
	return s
}
