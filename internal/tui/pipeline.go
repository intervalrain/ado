package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rainhu/ado/internal/api"
	"github.com/rainhu/ado/internal/util"
)

type pipelineView int

const (
	pipelineViewList   pipelineView = iota // Pipeline definitions with latest build
	pipelineViewBuilds                     // Recent builds for a selected pipeline
)

// Messages for async data loading
type pipelineDefsLoadedMsg struct {
	defs   []api.PipelineDefinition
	builds map[int]api.Build
}

type pipelineBuildsLoadedMsg struct {
	builds []api.Build
}

type pipelineModel struct {
	client *api.Client
	view   pipelineView

	termWidth  int
	termHeight int

	// Definition list
	defs     []api.PipelineDefinition
	builds   map[int]api.Build // latest build per definition ID
	defCur   int
	defsLoad bool
	defsErr  error

	// Builds list (for selected definition)
	selectedDef   api.PipelineDefinition
	defBuilds     []api.Build
	buildCur      int
	buildsLoad    bool
	buildsErr     error

	msg string
}

func newPipelineModel(client *api.Client) pipelineModel {
	return pipelineModel{
		client:     client,
		view:       pipelineViewList,
		builds:     make(map[int]api.Build),
		termWidth:  120,
		termHeight: 24,
	}
}

func (m pipelineModel) init() tea.Cmd {
	return m.fetchDefinitions()
}

func (m pipelineModel) fetchDefinitions() tea.Cmd {
	client := m.client
	return func() tea.Msg {
		defs, err := client.ListPipelineDefinitions()
		if err != nil {
			return errMsg{err}
		}

		// Fetch latest builds
		ids := make([]int, len(defs))
		for i, d := range defs {
			ids[i] = d.ID
		}
		latest, err := client.GetLatestBuilds(ids)
		if err != nil {
			return errMsg{err}
		}

		// Fetch stage progress for running builds
		for defID, b := range latest {
			if strings.EqualFold(b.Status, "inProgress") {
				if sp, sErr := client.GetBuildTimeline(b.ID); sErr == nil && sp != nil {
					b.Stages = sp
					latest[defID] = b
				}
			}
		}

		return pipelineDefsLoadedMsg{defs: defs, builds: latest}
	}
}

func (m pipelineModel) fetchBuilds(definitionID int) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		builds, err := client.ListBuilds([]int{definitionID}, 10)
		if err != nil {
			return errMsg{err}
		}

		// Fetch stage progress for running builds
		for i, b := range builds {
			if strings.EqualFold(b.Status, "inProgress") {
				if sp, sErr := client.GetBuildTimeline(b.ID); sErr == nil && sp != nil {
					builds[i].Stages = sp
				}
			}
		}

		return pipelineBuildsLoadedMsg{builds: builds}
	}
}

func (m pipelineModel) update(msg tea.Msg) (pipelineModel, tea.Cmd) {
	if sizeMsg, ok := msg.(tea.WindowSizeMsg); ok {
		m.termWidth = sizeMsg.Width
		m.termHeight = sizeMsg.Height
		return m, nil
	}

	switch msg := msg.(type) {
	case errMsg:
		if m.view == pipelineViewList {
			m.defsErr = msg.err
		} else {
			m.buildsErr = msg.err
		}
		return m, nil
	case pipelineDefsLoadedMsg:
		m.defs = msg.defs
		m.builds = msg.builds
		m.defsLoad = true
		m.defCur = 0
		return m, nil
	case pipelineBuildsLoadedMsg:
		m.defBuilds = msg.builds
		m.buildsLoad = true
		m.buildCur = 0
		return m, nil
	}

	switch m.view {
	case pipelineViewList:
		return m.updateList(msg)
	case pipelineViewBuilds:
		return m.updateBuilds(msg)
	}
	return m, nil
}

func (m pipelineModel) updateList(msg tea.Msg) (pipelineModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "up", "k":
			if m.defCur > 0 {
				m.defCur--
			}
		case "down", "j":
			if m.defCur < len(m.defs)-1 {
				m.defCur++
			}
		case "enter":
			if len(m.defs) > 0 {
				m.selectedDef = m.defs[m.defCur]
				m.view = pipelineViewBuilds
				m.buildsLoad = false
				m.defBuilds = nil
				m.buildsErr = nil
				m.buildCur = 0
				return m, m.fetchBuilds(m.selectedDef.ID)
			}
		case "d":
			if len(m.defs) > 0 {
				def := m.defs[m.defCur]
				if b, ok := m.builds[def.ID]; ok {
					openBrowser(b.WebURL)
					m.msg = fmt.Sprintf("Opened build #%d in browser", b.ID)
				}
			}
		case "r":
			m.defsLoad = false
			m.defs = nil
			m.builds = make(map[int]api.Build)
			m.defsErr = nil
			m.msg = "Refreshing..."
			return m, m.fetchDefinitions()
		}
	}
	return m, nil
}

func (m pipelineModel) updateBuilds(msg tea.Msg) (pipelineModel, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc":
			m.view = pipelineViewList
			m.msg = ""
			return m, nil
		case "up", "k":
			if m.buildCur > 0 {
				m.buildCur--
			}
		case "down", "j":
			if m.buildCur < len(m.defBuilds)-1 {
				m.buildCur++
			}
		case "enter", "d":
			if len(m.defBuilds) > 0 {
				b := m.defBuilds[m.buildCur]
				url := b.WebURL
				if url == "" {
					url = m.client.BuildWebURL(b.ID)
				}
				openBrowser(url)
				m.msg = fmt.Sprintf("Opened build #%d in browser", b.ID)
			}
		case "r":
			m.buildsLoad = false
			m.defBuilds = nil
			m.buildsErr = nil
			m.msg = "Refreshing..."
			return m, m.fetchBuilds(m.selectedDef.ID)
		}
	}
	return m, nil
}

func (m pipelineModel) viewStr() string {
	switch m.view {
	case pipelineViewBuilds:
		return m.viewBuilds()
	default:
		return m.viewList()
	}
}

func (m pipelineModel) viewList() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Pipelines"))
	b.WriteString("\n\n")

	if m.defsErr != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("  Error: %v", m.defsErr)))
		b.WriteString(helpStyle.Render("\n\n  r: retry  esc: back"))
		return b.String()
	}
	if !m.defsLoad {
		b.WriteString("  Loading pipelines...\n")
		return b.String()
	}
	if len(m.defs) == 0 {
		b.WriteString(itemStyle.Render("  (no pipelines found)"))
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("\n  esc: back"))
		return b.String()
	}

	// Build table rows
	type row struct {
		cols []string
	}
	headers := row{[]string{"Pipeline", "Build", "Branch", "Status", "Result", "Duration", "Triggered By", "Stages"}}
	rows := []row{headers}

	for _, def := range m.defs {
		r := row{cols: make([]string, 8)}
		r.cols[0] = def.Name
		if build, ok := m.builds[def.ID]; ok {
			r.cols[1] = fmt.Sprintf("#%d", build.ID)
			r.cols[2] = build.BranchName()
			r.cols[3] = build.StatusLabel()
			r.cols[4] = fmt.Sprintf("%s %s", build.ResultIcon(), build.ResultLabel())
			r.cols[5] = build.Duration()
			r.cols[6] = build.RequestedBy
			r.cols[7] = build.StagesLabel()
		} else {
			for j := 1; j < 8; j++ {
				r.cols[j] = "-"
			}
		}
		rows = append(rows, r)
	}

	// Calculate column widths
	colCount := 8
	widths := make([]int, colCount)
	for _, r := range rows {
		for i, c := range r.cols {
			if dw := util.DisplayWidth(c); dw > widths[i] {
				widths[i] = dw
			}
		}
	}

	// Available lines for the table
	availLines := m.termHeight - 10
	if availLines < 5 {
		availLines = 5
	}

	// Print header
	headerLine := formatTableRow(widths, rows[0].cols)
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99")).Render(headerLine))
	b.WriteString("\n")

	// Print separator
	var sep strings.Builder
	for i, w := range widths {
		if i > 0 {
			sep.WriteString("  ")
		}
		sep.WriteString(strings.Repeat("─", w))
	}
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(sep.String()))
	b.WriteString("\n")

	// Print data rows with scrolling
	dataRows := rows[1:]
	start, end := visibleRange(len(dataRows), m.defCur, availLines)

	if start > 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("  ↑ more pipelines above"))
		b.WriteString("\n")
	}

	for i := start; i < end; i++ {
		line := formatTableRow(widths, dataRows[i].cols)
		if i == m.defCur {
			// Color the result column based on status
			b.WriteString(selectedStyle.Render(line))
		} else {
			line = colorizeRow(dataRows[i].cols, widths)
			b.WriteString(itemStyle.Render(line))
		}
		b.WriteString("\n")
	}

	if end < len(dataRows) {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("  ↓ more pipelines below"))
		b.WriteString("\n")
	}

	if m.msg != "" {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("  "+m.msg))
		b.WriteString("\n")
	}

	b.WriteString(helpStyle.Render("\n  ↑↓: navigate  enter: view builds  d: open in browser  r: refresh  esc: back"))
	return b.String()
}

func (m pipelineModel) viewBuilds() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(fmt.Sprintf("Builds — %s", m.selectedDef.Name)))
	b.WriteString("\n\n")

	if m.buildsErr != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("  Error: %v", m.buildsErr)))
		b.WriteString(helpStyle.Render("\n\n  r: retry  esc: back"))
		return b.String()
	}
	if !m.buildsLoad {
		b.WriteString("  Loading builds...\n")
		return b.String()
	}
	if len(m.defBuilds) == 0 {
		b.WriteString(itemStyle.Render("  (no builds found)"))
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("\n  esc: back"))
		return b.String()
	}

	// Build table
	type row struct {
		cols []string
	}
	headers := row{[]string{"Build", "Branch", "Status", "Result", "Duration", "Triggered By", "Stages"}}
	rows := []row{headers}

	for _, build := range m.defBuilds {
		rows = append(rows, row{[]string{
			fmt.Sprintf("#%d", build.ID),
			build.BranchName(),
			build.StatusLabel(),
			fmt.Sprintf("%s %s", build.ResultIcon(), build.ResultLabel()),
			build.Duration(),
			build.RequestedBy,
			build.StagesLabel(),
		}})
	}

	colCount := 7
	widths := make([]int, colCount)
	for _, r := range rows {
		for i, c := range r.cols {
			if dw := util.DisplayWidth(c); dw > widths[i] {
				widths[i] = dw
			}
		}
	}

	// Header
	headerLine := formatTableRow(widths, rows[0].cols)
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99")).Render(headerLine))
	b.WriteString("\n")

	// Separator
	var sep strings.Builder
	for i, w := range widths {
		if i > 0 {
			sep.WriteString("  ")
		}
		sep.WriteString(strings.Repeat("─", w))
	}
	b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(sep.String()))
	b.WriteString("\n")

	// Data rows
	availLines := m.termHeight - 10
	if availLines < 5 {
		availLines = 5
	}
	dataRows := rows[1:]
	start, end := visibleRange(len(dataRows), m.buildCur, availLines)

	if start > 0 {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("  ↑ more builds above"))
		b.WriteString("\n")
	}

	for i := start; i < end; i++ {
		line := formatTableRow(widths, dataRows[i].cols)
		if i == m.buildCur {
			b.WriteString(selectedStyle.Render(line))
		} else {
			line = colorizeBuildRow(dataRows[i].cols, widths)
			b.WriteString(itemStyle.Render(line))
		}
		b.WriteString("\n")
	}

	if end < len(dataRows) {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render("  ↓ more builds below"))
		b.WriteString("\n")
	}

	if m.msg != "" {
		b.WriteString("\n")
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render("  "+m.msg))
		b.WriteString("\n")
	}

	b.WriteString(helpStyle.Render("\n  ↑↓: navigate  enter/d: open in browser  r: refresh  esc: back"))
	return b.String()
}

// formatTableRow formats columns with proper padding.
func formatTableRow(widths []int, cols []string) string {
	var b strings.Builder
	for i, c := range cols {
		if i > 0 {
			b.WriteString("  ")
		}
		b.WriteString(util.PadRight(c, widths[i]))
	}
	return b.String()
}

// colorizeRow applies color to result column based on build result.
func colorizeRow(cols []string, widths []int) string {
	var b strings.Builder
	for i, c := range cols {
		if i > 0 {
			b.WriteString("  ")
		}
		padded := util.PadRight(c, widths[i])
		switch {
		case i == 4 && strings.Contains(c, "✓"): // Result: Succeeded
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render(padded))
		case i == 4 && strings.Contains(c, "✗"): // Result: Failed
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render(padded))
		case i == 4 && strings.Contains(c, "⚠"): // Result: Partial
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render(padded))
		case i == 3 && c == "Running": // Status: Running
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render(padded))
		default:
			b.WriteString(padded)
		}
	}
	return b.String()
}

// colorizeBuildRow applies color to the result column in builds view.
func colorizeBuildRow(cols []string, widths []int) string {
	var b strings.Builder
	for i, c := range cols {
		if i > 0 {
			b.WriteString("  ")
		}
		padded := util.PadRight(c, widths[i])
		switch {
		case i == 3 && strings.Contains(c, "✓"): // Result: Succeeded
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Render(padded))
		case i == 3 && strings.Contains(c, "✗"): // Result: Failed
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render(padded))
		case i == 3 && strings.Contains(c, "⚠"): // Result: Partial
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render(padded))
		case i == 2 && c == "Running": // Status: Running
			b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Render(padded))
		default:
			b.WriteString(padded)
		}
	}
	return b.String()
}
