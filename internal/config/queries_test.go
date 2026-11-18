package config

import (
	"reflect"
	"testing"
)

func TestQueryList_LegacyQueryIDOnly(t *testing.T) {
	c := &Config{QueryID: "abc"}
	got := c.QueryList()
	want := []NamedQuery{{Name: "Default", ID: "abc"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestQueryList_DefaultAlreadyListed(t *testing.T) {
	c := &Config{QueryID: "b", Queries: []NamedQuery{{"Mine", "a"}, {"Team", "b"}}}
	got := c.QueryList()
	if len(got) != 2 || got[0].Name != "Mine" || got[1].Name != "Team" {
		t.Fatalf("unexpected list %+v", got)
	}
}

func TestQueryList_DefaultNotListedIsPrepended(t *testing.T) {
	c := &Config{QueryID: "z", Queries: []NamedQuery{{"Mine", "a"}}}
	got := c.QueryList()
	if len(got) != 2 || got[0].ID != "z" || got[1].ID != "a" {
		t.Fatalf("unexpected list %+v", got)
	}
}

func TestQueryList_SkipsBlankAndDuplicateIDs(t *testing.T) {
	c := &Config{Queries: []NamedQuery{{"A", "1"}, {"B", ""}, {"C", "1"}, {"", "2"}}}
	got := c.QueryList()
	want := []NamedQuery{{"A", "1"}, {"2", "2"}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, want %+v", got, want)
	}
}

func TestResolveQuery(t *testing.T) {
	c := &Config{QueryID: "def", Queries: []NamedQuery{{"My Tasks", "111"}, {"Team Bugs", "222"}}}
	cases := map[string]string{
		"":          "def", // empty → default
		"my tasks":  "111", // name, case-insensitive
		"Team Bugs": "222",
		"222":       "222", // id passes through
		"raw-guid":  "raw-guid",
	}
	for in, want := range cases {
		if got := c.ResolveQuery(in); got != want {
			t.Errorf("ResolveQuery(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestQueryIndex(t *testing.T) {
	qs := []NamedQuery{{"A", "1"}, {"B", "2"}}
	if i := QueryIndex(qs, "2"); i != 1 {
		t.Errorf("got %d, want 1", i)
	}
	if i := QueryIndex(qs, "nope"); i != -1 {
		t.Errorf("got %d, want -1", i)
	}
}
