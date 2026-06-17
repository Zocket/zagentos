# AgentOS Go — 项目规格说明书

## 项目概述

本项目旨在用 Go 语言从零实现一个完整的 Agent Harness（Agent 运行时脚手架），系统性学习将 LLM 变成可靠 Agent 所需的全部工程模块。最终产出是一个可通过配置文件声明式定义 Agent 的运行时系统（AgentOS）。

### 核心理念

- **协议优先**：从通信协议（JSON-RPC 2.0）和标准接口（MCP）出发，而非 hack 式集成
- **接口驱动**：每个模块先定义 Go interface，再逐步填充实现
- **渐进式构建**：每完成一个阶段都有可运行的里程碑 demo
- **横切可观测**：Trace/Logging 从第一天就埋入，不是事后补丁

---

## 技术栈

| 层面 | 选型 |
|------|------|
| 语言 | Go 1.23+ |
| 模块管理 | Go Modules (monorepo) |
| 通信协议 | JSON-RPC 2.0 over stdio / HTTP |
| Agent 协议 | MCP (Model Context Protocol) |
| 向量存储 | SQLite + 简单向量搜索（起步）→ 可选升级 pgvector |
| 配置格式 | YAML |
| 可观测性 | 结构化日志 + OpenTelemetry compatible tracing |
| 评估 | 自定义 eval framework，JSON 定义 test case |

---

## 阶段与项目清单

### 阶段一：最小闭环（跑通一个能调工具的 Agent）

#### P1 — LLM Gateway (`pkg/llm`)

**目标**：封装多个 LLM Provider 的 API，提供统一的调用接口。

**功能需求**：
- 统一的 `Provider` 接口，支持 Complete（同步）和 Stream（流式）
- 实现 OpenAI provider（Chat Completions API）
- 实现 Anthropic provider（Messages API）
- 实现本地模型 provider（Ollama）
- Token 计数（精确或估算）
- 重试策略：指数退避 + jitter
- Rate limiting（令牌桶或滑动窗口）
- Provider fallback：primary 失败自动切换 backup
- Gateway 层：注册多个 provider，设置默认，按需路由

**核心类型**：Message、ToolCall、ToolDefinition、CompletionRequest/Response、StreamChunk、Usage

---

#### P2 — Tool Protocol Runtime (`pkg/tool`)

**目标**：定义 Tool 的标准接口，实现注册、校验和调用分发。

**功能需求**：
- `Tool` 接口：Name / Description / InputSchema / Execute
- `Registry`：注册、查找、列举、移除工具
- 从 Go struct tag 反射生成 JSON Schema（利用 `internal/schema`）
- 输入参数校验（基于 JSON Schema）
- 支持 stdio 传输（标准输入输出的 JSON-RPC 通信）
- 支持 HTTP 传输
- 并行 tool 调用支持
- 超时和错误处理

**依赖**：`internal/jsonrpc`、`internal/schema`

---

#### P3 — Tool Use Loop / ReAct Loop (`pkg/loop`)

**目标**：实现 LLM → Tool Call → Result → LLM 的自主执行循环。

**功能需求**：
- 基本 ReAct 循环：解析 LLM 输出中的 tool_use block → 调用工具 → 将结果回填 → 再次调用 LLM
- 最大迭代次数限制
- 错误恢复：tool 执行失败时通知 LLM 并允许其重试或换策略
- 支持单步内的并行 tool call
- Pre/Post hooks：在 LLM 调用和工具调用前后注入逻辑
- Step 记录：每一步的 request/response/tool executions 都被记录
- 执行结果：最终输出 + 完整步骤 + 停止原因 + 总 token 消耗

**依赖**：P1 (`pkg/llm`)、P2 (`pkg/tool`)

**里程碑 Demo**：`examples/minimal-agent/` — 完成 P1+P2+P3 后，用一个简单任务（如调用计算器工具）展示完整循环。

---

### 阶段二：MCP 协议栈（把工具层升级为标准协议）

#### P4 — MCP Server SDK (`pkg/mcp/server`)

**目标**：实现完整的 MCP Server，使其能被任何 MCP Client（如 Claude Desktop）调用。

**功能需求**：
- Server lifecycle 管理：created → initializing → ready → shutdown
- 实现 MCP 标准方法：
  - `initialize` / `initialized`
  - `tools/list` / `tools/call`
  - `resources/list` / `resources/read`
  - `prompts/list` / `prompts/get`（可选）
- stdio 传输（主要模式）
- HTTP + SSE 传输（可选扩展）
- Capability negotiation（服务端声明支持的能力）
- 错误处理符合 MCP 规范

**依赖**：P2 (`pkg/tool`)、`internal/jsonrpc`

---

#### P5 — MCP Client & Registry (`pkg/mcp/client`, `pkg/mcp/registry`)

**目标**：连接和管理多个 MCP Server，聚合它们的 tools。

**功能需求**：

MCP Client (`pkg/mcp/client`)：
- 连接单个 MCP Server（stdio 和 HTTP 两种方式）
- 连接状态管理（disconnected → connecting → connected → error）
- 执行 initialize 握手
- 调用 `tools/list`、`tools/call`、`resources/list`、`resources/read`
- 自动重连

MCP Registry (`pkg/mcp/registry`)：
- 注册、移除、动态加载/卸载 server
- 聚合所有 server 的 tools，添加命名空间前缀（避免冲突）
- 按工具全名路由调用到正确的 server
- 健康检查
- 配置热加载（修改配置文件后无需重启）

**里程碑 Demo**：`examples/mcp-demo/` — 启动一个自己写的 MCP Server，通过 Registry 连接并调用。

---

### 阶段三：上下文组装（Harness 的核心）

#### P6 — Prompt Assembler / Context Engine (`pkg/context`)

**目标**：在有限的 token 预算内，智能拼装 prompt 的各个组成部分。

**功能需求**：
- Token Budget 管理：总预算 = model context window - reserved_for_output
- Segment 模型：每个内容块有 ID、Role、Priority、预估 token 数
- 优先级排序：Critical（system prompt）> High（tools、当前对话）> Medium（memory、steering）> Low（背景）
- 动态裁剪：超出预算时按优先级从低到高丢弃，报告被丢弃的内容
- Prompt 模板引擎：支持变量替换（`{{variable}}`）
- 动态注入：tool definitions、steering rules、检索到的上下文、用户自定义段

**依赖**：`internal/token`

---

### 阶段四：记忆与检索

#### P7 — Conversation Memory (`pkg/memory/conversation`)

**目标**：管理当前对话的消息历史，支持压缩。

**功能需求**：
- Message list 的 CRUD
- Token 预算内裁剪（简单截断 vs 保留首尾）
- Context compaction：调用 LLM 将旧历史摘要化，保留近期消息原样
- Session 持久化（JSON 文件或 SQLite）
- 多 session 管理

---

#### P8 — Long-term Memory Store (`pkg/memory/longterm`)

**目标**：基于 embedding 的持久化语义记忆。

**功能需求**：
- `Embedder` 接口：调用 LLM embedding API 生成向量
- 向量存储（先用内存 map + 余弦相似度，再升级 SQLite）
- 两种记忆类型：
  - Episodic（事件记忆）：Agent 做过什么
  - Semantic（知识记忆）：Agent 学到的事实
- 语义检索：query → embed → top-K 相似
- 遗忘策略：按时间、数量上限、最低相关性删除
- 批量操作

---

#### P9 — Context Retrieval Engine / RAG (`pkg/retrieval`)

**目标**：从大量文档中检索相关内容并注入 Agent 的上下文。

**功能需求**：
- 文档分片：按 chunk size 和 overlap 切分
- 索引：对 chunks 生成 embedding 并存储
- 检索：query embedding → 相似度搜索 → top-K
- Reranking：对初步结果做二次排序（可选用 LLM 或交叉编码器）
- Token budget 内格式化：把检索结果拼成可注入 prompt 的文本
- 代码库索引（按文件、按函数切分）
- 增量更新（文件变更后只重新索引变更的 chunk）

**依赖**：P8 (`pkg/memory/longterm` 的 Embedder)

---

### 阶段五：技能、规划与约束

#### P10 — Skill Registry (`pkg/skill`)

**目标**：管理 Agent 的技能，支持渐进式披露。

**功能需求**：
- Skill 数据模型：name、description（摘要）、detail（完整说明）、triggers、依赖的 tools、子技能
- 触发条件：关键词匹配、语义相似度匹配、显式调用
- 注册、发现、移除
- **渐进式披露**：平时只把技能摘要暴露给 LLM（节省 token），被触发/激活时才注入完整详情
- Composite skill：由多个子技能组合而成的复合技能
- 匹配返回得分和匹配原因

---

#### P11 — Task Planner (`pkg/planner`)

**目标**：将用户高层意图分解为可执行的任务 DAG。

**功能需求**：
- LLM-based task decomposition：让 LLM 将大目标拆成小步骤
- 任务依赖建模：DAG（有向无环图）
- 拓扑排序调度：计算执行批次（同一批内可并行）
- 任务状态机：pending → in_progress → completed / failed / skipped
- Replan：执行过程中根据中间结果动态调整后续计划
- 失败处理：单任务失败时决定是重试、跳过、还是终止整个计划

---

#### P12 — Steering & Guardrails + Hooks (`pkg/steering`)

**目标**：约束和引导 Agent 行为，提供事件钩子机制。

**功能需求**：
- 从目录加载 Markdown 格式的 steering 规则文件
- 规则包含时机：always / file_match（当特定文件在上下文中时）/ manual
- Hook 事件类型：pre_tool_use、post_tool_use、pre_task、post_task、prompt_submit、agent_stop
- Hook 动作：ask_agent（注入 prompt）/ run_command（执行 shell 命令）
- 工具调用前后执行安全检查
- 权限分级：低风险（直接执行）/ 中风险（提示）/ 高风险（需确认）
- Hook 链执行：一个事件可触发多个 hook，按顺序执行

---

### 阶段六：多 Agent、可观测性、评估、整合

#### P13 — Sub-Agent Framework (`pkg/subagent`)

**目标**：支持父 Agent 委派子 Agent 独立执行任务。

**功能需求**：
- Agent 配置：system prompt + tool set + model + constraints
- Agent 实例化：从配置创建独立运行的 Agent
- 委托协议：父 Agent 下发 prompt，子 Agent 独立执行并返回结果
- 并行委托：同时向多个子 Agent 分发不同任务
- 结果收集和聚合
- 生命周期管理：创建、执行、销毁

**依赖**：P1、P2、P3（子 Agent 复用核心循环）

---

#### P14 — Agent Router (`pkg/router`)

**目标**：根据任务类型智能路由到最合适的 Agent。

**功能需求**：
- 能力注册：每个 Agent 声明自己擅长的领域
- 匹配算法：任务描述 → 能力匹配 → 选择最合适的 Agent
- 路由策略：best_match / round_robin / least_load
- Failover：primary 执行失败时自动切换到备选 Agent
- 负载感知：跟踪各 Agent 当前负载

---

#### P15 — Trace & Observability (`pkg/trace`)

**目标**：记录 Agent 执行的完整轨迹，提供结构化可观测性。

**功能需求**：
- Span-based tracing：每个 LLM 调用、tool 调用、规划步骤都是一个 Span
- Span 层级：支持 parent-child 关系，还原执行树
- 聚合指标：总 token、延迟、LLM 调用次数、tool 调用次数、错误率
- Exporter 接口：导出到文件 / stdout / OpenTelemetry Collector
- **横切设计**：建议从 P3 就开始集成 Tracer，此模块负责收拢和导出

---

#### P16 — Evaluation Framework (`pkg/eval`)

**目标**：自动化评估 Agent 质量，支持回归测试。

**功能需求**：
- Eval Case 格式：JSON/YAML 定义 input + expected behavior
- Expected behavior 维度：输出包含/不包含文本、使用了哪些工具、步数限制、token 消耗限制
- Runner：执行 Agent + 收集输出
- Scorer：多维度评分（pass/fail + 0-1 分数）
- Report：汇总 pass rate、逐 case 结果
- 回归模式：对比两次运行的分数差异
- CLI 入口：`cmd/eval/`

---

#### P17 — AgentOS 整合 (`cmd/agent`)

**目标**：把所有模块通过配置文件组装成一个可运行的 runtime。

**功能需求**：
- YAML 配置解析（格式见 `config/agent.example.yaml`）
- 模块发现和注册：根据配置决定启用哪些模块
- 依赖注入 / wiring：把各 interface 的实现串起来
- CLI 入口：交互式对话 + 单次执行模式
- HTTP API 入口（可选）
- 配置校验：启动前检查配置完整性
- Graceful shutdown：优雅关闭所有连接和 goroutine

---

## 共享内部模块

### `internal/jsonrpc`
JSON-RPC 2.0 协议实现。Request / Response / Notification / Error 类型，标准错误码，被 `pkg/tool` 和 `pkg/mcp` 共用。

### `internal/schema`
JSON Schema 生成（从 Go struct 反射）和校验。被 tool 参数定义和 MCP 使用。

### `internal/token`
Token 计数工具。提供简单估算器（字符数/比例）和精确计数器（tiktoken 兼容）接口。

---

## 里程碑 Demo

| Demo | 位置 | 前置项目 | 展示内容 |
|------|------|----------|----------|
| Minimal Agent | `examples/minimal-agent/` | P1+P2+P3 | 一个能调用计算器工具回答数学问题的 agent |
| MCP Demo | `examples/mcp-demo/` | P4+P5 | 自己写的 MCP Server 被 Client 连接和调用 |
| RAG Agent | `examples/rag-agent/` | P1-P3+P7-P9 | 索引文档后能检索回答问题的 agent |
| Multi-Agent | `examples/multi-agent/` | P13+P14 | 父 agent 把任务分发给专业子 agent 并汇总结果 |

---

## 开发约定

1. **Interface First**：每个 `pkg/xxx/xxx.go` 是接口定义，实现放在同 package 的其他文件（如 `openai.go`、`anthropic.go`）
2. **Internal 共享**：跨模块复用的工具放 `internal/`，不对外暴露
3. **Cmd 薄层**：`cmd/` 只做 wiring（读配置、初始化模块、启动），不含业务逻辑
4. **Docs 先行**：每个项目开始前先在 `docs/` 写清设计决策，再编码
5. **Testdata 固定**：测试用的 fixture 数据放 `testdata/`，保证测试可重复
6. **横切 Trace**：从 P3 开始就在关键路径埋 Span，P15 收拢导出

---

## 非功能需求

- **可测试性**：所有核心逻辑都通过 interface 可 mock
- **并发安全**：Registry 类模块需支持并发读写
- **错误处理**：使用 Go 惯用的 error wrapping，不 panic
- **文档**：所有导出类型和函数都有 GoDoc 注释
- **零外部依赖起步**：尽量用标准库，只在必要时引入第三方（如 YAML 解析用 `gopkg.in/yaml.v3`）

---

## 推荐学习路径

```
P1 → P2 → P3（最小 agent，阶段一里程碑）
     ↓
P4 → P5（MCP 协议栈，阶段二里程碑）
     ↓
    P6（上下文组装）
     ↓
P7 → P8 → P9（记忆 + RAG，阶段四里程碑）
     ↓
P10 → P11 → P12（技能 + 规划 + 约束）
     ↓
P13 → P14（多 agent，阶段六里程碑）
     ↓
P15 → P16 → P17（可观测 + 评估 + 整合）
```

也可以走 MVP 捷径：P1 → P2 → P3 → P6 → P10 → 回头补其余模块。
