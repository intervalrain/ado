package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rainhu/ado/internal/api"
	configpkg "github.com/rainhu/ado/internal/config"
	"github.com/rainhu/ado/internal/llm"
)

var (
	titleStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99")).MarginBottom(1)
	itemStyle     = lipgloss.NewStyle().PaddingLeft(2)
	selectedStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57"))
	helpStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).MarginTop(1)

	// Homepage-only styles — Catppuccin Mocha palette, aurora gradient logo.
	logoGradient = []string{"#a6e3a1", "#94e2d5", "#89dceb", "#89b4fa", "#b4befe", "#cba6f7"}
	logoLines    = []string{
		" █████╗ ██████╗  ██████╗ ",
		"██╔══██╗██╔══██╗██╔═══██╗",
		"███████║██║  ██║██║   ██║",
		"██╔══██║██║  ██║██║   ██║",
		"██║  ██║██████╔╝╚██████╔╝",
		"╚═╝  ╚═╝╚═════╝  ╚═════╝ ",
	}

	homeBox = lipgloss.NewStyle().
			BorderStyle(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#89b4fa")).
			Padding(1, 4)
	homeSubtitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#bac2de")).Italic(true)
	homeStatusLabel   = lipgloss.NewStyle().Foreground(lipgloss.Color("#7f849c"))
	homeStatusValue   = lipgloss.NewStyle().Foreground(lipgloss.Color("#94e2d5")).Bold(true)
	homeStatusDim     = lipgloss.NewStyle().Foreground(lipgloss.Color("#f38ba8")).Italic(true)
	homeDot           = lipgloss.NewStyle().Foreground(lipgloss.Color("#a6e3a1")).Bold(true)

	homeIconStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#89dceb")).Bold(true)
	homeLabelStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#cdd6f4")).Bold(true)
	homeDescStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#7f849c"))

	homeSelectedIcon  = lipgloss.NewStyle().Foreground(lipgloss.Color("#fab387")).Bold(true)
	homeSelectedLabel = lipgloss.NewStyle().Foreground(lipgloss.Color("#f9e2af")).Bold(true)
	homeSelectedDesc  = lipgloss.NewStyle().Foreground(lipgloss.Color("#cdd6f4"))
	homeCursor        = lipgloss.NewStyle().Foreground(lipgloss.Color("#f9e2af")).Bold(true)

	homeHelpKey = lipgloss.NewStyle().Foreground(lipgloss.Color("#89b4fa")).Bold(true)
	homeHelpSep = lipgloss.NewStyle().Foreground(lipgloss.Color("#45475a"))
	homeHelpTxt = lipgloss.NewStyle().Foreground(lipgloss.Color("#7f849c"))
)

type menuItem struct {
	icon  string
	label string
	desc  string
}

type screen int

const (
	screenMenu screen = iota
	screenQuery
	screenSettings
	screenCreate
	screenPR
	screenSummary
	screenPipeline
)

type Model struct {
	client    *api.Client
	queryID   string
	llmClient llm.Client
	sumCfg    *configpkg.Config

	screen      screen
	cursor      int
	items       []menuItem
	queryMdl    queryModel
	settingsMdl settingsModel
	createMdl   createModel
	prMdl       prModel
	summaryMdl  summaryModel
	pipelineMdl pipelineModel
}

func NewModel(client *api.Client, queryID string, llmClient llm.Client, sumCfg *configpkg.Config) Model {
	return Model{
		client:    client,
		queryID:   queryID,
		llmClient: llmClient,
		sumCfg:    sumCfg,
		screen:    screenMenu,
		items: []menuItem{
			{icon: "◆", label: "Query", desc: "Run a saved query and browse work items"},
			{icon: "✚", label: "New", desc: "Create a new work item"},
			{icon: "⇄", label: "Pull Requests", desc: "Browse pull requests by repository"},
			{icon: "▶", label: "Pipelines", desc: "Browse pipeline definitions and builds"},
			{icon: "≡", label: "Summary", desc: "Generate weekly summary report"},
			{icon: "⚙", label: "Settings", desc: "View current configuration"},
		},
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.screen {
	case screenQuery:
		return m.updateQuery(msg)
	case screenSettings:
		return m.updateSettings(msg)
	case screenCreate:
		return m.updateCreate(msg)
	case screenPR:
		return m.updatePR(msg)
	case screenPipeline:
		return m.updatePipeline(msg)
	case screenSummary:
		return m.updateSummary(msg)
	default:
		return m.updateMenu(msg)
	}
}

func (m Model) updateMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.items)-1 {
				m.cursor++
			}
		case "enter":
			switch m.cursor {
			case 0: // Query
				m.queryMdl = newQueryModel(m.client, m.queryID)
				m.screen = screenQuery
				return m, m.queryMdl.init()
			case 1: // New
				m.createMdl = newCreateModel(m.client)
				m.screen = screenCreate
				return m, nil
			case 2: // Pull Requests
				m.prMdl = newPRModel(m.client)
				m.screen = screenPR
				return m, m.prMdl.init()
			case 3: // Pipelines
				m.pipelineMdl = newPipelineModel(m.client)
				m.screen = screenPipeline
				return m, m.pipelineMdl.init()
			case 4: // Summary
				commitsReady = false
				itemsReady = false
				m.summaryMdl = newSummaryModel(m.client, m.llmClient, m.sumCfg)
				m.screen = screenSummary
				return m, m.summaryMdl.init()
			case 5: // Settings
				m.settingsMdl = newSettingsModel(m.sumCfg)
				m.screen = screenSettings
				return m, nil
			}
		}
	}
	return m, nil
}

func (m Model) updateQuery(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Only return to menu on esc when in browse mode (not selecting/editing)
	if keyMsg, ok := msg.(tea.KeyMsg); ok && keyMsg.String() == "esc" && m.queryMdl.mode == modeBrowse {
		m.screen = screenMenu
		return m, nil
	}
	// Handle create request from query screen
	if _, ok := msg.(openCreateMsg); ok {
		m.createMdl = newCreateModel(m.client)
		m.screen = screenCreate
		return m, nil
	}
	var cmd tea.Cmd
	m.queryMdl, cmd = m.queryMdl.update(msg)
	return m, cmd
}

func (m Model) updateCreate(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc":
			if m.createMdl.step == stepType || m.createMdl.step == stepDone {
				m.screen = screenMenu
				return m, nil
			}
		case "ctrl+c":
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.createMdl, cmd = m.createMdl.update(msg)
	return m, cmd
}

func (m Model) updatePR(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc":
			if m.prMdl.view == prViewMenu {
				m.screen = screenMenu
				return m, nil
			}
		case "ctrl+c":
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.prMdl, cmd = m.prMdl.update(msg)
	return m, cmd
}

func (m Model) updatePipeline(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc":
			if m.pipelineMdl.view == pipelineViewList {
				m.screen = screenMenu
				return m, nil
			}
		case "ctrl+c":
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.pipelineMdl, cmd = m.pipelineMdl.update(msg)
	return m, cmd
}

func (m Model) updateSummary(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc":
			switch m.summaryMdl.step {
			case summaryStepSelectCommits, summaryStepViewing, summaryStepActions, summaryStepSaved:
				m.screen = screenMenu
				return m, nil
			case summaryStepSavePrompt:
				if !m.summaryMdl.editingPath {
					m.summaryMdl.step = summaryStepViewing
					return m, nil
				}
			}
		case "ctrl+c":
			return m, tea.Quit
		}
	}
	var cmd tea.Cmd
	m.summaryMdl, cmd = m.summaryMdl.update(msg)
	return m, cmd
}

func (m Model) updateSettings(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Only bubble esc up to the main menu when the settings screen has no
	// sub-modal active; otherwise let the settings model handle it.
	inSub := m.settingsMdl.editing ||
		m.settingsMdl.inReposList ||
		m.settingsMdl.inProfilesList ||
		m.settingsMdl.browsingDir ||
		m.settingsMdl.wizard != nil
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		// ctrl+c always quits, no matter the sub-mode.
		if keyMsg.String() == "ctrl+c" {
			return m, tea.Quit
		}
		if !inSub {
			switch keyMsg.String() {
			case "esc":
				m.screen = screenMenu
				return m, nil
			case "q":
				return m, tea.Quit
			}
		}
	}
	var cmd tea.Cmd
	m.settingsMdl, cmd = m.settingsMdl.update(msg)
	return m, cmd
}

func (m Model) View() string {
	switch m.screen {
	case screenQuery:
		return m.queryMdl.view()
	case screenSettings:
		return m.settingsMdl.view()
	case screenCreate:
		return m.createMdl.view()
	case screenPR:
		return m.prMdl.viewStr()
	case screenPipeline:
		return m.pipelineMdl.viewStr()
	case screenSummary:
		return m.summaryMdl.view()
	default:
		return m.viewMenu()
	}
}

func (m Model) viewMenu() string {
	var b strings.Builder

	// Gradient ASCII logo.
	logo := make([]string, len(logoLines))
	for i, line := range logoLines {
		color := logoGradient[i%len(logoGradient)]
		logo[i] = lipgloss.NewStyle().Foreground(lipgloss.Color(color)).Bold(true).Render(line)
	}

	subtitle := homeSubtitleStyle.Render("— Azure DevOps CLI —")

	// Status line: org / project, or a hint to configure.
	var status string
	if m.sumCfg != nil && m.sumCfg.Org != "" {
		org := strings.TrimPrefix(m.sumCfg.Org, "https://")
		org = strings.TrimPrefix(org, "http://")
		org = strings.TrimPrefix(org, "dev.azure.com/")
		org = strings.TrimSuffix(org, "/")
		target := org
		if m.sumCfg.Project != "" {
			target = org + " / " + m.sumCfg.Project
		}
		status = homeDot.Render("● ") + homeStatusLabel.Render("connected ") + homeStatusValue.Render(target)
	} else {
		status = homeStatusDim.Render("○ no config — run ado config init")
	}

	header := lipgloss.JoinVertical(lipgloss.Center,
		lipgloss.JoinVertical(lipgloss.Left, logo...),
		"",
		subtitle,
		status,
	)
	b.WriteString(homeBox.Render(header))
	b.WriteString("\n\n")

	// Compute label width so descriptions align.
	labelWidth := 0
	for _, it := range m.items {
		if w := lipgloss.Width(it.label); w > labelWidth {
			labelWidth = w
		}
	}

	for i, item := range m.items {
		padLabel := item.label + strings.Repeat(" ", labelWidth-lipgloss.Width(item.label))
		if i == m.cursor {
			b.WriteString("  ")
			b.WriteString(homeCursor.Render("▸ "))
			b.WriteString(homeSelectedIcon.Render(item.icon))
			b.WriteString("  ")
			b.WriteString(homeSelectedLabel.Render(padLabel))
			b.WriteString("   ")
			b.WriteString(homeSelectedDesc.Render(item.desc))
		} else {
			b.WriteString("    ")
			b.WriteString(homeIconStyle.Render(item.icon))
			b.WriteString("  ")
			b.WriteString(homeLabelStyle.Render(padLabel))
			b.WriteString("   ")
			b.WriteString(homeDescStyle.Render(item.desc))
		}
		b.WriteString("\n")
	}

	// Stylized help footer.
	sep := homeHelpSep.Render(" · ")
	help := fmt.Sprintf("  %s %s%s%s %s%s%s %s",
		homeHelpKey.Render("↑↓/jk"), homeHelpTxt.Render("navigate"), sep,
		homeHelpKey.Render("enter"), homeHelpTxt.Render("select"), sep,
		homeHelpKey.Render("q"), homeHelpTxt.Render("quit"),
	)
	b.WriteString("\n")
	b.WriteString(help)
	return b.String()
}

