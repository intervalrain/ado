package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rainhu/ado/internal/config"
)

type settingsSavedMsg struct{}

type settingsField struct {
	label  string
	envKey string
	input  textinput.Model
}

type settingsModel struct {
	fields  []settingsField
	cursor  int
	editing bool
	saved   bool
	err     error
	envPath string
}

func newSettingsModel(cfg *config.Config) settingsModel {
	keys := []struct {
		label, envKey, value string
	}{
		{"Org", "ADO_ORG", cfg.Org},
		{"Project", "ADO_PROJECT", cfg.Project},
		{"PAT", "ADO_PAT", cfg.PAT},
		{"Query ID", "ADO_QUERY_ID", cfg.QueryID},
		{"Assignee", "ADO_ASSIGNEE", cfg.Assignee},
		{"Team", "ADO_TEAM", cfg.Team},
	}

	fields := make([]settingsField, len(keys))
	for i, k := range keys {
		ti := textinput.New()
		ti.SetValue(k.value)
		ti.CharLimit = 256
		ti.Width = 60
		fields[i] = settingsField{
			label:  k.label,
			envKey: k.envKey,
			input:  ti,
		}
	}

	return settingsModel{
		fields:  fields,
		envPath: cfg.EnvPath(),
	}
}

func (m settingsModel) update(msg tea.Msg) (settingsModel, tea.Cmd) {
	switch msg := msg.(type) {
	case settingsSavedMsg:
		m.saved = true
		m.editing = false
		return m, nil
	case tea.KeyMsg:
		if m.editing {
			return m.updateEditing(msg)
		}
		return m.updateBrowsing(msg)
	}

	if m.editing {
		var cmd tea.Cmd
		m.fields[m.cursor].input, cmd = m.fields[m.cursor].input.Update(msg)
		return m, cmd
	}
	return m, nil
}

func (m settingsModel) updateBrowsing(msg tea.KeyMsg) (settingsModel, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		m.saved = false
		if m.cursor > 0 {
			m.cursor--
		}
	case "down", "j":
		m.saved = false
		if m.cursor < len(m.fields)-1 {
			m.cursor++
		}
	case "enter":
		m.saved = false
		m.editing = true
		m.fields[m.cursor].input.Focus()
		return m, m.fields[m.cursor].input.Cursor.BlinkCmd()
	}
	return m, nil
}

func (m settingsModel) updateEditing(msg tea.KeyMsg) (settingsModel, tea.Cmd) {
	switch msg.String() {
	case "enter":
		m.fields[m.cursor].input.Blur()
		m.editing = false
		return m, m.save()
	case "esc":
		m.fields[m.cursor].input.Blur()
		m.editing = false
		return m, nil
	default:
		var cmd tea.Cmd
		m.fields[m.cursor].input, cmd = m.fields[m.cursor].input.Update(msg)
		return m, cmd
	}
}

func (m settingsModel) save() tea.Cmd {
	return func() tea.Msg {
		lines := make([]string, len(m.fields))
		for i, f := range m.fields {
			lines[i] = fmt.Sprintf("%s=%s", f.envKey, f.input.Value())
		}
		content := strings.Join(lines, "\n") + "\n"
		if err := os.WriteFile(m.envPath, []byte(content), 0600); err != nil {
			return errMsg{err}
		}
		return settingsSavedMsg{}
	}
}

func (m settingsModel) view() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Settings"))
	b.WriteString("\n\n")

	labelWidth := 0
	for _, f := range m.fields {
		if len(f.label) > labelWidth {
			labelWidth = len(f.label)
		}
	}

	for i, f := range m.fields {
		label := fmt.Sprintf("%-*s", labelWidth, f.label)
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}

		if m.editing && i == m.cursor {
			b.WriteString(fmt.Sprintf("%s%s : %s\n", cursor, label, f.input.View()))
		} else {
			value := f.input.Value()
			if f.envKey == "ADO_PAT" {
				value = maskPAT(value)
			}
			style := lipgloss.NewStyle()
			if i == m.cursor {
				style = style.Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57"))
			}
			b.WriteString(style.Render(fmt.Sprintf("%s%s : %s", cursor, label, value)))
			b.WriteString("\n")
		}
	}

	if m.saved {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("42")).MarginTop(1).Render("\n  Saved to " + m.envPath))
	}
	if m.err != nil {
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("196")).MarginTop(1).Render(fmt.Sprintf("\n  Error: %v", m.err)))
	}

	b.WriteString(helpStyle.Render("\n  ↑↓: navigate  enter: edit  esc: back"))
	return b.String()
}

func maskPAT(pat string) string {
	if len(pat) <= 8 {
		return "****"
	}
	return pat[:4] + strings.Repeat("*", len(pat)-8) + pat[len(pat)-4:]
}
