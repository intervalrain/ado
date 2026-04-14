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
)

type menuItem struct {
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
			{label: "Query", desc: "Run a saved query and browse work items"},
			{label: "New", desc: "Create a new work item"},
			{label: "Pull Requests", desc: "Browse pull requests by repository"},
			{label: "Pipelines", desc: "Browse pipeline definitions and builds"},
			{label: "Summary", desc: "Generate weekly summary report"},
			{label: "Settings", desc: "View current configuration"},
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
			if m.summaryMdl.step == summaryStepViewing || m.summaryMdl.step == summaryStepActions {
				m.screen = screenMenu
				return m, nil
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
	// Only allow esc to go back when not editing a field
	if keyMsg, ok := msg.(tea.KeyMsg); ok && !m.settingsMdl.editing {
		switch keyMsg.String() {
		case "esc":
			m.screen = screenMenu
			return m, nil
		case "q", "ctrl+c":
			return m, tea.Quit
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

	b.WriteString(titleStyle.Render("Azure DevOps CLI"))
	b.WriteString("\n")

	for i, item := range m.items {
		label := fmt.Sprintf("%s  %s", item.label, lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render(item.desc))
		if i == m.cursor {
			b.WriteString(selectedStyle.Render("> " + label))
		} else {
			b.WriteString(itemStyle.Render("  " + label))
		}
		b.WriteString("\n")
	}

	b.WriteString(helpStyle.Render("  ↑↓/jk: navigate  enter: select  q: quit"))
	return b.String()
}

