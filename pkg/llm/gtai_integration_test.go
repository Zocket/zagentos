//go:build integration

package llm

import (
	"context"
	"encoding/json"
	"testing"
)

// TestGTAIProvider_RealComplete 调用真实 GTAI API 验证基本对话
func TestGTAIProvider_RealComplete(t *testing.T) {
	cfg := loadGTAITestConfig(t)

	// BaseURL 留空 → 自动使用默认地址 https://gtaiapi.gtcloud.cn/v1
	provider := NewGTAIProvider(ProviderConfig{
		APIKey: cfg.APIKey,
		Model:  cfg.Model,
	})

	t.Logf("测试模型: %s", cfg.Model)
	t.Logf("使用 BaseURL: %s", provider.inner.config.BaseURL)

	req := &CompletionRequest{
		Messages: []Message{
			{Role: RoleUser, Content: "你好，请用一句话介绍自己"},
		},
	}

	// 打印输入
	reqJSON, _ := json.MarshalIndent(req, "", "  ")
	t.Logf("===== 输入请求 =====\n%s", reqJSON)

	resp, err := provider.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("API 调用失败: %v", err)
	}

	// 打印输出
	respJSON, _ := json.MarshalIndent(resp, "", "  ")
	t.Logf("===== 输出响应 =====\n%s", respJSON)

	if resp.Message.Content == "" {
		t.Error("返回内容为空")
	}
}

// TestGTAIProvider_RealStream 调用真实 GTAI API 验证流式响应
func TestGTAIProvider_RealStream(t *testing.T) {
	cfg := loadGTAITestConfig(t)

	provider := NewGTAIProvider(ProviderConfig{
		APIKey: cfg.APIKey,
		Model:  cfg.Model,
	})

	t.Logf("测试模型: %s", cfg.Model)

	req := &CompletionRequest{
		Messages: []Message{
			{Role: RoleUser, Content: "用一句话介绍杭州"},
		},
	}

	// 打印输入
	reqJSON, _ := json.MarshalIndent(req, "", "  ")
	t.Logf("===== 输入请求 =====\n%s", reqJSON)

	ch, err := provider.Stream(context.Background(), req)
	if err != nil {
		t.Fatalf("流式调用失败: %v", err)
	}

	t.Logf("===== 流式输出 =====")
	var fullContent string
	for chunk := range ch {
		fullContent += chunk.Delta
		if chunk.Delta != "" {
			t.Logf("收到片段: %s", chunk.Delta)
		}
		if chunk.Done {
			break
		}
	}

	t.Logf("===== 完整回复 =====\n%s", fullContent)
	if fullContent == "" {
		t.Error("流式返回内容为空")
	}
}
