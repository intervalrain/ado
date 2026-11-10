package pr

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/rainhu/ado/internal/api"
)

func samplePR() *api.PullRequest {
	return &api.PullRequest{
		ID:            7308,
		Title:         "fix(shadow): unify current-row selection",
		Status:        "active",
		IsDraft:       true,
		MergeStatus:   "succeeded",
		CreatedBy:     api.Identity{DisplayName: "Rain Hu"},
		CreationDate:  time.Date(2026, 9, 8, 8, 0, 0, 0, time.UTC),
		SourceRefName: "refs/heads/feature/123_rh_fix",
		TargetRefName: "refs/heads/develop",
		Repository:    api.Repository{ID: "r1", Name: "shadow_agent"},
		Description:   "## Problem\nReaders served a stale row.",
		Reviewers: []api.PRReviewer{
			{DisplayName: "Jane Doe", Vote: 10, IsRequired: true},
			{DisplayName: "John Roe", Vote: 0},
		},
	}
}

func TestPrintPRDetail_ShowsHeaderBranchesReviewersAndDescription(t *testing.T) {
	var buf bytes.Buffer
	printPRDetail(&buf, prDetail{PR: samplePR(), WorkItems: []string{"123", "456"}, URL: "https://x/pr/7308"})
	out := buf.String()

	for _, want := range []string{
		"PR #7308: fix(shadow): unify current-row selection",
		"Status:      active (draft)",
		"Repo:        shadow_agent",
		"Author:      Rain Hu",
		"Created:     2026-09-08",
		"Branch:      feature/123_rh_fix -> develop",
		"Merge:       succeeded",
		"Work items:  #123, #456",
		"Jane Doe",
		"Approved",
		"Required",
		"John Roe",
		"No vote",
		"Optional",
		"  ## Problem",
		"  Readers served a stale row.",
		"https://x/pr/7308",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q\n---\n%s", want, out)
		}
	}
}

func TestPrintPRDetail_OmitsEmptyOptionalSections(t *testing.T) {
	p := samplePR()
	p.IsDraft = false
	p.Description = ""
	p.Reviewers = nil
	var buf bytes.Buffer
	printPRDetail(&buf, prDetail{PR: p, URL: "u"})
	out := buf.String()

	for _, unwanted := range []string{"(draft)", "Description:", "Reviewers:", "Work items:"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("output should not contain %q\n---\n%s", unwanted, out)
		}
	}
}

func TestToJSONPR_FlattensFields(t *testing.T) {
	j := toJSONPR(prDetail{PR: samplePR(), WorkItems: []string{"123"}, URL: "u"})
	data, err := json.Marshal(j)
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	checks := map[string]any{
		"id":           float64(7308),
		"repository":   "shadow_agent",
		"sourceBranch": "feature/123_rh_fix",
		"targetBranch": "develop",
		"isDraft":      true,
		"url":          "u",
	}
	for k, want := range checks {
		if got[k] != want {
			t.Errorf("%s = %v, want %v", k, got[k], want)
		}
	}
	revs := got["reviewers"].([]any)
	first := revs[0].(map[string]any)
	if first["vote"] != "Approved" || first["required"] != true {
		t.Errorf("unexpected first reviewer %v", first)
	}
}
