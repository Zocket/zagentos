package tool

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// mockTool 是测试用的工具实现
type mockTool struct {
	name        string
	description string
	schema      map[string]interface{}
	execFn      func(ctx context.Context, input map[string]interface{}) (*Result, error)
}

func (m *mockTool) Name() string                          { return m.name }
func (m *mockTool) Description() string                   { return m.description }
func (m *mockTool) InputSchema() map[string]interface{}   { return m.schema }
func (m *mockTool) Execute(ctx context.Context, input map[string]interface{}) (*Result, error) {
	return m.execFn(ctx, input)
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()
	tool := &mockTool{name: "echo", description: "echo tool"}

	if err := r.Register(tool); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	got, ok := r.Get("echo")
	if !ok {
		t.Fatal("expected to find tool 'echo'")
	}
	if got.Name() != "echo" {
		t.Errorf("expected name 'echo', got %q", got.Name())
	}
}

func TestRegistry_DuplicateRegister(t *testing.T) {
	r := NewRegistry()
	tool := &mockTool{name: "dup"}

	if err := r.Register(tool); err != nil {
		t.Fatalf("first Register failed: %v", err)
	}
	if err := r.Register(tool); err == nil {
		t.Error("expected error for duplicate register")
	}
}

func TestRegistry_GetNotFound(t *testing.T) {
	r := NewRegistry()
	_, ok := r.Get("nonexistent")
	if ok {
		t.Error("expected not found")
	}
}

func TestRegistry_List(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockTool{name: "charlie"})
	r.Register(&mockTool{name: "alpha"})
	r.Register(&mockTool{name: "bravo"})

	list := r.List()
	if len(list) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(list))
	}
	// 应按名称排序
	if list[0].Name() != "alpha" {
		t.Errorf("expected first 'alpha', got %q", list[0].Name())
	}
	if list[1].Name() != "bravo" {
		t.Errorf("expected second 'bravo', got %q", list[1].Name())
	}
	if list[2].Name() != "charlie" {
		t.Errorf("expected third 'charlie', got %q", list[2].Name())
	}
}

func TestRegistry_Unregister(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockTool{name: "temp"})

	if err := r.Unregister("temp"); err != nil {
		t.Fatalf("Unregister failed: %v", err)
	}
	_, ok := r.Get("temp")
	if ok {
		t.Error("expected not found after unregister")
	}
}

func TestRegistry_UnregisterNotFound(t *testing.T) {
	r := NewRegistry()
	if err := r.Unregister("nope"); err == nil {
		t.Error("expected error for unregistering nonexistent tool")
	}
}

func TestRegistry_EmptyName(t *testing.T) {
	r := NewRegistry()
	err := r.Register(&mockTool{name: ""})
	if err == nil {
		t.Error("expected error for empty name")
	}
}

func TestRegistry_ConcurrentAccess(t *testing.T) {
	r := NewRegistry()
	var wg sync.WaitGroup

	// 并发写
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			r.Register(&mockTool{name: fmt.Sprintf("tool-%d", n)})
		}(i)
	}

	// 并发读
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			r.Get(fmt.Sprintf("tool-%d", n))
		}(i)
	}

	wg.Wait()
	if len(r.List()) != 100 {
		t.Errorf("expected 100 tools, got %d", len(r.List()))
	}
}

// --- Executor 测试 ---

func TestExecutor_ExecuteSuccess(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockTool{
		name:   "add",
		schema: nil,
		execFn: func(ctx context.Context, input map[string]interface{}) (*Result, error) {
			a, _ := input["a"].(float64)
			b, _ := input["b"].(float64)
			return &Result{Content: fmt.Sprintf("%v", a+b)}, nil
		},
	})

	exec := NewExecutor(r, 0)
	result := exec.Execute(context.Background(), CallRequest{
		Name:  "add",
		Input: map[string]interface{}{"a": float64(1), "b": float64(2)},
	})

	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}
	if result.Result.Content != "3" {
		t.Errorf("expected '3', got %q", result.Result.Content)
	}
}

func TestExecutor_ExecuteToolNotFound(t *testing.T) {
	r := NewRegistry()
	exec := NewExecutor(r, 0)
	result := exec.Execute(context.Background(), CallRequest{Name: "nope"})

	if result.Error == nil {
		t.Fatal("expected error")
	}
}

func TestExecutor_ExecuteWithValidation(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockTool{
		name: "greet",
		schema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name": map[string]interface{}{"type": "string"},
			},
			"required": []interface{}{"name"},
		},
		execFn: func(ctx context.Context, input map[string]interface{}) (*Result, error) {
			name, _ := input["name"].(string)
			return &Result{Content: "hello " + name}, nil
		},
	})

	exec := NewExecutor(r, 0)

	// 合法输入
	result := exec.Execute(context.Background(), CallRequest{
		Name:  "greet",
		Input: map[string]interface{}{"name": "Alice"},
	})
	if result.Error != nil {
		t.Fatalf("unexpected error: %v", result.Error)
	}

	// 缺少 required
	result = exec.Execute(context.Background(), CallRequest{
		Name:  "greet",
		Input: map[string]interface{}{},
	})
	if result.Error == nil {
		t.Error("expected validation error for missing required")
	}
}

func TestExecutor_ExecuteWithTimeout(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockTool{
		name: "slow",
		execFn: func(ctx context.Context, input map[string]interface{}) (*Result, error) {
			select {
			case <-time.After(2 * time.Second):
				return &Result{Content: "done"}, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	})

	exec := NewExecutor(r, 100*time.Millisecond)
	result := exec.Execute(context.Background(), CallRequest{Name: "slow"})

	if result.Error == nil {
		t.Fatal("expected timeout error")
	}
}

func TestExecutor_ExecuteBatch(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockTool{
		name: "double",
		execFn: func(ctx context.Context, input map[string]interface{}) (*Result, error) {
			n, _ := input["n"].(float64)
			return &Result{Content: fmt.Sprintf("%v", n*2)}, nil
		},
	})

	exec := NewExecutor(r, 0)
	reqs := []CallRequest{
		{Name: "double", Input: map[string]interface{}{"n": float64(1)}},
		{Name: "double", Input: map[string]interface{}{"n": float64(2)}},
		{Name: "double", Input: map[string]interface{}{"n": float64(3)}},
	}

	results := exec.ExecuteBatch(context.Background(), reqs)

	if len(results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	for i, r := range results {
		if r.Error != nil {
			t.Fatalf("result %d error: %v", i, r.Error)
		}
		expected := fmt.Sprintf("%v", float64(i+1)*2)
		if r.Result.Content != expected {
			t.Errorf("result %d: expected %q, got %q", i, expected, r.Result.Content)
		}
	}
}

func TestExecutor_ExecuteBatchParallel(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockTool{
		name: "delay",
		execFn: func(ctx context.Context, input map[string]interface{}) (*Result, error) {
			time.Sleep(100 * time.Millisecond)
			return &Result{Content: "ok"}, nil
		},
	})

	exec := NewExecutor(r, 0)
	reqs := make([]CallRequest, 5)
	for i := range reqs {
		reqs[i] = CallRequest{Name: "delay"}
	}

	start := time.Now()
	results := exec.ExecuteBatch(context.Background(), reqs)
	elapsed := time.Since(start)

	if len(results) != 5 {
		t.Fatalf("expected 5 results, got %d", len(results))
	}
	// 并行执行 5 个 100ms 的任务，总耗时应远小于 500ms
	if elapsed > 300*time.Millisecond {
		t.Errorf("batch took too long: %v (expected parallel execution)", elapsed)
	}
}
