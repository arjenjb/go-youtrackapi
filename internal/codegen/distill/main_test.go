package main

import (
	"strings"
	"testing"

	"github.com/arjenjb/go-youtrackapi/internal/codegen"
)

func TestDistillModel(t *testing.T) {
	input := []byte(`{
  "components": {"schemas": {
    "Base": {
      "type": "object",
	  "description": "Represents a &lt;base&gt; resource.",
      "properties": {
		"id": {"type": "string", "description": "The stable ID.", "readOnly": true},
        "$type": {"type": "string", "readOnly": true}
      }
    },
    "Child": {
      "allOf": [
        {"$ref": "#/components/schemas/Base"},
        {"type": "object", "properties": {
          "enabled": {"type": "boolean"},
          "items": {"type": "array", "items": {"$ref": "#/components/schemas/Base"}},
          "value": {"type": "object"},
          "direction": {"type": "string", "enum": ["IN", "OUT"]}
        }}
      ]
    }
  }}
}`)

	openAPI, err := parseOpenAPI(input)
	if err != nil {
		t.Fatal(err)
	}
	got, err := distillModel(openAPI, overrides{
		AbstractTypes: []string{"Base"},
		DiscriminatorMappings: map[string]map[string]string{
			"Base": {"Base": "Child"},
		},
		FieldTypes: map[string]string{
			"Child.value": "Time",
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(got.Structs) != 2 {
		t.Fatalf("got %d structs, want 2", len(got.Structs))
	}
	base := findStruct(t, got, "Base")
	if !base.Abstract || len(base.Fields) != 1 || base.Fields[0].Name != "id" || base.Fields[0].Type != nil {
		t.Fatalf("unexpected Base descriptor: %#v", base)
	}
	if base.DiscriminatorMappings["Base"] != "Child" {
		t.Fatalf("unexpected Base discriminator mappings: %#v", base.DiscriminatorMappings)
	}
	if base.Description != "Represents a <base> resource." || base.Fields[0].Description != "The stable ID." {
		t.Fatalf("descriptions were not preserved: %#v", base)
	}
	child := findStruct(t, got, "Child")
	if child.Extends != "Base" {
		t.Fatalf("Child extends %q, want Base", child.Extends)
	}
	items := findField(t, child, "items")
	if items.Type == nil || items.Type.Kind != codegen.TypeDescriptorKindList || items.Type.Elems[0].Name != "Base" {
		t.Fatalf("unexpected items descriptor: %#v", items.Type)
	}
	value := findField(t, child, "value")
	if value.Type == nil || value.Type.Name != "Time" {
		t.Fatalf("unexpected value descriptor: %#v", value.Type)
	}
	direction := findField(t, child, "direction")
	if len(direction.Enum) != 2 || direction.Enum[0] != "IN" || direction.Enum[1] != "OUT" {
		t.Fatalf("unexpected direction enum: %#v", direction.Enum)
	}
	for _, field := range append(base.Fields, child.Fields...) {
		if field.Name == "$type" {
			t.Fatal("discriminator property should not become a model field")
		}
	}
}

func TestDistillRejectsStaleOverride(t *testing.T) {
	input := []byte(`{"components":{"schemas":{"Thing":{"type":"object"}}}}`)
	openAPI, err := parseOpenAPI(input)
	if err != nil {
		t.Fatal(err)
	}
	_, err = distillModel(openAPI, overrides{FieldTypes: map[string]string{"Thing.missing": "Time"}})
	if err == nil || !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("expected stale override error, got %v", err)
	}
}

func TestNormalizeDescription(t *testing.T) {
	input := `<p>Read the <a href="https://example.test/docs">documentation</a>.</p>
<p>Requires <control>Admin</control> permission.</p>`
	want := "Read the documentation (https://example.test/docs).\nRequires Admin permission."
	if got := normalizeDescription(input); got != want {
		t.Fatalf("normalizeDescription() = %q, want %q", got, want)
	}
}

func findStruct(t *testing.T, document *codegen.Document, name string) *codegen.StructDescriptor {
	t.Helper()
	for _, descriptor := range document.Structs {
		if descriptor.Name == name {
			return descriptor
		}
	}
	t.Fatalf("struct %q not found", name)
	return nil
}

func findField(t *testing.T, descriptor *codegen.StructDescriptor, name string) *codegen.FieldDescriptor {
	t.Helper()
	for _, field := range descriptor.Fields {
		if field.Name == name {
			return field
		}
	}
	t.Fatalf("field %q not found on %s", name, descriptor.Name)
	return nil
}
