package cmd

import (
	"strings"
	"testing"
)

func TestResolveWorkItemType_AcceptsEveryBacklogType(t *testing.T) {
	cases := map[string]string{
		"task": "Task", "bug": "Bug", "epic": "Epic", "issue": "Issue",
		"feature": "Feature", "Feature": "Feature",
		"user story": "User Story", "story": "User Story", "userstory": "User Story",
	}
	for in, want := range cases {
		got, err := resolveWorkItemType(in)
		if err != nil {
			t.Errorf("resolveWorkItemType(%q) error: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("resolveWorkItemType(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestResolveWorkItemType_ErrorListsFeature(t *testing.T) {
	_, err := resolveWorkItemType("saga")
	if err == nil {
		t.Fatal("expected an error for an unknown type")
	}
	if !strings.Contains(err.Error(), "feature") {
		t.Errorf("error should list feature as a valid type: %v", err)
	}
}
