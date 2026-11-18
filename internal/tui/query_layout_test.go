package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rainhu/ado/internal/api"
	"github.com/rainhu/ado/internal/util"
)

// loadN loads n work items whose titles are `title-<id>` + pad.
func loadN(m queryModel, n int, pad string) queryModel {
	refs := make([]api.WorkItemRef, n)
	for i := range refs {
		refs[i] = api.WorkItemRef{ID: i + 1}
	}
	m, _ = m.update(queryResultMsg{gen: m.gen, refs: refs})
	for i := 1; i <= n; i++ {
		wi := &api.WorkItem{ID: i}
		wi.Fields.Title = fmt.Sprintf("title-%d%s", i, pad)
		wi.Fields.Tags = "backend; frontend; infra; release-blocker; needs-triage"
		wi.Fields.State = "Active"
		wi.Fields.WorkItemType = "Task"
		m, _ = m.update(workItemMsg{gen: m.gen, item: wi})
	}
	return m
}

func sized(m queryModel, w, h int) queryModel {
	m, _ = m.update(tea.WindowSizeMsg{Width: w, Height: h})
	return m
}

func maxLineWidth(s string) int {
	w := 0
	for line := range strings.SplitSeq(s, "\n") {
		w = max(w, util.DisplayWidth(stripANSI(line)))
	}
	return w
}

func stripANSI(s string) string {
	var b strings.Builder
	inEsc := false
	for _, r := range s {
		switch {
		case r == 0x1b:
			inEsc = true
		case inEsc && (r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'):
			inEsc = false
		case !inEsc:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func TestTableFitsTerminalWidthWithLongCells(t *testing.T) {
	m := sized(newQueryModel(nil, twoQueries(), "q1"), 120, 30)
	m = loadN(m, 3, strings.Repeat(" very long title", 20))
	if w := maxLineWidth(m.renderWithHighlight()); w > 120 {
		t.Fatalf("table is %d cols wide, terminal is 120", w)
	}
	cols := m.table.Columns()
	if cols[0].Width > 21 {
		t.Fatalf("Tags column not capped: %d", cols[0].Width)
	}
	if cols[4].Width < 40 {
		t.Fatalf("Title should take the spare width, got %d", cols[4].Width)
	}
}

func TestShortTitleIsNotStretched(t *testing.T) {
	m := sized(newQueryModel(nil, twoQueries(), "q1"), 200, 30)
	m = loadN(m, 2, "")
	if w := m.table.Columns()[4].Width; w > len("title-2")+1 {
		t.Fatalf("Title width %d, want content width", w)
	}
}

func TestRowsArePaged(t *testing.T) {
	m := sized(newQueryModel(nil, twoQueries(), "q1"), 120, 20)
	m = loadN(m, 50, "")
	size := m.pageSize()
	out := stripANSI(m.renderWithHighlight())
	if strings.Contains(out, fmt.Sprintf("title-%d ", size+1)) {
		t.Fatalf("row %d rendered on page 1", size+1)
	}
	if !strings.Contains(out, "title-1 ") {
		t.Fatal("first row missing")
	}
	pages := (50 + size - 1) / size
	if !strings.Contains(stripANSI(m.view()), fmt.Sprintf("Page 1/%d", pages)) {
		t.Fatalf("page indicator missing:\n%s", stripANSI(m.view()))
	}

	m, _ = m.update(tea.KeyMsg{Type: tea.KeyRight})
	if m.table.Cursor() != size {
		t.Fatalf("right should jump to next page start, cursor=%d", m.table.Cursor())
	}
	out = stripANSI(m.renderWithHighlight())
	if !strings.Contains(out, fmt.Sprintf("title-%d ", size+1)) || strings.Contains(out, "title-1 ") {
		t.Fatalf("page 2 content wrong:\n%s", out)
	}

	m, _ = m.update(tea.KeyMsg{Type: tea.KeyLeft})
	if m.table.Cursor() != 0 {
		t.Fatalf("left should return to page 1, cursor=%d", m.table.Cursor())
	}
}

func TestTruncate(t *testing.T) {
	cases := []struct {
		in   string
		w    int
		want string
	}{
		{"abcdef", 10, "abcdef"},
		{"abcdef", 4, "abc…"},
		{"中文標題", 5, "中文…"},
		{"abc", 0, ""},
	}
	for _, c := range cases {
		if got := util.Truncate(c.in, c.w); got != c.want {
			t.Errorf("Truncate(%q,%d)=%q want %q", c.in, c.w, got, c.want)
		}
	}
}
