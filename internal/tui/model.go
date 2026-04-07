package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rainhu/ado/internal/api"
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
)

type Model struct {
	client  *api.Client
	queryID string

	screen      screen
	cursor      int
	items       []menuItem
	queryMdl    queryModel
	settingsMdl settingsModel
}

func NewModel(client *api.Client, queryID string) Model {
	return Model{
		client:  client,
		queryID: queryID,
		screen:  screenMenu,
		items: []menuItem{
			{label: "Query", desc: "Run a saved query and browse work items"},
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
			case 1: // Settings
				m.settingsMdl = newSettingsModel(m.client.Config())
				m.screen = screenSettings
				return m, nil
			}
		}
	}
	return m, nil
}

func (m Model) updateQuery(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "esc" {
			m.screen = screenMenu
			return m, nil
		}
	}
	var cmd tea.Cmd
	m.queryMdl, cmd = m.queryMdl.update(msg)
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

