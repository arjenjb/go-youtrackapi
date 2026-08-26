package main

import (
	"net/http"
	"strings"
	"testing"
)

func TestDetermineMethodName(t *testing.T) {
	tests := []struct {
		name     string
		method   string
		path     string
		response endpointResponse
		isUpdate bool
		want     string
	}{
		{
			name:     "GET collection",
			method:   http.MethodGet,
			path:     "/admin/projects",
			response: endpointResponse{Shape: responseList, TypeName: "Project"},
			want:     "ListProjects",
		},
		{
			name:     "POST collection",
			method:   http.MethodPost,
			path:     "/admin/projects",
			response: endpointResponse{Shape: responseSingle, TypeName: "Project"},
			want:     "CreateProject",
		},
		{
			name:     "GET item",
			method:   http.MethodGet,
			path:     "/admin/projects/{id}",
			response: endpointResponse{Shape: responseSingle, TypeName: "Project"},
			want:     "GetProject",
		},
		{
			name:     "POST item",
			method:   http.MethodPost,
			path:     "/admin/projects/{id}",
			response: endpointResponse{Shape: responseSingle, TypeName: "Project"},
			isUpdate: true,
			want:     "UpdateProject",
		},
		{
			name:     "DELETE item",
			method:   http.MethodDelete,
			path:     "/admin/projects/{id}",
			response: endpointResponse{Shape: responseEmpty},
			want:     "DeleteProject",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := determineMethodName(tt.method, tt.path, tt.response, tt.isUpdate); got != tt.want {
				t.Fatalf("determineMethodName() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDistillClient(t *testing.T) {
	input := []byte(`{
  "paths": {
    "/projects": {
	  "description": "This resource provides access to projects.",
      "get": {
		"parameters": [{"name":"fields","description":"Selects fields to return.","in":"query","schema":{"type":"string"}}],
        "responses": {"200":{"content":{"application/json":{"schema":{"type":"array","items":{"$ref":"#/components/schemas/Project"}}}}}}
      },
      "post": {
        "requestBody":{"content":{"application/json":{"schema":{"$ref":"#/components/schemas/Project"}}}},
        "responses": {"200":{"content":{"application/json":{"schema":{"$ref":"#/components/schemas/Project"}}}}}
      }
    },
    "/projects/{id}": {
      "parameters":[{"name":"id","in":"path","required":true,"schema":{"type":"string"}}],
      "get":{"responses":{"200":{"content":{"application/json":{"schema":{"$ref":"#/components/schemas/Project"}}}}}},
      "delete":{"responses":{"200":{"description":"OK"}}}
    }
  },
  "components":{"schemas":{"Project":{"type":"object"}}}
}`)

	got, err := distillClient(input, overrides{}, "fixture.json")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"type ListProjectsRequest struct",
		"func (c *Client) ListProjects(",
		"func (c *Client) CreateProject(",
		"func (c *Client) GetProject(",
		"func (c *Client) DeleteProject(",
		"url.PathEscape(r.ID)",
		"// ListProjects calls GET /projects.",
		"// This resource provides access to projects.",
		"// Fields: Selects fields to return.",
	} {
		if !strings.Contains(string(got), want) {
			t.Errorf("generated client does not contain %q:\n%s", want, got)
		}
	}
}
