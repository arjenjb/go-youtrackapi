package main

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type pathItem struct {
	Description string      `json:"description"`
	Parameters  []parameter `json:"parameters"`
	Get         *operation  `json:"get"`
	Post        *operation  `json:"post"`
	Put         *operation  `json:"put"`
	Patch       *operation  `json:"patch"`
	Delete      *operation  `json:"delete"`
}

type operation struct {
	Summary     string              `json:"summary"`
	Description string              `json:"description"`
	Parameters  []parameter         `json:"parameters"`
	RequestBody *requestBody        `json:"requestBody"`
	Responses   map[string]response `json:"responses"`
}

type parameter struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	In          string `json:"in"`
	Required    bool   `json:"required"`
	Schema      schema `json:"schema"`
}

type requestBody struct {
	Description string               `json:"description"`
	Content     map[string]mediaType `json:"content"`
}

type response struct {
	Description string               `json:"description"`
	Content     map[string]mediaType `json:"content"`
}

type mediaType struct {
	Schema schema `json:"schema"`
}

type responseShape uint8

const (
	responseEmpty responseShape = iota
	responseSingle
	responseList
)

type endpointResponse struct {
	Shape       responseShape
	TypeName    string
	Abstract    bool
	Description string
}

type requestBodyShape uint8

const (
	bodyNone requestBodyShape = iota
	bodyJSON
	bodyMultipart
)

type endpointBody struct {
	Shape       requestBodyShape
	TypeName    string
	Abstract    bool
	Description string
}

type endpoint struct {
	HTTPMethod  string
	Path        string
	Name        string
	Description string
	Parameters  []parameter
	Body        endpointBody
	Response    endpointResponse
}

func distillClient(input []byte, config overrides, source string) ([]byte, error) {
	document, err := parseOpenAPI(input)
	if err != nil {
		return nil, fmt.Errorf("decode OpenAPI document for client: %w", err)
	}
	return distillClientDocument(document, config, source)
}

func distillClientDocument(document openAPIDocument, config overrides, source string) ([]byte, error) {
	if len(document.Paths) == 0 {
		return nil, errors.New("OpenAPI document has no paths")
	}

	abstract := make(map[string]bool, len(config.AbstractTypes))
	for _, name := range config.AbstractTypes {
		abstract[name] = true
	}
	endpoints, err := collectEndpoints(document.Paths, document.Components.Schemas, abstract)
	if err != nil {
		return nil, err
	}
	assignUniqueMethodNames(endpoints)
	return renderClient(endpoints, source), nil
}

func collectEndpoints(paths map[string]pathItem, schemas map[string]schema, abstract map[string]bool) ([]*endpoint, error) {
	pathNames := sortedKeys(paths)
	var endpoints []*endpoint
	for _, path := range pathNames {
		item := paths[path]
		operations := []struct {
			method    string
			operation *operation
		}{
			{http.MethodGet, item.Get},
			{http.MethodPost, item.Post},
			{http.MethodPut, item.Put},
			{http.MethodPatch, item.Patch},
			{http.MethodDelete, item.Delete},
		}
		for _, candidate := range operations {
			if candidate.operation == nil {
				continue
			}
			response, err := endpointResponseFrom(candidate.operation, abstract)
			if err != nil {
				return nil, fmt.Errorf("%s %s response: %w", candidate.method, path, err)
			}
			body, err := endpointBodyFrom(candidate.operation, abstract)
			if err != nil {
				return nil, fmt.Errorf("%s %s request: %w", candidate.method, path, err)
			}
			parameters, err := mergeParameters(item.Parameters, candidate.operation.Parameters)
			if err != nil {
				return nil, fmt.Errorf("%s %s parameters: %w", candidate.method, path, err)
			}
			isUpdate := isUpdateOperation(candidate.method, path, response, item.Get)
			endpoints = append(endpoints, &endpoint{
				HTTPMethod:  candidate.method,
				Path:        path,
				Name:        determineMethodName(candidate.method, path, response, isUpdate),
				Description: operationDescription(item, candidate.operation),
				Parameters:  parameters,
				Body:        describeBody(body, candidate.operation.RequestBody, schemas),
				Response:    response,
			})
		}
	}
	return endpoints, nil
}

func operationDescription(item pathItem, operation *operation) string {
	if operation.Description != "" {
		return normalizeDescription(operation.Description)
	}
	if operation.Summary != "" {
		return normalizeDescription(operation.Summary)
	}
	return normalizeDescription(item.Description)
}

func describeBody(body endpointBody, request *requestBody, schemas map[string]schema) endpointBody {
	if request != nil && request.Description != "" {
		body.Description = normalizeDescription(request.Description)
	} else if body.TypeName != "" {
		body.Description = normalizeDescription(schemaDescriptionFor(body.TypeName, schemas, nil))
	}
	return body
}

// determineMethodName derives the public operation name from HTTP semantics and
// the response type. Paths are only used when an empty DELETE response leaves
// no response type from which to derive a resource name.
func determineMethodName(method, path string, response endpointResponse, isUpdate bool) string {
	typeName := response.TypeName
	if typeName == "" {
		typeName = resourceNameFromPath(path)
	}
	switch method {
	case http.MethodGet:
		if response.Shape == responseList {
			return "List" + pluralize(typeName)
		}
		return "Get" + typeName
	case http.MethodPost:
		if isUpdate {
			return "Update" + typeName
		}
		return "Create" + typeName
	case http.MethodPut, http.MethodPatch:
		return "Update" + typeName
	case http.MethodDelete:
		return "Delete" + typeName
	default:
		return exportedName(strings.ToLower(method)) + typeName
	}
}

func isUpdateOperation(method, path string, response endpointResponse, get *operation) bool {
	if method != http.MethodPost {
		return method == http.MethodPut || method == http.MethodPatch
	}
	segments := pathSegments(path)
	if len(segments) > 0 && isPathParameter(segments[len(segments)-1]) {
		return true
	}
	if get == nil || response.Shape != responseSingle {
		return false
	}
	getResponse, err := endpointResponseFrom(get, nil)
	return err == nil && getResponse.Shape == responseSingle && getResponse.TypeName == response.TypeName
}

func endpointResponseFrom(operation *operation, abstract map[string]bool) (endpointResponse, error) {
	statusNames := sortedKeys(operation.Responses)
	for _, status := range statusNames {
		if len(status) != 3 || status[0] != '2' {
			continue
		}
		media, ok := operation.Responses[status].Content["application/json"]
		if !ok {
			return endpointResponse{
				Shape:       responseEmpty,
				Description: normalizeDescription(operation.Responses[status].Description),
			}, nil
		}
		typeName, err := responseSchemaType(media.Schema)
		if err != nil {
			return endpointResponse{}, err
		}
		shape := responseSingle
		if media.Schema.Type == "array" {
			shape = responseList
		}
		return endpointResponse{
			Shape:       shape,
			TypeName:    typeName,
			Abstract:    abstract[typeName],
			Description: normalizeDescription(operation.Responses[status].Description),
		}, nil
	}
	return endpointResponse{}, errors.New("no successful response")
}

func responseSchemaType(s schema) (string, error) {
	if s.Type == "array" {
		if s.Items == nil {
			return "", errors.New("array response has no item schema")
		}
		s = *s.Items
	}
	if s.Ref == "" {
		return "", fmt.Errorf("response schema has no component reference")
	}
	return referenceName(s.Ref)
}

func endpointBodyFrom(operation *operation, abstract map[string]bool) (endpointBody, error) {
	if operation.RequestBody == nil {
		return endpointBody{Shape: bodyNone}, nil
	}
	if media, ok := operation.RequestBody.Content["application/json"]; ok {
		if media.Schema.Ref == "" {
			return endpointBody{}, errors.New("JSON request schema has no component reference")
		}
		typeName, err := referenceName(media.Schema.Ref)
		if err != nil {
			return endpointBody{}, err
		}
		return endpointBody{Shape: bodyJSON, TypeName: typeName, Abstract: abstract[typeName]}, nil
	}
	if _, ok := operation.RequestBody.Content["multipart/form-data"]; ok {
		return endpointBody{Shape: bodyMultipart}, nil
	}
	return endpointBody{}, errors.New("unsupported request content type")
}

func mergeParameters(shared, specific []parameter) ([]parameter, error) {
	byKey := make(map[string]parameter, len(shared)+len(specific))
	for _, p := range append(append([]parameter(nil), shared...), specific...) {
		if p.In != "path" && p.In != "query" {
			return nil, fmt.Errorf("unsupported parameter location %q", p.In)
		}
		byKey[p.In+"\x00"+p.Name] = p
	}
	parameters := make([]parameter, 0, len(byKey))
	for _, p := range byKey {
		parameters = append(parameters, p)
	}
	sort.Slice(parameters, func(i, j int) bool {
		if parameters[i].In != parameters[j].In {
			return parameters[i].In == "path"
		}
		return parameters[i].Name < parameters[j].Name
	})
	return parameters, nil
}

func assignUniqueMethodNames(endpoints []*endpoint) {
	byName := make(map[string][]*endpoint)
	for _, endpoint := range endpoints {
		byName[endpoint.Name] = append(byName[endpoint.Name], endpoint)
	}
	for _, duplicates := range byName {
		if len(duplicates) < 2 {
			continue
		}
		sort.Slice(duplicates, func(i, j int) bool {
			iSegments, jSegments := len(pathSegments(duplicates[i].Path)), len(pathSegments(duplicates[j].Path))
			if iSegments != jSegments {
				return iSegments < jSegments
			}
			if duplicates[i].Path != duplicates[j].Path {
				return duplicates[i].Path < duplicates[j].Path
			}
			return duplicates[i].HTTPMethod < duplicates[j].HTTPMethod
		})
		used := map[string]bool{duplicates[0].Name: true}
		for _, endpoint := range duplicates[1:] {
			candidate := endpoint.Name + "At" + pathName(endpoint.Path)
			if used[candidate] {
				candidate += exportedName(strings.ToLower(endpoint.HTTPMethod))
			}
			endpoint.Name = candidate
			used[candidate] = true
		}
	}
}

func renderClient(endpoints []*endpoint, source string) []byte {
	var out bytes.Buffer
	fmt.Fprintln(&out, "// Code generated by go generate; DO NOT EDIT.")
	fmt.Fprintf(&out, "// Source: %s\n\n", source)
	fmt.Fprintln(&out, "package youtrackapi")
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "import (")
	fmt.Fprintln(&out, "\t\"bytes\"")
	fmt.Fprintln(&out, "\t\"context\"")
	fmt.Fprintln(&out, "\t\"net/http\"")
	fmt.Fprintln(&out, "\t\"net/url\"")
	fmt.Fprintln(&out, "\t\"strconv\"")
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "\t\"github.com/arjenjb/go-youtrackapi/internal/jsoncodec\"")
	fmt.Fprintln(&out, ")")
	fmt.Fprintln(&out)
	for _, endpoint := range endpoints {
		renderEndpoint(&out, endpoint)
	}
	return out.Bytes()
}

func renderEndpoint(out *bytes.Buffer, endpoint *endpoint) {
	requestName := endpoint.Name + "Request"
	writeDocComment(out, "", requestName, "contains parameters for "+endpoint.Name+".")
	fmt.Fprintf(out, "type %s struct {\n", requestName)
	for _, p := range endpoint.Parameters {
		fieldName := parameterFieldName(p.Name)
		writeDocComment(out, "\t", fieldName, parameterDescription(p))
		fmt.Fprintf(out, "\t%s %s\n", fieldName, parameterType(p.Schema))
	}
	switch endpoint.Body.Shape {
	case bodyJSON:
		writeDocComment(out, "\t", endpoint.Body.TypeName, "is the JSON request body.", endpoint.Body.Description)
		fmt.Fprintf(out, "\t%s %s\n", endpoint.Body.TypeName, endpoint.Body.TypeName)
	case bodyMultipart:
		writeDocComment(out, "\t", "Files", "contains the files uploaded with the request.")
		fmt.Fprintln(out, "\tFiles []Upload")
	}
	fmt.Fprintln(out, "}")
	fmt.Fprintln(out)

	resultType := endpointResultType(endpoint.Response)
	writeDocComment(
		out,
		"",
		endpoint.Name,
		fmt.Sprintf("calls %s %s.", endpoint.HTTPMethod, endpoint.Path),
		endpoint.Description,
		responseDescription(endpoint.Response),
	)
	fmt.Fprintf(out, "func (c *Client) %s(ctx context.Context, r %s) %s {\n", endpoint.Name, requestName, resultType)
	fmt.Fprintf(out, "\tpath := %s\n", pathExpression(endpoint.Path))
	fmt.Fprintln(out, "\tvalues := make(url.Values)")
	for _, p := range endpoint.Parameters {
		if p.In == "query" {
			renderQueryParameter(out, p)
		}
	}

	switch endpoint.Body.Shape {
	case bodyJSON:
		fmt.Fprintln(out, "\tvar body bytes.Buffer")
		fmt.Fprintf(out, "\tif err := marshal%s(jsoncodec.NewMarshaler(&body), r.%s); err != nil {\n", endpoint.Body.TypeName, endpoint.Body.TypeName)
		renderErrorReturn(out, endpoint.Response, "err")
		fmt.Fprintln(out, "\t}")
		fmt.Fprintf(out, "\treq := c.makeRequestWithBody(ctx, http.Method%s, path, values, body.Bytes())\n", exportedName(strings.ToLower(endpoint.HTTPMethod)))
	case bodyMultipart:
		fmt.Fprintln(out, "\tbody, contentType, err := encodeMultipartFiles(r.Files)")
		fmt.Fprintln(out, "\tif err != nil {")
		renderErrorReturn(out, endpoint.Response, "err")
		fmt.Fprintln(out, "\t}")
		fmt.Fprintf(out, "\treq := c.makeRequestWithBody(ctx, http.Method%s, path, values, body)\n", exportedName(strings.ToLower(endpoint.HTTPMethod)))
		fmt.Fprintln(out, "\treq.Header.Set(\"Content-Type\", contentType)")
	default:
		fmt.Fprintf(out, "\treq := c.makeRequest(ctx, http.Method%s, path, values)\n", exportedName(strings.ToLower(endpoint.HTTPMethod)))
	}

	switch endpoint.Response.Shape {
	case responseEmpty:
		fmt.Fprintln(out, "\treturn makeEmptyCall(c.client, req)")
	case responseSingle:
		fmt.Fprintf(out, "\treturn makeDecodedCall(c.client, req, unmarshal%s)\n", endpoint.Response.TypeName)
	case responseList:
		if endpoint.Response.Abstract {
			fmt.Fprintf(out, "\treturn makeDecodedAbstractListCall(c.client, req, unmarshal%s)\n", endpoint.Response.TypeName)
		} else {
			fmt.Fprintf(out, "\treturn makeDecodedListCall(c.client, req, unmarshal%s)\n", endpoint.Response.TypeName)
		}
	}
	fmt.Fprintln(out, "}")
	fmt.Fprintln(out)
}

func parameterDescription(parameter parameter) string {
	if parameter.Description != "" {
		return normalizeDescription(parameter.Description)
	}
	switch parameter.Name {
	case "fields":
		return "selects the fields returned by YouTrack."
	case "$skip":
		return "is the number of matching resources to skip."
	case "$top":
		return "is the maximum number of resources to return."
	}
	if parameter.In == "path" {
		return "identifies a resource in the request path."
	}
	return ""
}

func responseDescription(response endpointResponse) string {
	if response.Description == "" || response.Description == "OK" {
		return ""
	}
	return "Response: " + response.Description + "."
}

func writeDocComment(out *bytes.Buffer, indent, subject, lead string, paragraphs ...string) {
	lead = strings.TrimSpace(lead)
	if lead == "" && len(paragraphs) == 0 {
		return
	}
	allParagraphs := []string{joinDocLead(subject, lead)}
	for _, paragraph := range paragraphs {
		for _, line := range strings.Split(strings.TrimSpace(paragraph), "\n") {
			if line = strings.TrimSpace(line); line != "" {
				allParagraphs = append(allParagraphs, line)
			}
		}
	}
	for index, paragraph := range allParagraphs {
		if paragraph == "" {
			continue
		}
		if index > 0 {
			fmt.Fprintf(out, "%s//\n", indent)
		}
		for _, line := range wrapDocText(paragraph, 100-len(indent)-3) {
			fmt.Fprintf(out, "%s// %s\n", indent, line)
		}
	}
}

func joinDocLead(subject, lead string) string {
	if subject == "" {
		return strings.TrimSpace(lead)
	}
	lead = strings.TrimSpace(lead)
	if lead == "" {
		return subject
	}
	first, _ := utf8.DecodeRuneInString(lead)
	if unicode.IsUpper(first) {
		return subject + ": " + lead
	}
	return subject + " " + lead
}

func wrapDocText(text string, width int) []string {
	words := strings.Fields(text)
	if len(words) == 0 {
		return nil
	}
	lines := []string{words[0]}
	for _, word := range words[1:] {
		last := len(lines) - 1
		if len(lines[last])+1+len(word) <= width {
			lines[last] += " " + word
		} else {
			lines = append(lines, word)
		}
	}
	return lines
}

func endpointResultType(response endpointResponse) string {
	switch response.Shape {
	case responseEmpty:
		return "error"
	case responseList:
		return "([]" + response.TypeName + ", error)"
	case responseSingle:
		if response.Abstract {
			return "(" + response.TypeName + ", error)"
		}
		return "(*" + response.TypeName + ", error)"
	default:
		panic("unknown response shape")
	}
}

func renderErrorReturn(out *bytes.Buffer, response endpointResponse, errName string) {
	switch response.Shape {
	case responseEmpty:
		fmt.Fprintf(out, "\t\treturn %s\n", errName)
	default:
		fmt.Fprintf(out, "\t\treturn nil, %s\n", errName)
	}
}

func renderQueryParameter(out *bytes.Buffer, p parameter) {
	field := "r." + parameterFieldName(p.Name)
	switch parameterType(p.Schema) {
	case "string":
		fmt.Fprintf(out, "\tif %s != \"\" {\n\t\tvalues.Set(%q, %s)\n\t}\n", field, p.Name, field)
	case "bool":
		fmt.Fprintf(out, "\tif %s {\n\t\tvalues.Set(%q, strconv.FormatBool(%s))\n\t}\n", field, p.Name, field)
	case "int":
		fmt.Fprintf(out, "\tif %s != 0 {\n\t\tvalues.Set(%q, strconv.Itoa(%s))\n\t}\n", field, p.Name, field)
	case "int64":
		fmt.Fprintf(out, "\tif %s != 0 {\n\t\tvalues.Set(%q, strconv.FormatInt(%s, 10))\n\t}\n", field, p.Name, field)
	}
}

func parameterType(s schema) string {
	switch s.Type {
	case "boolean":
		return "bool"
	case "integer":
		if s.Format == "int64" {
			return "int64"
		}
		return "int"
	default:
		return "string"
	}
}

func parameterFieldName(name string) string {
	return exportedName(strings.TrimPrefix(name, "$"))
}

func pathExpression(path string) string {
	segments := pathSegments(path)
	parts := []string{strconv.Quote("")}
	for _, segment := range segments {
		parts = append(parts, strconv.Quote("/"))
		if isPathParameter(segment) {
			name := strings.TrimSuffix(strings.TrimPrefix(segment, "{"), "}")
			parts = append(parts, "url.PathEscape(r."+parameterFieldName(name)+")")
		} else {
			parts = append(parts, strconv.Quote(segment))
		}
	}
	return strings.Join(parts, " + ")
}

func resourceNameFromPath(path string) string {
	segments := pathSegments(path)
	if len(segments) == 0 {
		return "Resource"
	}
	last := segments[len(segments)-1]
	if isPathParameter(last) {
		parameterName := strings.TrimSuffix(strings.TrimPrefix(last, "{"), "}")
		if parameterName != "id" && strings.HasSuffix(strings.ToLower(parameterName), "id") {
			return exportedName(parameterName[:len(parameterName)-2])
		}
		if len(segments) > 1 {
			return singularize(exportedName(segments[len(segments)-2]))
		}
	}
	return singularize(exportedName(last))
}

func pathName(path string) string {
	var result strings.Builder
	for _, segment := range pathSegments(path) {
		if isPathParameter(segment) {
			name := strings.TrimSuffix(strings.TrimPrefix(segment, "{"), "}")
			result.WriteString("By")
			result.WriteString(exportedName(name))
		} else {
			result.WriteString(exportedName(segment))
		}
	}
	return result.String()
}

func pathSegments(path string) []string {
	return strings.FieldsFunc(path, func(r rune) bool { return r == '/' })
}

func isPathParameter(segment string) bool {
	return strings.HasPrefix(segment, "{") && strings.HasSuffix(segment, "}")
}

func exportedName(name string) string {
	if name == "" {
		return ""
	}
	runes := []rune(name)
	runes[0] = unicode.ToUpper(runes[0])
	result := string(runes)
	for _, replacement := range []struct{ old, new string }{
		{"Id", "ID"},
		{"Url", "URL"},
		{"Vcs", "VCS"},
	} {
		result = strings.ReplaceAll(result, replacement.old, replacement.new)
	}
	return result
}

func pluralize(name string) string {
	if strings.HasSuffix(name, "y") && len(name) > 1 && !strings.ContainsRune("aeiouAEIOU", rune(name[len(name)-2])) {
		return name[:len(name)-1] + "ies"
	}
	for _, suffix := range []string{"s", "x", "z", "ch", "sh"} {
		if strings.HasSuffix(strings.ToLower(name), suffix) {
			return name + "es"
		}
	}
	return name + "s"
}

func singularize(name string) string {
	lower := strings.ToLower(name)
	if strings.HasSuffix(lower, "ies") && len(name) > 3 {
		return name[:len(name)-3] + "y"
	}
	if strings.HasSuffix(lower, "ses") || strings.HasSuffix(lower, "xes") || strings.HasSuffix(lower, "zes") || strings.HasSuffix(lower, "ches") || strings.HasSuffix(lower, "shes") {
		return name[:len(name)-2]
	}
	if strings.HasSuffix(lower, "s") && !strings.HasSuffix(lower, "ss") {
		return name[:len(name)-1]
	}
	return name
}
