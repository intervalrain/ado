package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rainhu/ado/internal/config"
)

func key(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "space":
		return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func typeText(e queriesEditor, s string) queriesEditor {
	for _, r := range s {
		e, _, _, _ = e.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return e
}

func TestQueriesEditor_MigratesLegacyQueryID(t *testing.T) {
	e := newQueriesEditor(&config.Config{QueryID: "legacy"})
	cfg := &config.Config{}
	e.apply(cfg)
	if cfg.QueryID != "legacy" || len(cfg.Queries) != 1 || cfg.Queries[0].Name != "Default" {
		t.Fatalf("cfg=%+v", cfg)
	}
}

func TestQueriesEditor_AddAndSetDefault(t *testing.T) {
	e := newQueriesEditor(&config.Config{QueryID: "q1"})
	e, _, _, _ = e.update(key("a"))
	e = typeText(e, "Team Bugs")
	e, _, _, _ = e.update(key("enter")) // → id field
	e = typeText(e, "q2")
	var save bool
	e, _, _, save = e.update(key("enter"))
	if !save || len(e.items) != 2 || e.items[1] != (config.NamedQuery{Name: "Team Bugs", ID: "q2"}) {
		t.Fatalf("save=%v items=%+v", save, e.items)
	}
	e, _, _, save = e.update(key("space")) // cursor is on the new item
	cfg := &config.Config{}
	e.apply(cfg)
	if !save || cfg.QueryID != "q2" {
		t.Fatalf("save=%v default=%s", save, cfg.QueryID)
	}
}

func TestQueriesEditor_RejectsDuplicateID(t *testing.T) {
	e := newQueriesEditor(&config.Config{QueryID: "q1"})
	e, _, _, _ = e.update(key("a"))
	e, _, _, _ = e.update(key("tab"))
	e = typeText(e, "q1")
	e, _, _, save := e.update(key("enter"))
	if save || e.formErr == "" || len(e.items) != 1 {
		t.Fatalf("save=%v err=%q items=%+v", save, e.formErr, e.items)
	}
}

func TestQueriesEditor_DeletingDefaultFallsBackToFirst(t *testing.T) {
	e := newQueriesEditor(&config.Config{QueryID: "q1", Queries: []config.NamedQuery{{Name: "A", ID: "q1"}, {Name: "B", ID: "q2"}}})
	e, _, _, _ = e.update(key("x"))
	e, _, _, save := e.update(key("x")) // second press confirms
	cfg := &config.Config{}
	e.apply(cfg)
	if !save || len(cfg.Queries) != 1 || cfg.QueryID != "q2" {
		t.Fatalf("save=%v cfg=%+v", save, cfg)
	}
}
