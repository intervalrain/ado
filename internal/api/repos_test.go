package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/rainhu/ado/internal/config"
)

// Project wikis are hidden Git repositories: Azure DevOps omits them from
// _apis/git/repositories unless includeHidden=true is passed. Without them
// `ado pr` cannot open a PR into a wiki ("repository ... not found in project").
func TestListRepositories_IncludesHiddenRepos(t *testing.T) {
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		repos := []Repository{{ID: "r1", Name: "shadow_agent"}}
		if r.URL.Query().Get("includeHidden") == "true" {
			repos = append(repos, Repository{ID: "w1", Name: "IoT-Platform.wiki"})
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"value": repos})
	}))
	defer srv.Close()

	c := NewClient(&config.Config{Org: srv.URL, Project: "IoT%20Platform", PAT: "x"})
	repos, err := c.ListRepositories()
	if err != nil {
		t.Fatalf("ListRepositories: %v", err)
	}

	found := false
	for _, r := range repos {
		if r.Name == "IoT-Platform.wiki" {
			found = true
		}
	}
	if !found {
		t.Fatalf("hidden wiki repo not listed; query was %q", gotQuery)
	}
}
