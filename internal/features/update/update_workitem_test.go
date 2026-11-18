package update

import (
	"strings"
	"testing"

	"github.com/rainhu/ado/internal/api"
)

const base = "https://dev.azure.com/org/proj/_apis/wit/workItems/"

func ptr[T any](v T) *T { return &v }

func parentRel(id string) api.Relation {
	return api.Relation{Rel: "System.LinkTypes.Hierarchy-Reverse", URL: "https://dev.azure.com/org/guid/_apis/wit/workItems/" + id}
}

func TestBuildPatch_AssignAreaIterationComment(t *testing.T) {
	r := &UpdateWorkItemRequest{
		ID:         1,
		AssignedTo: ptr("dev-1@example.com"),
		Area:       ptr(`Proj\Team A`),
		Iteration:  ptr(`Proj\Sprint 12`),
		Comment:    ptr("blocked on review"),
	}
	ops, changed, err := buildPatch(r, nil, base)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"/fields/System.AssignedTo":    "dev-1@example.com",
		"/fields/System.AreaPath":      `Proj\Team A`,
		"/fields/System.IterationPath": `Proj\Sprint 12`,
		"/fields/System.History":       "blocked on review",
	}
	if len(ops) != len(want) {
		t.Fatalf("got %d ops, want %d: %+v", len(ops), len(want), ops)
	}
	for _, op := range ops {
		if want[op.Path] != op.Value {
			t.Errorf("op %s = %v, want %v", op.Path, op.Value, want[op.Path])
		}
	}
	if strings.Join(changed, ",") != "AssignedTo,Area,Iteration,Comment" {
		t.Errorf("changed = %v", changed)
	}
}

func TestBuildPatch_ParentAddedWhenNoneExists(t *testing.T) {
	r := &UpdateWorkItemRequest{ID: 1, Parent: ptr(42)}
	ops, _, err := buildPatch(r, &api.WorkItem{}, base)
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 1 || ops[0].Op != "add" || ops[0].Path != "/relations/-" {
		t.Fatalf("want one add /relations/- op, got %+v", ops)
	}
	v := ops[0].Value.(map[string]string)
	if v["rel"] != "System.LinkTypes.Hierarchy-Reverse" || v["url"] != base+"42" {
		t.Errorf("relation = %v", v)
	}
}

func TestBuildPatch_ParentReplacedRemovesOldLinkFirst(t *testing.T) {
	cur := &api.WorkItem{Relations: []api.Relation{
		{Rel: "System.LinkTypes.Related", URL: base + "7"},
		parentRel("10"),
	}}
	ops, _, err := buildPatch(&UpdateWorkItemRequest{ID: 1, Parent: ptr(42)}, cur, base)
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 2 || ops[0].Op != "remove" || ops[0].Path != "/relations/1" || ops[1].Op != "add" {
		t.Fatalf("want remove /relations/1 then add, got %+v", ops)
	}
}

func TestBuildPatch_SameParentIsNoOp(t *testing.T) {
	cur := &api.WorkItem{Relations: []api.Relation{parentRel("42")}}
	ops, _, err := buildPatch(&UpdateWorkItemRequest{ID: 1, Parent: ptr(42)}, cur, base)
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 0 {
		t.Fatalf("want no ops, got %+v", ops)
	}
}

func TestBuildPatch_ParentZeroUnlinks(t *testing.T) {
	cur := &api.WorkItem{Relations: []api.Relation{parentRel("42")}}
	ops, changed, err := buildPatch(&UpdateWorkItemRequest{ID: 1, Parent: ptr(0)}, cur, base)
	if err != nil {
		t.Fatal(err)
	}
	if len(ops) != 1 || ops[0].Op != "remove" || ops[0].Path != "/relations/0" || changed[0] != "Parent" {
		t.Fatalf("want remove /relations/0, got %+v %v", ops, changed)
	}
}

func TestBuildPatch_NothingToUpdateListsNewFlags(t *testing.T) {
	_, _, err := buildPatch(&UpdateWorkItemRequest{ID: 1}, nil, base)
	if err == nil {
		t.Fatal("expected error")
	}
	for _, f := range []string{"--parent", "--assign", "--area", "--iteration", "--comment"} {
		if !strings.Contains(err.Error(), f) {
			t.Errorf("error should mention %s: %v", f, err)
		}
	}
}
