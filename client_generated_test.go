package youtrackapi

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGeneratedListProjects(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/admin/projects" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.URL.Query().Get("fields"); got != "id,name" {
			t.Errorf("fields = %q", got)
		}
		if got := r.URL.Query().Get("$top"); got != "25" {
			t.Errorf("$top = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer token" {
			t.Errorf("Authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `[{"$type":"Project","id":"p-1","name":"One"}]`)
	}))
	defer server.Close()

	client := NewClient(server.URL, []byte("token"))
	projects, err := client.ListProjects(context.Background(), ListProjectsRequest{Fields: "id,name", Top: 25})
	if err != nil {
		t.Fatal(err)
	}
	if len(projects) != 1 || projects[0].Id == nil || *projects[0].Id != "p-1" {
		t.Fatalf("projects = %#v", projects)
	}
}

func TestGeneratedCreateProject(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/admin/projects" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), `"$type":"Project"`) || !strings.Contains(string(body), `"name":"One"`) {
			t.Errorf("body = %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"$type":"Project","id":"p-1","name":"One"}`)
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	project, err := client.CreateProject(context.Background(), CreateProjectRequest{
		Fields:  "id,name",
		Project: Project{Name: stringPointer("One")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if project.Id == nil || *project.Id != "p-1" {
		t.Fatalf("project = %#v", project)
	}
}

func TestGeneratedDeleteProjectEscapesPathParameter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete || r.URL.EscapedPath() != "/admin/projects/a%2Fb" {
			t.Errorf("request = %s %s", r.Method, r.URL.EscapedPath())
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	if err := client.DeleteProject(context.Background(), DeleteProjectRequest{ID: "a/b"}); err != nil {
		t.Fatal(err)
	}
}

func stringPointer(value string) *string {
	return &value
}
