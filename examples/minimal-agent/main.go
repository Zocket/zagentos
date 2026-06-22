// minimal-agent 是阶段一完成后的里程碑 demo。
// 完成 P1 + P2 + P3 后，在这里组装一个能用工具回答问题的最小 agent。
//
// 使用方式:
//
//	go run ./examples/minimal-agent
//
// 配置文件:
//
//	testdata/llm/gtai_test_config.json  - GTAI API Key 和模型名
//	SEARXNG_URL 环境变量                 - SearXNG 服务地址（可选，默认 https://searxng-test.gtcloud.cn）
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Zocket/zagentos/pkg/llm"
	"github.com/Zocket/zagentos/pkg/loop"
	"github.com/Zocket/zagentos/pkg/tool"
)

const systemPrompt = `你是一个有能力的 AI 助手。你可以使用工具来帮助用户回答问题。
当需要获取最新信息时，请使用 web_search 工具搜索互联网。
请用中文回答用户的问题。`

// gtaiConfig 对应 testdata/llm/gtai_test_config.json
type gtaiConfig struct {
	APIKey string `json:"api_key"`
	Model  string `json:"model"`
}

// loadConfig 从 testdata/llm/gtai_test_config.json 加载配置
func loadConfig() (*gtaiConfig, error) {
	configPath := filepath.Join("testdata", "llm", "gtai_test_config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}
	var cfg gtaiConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}
	return &cfg, nil
}

func main() {
	fmt.Println("=== Minimal Agent Demo ===")
	fmt.Println("P1 (LLM Gateway) + P2 (Tool Runtime) + P3 (ReAct Loop)")
	fmt.Println()

	// 1. 加载配置
	cfg, err := loadConfig()
	if err != nil {
		fmt.Printf("错误: %v\n", err)
		os.Exit(1)
	}

	searxngURL := os.Getenv("SEARXNG_URL")
	if searxngURL == "" {
		searxngURL = "https://searxng-test.gtcloud.cn"
	}

	fmt.Printf("LLM:    GTAI (%s)\n", cfg.Model)
	fmt.Printf("Tool:   web_search (SearXNG @ %s)\n", searxngURL)
	fmt.Printf("Max Iter: 10\n")
	fmt.Println()
	fmt.Println("输入 'quit' 或 'exit' 退出")
	fmt.Println("----------------------------------------")

	// 2. 创建 LLM Provider
	provider := llm.NewGTAIProvider(llm.ProviderConfig{
		APIKey: cfg.APIKey,
		Model:  cfg.Model,
	})

	// 3. 创建 Tool Registry 并注册 SearXNG 搜索工具
	registry := tool.NewRegistry()
	if err := registry.Register(tool.NewSearXNGTool(searxngURL)); err != nil {
		fmt.Printf("注册工具失败: %v\n", err)
		os.Exit(1)
	}

	// 4. 创建 ReAct Loop
	agentLoop := loop.NewLoop(provider, registry, loop.Config{
		MaxIterations: 10,
		MaxTokens:     4096,
		Verbose:       true,
	})

	// 5. 交互式对话
	ctx := context.Background()
	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: systemPrompt},
	}

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("\n用户> ")
		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if input == "quit" || input == "exit" {
			fmt.Println("再见！")
			break
		}

		messages = append(messages, llm.Message{
			Role:    llm.RoleUser,
			Content: input,
		})

		result, err := agentLoop.Run(ctx, messages)
		if err != nil {
			fmt.Printf("错误: %v\n", err)
			continue
		}

		// 打印执行过程
		fmt.Println()
		for _, step := range result.Steps {
			fmt.Printf("--- 步骤 %d ---\n", step.Iteration)

			// 打印发送给 LLM 的输入（消息列表）
			if step.Request != nil {
				fmt.Println("  [输入] 发送给 LLM 的消息:")
				reqJSON, _ := json.MarshalIndent(step.Request, "    ", "  ")
				printIndented(string(reqJSON), "    ")
			}

			// 打印 LLM 的输出响应
			if step.Response != nil {
				fmt.Println("  [输出] LLM 返回:")
				respJSON, _ := json.MarshalIndent(step.Response, "    ", "  ")
				printIndented(string(respJSON), "    ")
			}

			// 打印工具调用输入和输出
			for _, tc := range step.ToolCalls {
				fmt.Printf("  [工具调用] %s\n", tc.ToolName)
				fmt.Printf("    输入参数: ")
				inputJSON, _ := json.Marshal(tc.Input)
				fmt.Println(string(inputJSON))

				if tc.Error != nil {
					fmt.Printf("    输出错误: %v\n", tc.Error)
				} else if tc.Result != nil {
					fmt.Println("    输出结果:")
					printIndented(tc.Result.Content, "      ")
				}
			}
		}

		fmt.Println()
		fmt.Println("========================================")
		fmt.Printf("停止原因: %s\n", result.StopReason)
		fmt.Printf("总步数:   %d\n", len(result.Steps))
		fmt.Printf("Token:    输入=%d, 输出=%d\n",
			result.TotalUsage.InputTokens, result.TotalUsage.OutputTokens)
		fmt.Println("========================================")
		fmt.Println()
		fmt.Println("助手>", result.FinalMessage)

		// 将助手回复加入对话历史
		messages = append(messages, llm.Message{
			Role:    llm.RoleAssistant,
			Content: result.FinalMessage,
		})
	}
}

// printIndented 缩进打印多行文本
func printIndented(text, indent string) {
	for _, line := range strings.Split(text, "\n") {
		fmt.Println(indent + line)
	}
}

// debugJSON 调试时打印 JSON（预留）
func debugJSON(v interface{}) {
	data, _ := json.MarshalIndent(v, "", "  ")
	fmt.Println(string(data))
}
