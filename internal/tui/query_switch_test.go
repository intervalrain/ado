package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rainhu/ado/internal/api"
	"github.com/rainhu/ado/internal/config"
)

func twoQueries() []config.NamedQuery {
	return []config.NamedQuery{{Name: "Mine", ID: "q1"}, {Name: "Team", ID: "q2"}}
}

func loadOne(m queryModel, id int, title string) queryModel {
	m, _ = m.update(queryResultMsg{gen: m.gen, refs: []api.WorkItemRef{{ID: id}}})
	wi := &api.WorkItem{ID: id}
	wi.Fields.Title = title
	m, _ = m.update(workItemMsg{gen: m.gen, item: wi})
	return m
}

func TestNewQueryModel_StartsOnInitialID(t *testing.T) {
	m := newQueryModel(nil, twoQueries(), "q2")
	if m.activeQ != 1 || m.queryID != "q2" {
		t.Fatalf("activeQ=%d queryID=%s", m.activeQ, m.queryID)
	}
}

func TestNewQueryModel_UnknownInitialIDIsPrepended(t *testing.T) {
	m := newQueryModel(nil, twoQueries(), "raw")
	if len(m.queries) != 3 || m.activeQ != 0 || m.queryID != "raw" {
		t.Fatalf("queries=%+v activeQ=%d", m.queries, m.activeQ)
	}
}

func TestTabSwitchesQueryAndFetches(t *testing.T) {
	m := loadOne(newQueryModel(nil, twoQueries(), "q1"), 1, "a")
	m, cmd := m.update(tea.KeyMsg{Type: tea.KeyTab})
	if m.queryID != "q2" || m.loaded || cmd == nil {
		t.Fatalf("queryID=%s loaded=%v cmd=%v", m.queryID, m.loaded, cmd)
	}
}

func TestSwitchBackUsesCache(t *testing.T) {
	m := loadOne(newQueryModel(nil, twoQueries(), "q1"), 1, "a")
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyTab})
	m = loadOne(m, 2, "b")
	m, cmd := m.update(tea.KeyMsg{Type: tea.KeyShiftTab})
	if m.queryID != "q1" || !m.loaded || cmd != nil {
		t.Fatalf("expected cached q1, got queryID=%s loaded=%v cmd=%v", m.queryID, m.loaded, cmd)
	}
	if len(m.rows) != 1 || m.rows[0][4] != "a" {
		t.Fatalf("rows=%v", m.rows)
	}
}

func TestStaleResponsesAfterSwitchAreDropped(t *testing.T) {
	m := newQueryModel(nil, twoQueries(), "q1")
	staleGen := m.gen
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyTab}) // switch to q2 while q1 loads
	wi := &api.WorkItem{ID: 99}
	m, _ = m.update(workItemMsg{gen: staleGen, item: wi})
	m, _ = m.update(queryErrMsg{gen: staleGen, err: errTest})
	if len(m.rows) != 0 || m.err != nil {
		t.Fatalf("stale response leaked: rows=%v err=%v", m.rows, m.err)
	}
}

type testErr struct{}

func (testErr) Error() string { return "boom" }

var errTest = testErr{}

func TestSwitchPicker(t *testing.T) {
	m := newQueryModel(nil, twoQueries(), "q1")
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	if m.mode != modeSwitch {
		t.Fatalf("mode=%v", m.mode)
	}
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyDown})
	m, _ = m.update(tea.KeyMsg{Type: tea.KeyEnter})
	if m.mode != modeBrowse || m.queryID != "q2" {
		t.Fatalf("mode=%v queryID=%s", m.mode, m.queryID)
	}
}
