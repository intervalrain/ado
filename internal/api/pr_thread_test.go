package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rainhu/ado/internal/config"
)

// A review comment is a new, active PR thread with one text comment. Without it
// `ado` cannot give review feedback on a PR (tool-gaps #11).
func TestCreatePRThread_PostsOneActiveTextComment(t *testing.T) {
	var gotMethod, gotPath, gotQuery string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod, gotPath, gotQuery = r.Method, r.URL.Path, r.URL.RawQuery
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 42})
	}))
	defer srv.Close()

	c := NewClient(&config.Config{Org: srv.URL, Project: "IoT%20Platform", PAT: "x"})
	id, err := c.CreatePRThread("w1", 9975, "## A1\nCR-01 is not decided.")
	if err != nil {
		t.Fatalf("CreatePRThread: %v", err)
	}
	if id != 42 {
		t.Errorf("thread id = %d, want 42", id)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}
	if !strings.HasSuffix(gotPath, "/_apis/git/repositories/w1/pullRequests/9975/threads") {
		t.Errorf("path = %s", gotPath)
	}
	if !strings.Contains(gotQuery, "api-version=7.1") {
		t.Errorf("query = %s", gotQuery)
	}
	if gotBody["status"] != "active" {
		t.Errorf("status = %v, want active", gotBody["status"])
	}
	comments, _ := gotBody["comments"].([]any)
	if len(comments) != 1 {
		t.Fatalf("comments = %v, want exactly one", gotBody["comments"])
	}
	cm := comments[0].(map[string]any)
	if cm["content"] != "## A1\nCR-01 is not decided." || cm["commentType"] != "text" {
		t.Errorf("comment = %v", cm)
	}
}

func TestCreatePRThread_RejectsEmptyContent(t *testing.T) {
	c := NewClient(&config.Config{Org: "http://unused", Project: "p", PAT: "x"})
	if _, err := c.CreatePRThread("w1", 1, "  \n"); err == nil {
		t.Fatal("expected an error for an empty comment")
	}
}
