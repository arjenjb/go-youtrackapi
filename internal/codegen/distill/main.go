// Command distill generates YouTrack API models and Client methods directly
// from YouTrack's OpenAPI document.
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/format"
	"go/token"
	"html"
	"io"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/arjenjb/go-youtrackapi/internal/codegen"
)

type openAPIDocument struct {
	Paths      map[string]pathItem `json:"paths"`
	Components struct {
		Schemas map[string]schema `json:"schemas"`
	} `json:"components"`
}

type schema struct {
	Ref         string            `json:"$ref"`
	Type        string            `json:"type"`
	Description string            `json:"description"`
	Properties  map[string]schema `json:"properties"`
	Items       *schema           `json:"items"`
	AllOf       []schema          `json:"allOf"`
	Enum        []string          `json:"enum"`
	Format      string            `json:"format"`
}

type overrides struct {
	AbstractTypes         []string                     `json:"abstractTypes"`
	DiscriminatorMappings map[string]map[string]string `json:"discriminatorMappings"`
	FieldTypes            map[string]string            `json:"fieldTypes"`
}

func main() {
	source := flag.String("source", "-", "OpenAPI JSON URL or local path; - reads stdin")
	overridePath := flag.String("overrides", "", "optional JSON file with semantic overrides")
	modelOutput := flag.String("model-output", "-", "output path for generated model Go; - writes stdout")
	clientOutput := flag.String("client-output", "", "optional output path for generated Client methods")
	flag.Parse()

	if err := run(*source, *overridePath, *modelOutput, *clientOutput); err != nil {
		fmt.Fprintln(os.Stderr, "distill:", err)
		os.Exit(1)
	}
}

func run(source, overridePath, modelOutput, clientOutput string) error {
	input, err := readSource(source)
	if err != nil {
		return err
	}

	var config overrides
	if overridePath != "" {
		data, err := os.ReadFile(overridePath)
		if err != nil {
			return fmt.Errorf("read overrides: %w", err)
		}
		if err := json.Unmarshal(data, &config); err != nil {
			return fmt.Errorf("decode overrides: %w", err)
		}
	}

	document, err := parseOpenAPI(input)
	if err != nil {
		return err
	}
	modelDocument, err := distillModel(document, config)
	if err != nil {
		return err
	}
	modelResult, err := renderModelGo(modelDocument, source)
	if err != nil {
		return err
	}
	var clientResult []byte
	if clientOutput != "" {
		clientResult, err = distillClientDocument(document, config, source)
		if err != nil {
			return err
		}
	}
	if err := writeOutput(modelOutput, modelResult); err != nil {
		return err
	}
	if clientOutput != "" {
		return writeOutput(clientOutput, clientResult)
	}
	return nil
}

func readSource(source string) ([]byte, error) {
	if source == "-" {
		return io.ReadAll(os.Stdin)
	}
	if !strings.HasPrefix(source, "http://") && !strings.HasPrefix(source, "https://") {
		data, err := os.ReadFile(source)
		if err != nil {
			return nil, fmt.Errorf("read OpenAPI document: %w", err)
		}
		return data, nil
	}

	client := &http.Client{Timeout: 30 * time.Second}
	response, err := client.Get(source)
	if err != nil {
		return nil, fmt.Errorf("download OpenAPI document: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, fmt.Errorf("download OpenAPI document: %s", response.Status)
	}
	data, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("read OpenAPI response: %w", err)
	}
	return data, nil
}

func parseOpenAPI(input []byte) (openAPIDocument, error) {
	var document openAPIDocument
	if err := json.Unmarshal(input, &document); err != nil {
		return openAPIDocument{}, fmt.Errorf("decode OpenAPI document: %w", err)
	}
	if len(document.Components.Schemas) == 0 {
		return openAPIDocument{}, errors.New("OpenAPI document has no component schemas")
	}
	return document, nil
}

func distillModel(document openAPIDocument, config overrides) (*codegen.Document, error) {
	abstract := make(map[string]bool, len(config.AbstractTypes))
	for _, name := range config.AbstractTypes {
		if _, ok := document.Components.Schemas[name]; !ok {
			return nil, fmt.Errorf("abstract type override %q does not exist", name)
		}
		abstract[name] = true
	}

	typeNames := sortedKeys(document.Components.Schemas)
	result := &codegen.Document{}
	seenOverrides := make(map[string]bool, len(config.FieldTypes))
	for _, name := range typeNames {
		s := document.Components.Schemas[name]
		extends, properties, err := flattenSchema(s)
		if err != nil {
			return nil, fmt.Errorf("schema %s: %w", name, err)
		}

		fieldNames := sortedKeys(properties)
		typeDescriptor := &codegen.StructDescriptor{
			Name:                  name,
			Description:           normalizeDescription(schemaDescriptionFor(name, document.Components.Schemas, nil)),
			Extends:               extends,
			Abstract:              abstract[name],
			DiscriminatorMappings: maps.Clone(config.DiscriminatorMappings[name]),
		}
		for _, fieldName := range fieldNames {
			if fieldName == "$type" {
				continue
			}
			property := properties[fieldName]
			fieldType, err := schemaType(property)
			if err != nil {
				return nil, fmt.Errorf("schema %s field %s: %w", name, fieldName, err)
			}
			overrideName := name + "." + fieldName
			if replacement, ok := config.FieldTypes[overrideName]; ok {
				fieldType = replacement
				seenOverrides[overrideName] = true
			}
			fieldDescriptor := &codegen.FieldDescriptor{
				Name:        fieldName,
				Description: normalizeDescription(property.Description),
				Enum:        append([]string(nil), property.Enum...),
			}
			if fieldType != "string" {
				fieldDescriptor.Type = modelTypeDescriptor(fieldType)
			}
			typeDescriptor.Fields = append(typeDescriptor.Fields, fieldDescriptor)
		}
		result.Structs = append(result.Structs, typeDescriptor)
	}

	for name := range config.FieldTypes {
		if !seenOverrides[name] {
			return nil, fmt.Errorf("field type override %q does not exist", name)
		}
	}
	for name, mappings := range config.DiscriminatorMappings {
		base := findStructDescriptor(result, name)
		if base == nil {
			return nil, fmt.Errorf("discriminator mapping type %q does not exist", name)
		}
		if !base.Abstract {
			return nil, fmt.Errorf("discriminator mapping type %q is not abstract", name)
		}
		children := make(map[string]bool)
		for _, child := range result.AllChildrenOf(*base) {
			children[child.Name] = true
		}
		for discriminator, target := range mappings {
			if discriminator == "" {
				return nil, fmt.Errorf("discriminator mapping for %q has an empty discriminator", name)
			}
			if !children[target] {
				return nil, fmt.Errorf("discriminator mapping %q.%s targets non-child type %q", name, discriminator, target)
			}
		}
	}
	return result, nil
}

func findStructDescriptor(document *codegen.Document, name string) *codegen.StructDescriptor {
	for _, descriptor := range document.Structs {
		if descriptor.Name == name {
			return descriptor
		}
	}
	return nil
}

func schemaDescription(s schema) string {
	if s.Description != "" {
		return s.Description
	}
	for _, part := range s.AllOf {
		if part.Description != "" {
			return part.Description
		}
	}
	return ""
}

func schemaDescriptionFor(name string, schemas map[string]schema, seen map[string]bool) string {
	if seen == nil {
		seen = make(map[string]bool)
	}
	if seen[name] {
		return ""
	}
	seen[name] = true
	s := schemas[name]
	if description := schemaDescription(s); description != "" {
		return description
	}
	for _, part := range s.AllOf {
		if part.Ref == "" {
			continue
		}
		parent, err := referenceName(part.Ref)
		if err == nil {
			if description := schemaDescriptionFor(parent, schemas, seen); description != "" {
				return description
			}
		}
	}
	return ""
}

var (
	linkPattern           = regexp.MustCompile(`(?is)<a\s+[^>]*href=["']([^"']+)["'][^>]*>(.*?)</a>`)
	paragraphBreakPattern = regexp.MustCompile(`(?is)</p\s*>`)
	lineBreakPattern      = regexp.MustCompile(`(?is)<br\s*/?>`)
	tagPattern            = regexp.MustCompile(`(?is)<[^>]+>`)
)

func normalizeDescription(description string) string {
	if description == "" {
		return ""
	}
	description = linkPattern.ReplaceAllString(description, `$2 ($1)`)
	description = paragraphBreakPattern.ReplaceAllString(description, "\n\n")
	description = lineBreakPattern.ReplaceAllString(description, "\n")
	description = tagPattern.ReplaceAllString(description, "")
	description = html.UnescapeString(description)

	var paragraphs []string
	var paragraph []string
	flush := func() {
		if len(paragraph) > 0 {
			paragraphs = append(paragraphs, strings.Join(paragraph, " "))
			paragraph = nil
		}
	}
	for _, line := range strings.Split(description, "\n") {
		line = strings.Join(strings.Fields(line), " ")
		if line == "" {
			flush()
		} else {
			paragraph = append(paragraph, line)
		}
	}
	flush()
	return strings.Join(paragraphs, "\n")
}

func modelTypeDescriptor(name string) *codegen.TypeDescriptor {
	if elementName, ok := strings.CutPrefix(name, "[]"); ok {
		return &codegen.TypeDescriptor{
			Kind:  codegen.TypeDescriptorKindList,
			Elems: []*codegen.TypeDescriptor{modelTypeDescriptor(elementName)},
		}
	}
	return &codegen.TypeDescriptor{Kind: codegen.TypeDescriptorKindBasic, Name: name}
}

func flattenSchema(s schema) (string, map[string]schema, error) {
	properties := make(map[string]schema)
	for name, property := range s.Properties {
		properties[name] = property
	}

	extends := ""
	for _, part := range s.AllOf {
		if part.Ref != "" {
			if extends != "" {
				return "", nil, errors.New("multiple inherited schemas are not supported")
			}
			var err error
			extends, err = referenceName(part.Ref)
			if err != nil {
				return "", nil, err
			}
		}
		for name, property := range part.Properties {
			properties[name] = property
		}
	}
	return extends, properties, nil
}

func schemaType(s schema) (string, error) {
	if s.Ref != "" {
		return referenceName(s.Ref)
	}
	switch s.Type {
	case "string":
		return "string", nil
	case "boolean":
		return "boolean", nil
	case "integer":
		return "integer", nil
	case "number":
		return "float", nil
	case "object", "":
		return "any", nil
	case "array":
		if s.Items == nil {
			return "", errors.New("array has no item schema")
		}
		itemType, err := schemaType(*s.Items)
		if err != nil {
			return "", err
		}
		return "[]" + itemType, nil
	default:
		return "", fmt.Errorf("unsupported OpenAPI type %q", s.Type)
	}
}

func referenceName(ref string) (string, error) {
	const prefix = "#/components/schemas/"
	name, ok := strings.CutPrefix(ref, prefix)
	if !ok || name == "" || strings.Contains(name, "/") {
		return "", fmt.Errorf("unsupported schema reference %q", ref)
	}
	return name, nil
}

func renderModelGo(document *codegen.Document, source string) ([]byte, error) {
	node, err := codegen.NewGenerator(document).Generate()
	if err != nil {
		return nil, fmt.Errorf("generate model: %w", err)
	}
	var out bytes.Buffer
	fmt.Fprintln(&out, "// Code generated by go generate; DO NOT EDIT.")
	fmt.Fprintf(&out, "// Source: %s\n\n", source)
	if err := format.Node(&out, token.NewFileSet(), node); err != nil {
		return nil, fmt.Errorf("format model: %w", err)
	}
	return out.Bytes(), nil
}

func writeOutput(path string, data []byte) error {
	if path == "-" {
		_, err := os.Stdout.Write(data)
		return err
	}
	directory := filepath.Dir(path)
	temporary, err := os.CreateTemp(directory, ".generated-*")
	if err != nil {
		return fmt.Errorf("create temporary output: %w", err)
	}
	temporaryName := temporary.Name()
	defer os.Remove(temporaryName)
	if _, err := temporary.Write(data); err != nil {
		temporary.Close()
		return fmt.Errorf("write temporary output: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary output: %w", err)
	}
	if err := os.Rename(temporaryName, path); err != nil {
		return fmt.Errorf("replace output: %w", err)
	}
	return nil
}

func sortedKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
