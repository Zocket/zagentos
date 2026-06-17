# AgentOS Go

从零用 Go 实现 Agent Harness Engineering 各核心模块的学习项目。

## 项目结构

```
zagentos/
├── cmd/                        # 各模块的可执行入口
│   ├── agent/                  # 完整 agent CLI（阶段六整合）
│   ├── mcp-server/             # MCP Server 示例
│   └── eval/                   # Eval 运行器
├── pkg/                        # 核心库代码
│   ├── llm/                    # P1: LLM Gateway
│   ├── tool/                   # P2: Tool Protocol Runtime
│   ├── loop/                   # P3: Tool Use Loop (ReAct)
│   ├── mcp/                    # P4-P5: MCP Server SDK & Client Registry
│   │   ├── server/
│   │   ├── client/
│   │   └── registry/
│   ├── context/                # P6: Prompt Assembler / Context Engine
│   ├── memory/                 # P7-P8: Memory Management
│   │   ├── conversation/
│   │   └── longterm/
│   ├── retrieval/              # P9: Context Retrieval Engine (RAG)
│   ├── skill/                  # P10: Skill Registry
│   ├── planner/                # P11: Task Planner
│   ├── steering/               # P12: Steering & Guardrails + Hooks
│   ├── subagent/               # P13: Sub-Agent Framework
│   ├── router/                 # P14: Agent Router
│   ├── trace/                  # P15: Trace & Observability
│   └── eval/                   # P16: Evaluation Framework
├── internal/                   # 内部共享工具
│   ├── jsonrpc/                # JSON-RPC 2.0 实现（被 tool/mcp 共用）
│   ├── schema/                 # JSON Schema 生成/校验
│   └── token/                  # Token 计数工具
├── config/                     # 配置文件示例
│   └── agent.example.yaml
├── docs/                       # 设计文档
├── examples/                   # 各阶段的集成 demo
│   ├── minimal-agent/          # 阶段一完成后的最小 agent
│   ├── mcp-demo/               # MCP Server/Client 交互演示
│   ├── rag-agent/              # 带 RAG 的 agent
│   └── multi-agent/            # 多 agent 协作演示
└── testdata/                   # 测试用的固定数据
    ├── tools/
    ├── prompts/
    └── eval-cases/
```

## 学习阶段

| 阶段 | 项目 | 目标 |
|------|------|------|
| 一：最小闭环 | P1 LLM Gateway → P2 Tool Runtime → P3 ReAct Loop | 跑通一个能调工具的 agent |
| 二：MCP 协议 | P4 MCP Server SDK → P5 MCP Client & Registry | 完整实现 MCP 协议栈 |
| 三：上下文组装 | P6 Context Engine | 智能 prompt 拼装 |
| 四：记忆与检索 | P7 对话记忆 → P8 长期记忆 → P9 RAG | 让 agent 有记忆 |
| 五：技能与规划 | P10 Skill Registry → P11 Task Planner → P12 Steering | 让 agent 能规划和受约束 |
| 六：多 Agent 与运维 | P13 Sub-Agent → P14 Router → P15 Observability → P16 Eval → P17 整合 | 完整系统 |

## 开发约定

- 每个 `pkg/xxx/` 包的根文件是接口定义（`xxx.go`），实现放在子文件里
- `internal/` 是跨模块共享但不对外暴露的工具
- `cmd/` 里是可执行程序，保持薄层（只做 wiring）
- `examples/` 是阶段里程碑 demo
- `docs/` 里每个模块一篇设计文档，先写设计再写代码
