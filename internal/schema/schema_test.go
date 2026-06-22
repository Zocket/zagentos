package schema

import (
	"testing"
)

func TestGenerate_BasicTypes(t *testing.T) {
	type simple struct {
		Name string `json:"name" schema:"desc=用户名;required"`
		Age  int    `json:"age"`
	}

	s, err := Generate(simple{})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	if s.Type != "object" {
		t.Errorf("expected type 'object', got %q", s.Type)
	}
	if len(s.Properties) != 2 {
		t.Errorf("expected 2 properties, got %d", len(s.Properties))
	}
	if s.Properties["name"].Type != "string" {
		t.Errorf("expected name type 'string', got %q", s.Properties["name"].Type)
	}
	if s.Properties["name"].Description != "用户名" {
		t.Errorf("expected name desc '用户名', got %q", s.Properties["name"].Description)
	}
	if s.Properties["age"].Type != "integer" {
		t.Errorf("expected age type 'integer', got %q", s.Properties["age"].Type)
	}
	// name should be required
	found := false
	for _, r := range s.Required {
		if r == "name" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'name' in required")
	}
}

func TestGenerate_OmitEmpty(t *testing.T) {
	type withOmitEmpty struct {
		Name string `json:"name,omitempty" schema:"required"`
	}
	s, err := Generate(withOmitEmpty{})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	// omitempty 应该覆盖 required
	for _, r := range s.Required {
		if r == "name" {
			t.Error("omitempty field should not be required")
		}
	}
}

func TestGenerate_ArrayAndMap(t *testing.T) {
	type complex struct {
		Tags []string `json:"tags"`
		Meta map[string]interface{} `json:"meta"`
	}
	s, err := Generate(complex{})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if s.Properties["tags"].Type != "array" {
		t.Errorf("expected tags type 'array', got %q", s.Properties["tags"].Type)
	}
	if s.Properties["tags"].Items.Type != "string" {
		t.Errorf("expected tags items type 'string', got %q", s.Properties["tags"].Items.Type)
	}
	if s.Properties["meta"].Type != "object" {
		t.Errorf("expected meta type 'object', got %q", s.Properties["meta"].Type)
	}
}

func TestGenerate_Enum(t *testing.T) {
	type withEnum struct {
		Status string `json:"status" schema:"enum=active,inactive,pending"`
	}
	s, err := Generate(withEnum{})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if len(s.Properties["status"].Enum) != 3 {
		t.Errorf("expected 3 enum values, got %d", len(s.Properties["status"].Enum))
	}
}

func TestGenerate_SkipUnexported(t *testing.T) {
	type withPrivate struct {
		Public  string `json:"public"`
		private string
	}
	s, err := Generate(withPrivate{})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if _, ok := s.Properties["private"]; ok {
		t.Error("private field should be skipped")
	}
	if len(s.Properties) != 1 {
		t.Errorf("expected 1 property, got %d", len(s.Properties))
	}
}

func TestValidate_String(t *testing.T) {
	s := &Schema{Type: "string"}

	if err := Validate(s, "hello"); err != nil {
		t.Errorf("expected valid: %v", err)
	}
	if err := Validate(s, 42); err == nil {
		t.Error("expected error for non-string")
	}
}

func TestValidate_Integer(t *testing.T) {
	s := &Schema{Type: "integer"}

	// JSON 解码后数字都是 float64
	if err := Validate(s, float64(42)); err != nil {
		t.Errorf("expected valid: %v", err)
	}
	if err := Validate(s, float64(42.5)); err == nil {
		t.Error("expected error for non-integer")
	}
	if err := Validate(s, "42"); err == nil {
		t.Error("expected error for string")
	}
}

func TestValidate_Object_Required(t *testing.T) {
	s := &Schema{
		Type: "object",
		Properties: map[string]*Schema{
			"name": {Type: "string"},
			"age":  {Type: "integer"},
		},
		Required: []string{"name"},
	}

	// 合法
	if err := Validate(s, map[string]interface{}{
		"name": "Alice",
		"age":  float64(30),
	}); err != nil {
		t.Errorf("expected valid: %v", err)
	}

	// 缺少 required
	if err := Validate(s, map[string]interface{}{
		"age": float64(30),
	}); err == nil {
		t.Error("expected error for missing required field")
	}

	// 类型错误
	if err := Validate(s, map[string]interface{}{
		"name": 123,
	}); err == nil {
		t.Error("expected error for wrong type")
	}
}

func TestValidate_Array(t *testing.T) {
	s := &Schema{
		Type:  "array",
		Items: &Schema{Type: "string"},
	}

	if err := Validate(s, []interface{}{"a", "b"}); err != nil {
		t.Errorf("expected valid: %v", err)
	}
	if err := Validate(s, []interface{}{"a", 123}); err == nil {
		t.Error("expected error for non-string item")
	}
}

func TestValidate_Enum(t *testing.T) {
	s := &Schema{
		Type: "string",
		Enum: []interface{}{"red", "green", "blue"},
	}

	if err := Validate(s, "red"); err != nil {
		t.Errorf("expected valid: %v", err)
	}
	if err := Validate(s, "yellow"); err == nil {
		t.Error("expected error for value not in enum")
	}
}

func TestValidate_AnyType(t *testing.T) {
	s := &Schema{Type: ""} // any
	if err := Validate(s, "anything"); err != nil {
		t.Errorf("expected valid: %v", err)
	}
	if err := Validate(s, 42); err != nil {
		t.Errorf("expected valid: %v", err)
	}
	if err := Validate(s, nil); err != nil {
		t.Errorf("expected valid: %v", err)
	}
}

func TestMustGenerate(t *testing.T) {
	type simple struct {
		Name string `json:"name"`
	}
	s := MustGenerate(simple{})
	if s.Type != "object" {
		t.Errorf("expected type 'object', got %q", s.Type)
	}
}
