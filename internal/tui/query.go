package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rainhu/ado/internal/api"
)

var tableStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))

type errMsg struct{ err error }
type queryResultMsg struct{ refs []api.WorkItemRef }
type workItemMsg struct{ item *api.WorkItem }

type queryModel struct {
	client  *api.Client
	queryID string
	table   table.Model
	rows    []table.Row
	pending int
	err     error
	loaded  bool
}

func newQueryModel(client *api.Client, queryID string) queryModel {
	columns := []table.Column{
		{Title: "Tags", Width: 16},
		{Title: "ID", Width: 6},
		{Title: "Type", Width: 12},
		{Title: "State", Width: 10},
		{Title: "Title", Width: 40},
		{Title: "Assigned To", Width: 18},
		{Title: "Estimate", Width: 8},
		{Title: "Remaining", Width: 9},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(20),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	return queryModel{
		client:  client,
		queryID: queryID,
		table:   t,
	}
}

func (m queryModel) init() tea.Cmd {
	return m.fetchQuery
}

func (m queryModel) fetchQuery() tea.Msg {
	result, err := m.client.RunQuery(m.queryID)
	if err != nil {
		return errMsg{err}
	}
	return queryResultMsg{refs: result.WorkItems}
}

func fetchWorkItem(client *api.Client, id int) tea.Cmd {
	return func() tea.Msg {
		wi, err := client.GetWorkItem(id)
		if err != nil {
			return workItemMsg{item: &api.WorkItem{ID: id}}
		}
		return workItemMsg{item: wi}
	}
}

func (m queryModel) update(msg tea.Msg) (queryModel, tea.Cmd) {
	switch msg := msg.(type) {
	case errMsg:
		m.err = msg.err
		return m, nil
	case queryResultMsg:
		m.pending = len(msg.refs)
		if m.pending == 0 {
			m.loaded = true
			return m, nil
		}
		cmds := make([]tea.Cmd, len(msg.refs))
		for i, ref := range msg.refs {
			cmds[i] = fetchWorkItem(m.client, ref.ID)
		}
		return m, tea.Batch(cmds...)
	case workItemMsg:
		wi := msg.item
		estimate := ""
		if wi.Fields.OriginalEstimate > 0 {
			estimate = fmt.Sprintf("%.1f", wi.Fields.OriginalEstimate)
		}
		remaining := ""
		if wi.Fields.RemainingWork > 0 {
			remaining = fmt.Sprintf("%.1f", wi.Fields.RemainingWork)
		}
		m.rows = append(m.rows, table.Row{
			wi.Fields.Tags,
			fmt.Sprintf("%d", wi.ID),
			wi.Fields.WorkItemType,
			wi.Fields.State,
			wi.Fields.Title,
			wi.Fields.AssignedTo.DisplayName,
			estimate,
			remaining,
		})
		m.table.SetRows(m.rows)
		m.pending--
		if m.pending <= 0 {
			m.loaded = true
			m.resizeColumns()
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *queryModel) resizeColumns() {
	headers := []string{"Tags", "ID", "Type", "State", "Title", "Assigned To", "Estimate", "Remaining"}
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range m.rows {
		for i, cell := range row {
			if len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}
	cols := m.table.Columns()
	for i := range cols {
		cols[i].Width = widths[i] + 2
	}
	m.table.SetColumns(cols)
}

func (m queryModel) view() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\n  esc: back", m.err)
	}
	if !m.loaded {
		return fmt.Sprintf("Loading work items... (%d remaining)\n", m.pending)
	}
	return tableStyle.Render(m.table.View()) + "\n  esc: back  ↑↓: navigate  q: quit\n"
}
