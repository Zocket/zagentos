package tool

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/Zocket/zagentos/internal/jsonrpc"
)

func TestStdioTransport_ToolsList(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockTool{name: "tool1", description: "first tool"})
	r.Register(&mockTool{name: "tool2", description: "second tool"})

	input := `{"jsonrpc":"2.0","id":1,"method":"tools/list"}` + "\n"
	reader := strings.NewReader(input)
	var buf bytes.Buffer

	transport := NewStdioTransport(reader, &buf)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	transport.Serve(ctx, r)

	var resp jsonrpc.Response
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v\nraw: %s", err, buf.String())
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", resp.Result)
	}
	tools, ok := result["tools"].([]interface{})
	if !ok {
		t.Fatalf("expected tools array, got %T", result["tools"])
	}
	if len(tools) != 2 {
		t.Errorf("expected 2 tools, got %d", len(tools))
	}
}

func TestStdioTransport_ToolsCall(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockTool{
		name: "echo",
		execFn: func(ctx context.Context, input map[string]interface{}) (*Result, error) {
			msg, _ := input["msg"].(string)
			return &Result{Content: msg}, nil
		},
	})

	input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"echo","input":{"msg":"hello"}}}` + "\n"
	reader := strings.NewReader(input)
	var buf bytes.Buffer

	transport := NewStdioTransport(reader, &buf)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	transport.Serve(ctx, r)

	var resp jsonrpc.Response
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v\nraw: %s", err, buf.String())
	}

	result, ok := resp.Result.(map[string]interface{})
	if !ok {
		t.Fatalf("expected map result, got %T", resp.Result)
	}
	if result["content"] != "hello" {
		t.Errorf("expected content 'hello', got %v", result["content"])
	}
}

func TestStdioTransport_ToolNotFound(t *testing.T) {
	r := NewRegistry()

	input := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"nope"}}` + "\n"
	reader := strings.NewReader(input)
	var buf bytes.Buffer

	transport := NewStdioTransport(reader, &buf)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	transport.Serve(ctx, r)

	var resp jsonrpc.Response
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("expected error response")
	}
	if resp.Error.Code != jsonrpc.CodeMethodNotFound {
		t.Errorf("expected code %d, got %d", jsonrpc.CodeMethodNotFound, resp.Error.Code)
	}
}

func TestStdioTransport_UnknownMethod(t *testing.T) {
	r := NewRegistry()

	input := `{"jsonrpc":"2.0","id":1,"method":"unknown/method"}` + "\n"
	reader := strings.NewReader(input)
	var buf bytes.Buffer

	transport := NewStdioTransport(reader, &buf)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	transport.Serve(ctx, r)

	var resp jsonrpc.Response
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("expected error response")
	}
	if resp.Error.Code != jsonrpc.CodeMethodNotFound {
		t.Errorf("expected code %d, got %d", jsonrpc.CodeMethodNotFound, resp.Error.Code)
	}
}

func TestStdioTransport_InvalidJSON(t *testing.T) {
	r := NewRegistry()

	input := `not valid json` + "\n"
	reader := strings.NewReader(input)
	var buf bytes.Buffer

	transport := NewStdioTransport(reader, &buf)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	transport.Serve(ctx, r)

	var resp jsonrpc.Response
	if err := json.Unmarshal(buf.Bytes(), &resp); err != nil {
		t.Fatalf("failed to parse response: %v", err)
	}

	if resp.Error == nil {
		t.Fatal("expected error response")
	}
	if resp.Error.Code != jsonrpc.CodeParseError {
		t.Errorf("expected code %d, got %d", jsonrpc.CodeParseError, resp.Error.Code)
	}
}

func TestHTTPTransport_ToolsList(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockTool{name: "tool1", description: "first"})

	transport := NewHTTPTransport("127.0.0.1:0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go transport.Serve(ctx, r)

	// 等待服务器启动
	time.Sleep(100 * time.Millisecond)

	addr := transport.(*httpTransport).Addr()
	url := "http://" + addr + "/tools"

	resp, err := testHTTPGet(url)
	if err != nil {
		t.Fatalf("HTTP GET failed: %v", err)
	}
	if !strings.Contains(resp, "tool1") {
		t.Errorf("expected response to contain 'tool1', got: %s", resp)
	}
}

func TestHTTPTransport_RPC(t *testing.T) {
	r := NewRegistry()
	r.Register(&mockTool{
		name: "echo",
		execFn: func(ctx context.Context, input map[string]interface{}) (*Result, error) {
			msg, _ := input["msg"].(string)
			return &Result{Content: msg}, nil
		},
	})

	transport := NewHTTPTransport("127.0.0.1:0")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go transport.Serve(ctx, r)
	time.Sleep(100 * time.Millisecond)

	addr := transport.(*httpTransport).Addr()
	url := "http://" + addr + "/rpc"

	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"echo","input":{"msg":"hello"}}}`
	resp, err := testHTTPPost(url, body)
	if err != nil {
		t.Fatalf("HTTP POST failed: %v", err)
	}
	if !strings.Contains(resp, "hello") {
		t.Errorf("expected response to contain 'hello', got: %s", resp)
	}
}

// testHTTPGet 是测试辅助函数
func testHTTPGet(url string) (string, error) {
	return testHTTPDo("GET", url, "")
}

// testHTTPPost 是测试辅助函数
func testHTTPPost(url, body string) (string, error) {
	return testHTTPDo("POST", url, body)
}

func testHTTPDo(method, url, body string) (string, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	var bodyReader *strings.Reader
	if body != "" {
		bodyReader = strings.NewReader(body)
	}
	var req *http.Request
	var err error
	if bodyReader != nil {
		req, err = http.NewRequest(method, url, bodyReader)
	} else {
		req, err = http.NewRequest(method, url, nil)
	}
	if err != nil {
		return "", err
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	return buf.String(), nil
}
