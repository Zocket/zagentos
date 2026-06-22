# P2: Tool Protocol Runtime

## 目标
实现 Tool 的标准接口和调用协议。

## 核心接口
见 `pkg/tool/tool.go`

## 实现要点
- [x] Tool interface 实现
- [x] Tool Registry（注册/查找/列举）
- [x] JSON Schema 生成（从 struct tag 反射）
- [x] 参数校验
- [x] JSON-RPC 2.0 通信（stdio 传输）
- [x] JSON-RPC 2.0 通信（HTTP 传输）
- [x] 并行 tool 调用支持
- [x] 错误处理和超时

## 设计决策

### 1. Tool 接口设计
Tool 接口保持极简：`Name / Description / InputSchema / Execute`。
InputSchema 返回 `map[string]interface{}` 而非自定义 Schema 类型，保持与 LLM API（OpenAI tool format）的直接兼容。

### 2. Registry 实现
- 使用 `sync.RWMutex` 保护并发读写
- `List()` 按名称排序返回，保证输出稳定
- 重复注册返回 error，不允许覆盖

### 3. Schema 生成（`internal/schema`）
- 通过 struct tag 反射生成 JSON Schema
- 支持两种 tag：`json:"name,omitempty"` 和 `schema:"desc=说明;required;enum=a,b;type=string"`
- `omitempty` 会自动取消 `required` 标记
- `interface{}` 映射为无类型约束（JSON Schema 的 any）

### 4. Executor 设计
- `Execute` 单次调用，自动做参数校验 + 超时控制
- `ExecuteBatch` 并行调用多个工具，结果顺序与输入一致
- 超时通过 `context.WithTimeout` 实现，工具内部应尊重 context

### 5. 传输层
- **stdio**: 按行读取 JSON-RPC 请求，按行输出响应。适合作为子进程被调用
- **HTTP**: 提供 `/rpc`（JSON-RPC 端点）和 `/tools`（REST 便捷接口）。适合远程调用
- 两者共用 `tools/list` 和 `tools/call` 方法路由

### 6. JSON-RPC 方法
| 方法 | 说明 |
|------|------|
| `tools/list` | 列出所有已注册工具 |
| `tools/call` | 调用指定工具，参数：`{name, input}` |

## 测试
```bash
# schema 测试
go test ./internal/schema/ -v

# tool 测试（含 registry + executor + transport）
go test ./pkg/tool/ -v
```

## 文件结构
```
pkg/tool/
├── tool.go        # 接口定义
├── registry.go    # 内存 Registry 实现
├── executor.go    # 执行器（校验 + 超时 + 并行）
├── stdio.go       # stdio 传输
├── http.go        # HTTP 传输
├── tool_test.go   # Registry + Executor 测试
└── transport_test.go  # 传输层测试
```
