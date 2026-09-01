package youtrackapi

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
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

func TestGeneratedListTagsDecodesCurrentTagType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/tags" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `[{"$type":"Tag","id":"tag-1","name":"support"}]`)
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	tags, err := client.ListTags(context.Background(), ListTagsRequest{Fields: "id,name"})
	if err != nil {
		t.Fatal(err)
	}
	if len(tags) != 1 {
		t.Fatalf("tags = %#v", tags)
	}
	if tags[0].Id == nil || *tags[0].Id != "tag-1" || tags[0].Name == nil || *tags[0].Name != "support" {
		t.Fatalf("tag = %#v", tags[0])
	}
}

func TestGeneratedCreateTagUsesCurrentTagType(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/tags" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), `"$type":"Tag"`) || !strings.Contains(string(body), `"name":"support"`) {
			t.Errorf("body = %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"$type":"Tag","id":"tag-1","name":"support"}`)
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	tag, err := client.CreateTag(context.Background(), CreateTagRequest{
		Fields: "id,name",
		Tag:    Tag{Name: stringPointer("support")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if tag.Id == nil || *tag.Id != "tag-1" {
		t.Fatalf("tag = %#v", tag)
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

func TestGeneratedGetIssueDecodesRichResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.EscapedPath() != "/issues/DEMO-1" {
			t.Errorf("request = %s %s", r.Method, r.URL.EscapedPath())
		}
		if got := r.URL.Query().Get("fields"); got != "id,summary,project,tags,customFields,visibility" {
			t.Errorf("fields = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{
			"$type":"Issue",
			"id":"issue-1",
			"summary":"Rich response",
			"description":null,
			"isDraft":false,
			"commentsCount":2,
			"created":1787740496789,
			"project":{"$type":"Project","id":"project-1","name":"Demo"},
			"tags":[{"$type":"IssueTag","id":"tag-1","name":"backend"}],
			"customFields":[{"$type":"SimpleIssueCustomField","id":"field-1","name":"Estimate","value":3.5}],
			"visibility":{"$type":"UnlimitedVisibility","id":"visibility-1"},
			"unsupported":{"nested":[true,null]}
		}`)
	}))
	defer server.Close()

	client := NewClient(server.URL, nil)
	issue, err := client.GetIssue(context.Background(), GetIssueRequest{
		ID:     "DEMO-1",
		Fields: "id,summary,project,tags,customFields,visibility",
	})
	if err != nil {
		t.Fatal(err)
	}

	if issue.Id == nil || *issue.Id != "issue-1" || issue.Summary == nil || *issue.Summary != "Rich response" {
		t.Fatalf("issue identity = %#v", issue)
	}
	if issue.Description != nil || issue.IsDraft == nil || *issue.IsDraft || issue.CommentsCount == nil || *issue.CommentsCount != 2 {
		t.Fatalf("issue scalar fields = %#v", issue)
	}
	if issue.Created == nil || !issue.Created.Equal(time.UnixMilli(1787740496789).UTC()) {
		t.Fatalf("created = %#v", issue.Created)
	}
	if issue.Project == nil || issue.Project.Name == nil || *issue.Project.Name != "Demo" {
		t.Fatalf("project = %#v", issue.Project)
	}
	if len(issue.Tags) != 1 {
		t.Fatalf("tags = %#v", issue.Tags)
	}
	tag := issue.Tags[0]
	if tag.Name == nil || *tag.Name != "backend" {
		t.Fatalf("tag = %#v", issue.Tags[0])
	}
	if len(issue.CustomFields) != 1 {
		t.Fatalf("custom fields = %#v", issue.CustomFields)
	}
	field, ok := issue.CustomFields[0].(*SimpleIssueCustomField)
	if !ok || field.Value == nil {
		t.Fatalf("custom field = %#v", issue.CustomFields[0])
	}
	value, ok := (*field.Value).(float64)
	if !ok || value != 3.5 {
		t.Fatalf("custom field value = %#v", *field.Value)
	}
	visibility, ok := issue.Visibility.(*UnlimitedVisibility)
	if !ok || visibility.Id == nil || *visibility.Id != "visibility-1" {
		t.Fatalf("visibility = %#v", issue.Visibility)
	}
}

func stringPointer(value string) *string {
	return &value
}
