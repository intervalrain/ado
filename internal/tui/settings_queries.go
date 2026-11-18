package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rainhu/ado/internal/config"
)

// queriesEditor is the Settings → ADO → Queries sub-list: a named list of
// saved queries the Query screen can switch between, one marked default.
type queriesEditor struct {
	items         []config.NamedQuery
	defaultID     string
	cursor        int
	pendingDelete int // index awaiting a second x to confirm; -1 = none

	// Add/edit form; formOpen is false while browsing the list.
	formOpen  bool
	formIdx   int // index being edited, -1 when adding
	formField int // 0 = name, 1 = id
	nameIn    textinput.Model
	idIn      textinput.Model
	formErr   string
}

func newQueriesEditor(cfg *config.Config) queriesEditor {
	nameIn := textinput.New()
	nameIn.CharLimit = 64
	nameIn.Width = 40
	nameIn.Placeholder = "e.g. My Tasks"
	idIn := textinput.New()
	idIn.CharLimit = 64
	idIn.Width = 40
	idIn.Placeholder = "saved query GUID"
	return queriesEditor{
		items:         cfg.QueryList(),
		defaultID:     cfg.QueryID,
		pendingDelete: -1,
		nameIn:        nameIn,
		idIn:          idIn,
	}
}

// apply writes the editor state back into cfg. The default falls back to
// the first query if the previous default was deleted.
func (e queriesEditor) apply(cfg *config.Config) {
	cfg.Queries = append([]config.NamedQuery(nil), e.items...)
	def := e.defaultID
	if config.QueryIndex(e.items, def) < 0 {
		def = ""
		if len(e.items) > 0 {
			def = e.items[0].ID
		}
	}
	cfg.QueryID = def
}

func (e *queriesEditor) openForm(idx int) tea.Cmd {
	e.formOpen = true
	e.formIdx = idx
	e.formField = 0
	e.formErr = ""
	if idx >= 0 {
		e.nameIn.SetValue(e.items[idx].Name)
		e.idIn.SetValue(e.items[idx].ID)
	} else {
		e.nameIn.SetValue("")
		e.idIn.SetValue("")
	}
	e.idIn.Blur()
	e.nameIn.Focus()
	return e.nameIn.Cursor.BlinkCmd()
}

// update handles a key while the queries sub-list is active. It reports
// whether the list should close (esc) and whether config must be saved.
func (e queriesEditor) update(msg tea.KeyMsg) (q queriesEditor, cmd tea.Cmd, closeList, save bool) {
	if e.formOpen {
		return e.updateForm(msg)
	}
	key := msg.String()
	if e.pendingDelete >= 0 && key != "x" && key != "d" {
		e.pendingDelete = -1
	}
	n := len(e.items)
	switch key {
	case "up", "k":
		if e.cursor > 0 {
			e.cursor--
		}
	case "down", "j":
		if e.cursor < n-1 {
			e.cursor++
		}
	case "a":
		return e, e.openForm(-1), false, false
	case "enter", "e":
		if n == 0 {
			return e, e.openForm(-1), false, false
		}
		return e, e.openForm(e.cursor), false, false
	case " ", "*":
		if n > 0 {
			e.defaultID = e.items[e.cursor].ID
			return e, nil, false, true
		}
	case "K", "shift+up":
		if e.cursor > 0 {
			e.items[e.cursor-1], e.items[e.cursor] = e.items[e.cursor], e.items[e.cursor-1]
			e.cursor--
			return e, nil, false, true
		}
	case "J", "shift+down":
		if e.cursor < n-1 {
			e.items[e.cursor+1], e.items[e.cursor] = e.items[e.cursor], e.items[e.cursor+1]
			e.cursor++
			return e, nil, false, true
		}
	case "x", "d":
		if n == 0 {
			return e, nil, false, false
		}
		if e.pendingDelete != e.cursor {
			e.pendingDelete = e.cursor
			return e, nil, false, false
		}
		e.items = append(e.items[:e.cursor], e.items[e.cursor+1:]...)
		e.pendingDelete = -1
		if e.cursor >= len(e.items) && e.cursor > 0 {
			e.cursor--
		}
		return e, nil, false, true
	case "esc":
		return e, nil, true, false
	}
	return e, nil, false, false
}

func (e queriesEditor) updateForm(msg tea.KeyMsg) (queriesEditor, tea.Cmd, bool, bool) {
	switch msg.String() {
	case "esc":
		e.formOpen = false
		e.nameIn.Blur()
		e.idIn.Blur()
		return e, nil, false, false
	case "tab", "shift+tab", "up", "down":
		return e, e.toggleField(), false, false
	case "enter":
		if e.formField == 0 {
			return e, e.toggleField(), false, false
		}
		name := strings.TrimSpace(e.nameIn.Value())
		id := strings.TrimSpace(e.idIn.Value())
		if id == "" {
			e.formErr = "query ID is required"
			return e, nil, false, false
		}
		if name == "" {
			name = id
		}
		if i := config.QueryIndex(e.items, id); i >= 0 && i != e.formIdx {
			e.formErr = fmt.Sprintf("query ID already listed as %q", e.items[i].Name)
			return e, nil, false, false
		}
		for i, q := range e.items {
			if i != e.formIdx && strings.EqualFold(q.Name, name) {
				e.formErr = fmt.Sprintf("name %q is already used", name)
				return e, nil, false, false
			}
		}
		nq := config.NamedQuery{Name: name, ID: id}
		if e.formIdx >= 0 {
			if e.items[e.formIdx].ID == e.defaultID {
				e.defaultID = id // keep the default when its ID is edited
			}
			e.items[e.formIdx] = nq
		} else {
			e.items = append(e.items, nq)
			e.cursor = len(e.items) - 1
			if len(e.items) == 1 {
				e.defaultID = id
			}
		}
		e.formOpen = false
		e.nameIn.Blur()
		e.idIn.Blur()
		return e, nil, false, true
	}
	var cmd tea.Cmd
	if e.formField == 0 {
		e.nameIn, cmd = e.nameIn.Update(msg)
	} else {
		e.idIn, cmd = e.idIn.Update(msg)
	}
	e.formErr = ""
	return e, cmd, false, false
}

func (e *queriesEditor) toggleField() tea.Cmd {
	if e.formField == 0 {
		e.formField = 1
		e.nameIn.Blur()
		e.idIn.Focus()
		return e.idIn.Cursor.BlinkCmd()
	}
	e.formField = 0
	e.idIn.Blur()
	e.nameIn.Focus()
	return e.nameIn.Cursor.BlinkCmd()
}

// summary is the one-line value shown next to the "Queries" label.
func (e queriesEditor) summary() string {
	if len(e.items) == 0 {
		return "(none)"
	}
	def := ""
	if i := config.QueryIndex(e.items, e.defaultID); i >= 0 {
		def = e.items[i].Name
	}
	s := fmt.Sprintf("%d saved", len(e.items))
	if def != "" {
		s += " · default: " + def
	}
	return s
}

// view renders the expanded list (pad aligns it under the field value).
func (e queriesEditor) view(pad string, active bool) string {
	dim := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	warn := lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	sel := lipgloss.NewStyle().Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57"))

	var b strings.Builder
	if len(e.items) == 0 && active {
		b.WriteString(pad + dim.Render("  (no queries — press a to add)") + "\n")
	}
	for j, q := range e.items {
		marker := "  "
		if q.ID == e.defaultID {
			marker = "* "
		}
		if !active {
			b.WriteString(pad + dim.Render("  "+marker+q.Name) + "\n")
			continue
		}
		rc := "  "
		style := lipgloss.NewStyle()
		if j == e.cursor && !e.formOpen {
			rc = "> "
			style = sel
		}
		line := rc + marker + q.Name + dim.Render("  "+q.ID)
		if j == e.pendingDelete {
			line += "  " + warn.Render("press x again to delete")
		}
		b.WriteString(pad + style.Render(line) + "\n")
	}
	if active && e.formOpen {
		title := "Add query"
		if e.formIdx >= 0 {
			title = "Edit query"
		}
		b.WriteString("\n" + pad + "  " + lipgloss.NewStyle().Bold(true).Render(title) + "\n")
		fmt.Fprintf(&b, "%s    Name : %s\n", pad, e.nameIn.View())
		fmt.Fprintf(&b, "%s    ID   : %s\n", pad, e.idIn.View())
		if e.formErr != "" {
			b.WriteString(pad + "    " + warn.Render(e.formErr) + "\n")
		}
	}
	return b.String()
}

func (e queriesEditor) help() string {
	if e.formOpen {
		return "\n  tab: switch field  enter: next / save  esc: cancel"
	}
	return "\n  ↑↓: navigate  a: add  enter/e: edit  space: set default  K/J: move up/down  x: delete (2×)  esc: back"
}
