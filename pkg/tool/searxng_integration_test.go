//go:build integration

package tool

import (
	"context"
	"encoding/json"
	"testing"
)

func TestSearXNGTool_Search(t *testing.T) {
	tool := NewSearXNGTool("https://searxng-test.gtcloud.cn")

	// 打印工具信息
	t.Logf("工具名称: %s", tool.Name())
	t.Logf("工具描述: %s", tool.Description())

	schemaJSON, _ := json.MarshalIndent(tool.InputSchema(), "", "  ")
	t.Logf("输入 Schema:\n%s", schemaJSON)

	// 执行搜索
	result, err := tool.Execute(context.Background(), map[string]interface{}{
		"query":       "Go语言教程",
		"max_results": float64(3),
	})
	if err != nil {
		t.Fatalf("搜索失败: %v", err)
	}

	t.Logf("===== 搜索结果 =====\n%s", result.Content)

	if result.Content == "" {
		t.Error("搜索结果为空")
	}
}

func TestSearXNGTool_SearchWithRegistry(t *testing.T) {
	// 通过 Registry 注册并调用，验证完整流程
	registry := NewRegistry()

	searchTool := NewSearXNGTool("https://searxng-test.gtcloud.cn")
	if err := registry.Register(searchTool); err != nil {
		t.Fatalf("注册失败: %v", err)
	}

	t.Logf("已注册工具: %s", searchTool.Name())

	// 列出工具
	tools := registry.List()
	t.Logf("Registry 中工具数量: %d", len(tools))
	for _, tl := range tools {
		t.Logf("  - %s: %s", tl.Name(), tl.Description())
	}

	// 通过 Executor 调用
	executor := NewExecutor(registry, 0)
	result := executor.Execute(context.Background(), CallRequest{
		Name: "web_search",
		Input: map[string]interface{}{
			"query": "绿城中国",
		},
	})

	if result.Error != nil {
		t.Fatalf("Executor 调用失败: %v", result.Error)
	}

	t.Logf("===== Executor 调用结果 =====\n%s", result.Result.Content)
}

func TestSearXNGTool_MissingQuery(t *testing.T) {
	tool := NewSearXNGTool("https://searxng-test.gtcloud.cn")

	_, err := tool.Execute(context.Background(), map[string]interface{}{})
	if err == nil {
		t.Error("expected error for missing query")
	}
	t.Logf("预期错误: %v", err)
}
