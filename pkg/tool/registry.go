package tool

import (
	"fmt"
	"sort"
	"sync"
)

// memoryRegistry 是 Registry 的内存实现，并发安全。
type memoryRegistry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewRegistry 创建一个内存 Registry
func NewRegistry() Registry {
	return &memoryRegistry{
		tools: make(map[string]Tool),
	}
}

func (r *memoryRegistry) Register(t Tool) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := t.Name()
	if name == "" {
		return fmt.Errorf("tool: tool name cannot be empty")
	}
	if _, exists := r.tools[name]; exists {
		return fmt.Errorf("tool: tool %q already registered", name)
	}
	r.tools[name] = t
	return nil
}

func (r *memoryRegistry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

func (r *memoryRegistry) List() []Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 按名称排序，保证输出稳定
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)

	result := make([]Tool, 0, len(names))
	for _, name := range names {
		result = append(result, r.tools[name])
	}
	return result
}

func (r *memoryRegistry) Unregister(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.tools[name]; !exists {
		return fmt.Errorf("tool: tool %q not found", name)
	}
	delete(r.tools, name)
	return nil
}
