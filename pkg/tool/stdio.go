package tool

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/Zocket/zagentos/internal/jsonrpc"
)

// stdioTransport 通过标准输入/输出实现 JSON-RPC 2.0 传输。
type stdioTransport struct {
	reader  io.Reader
	writer  io.Writer
	writerMu sync.Mutex
}

// NewStdioTransport 创建 stdio 传输，默认使用 os.Stdin / os.Stdout。
// 可传入自定义的 reader/writer 用于测试。
func NewStdioTransport(reader io.Reader, writer io.Writer) Transport {
	if reader == nil {
		return nil
	}
	if writer == nil {
		return nil
	}
	return &stdioTransport{
		reader: reader,
		writer: writer,
	}
}

// Serve 启动 stdio 服务，读取 JSON-RPC 请求并分发到 registry 中的工具。
// 阻塞直到 context 取消或输入 EOF。
func (t *stdioTransport) Serve(ctx context.Context, registry Registry) error {
	scanner := bufio.NewScanner(t.reader)
	// 增大 buffer 以支持较大的 JSON 请求
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for scanner.Scan() {
			line := scanner.Bytes()
			if len(line) == 0 {
				continue
			}
			t.handleLine(ctx, registry, line)
		}
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
		return nil
	}
}

// handleLine 处理一行 JSON-RPC 请求
func (t *stdioTransport) handleLine(ctx context.Context, registry Registry, line []byte) {
	var req jsonrpc.Request
	if err := json.Unmarshal(line, &req); err != nil {
		t.send(jsonrpc.NewErrorResponse(nil, jsonrpc.CodeParseError, "parse error"))
		return
	}

	// 路由方法
	switch req.Method {
	case "tools/list":
		t.handleToolsList(&req, registry)

	case "tools/call":
		t.handleToolsCall(ctx, &req, registry)

	default:
		t.send(jsonrpc.NewErrorResponse(req.ID, jsonrpc.CodeMethodNotFound,
			fmt.Sprintf("method %q not found", req.Method)))
	}
}

// handleToolsList 处理 tools/list 请求
func (t *stdioTransport) handleToolsList(req *jsonrpc.Request, registry Registry) {
	tools := registry.List()
	list := make([]map[string]interface{}, 0, len(tools))
	for _, tool := range tools {
		list = append(list, map[string]interface{}{
			"name":        tool.Name(),
			"description": tool.Description(),
			"inputSchema": tool.InputSchema(),
		})
	}
	result := map[string]interface{}{"tools": list}
	t.send(jsonrpc.NewResponse(req.ID, result))
}

// handleToolsCall 处理 tools/call 请求
func (t *stdioTransport) handleToolsCall(ctx context.Context, req *jsonrpc.Request, registry Registry) {
	// 解析参数
	params, ok := req.Params.(map[string]interface{})
	if !ok {
		t.send(jsonrpc.NewErrorResponse(req.ID, jsonrpc.CodeInvalidParams, "invalid params"))
		return
	}

	name, ok := params["name"].(string)
	if !ok {
		t.send(jsonrpc.NewErrorResponse(req.ID, jsonrpc.CodeInvalidParams, "missing tool name"))
		return
	}

	input, _ := params["input"].(map[string]interface{})

	tool, found := registry.Get(name)
	if !found {
		t.send(jsonrpc.NewErrorResponse(req.ID, jsonrpc.CodeMethodNotFound,
			fmt.Sprintf("tool %q not found", name)))
		return
	}

	result, err := tool.Execute(ctx, input)
	if err != nil {
		t.send(jsonrpc.NewErrorResponse(req.ID, jsonrpc.CodeInternalError, err.Error()))
		return
	}

	t.send(jsonrpc.NewResponse(req.ID, result))
}

// send 发送 JSON-RPC 响应（线程安全）
func (t *stdioTransport) send(resp *jsonrpc.Response) {
	data, err := json.Marshal(resp)
	if err != nil {
		return
	}
	t.writerMu.Lock()
	defer t.writerMu.Unlock()
	t.writer.Write(data)
	t.writer.Write([]byte("\n"))
}

// Close 关闭传输（stdio 模式下无需特殊处理）
func (t *stdioTransport) Close() error {
	return nil
}
