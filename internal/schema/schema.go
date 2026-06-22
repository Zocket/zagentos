// Package schema 提供 JSON Schema 生成和校验工具。
// 用于从 Go struct 自动生成 tool 的 input schema。
package schema

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// Schema 表示一个 JSON Schema
type Schema struct {
	Type        string             `json:"type"`
	Properties  map[string]*Schema `json:"properties,omitempty"`
	Required    []string           `json:"required,omitempty"`
	Description string             `json:"description,omitempty"`
	Items       *Schema            `json:"items,omitempty"`
	Enum        []interface{}      `json:"enum,omitempty"`
	Default     interface{}        `json:"default,omitempty"`
}

// Generate 从 Go struct 反射生成 JSON Schema。
// 支持 struct tag: `json:"name,omitempty"`, `schema:"type=string;desc=说明;required;enum=a,b"`
func Generate(v interface{}) (*Schema, error) {
	t := reflect.TypeOf(v)
	if t == nil {
		return nil, fmt.Errorf("schema: nil type")
	}
	// 处理指针类型
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return generateType(t, "")
}

func generateType(t reflect.Type, desc string) (*Schema, error) {
	s := &Schema{Description: desc}

	switch t.Kind() {
	case reflect.String:
		s.Type = "string"

	case reflect.Bool:
		s.Type = "boolean"

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		s.Type = "integer"

	case reflect.Float32, reflect.Float64:
		s.Type = "number"

	case reflect.Array, reflect.Slice:
		s.Type = "array"
		var elemType reflect.Type
		if t.Kind() == reflect.Slice {
			elemType = t.Elem()
		} else {
			elemType = t.Elem()
		}
		if elemType.Kind() == reflect.Uint8 {
			// []byte → string
			s.Type = "string"
		} else {
			elemSchema, err := generateType(elemType, "")
			if err != nil {
				return nil, err
			}
			s.Items = elemSchema
		}

	case reflect.Map:
		// map[string]X → object with additionalProperties
		s.Type = "object"

	case reflect.Struct:
		s.Type = "object"
		s.Properties = make(map[string]*Schema)
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)
			// 跳过非导出字段
			if !field.IsExported() {
				continue
			}

			jsonTag := field.Tag.Get("json")
			if jsonTag == "-" {
				continue
			}

			name := field.Name
			omitempty := false
			if jsonTag != "" {
				parts := strings.Split(jsonTag, ",")
				if parts[0] != "" {
					name = parts[0]
				}
				for _, p := range parts[1:] {
					if p == "omitempty" {
						omitempty = true
					}
				}
			}

			// 解析 schema tag
			schemaTag := field.Tag.Get("schema")
			fieldDesc := ""
			fieldType := ""
			fieldRequired := false
			fieldEnum := ""
			if schemaTag != "" {
				for _, kv := range strings.Split(schemaTag, ";") {
					kv = strings.TrimSpace(kv)
					if kv == "required" {
						fieldRequired = true
					} else if strings.HasPrefix(kv, "desc=") {
						fieldDesc = strings.TrimPrefix(kv, "desc=")
					} else if strings.HasPrefix(kv, "type=") {
						fieldType = strings.TrimPrefix(kv, "type=")
					} else if strings.HasPrefix(kv, "enum=") {
						fieldEnum = strings.TrimPrefix(kv, "enum=")
					}
				}
			}

			// omitempty 的字段不是 required
			if omitempty {
				fieldRequired = false
			}

			fieldSchema, err := generateType(field.Type, fieldDesc)
			if err != nil {
				return nil, err
			}
			// 允许 schema tag 覆盖类型
			if fieldType != "" {
				fieldSchema.Type = fieldType
			}
			// enum
			if fieldEnum != "" {
				for _, e := range strings.Split(fieldEnum, ",") {
					e = strings.TrimSpace(e)
					fieldSchema.Enum = append(fieldSchema.Enum, e)
				}
			}

			s.Properties[name] = fieldSchema
			if fieldRequired {
				s.Required = append(s.Required, name)
			}
		}

	case reflect.Interface:
		// interface{} → any (不限制类型)
		s.Type = ""

	default:
		return nil, fmt.Errorf("schema: unsupported type kind: %s", t.Kind())
	}

	return s, nil
}

// Validate 校验数据是否符合 schema。
// data 通常是 map[string]interface{}（从 JSON 解码得到）。
func Validate(s *Schema, data interface{}) error {
	if s == nil {
		return nil
	}
	return validateValue(s, data)
}

func validateValue(s *Schema, data interface{}) error {
	if s.Type == "" {
		return nil // any type
	}

	switch s.Type {
	case "string":
		if _, ok := data.(string); !ok {
			return fmt.Errorf("schema: expected string, got %T", data)
		}

	case "boolean":
		if _, ok := data.(bool); !ok {
			return fmt.Errorf("schema: expected boolean, got %T", data)
		}

	case "integer":
		switch v := data.(type) {
		case float64:
			if v != float64(int64(v)) {
				return fmt.Errorf("schema: expected integer, got non-integer number %v", v)
			}
		case int, int64, int32:
			// ok
		default:
			return fmt.Errorf("schema: expected integer, got %T", data)
		}

	case "number":
		switch data.(type) {
		case float64, float32, int, int64, int32:
			// ok
		default:
			return fmt.Errorf("schema: expected number, got %T", data)
		}

	case "array":
		arr, ok := data.([]interface{})
		if !ok {
			return fmt.Errorf("schema: expected array, got %T", data)
		}
		if s.Items != nil {
			for i, item := range arr {
				if err := validateValue(s.Items, item); err != nil {
					return fmt.Errorf("schema: array item %d: %w", i, err)
				}
			}
		}

	case "object":
		obj, ok := data.(map[string]interface{})
		if !ok {
			return fmt.Errorf("schema: expected object, got %T", data)
		}
		// 校验 required 字段
		for _, req := range s.Required {
			if _, exists := obj[req]; !exists {
				return fmt.Errorf("schema: missing required field: %s", req)
			}
		}
		// 校验每个属性
		for key, val := range obj {
			if propSchema, ok := s.Properties[key]; ok {
				if err := validateValue(propSchema, val); err != nil {
					return fmt.Errorf("schema: field %q: %w", key, err)
				}
			}
		}
	}

	// enum 校验
	if len(s.Enum) > 0 {
		strVal := fmt.Sprintf("%v", data)
		found := false
		for _, e := range s.Enum {
			if fmt.Sprintf("%v", e) == strVal {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("schema: value %v not in enum %v", data, s.Enum)
		}
	}

	return nil
}

// MustGenerate 是 Generate 的 panic 版本，用于全局变量初始化
func MustGenerate(v interface{}) *Schema {
	s, err := Generate(v)
	if err != nil {
		panic(err)
	}
	return s
}

// ToString 把 schema 转为可读字符串（调试用）
func (s *Schema) String() string {
	if s == nil {
		return "<nil>"
	}
	parts := []string{strconv.Quote(s.Type)}
	if s.Description != "" {
		parts = append(parts, "desc="+s.Description)
	}
	if len(s.Required) > 0 {
		parts = append(parts, "required="+strings.Join(s.Required, ","))
	}
	return strings.Join(parts, " ")
}
