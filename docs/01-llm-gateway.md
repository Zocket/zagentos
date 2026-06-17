# P1: LLM Gateway

## 目标
封装多个 LLM provider 的 API，提供统一接口。

## 核心接口
见 `pkg/llm/llm.go`

## 实现清单
- [x] OpenAI provider（Chat Completions API）
- [x] Anthropic provider（Messages API）
- [x] 本地模型 provider（Ollama）
- [x] 流式输出（SSE for OpenAI/Anthropic, NDJSON for Ollama）
- [x] Token 计数（简单估算）
- [x] 重试 + 指数退避 + jitter
- [x] Rate limiting（令牌桶）
- [x] Provider fallback（primary 失败自动切换 backup）

## 文件结构

```
pkg/llm/
├── llm.go          # 接口定义（Provider, Gateway, 核心类型）
├── errors.go       # 统一错误类型（APIError, IsRetryable, IsRateLimited）
├── options.go      # 配置（RetryConfig, RateLimiter, ProviderConfig）
├── openai.go       # OpenAI Chat Completions 实现
├── anthropic.go    # Anthropic Messages API 实现
├── ollama.go       # Ollama 本地模型实现
├── gateway.go      # Gateway 实现（多 provider 管理 + fallback）
└── llm_test.go     # 单元测试（mock HTTP server）
```

## 设计决策

### 1. 重试配置的零值问题
`RetryConfig` 的零值（所有字段为 0）等同于"使用默认配置"。如果要显式禁用重试，使用 `NoRetry()` 函数返回 `MaxRetries: -1`。

### 2. 流式输出用 channel
`Stream()` 返回 `<-chan StreamChunk`，消费者从 channel 读取直到收到 `Done: true` 或 channel 关闭。这比 callback 模式更 Go-idiomatic，也方便 select 多路复用。

### 3. Provider 间消息格式转换
每个 provider 内部维护自己的 API 数据结构（`openaiRequest`, `anthropicRequest` 等），在 `buildRequest` / `parseResponse` 中做格式转换。核心类型（`Message`, `ToolCall`）是 provider-agnostic 的。

### 4. Anthropic tool_result 的映射
Anthropic 的 tool result 必须作为 user 角色消息发送（`content: [{type: "tool_result", ...}]`），而 OpenAI/Ollama 使用独立的 tool 角色。我们在 `buildRequest` 中处理这个差异，对外统一用 `RoleTool`。

### 5. Gateway Fallback 策略
当前实现是线性 fallback：按注册顺序逐个尝试。可以通过 `SetFallbackOrder` 自定义顺序。未来可扩展为更复杂的路由策略（按 model 能力、按成本等）。

### 6. Token 计数是估算
精确计数需要 tiktoken（OpenAI）或对应的分词器，引入外部依赖。当前用字符数比例估算，对 context budget 管理来说够用。后续如果需要精确值，可以替换 `CountTokens` 的实现。

## 使用示例

```go
// 创建 provider
openai := llm.NewOpenAIProvider(llm.ProviderConfig{
    APIKey: os.Getenv("OPENAI_API_KEY"),
    Model:  "gpt-4o",
})

anthropic := llm.NewAnthropicProvider(llm.ProviderConfig{
    APIKey: os.Getenv("ANTHROPIC_API_KEY"),
    Model:  "claude-3-5-sonnet-20241022",
})

// 创建 Gateway，注册多个 provider
gw := llm.NewGateway()
gw.RegisterProvider("anthropic", anthropic)
gw.RegisterProvider("openai", openai)
gw.SetDefault("anthropic")

// 使用
resp, err := gw.Complete(ctx, &llm.CompletionRequest{
    Messages: []llm.Message{
        {Role: llm.RoleUser, Content: "Hello!"},
    },
    MaxTokens: 1024,
})
```

## 参考资料
- OpenAI API: https://platform.openai.com/docs/api-reference
- Anthropic API: https://docs.anthropic.com/en/api
- Ollama API: https://github.com/ollama/ollama/blob/main/docs/api.md
