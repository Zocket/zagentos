package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"time"

	"github.com/Zocket/zagentos/internal/jsonrpc"
)

// httpTransport 通过 HTTP 实现 JSON-RPC 2.0 传输。
type httpTransport struct {
	server *http.Server
	addr   string
	ln     net.Listener
}

// NewHTTPTransport 创建 HTTP 传输。
// addr 是监听地址，如 ":8080" 或 "127.0.0.1:9090"。
func NewHTTPTransport(addr string) Transport {
	return &httpTransport{
		addr: addr,
	}
}

// Serve 启动 HTTP 服务，阻塞直到 context 取消。
func (t *httpTransport) Serve(ctx context.Context, registry Registry) error {
	mux := http.NewServeMux()

	// POST /rpc — JSON-RPC 端点
	mux.HandleFunc("/rpc", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req jsonrpc.Request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, jsonrpc.NewErrorResponse(nil, jsonrpc.CodeParseError, "parse error"))
			return
		}

		resp := t.handleRequest(ctx, &req, registry)
		writeJSON(w, resp)
	})

	// GET /tools — 列出所有工具（便捷接口）
	mux.HandleFunc("/tools", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		tools := registry.List()
		list := make([]map[string]interface{}, 0, len(tools))
		for _, tool := range tools {
			list = append(list, map[string]interface{}{
				"name":        tool.Name(),
				"description": tool.Description(),
				"inputSchema": tool.InputSchema(),
			})
		}
		writeJSON(w, map[string]interface{}{"tools": list})
	})

	t.ln, _ = net.Listen("tcp", t.addr)

	t.server = &http.Server{
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- t.server.Serve(t.ln)
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		t.server.Shutdown(shutdownCtx)
		return nil
	case err := <-errCh:
		if err == http.ErrServerClosed {
			return nil
		}
		return err
	}
}

// handleRequest 路由 JSON-RPC 请求
func (t *httpTransport) handleRequest(ctx context.Context, req *jsonrpc.Request, registry Registry) *jsonrpc.Response {
	switch req.Method {
	case "tools/list":
		tools := registry.List()
		list := make([]map[string]interface{}, 0, len(tools))
		for _, tool := range tools {
			list = append(list, map[string]interface{}{
				"name":        tool.Name(),
				"description": tool.Description(),
				"inputSchema": tool.InputSchema(),
			})
		}
		return jsonrpc.NewResponse(req.ID, map[string]interface{}{"tools": list})

	case "tools/call":
		params, ok := req.Params.(map[string]interface{})
		if !ok {
			return jsonrpc.NewErrorResponse(req.ID, jsonrpc.CodeInvalidParams, "invalid params")
		}

		name, ok := params["name"].(string)
		if !ok {
			return jsonrpc.NewErrorResponse(req.ID, jsonrpc.CodeInvalidParams, "missing tool name")
		}

		input, _ := params["input"].(map[string]interface{})

		tool, found := registry.Get(name)
		if !found {
			return jsonrpc.NewErrorResponse(req.ID, jsonrpc.CodeMethodNotFound,
				fmt.Sprintf("tool %q not found", name))
		}

		result, err := tool.Execute(ctx, input)
		if err != nil {
			return jsonrpc.NewErrorResponse(req.ID, jsonrpc.CodeInternalError, err.Error())
		}
		return jsonrpc.NewResponse(req.ID, result)

	default:
		return jsonrpc.NewErrorResponse(req.ID, jsonrpc.CodeMethodNotFound,
			fmt.Sprintf("method %q not found", req.Method))
	}
}

// Addr 返回实际监听地址（Serve 后可用）
func (t *httpTransport) Addr() string {
	if t.ln != nil {
		return t.ln.Addr().String()
	}
	return t.addr
}

// Close 关闭 HTTP 服务
func (t *httpTransport) Close() error {
	if t.server != nil {
		return t.server.Close()
	}
	return nil
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
