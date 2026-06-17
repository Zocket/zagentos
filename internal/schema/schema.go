// Package schema 提供 JSON Schema 生成和校验工具。
// 用于从 Go struct 自动生成 tool 的 input schema。
package schema

// Schema 表示一个 JSON Schema
type Schema struct {
	Type        string              `json:"type"`
	Properties  map[string]*Schema  `json:"properties,omitempty"`
	Required    []string            `json:"required,omitempty"`
	Description string              `json:"description,omitempty"`
	Items       *Schema             `json:"items,omitempty"`
	Enum        []interface{}       `json:"enum,omitempty"`
	Default     interface{}         `json:"default,omitempty"`
}

// Generate 从 Go struct 反射生成 JSON Schema
// TODO: 实现基于 struct tag 的 schema 生成
func Generate(v interface{}) (*Schema, error) {
	panic("not implemented")
}

// Validate 校验数据是否符合 schema
// TODO: 实现 JSON Schema 校验
func Validate(schema *Schema, data interface{}) error {
	panic("not implemented")
}
