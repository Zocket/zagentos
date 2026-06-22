// Package tool 内置工具实现。
// searxng.go: SearXNG 搜索工具
package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// SearXNGTool 是基于 SearXNG 的网络搜索工具。
type SearXNGTool struct {
	baseURL    string
	httpClient *http.Client
}

// NewSearXNGTool 创建 SearXNG 搜索工具。
// baseURL 是 SearXNG 服务地址，如 "https://searxng-test.gtcloud.cn"。
func NewSearXNGTool(baseURL string) *SearXNGTool {
	return &SearXNGTool{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (t *SearXNGTool) Name() string { return "web_search" }
func (t *SearXNGTool) Description() string {
	return "搜索互联网获取信息。输入查询关键词，返回相关网页结果。"
}

func (t *SearXNGTool) InputSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"query": map[string]interface{}{
				"type":        "string",
				"description": "搜索关键词",
			},
			"max_results": map[string]interface{}{
				"type":        "integer",
				"description": "最大返回结果数量，默认 5",
			},
		},
		"required": []interface{}{"query"},
	}
}

// searxngResponse 是 SearXNG API 的响应结构
type searxngResponse struct {
	Query           string          `json:"query"`
	NumberOfResults int             `json:"number_of_results"`
	Results         []searxngResult `json:"results"`
}

type searxngResult struct {
	Title   string   `json:"title"`
	URL     string   `json:"url"`
	Content string   `json:"content"`
	Engines []string `json:"engines"`
	Score   float64  `json:"score"`
}

func (t *SearXNGTool) Execute(ctx context.Context, input map[string]interface{}) (*Result, error) {
	query, ok := input["query"].(string)
	if !ok || query == "" {
		return nil, fmt.Errorf("tool: missing or invalid 'query' parameter")
	}

	maxResults := 5
	if n, ok := input["max_results"].(float64); ok && n > 0 {
		maxResults = int(n)
	}

	// 构建请求 URL
	searchURL := t.baseURL + "/search?q=" + url.QueryEscape(query) + "&format=json"

	httpReq, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, fmt.Errorf("searxng: failed to create request: %w", err)
	}

	resp, err := t.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("searxng: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("searxng: HTTP %d: %s", resp.StatusCode, string(body))
	}

	var searxngResp searxngResponse
	if err := json.NewDecoder(resp.Body).Decode(&searxngResp); err != nil {
		return nil, fmt.Errorf("searxng: failed to decode response: %w", err)
	}

	// 格式化结果
	result := fmt.Sprintf("搜索 \"%s\"，共约 %d 条结果，显示前 %d 条：\n\n",
		query, searxngResp.NumberOfResults, min(len(searxngResp.Results), maxResults))

	for i, r := range searxngResp.Results {
		if i >= maxResults {
			break
		}
		result += fmt.Sprintf("%d. %s\n   URL: %s\n   摘要: %s\n\n",
			i+1, r.Title, r.URL, truncate(r.Content, 200))
	}

	return &Result{Content: result}, nil
}

func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
