// Package jsonrpc 实现 JSON-RPC 2.0 协议。
// 被 pkg/tool 和 pkg/mcp 共用。
package jsonrpc

// Request 是 JSON-RPC 2.0 请求
type Request struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"` // string | number | null
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// Response 是 JSON-RPC 2.0 响应
type Response struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *Error      `json:"error,omitempty"`
}

// Error 是 JSON-RPC 2.0 错误对象
type Error struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// 标准错误码
const (
	CodeParseError     = -32700
	CodeInvalidRequest = -32600
	CodeMethodNotFound = -32601
	CodeInvalidParams  = -32602
	CodeInternalError  = -32603
)

// Notification 是不需要响应的请求（id 为空）
type Notification struct {
	JSONRPC string      `json:"jsonrpc"`
	Method  string      `json:"method"`
	Params  interface{} `json:"params,omitempty"`
}

// NewRequest 创建一个新的请求
func NewRequest(id interface{}, method string, params interface{}) *Request {
	return &Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  method,
		Params:  params,
	}
}

// NewResponse 创建一个成功响应
func NewResponse(id interface{}, result interface{}) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
}

// NewErrorResponse 创建一个错误响应
func NewErrorResponse(id interface{}, code int, message string) *Response {
	return &Response{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &Error{Code: code, Message: message},
	}
}
