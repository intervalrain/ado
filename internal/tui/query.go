package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rainhu/ado/internal/api"
	"github.com/rainhu/ado/internal/util"
)

var stateStyles = map[string]lipgloss.Style{
	"active":   lipgloss.NewStyle().Foreground(lipgloss.Color("196")),
	"new":      lipgloss.NewStyle().Foreground(lipgloss.Color("255")),
	"closed":   lipgloss.NewStyle().Foreground(lipgloss.Color("46")),
	"resolved": lipgloss.NewStyle().Foreground(lipgloss.Color("208")),
	"removed":  lipgloss.NewStyle().Foreground(lipgloss.Color("244")),
}

func stateStyle(state string) lipgloss.Style {
	if s, ok := stateStyles[strings.ToLower(strings.TrimSpace(state))]; ok {
		return s
	}
	return lipgloss.NewStyle()
}

var tableStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))

type openCreateMsg struct{}
type errMsg struct{ err error }
type queryResultMsg struct{ refs []api.WorkItemRef }
type workItemMsg struct{ item *api.WorkItem }
type fieldSavedMsg struct{ row, col int }

// column index → ADO field path (empty = not editable)
var columnFields = []string{
	"System.Tags",                                // 0: Tags
	"",                                           // 1: ID (read-only)
	"",                                           // 2: Type (read-only)
	"System.State",                               // 3: State
	"System.Title",                               // 4: Title
	"",                                           // 5: Assigned To (read-only, identity)
	"Microsoft.VSTS.Scheduling.OriginalEstimate", // 6: Estimate
	"Microsoft.VSTS.Scheduling.RemainingWork",    // 7: Remaining
}

var stateOptions = map[string][]string{
	"Task":       {"New", "Active", "Closed", "Removed"},
	"Bug":        {"New", "Confirmed", "Reopen", "Rejected", "Resolved", "Closed"},
	"Epic":       {"New", "Active", "Resolved", "Closed", "Removed"},
	"Feature":    {"New", "Active", "Resolved", "Closed", "Removed"},
	"Issue":      {"Active", "Resolved", "Closed"},
	"User Story": {"New", "Active", "Resolved", "Closed", "Removed"},
}

type queryMode int

const (
	modeBrowse queryMode = iota // ↑↓ navigate rows
	modeSelect                  // ←→ navigate columns in selected row
	modeEdit                    // editing a cell value
	modePick                    // picking from a list (e.g. State)
	modeFilter                  // typing a '/' filter query
)

type queryModel struct {
	client  *api.Client
	queryID string
	table   table.Model
	rows    []table.Row
	pending int
	err     error
	msg     string
	loaded  bool

	mode       queryMode
	selCol     int
	input      textinput.Model
	pickItems  []string
	pickIdx    int
	filter     string
	filterIn   textinput.Model
	visibleIdx []int

	termWidth  int
	termHeight int
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

	ti := textinput.New()
	ti.CharLimit = 256
	ti.Width = 40

	fi := textinput.New()
	fi.CharLimit = 128
	fi.Width = 40
	fi.Prompt = "/"

	return queryModel{
		client:     client,
		queryID:    queryID,
		table:      t,
		input:      ti,
		filterIn:   fi,
		termWidth:  120,
		termHeight: 24,
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
	case tea.WindowSizeMsg:
		m.termWidth = msg.Width
		m.termHeight = msg.Height
		// Reserve lines for title, status, help, etc.
		h := max(m.termHeight-8, 5)
		m.table.SetHeight(h)
		if m.loaded {
			m.resizeColumns()
		}
		return m, nil
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
		m.pending--
		if m.pending <= 0 {
			m.loaded = true
			m.sortRows()
			m.resizeColumns()
		}
		m.applyTableRows()
		return m, nil
	case fieldSavedMsg:
		id := ""
		if msg.row >= 0 && msg.row < len(m.rows) {
			id = m.rows[msg.row][1]
		}
		m.msg = fmt.Sprintf("Saved [%s] for work item %s",
			m.table.Columns()[msg.col].Title, id)
		m.mode = modeSelect
		return m, nil
	}

	switch m.mode {
	case modeBrowse:
		return m.updateBrowse(msg)
	case modeSelect:
		return m.updateSelect(msg)
	case modeEdit:
		return m.updateEdit(msg)
	case modePick:
		return m.updatePick(msg)
	case modeFilter:
		return m.updateFilter(msg)
	}
	return m, nil
}

func (m queryModel) updateBrowse(msg tea.Msg) (queryModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "enter":
			if m.realRow() >= 0 {
				m.mode = modeSelect
				m.selCol = 0
				m.msg = ""
				m.table.SetStyles(m.unfocusedStyles())
				return m, nil
			}
		case "d":
			row := m.realRow()
			if row >= 0 {
				id := m.rows[row][1] // column 1 = ID
				url := fmt.Sprintf("%s/%s/_workitems/edit/%s",
					m.client.BaseURL(), m.client.Project(), id)
				openBrowser(url)
				m.msg = fmt.Sprintf("Opened work item %s in browser", id)
				return m, nil
			}
		case "n":
			return m, func() tea.Msg { return openCreateMsg{} }
		case "r":
			m.rows = nil
			m.visibleIdx = nil
			m.table.SetRows(nil)
			m.loaded = false
			m.pending = 0
			m.err = nil
			m.msg = "Refreshing..."
			return m, m.fetchQuery
		case "/":
			m.mode = modeFilter
			m.filterIn.SetValue(m.filter)
			m.filterIn.Focus()
			m.msg = ""
			return m, m.filterIn.Cursor.BlinkCmd()
		}
	}
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m queryModel) updateSelect(msg tea.Msg) (queryModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc":
			m.mode = modeBrowse
			m.table.SetStyles(m.focusedStyles())
			return m, nil
		case "left", "h":
			if m.selCol > 0 {
				m.selCol--
			}
		case "right", "l":
			if m.selCol < len(m.table.Columns())-1 {
				m.selCol++
			}
		case "enter":
			if columnFields[m.selCol] == "" {
				m.msg = fmt.Sprintf("[%s] is read-only", m.table.Columns()[m.selCol].Title)
				return m, nil
			}
			row := m.realRow()
			if row < 0 {
				return m, nil
			}
			// State column → pick mode
			if m.selCol == 3 {
				wiType := m.rows[row][2] // column 2 = Type
				opts, ok := stateOptions[wiType]
				if !ok {
					m.msg = fmt.Sprintf("No state options for type [%s]", wiType)
					return m, nil
				}
				m.pickItems = opts
				m.pickIdx = 0
				current := m.rows[row][3]
				for i, o := range opts {
					if o == current {
						m.pickIdx = i
						break
					}
				}
				m.mode = modePick
				m.msg = ""
				return m, nil
			}
			m.input.SetValue(m.rows[row][m.selCol])
			m.input.Focus()
			m.mode = modeEdit
			m.msg = ""
			return m, m.input.Cursor.BlinkCmd()
		}
	}
	return m, nil
}

func (m queryModel) updatePick(msg tea.Msg) (queryModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "up", "k":
			if m.pickIdx > 0 {
				m.pickIdx--
			}
		case "down", "j":
			if m.pickIdx < len(m.pickItems)-1 {
				m.pickIdx++
			}
		case "enter":
			row := m.realRow()
			if row < 0 {
				return m, nil
			}
			col := m.selCol
			newVal := m.pickItems[m.pickIdx]
			m.rows[row][col] = newVal
			m.applyTableRows()
			m.resizeColumns()
			return m, m.saveField(row, col, newVal)
		case "esc":
			m.mode = modeSelect
			return m, nil
		}
	}
	return m, nil
}

func (m queryModel) updateFilter(msg tea.Msg) (queryModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "enter":
			m.filter = m.filterIn.Value()
			m.filterIn.Blur()
			m.applyTableRows()
			m.mode = modeBrowse
			if m.filter == "" {
				m.msg = "Filter cleared"
			} else {
				m.msg = fmt.Sprintf("Filter: %s (%d rows)", m.filter, len(m.visibleIdx))
			}
			return m, nil
		case "esc":
			m.filterIn.Blur()
			m.filterIn.SetValue(m.filter)
			m.mode = modeBrowse
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.filterIn, cmd = m.filterIn.Update(msg)
	// live filter preview
	m.filter = m.filterIn.Value()
	m.applyTableRows()
	return m, cmd
}

func (m queryModel) updateEdit(msg tea.Msg) (queryModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "enter":
			m.input.Blur()
			row := m.realRow()
			if row < 0 {
				m.mode = modeSelect
				return m, nil
			}
			col := m.selCol
			newVal := m.input.Value()

			// Update local row
			m.rows[row][col] = newVal
			m.applyTableRows()
			m.resizeColumns()

			// Save to ADO
			return m, m.saveField(row, col, newVal)
		case "esc":
			m.input.Blur()
			m.mode = modeSelect
			return m, nil
		default:
			var cmd tea.Cmd
			m.input, cmd = m.input.Update(msg)
			return m, cmd
		}
	}
	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m queryModel) saveField(row, col int, value string) tea.Cmd {
	client := m.client
	id, _ := strconv.Atoi(m.rows[row][1]) // column 1 = ID
	field := columnFields[col]

	return func() tea.Msg {
		var apiVal any = value
		// Numeric fields
		if field == "Microsoft.VSTS.Scheduling.OriginalEstimate" ||
			field == "Microsoft.VSTS.Scheduling.RemainingWork" {
			if value == "" {
				apiVal = nil
			} else {
				v, err := strconv.ParseFloat(value, 64)
				if err != nil {
					return errMsg{fmt.Errorf("invalid number: %s", value)}
				}
				apiVal = v
			}
		}
		if err := client.UpdateWorkItemField(id, field, apiVal); err != nil {
			return errMsg{err}
		}
		return fieldSavedMsg{row: row, col: col}
	}
}

func (m queryModel) focusedStyles() table.Styles {
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
	return s
}

func (m queryModel) unfocusedStyles() table.Styles {
	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("62")).
		Bold(false)
	return s
}

func (m *queryModel) sortRows() {
	sort.SliceStable(m.rows, func(i, j int) bool {
		a, _ := strconv.Atoi(m.rows[i][1])
		b, _ := strconv.Atoi(m.rows[j][1])
		return a < b
	})
}

func (m *queryModel) computeVisible() {
	needle := strings.ToLower(strings.TrimSpace(m.filter))
	m.visibleIdx = m.visibleIdx[:0]
	for i, row := range m.rows {
		if needle == "" {
			m.visibleIdx = append(m.visibleIdx, i)
			continue
		}
		for _, cell := range row {
			if strings.Contains(strings.ToLower(cell), needle) {
				m.visibleIdx = append(m.visibleIdx, i)
				break
			}
		}
	}
}

func (m *queryModel) applyTableRows() {
	m.computeVisible()
	display := make([]table.Row, len(m.visibleIdx))
	for i, idx := range m.visibleIdx {
		display[i] = append(table.Row(nil), m.rows[idx]...)
	}
	m.table.SetRows(display)
	if c := m.table.Cursor(); c >= len(m.visibleIdx) {
		if len(m.visibleIdx) == 0 {
			m.table.SetCursor(0)
		} else {
			m.table.SetCursor(len(m.visibleIdx) - 1)
		}
	}
}

func (m queryModel) realRow() int {
	c := m.table.Cursor()
	if c < 0 || c >= len(m.visibleIdx) {
		return -1
	}
	return m.visibleIdx[c]
}

func (m *queryModel) resizeColumns() {
	headers := []string{"Tags", "ID", "Type", "State", "Title", "Assigned To", "Estimate", "Remaining"}
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = util.DisplayWidth(h)
	}
	for _, row := range m.rows {
		for i, cell := range row {
			dw := util.DisplayWidth(cell)
			if dw > widths[i] {
				widths[i] = dw
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

	var b strings.Builder

	b.WriteString(titleStyle.Render("Query"))
	if m.filter != "" && m.mode != modeFilter {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("214")).
			Render(fmt.Sprintf("  /%s  (%d/%d)", m.filter, len(m.visibleIdx), len(m.rows))))
	}
	b.WriteString("\n")

	b.WriteString(m.renderWithHighlight())
	b.WriteString("\n")

	// Status message
	if m.msg != "" {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("  " + m.msg))
		b.WriteString("\n")
	}

	// Edit input / pick list
	switch m.mode {
	case modeFilter:
		fmt.Fprintf(&b, "  Filter: %s\n", m.filterIn.View())
	case modeEdit:
		colTitle := m.table.Columns()[m.selCol].Title
		fmt.Fprintf(&b, "  Edit [%s]: %s\n", colTitle, m.input.View())
	case modePick:
		colTitle := m.table.Columns()[m.selCol].Title
		fmt.Fprintf(&b, "  Select [%s]:\n", colTitle)
		for i, item := range m.pickItems {
			if i == m.pickIdx {
				b.WriteString(selectedStyle.Render("  > " + item))
			} else {
				b.WriteString(itemStyle.Render("    " + item))
			}
			b.WriteString("\n")
		}
	}

	// Help
	switch m.mode {
	case modeBrowse:
		b.WriteString("  esc: back  ↑↓: navigate  enter: select row  /: filter  d: details  n: new  r: refresh  q: quit\n")
	case modeSelect:
		b.WriteString("  esc: back to rows  ←→: select column  enter: edit\n")
	case modeEdit:
		b.WriteString("  enter: save  esc: cancel\n")
	case modePick:
		b.WriteString("  ↑↓: select  enter: save  esc: cancel\n")
	case modeFilter:
		b.WriteString("  type to filter  enter: apply  esc: cancel\n")
	}
	return b.String()
}

func (m queryModel) renderWithHighlight() string {
	cols := m.table.Columns()
	cursorRow := m.table.Cursor()

	cellHighlight := lipgloss.NewStyle().
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("208")).
		Bold(true)
	rowHighlight := lipgloss.NewStyle().
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("62"))
	headerStyle := lipgloss.NewStyle().Bold(true)

	var b strings.Builder

	colActive := m.mode == modeSelect || m.mode == modeEdit || m.mode == modePick
	rowActive := colActive || m.mode == modeBrowse || m.mode == modeFilter

	// Header
	for i, col := range cols {
		cell := util.PadRight(col.Title, col.Width)
		if colActive && i == m.selCol {
			b.WriteString(cellHighlight.Render(cell))
		} else {
			b.WriteString(headerStyle.Render(cell))
		}
		if i < len(cols)-1 {
			b.WriteString(" ")
		}
	}
	b.WriteString("\n")

	// Separator
	for i, col := range cols {
		b.WriteString(strings.Repeat("─", col.Width))
		if i < len(cols)-1 {
			b.WriteString(" ")
		}
	}
	b.WriteString("\n")

	// Rows
	for ri, idx := range m.visibleIdx {
		row := m.rows[idx]
		rowStyle := stateStyle(row[3])
		for ci, col := range cols {
			val := ""
			if ci < len(row) {
				val = row[ci]
			}
			cell := util.PadRight(val, col.Width)
			switch {
			case rowActive && ri == cursorRow && colActive && ci == m.selCol:
				b.WriteString(cellHighlight.Render(cell))
			case rowActive && ri == cursorRow:
				b.WriteString(rowHighlight.Render(cell))
			default:
				b.WriteString(rowStyle.Render(cell))
			}
			if ci < len(cols)-1 {
				b.WriteString(" ")
			}
		}
		b.WriteString("\n")
	}

	return tableStyle.Render(b.String())
}
