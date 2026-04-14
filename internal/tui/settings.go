package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rainhu/ado/internal/config"
)

type settingsSavedMsg struct{}

type settingsField struct {
	label   string
	key     string // yaml key path for reference
	section string // "ado", "summary", or "llm"
	input   textinput.Model
}

type settingsModel struct {
	fields  []settingsField
	cursor  int
	editing bool
	saved   bool
	err     error
	cfg     *config.Config
}

func newSettingsModel(cfg *config.Config) settingsModel {
	var fields []settingsField

	// ADO connection settings
	adoKeys := []struct {
		label, key, value string
	}{
		{"Org", "org", cfg.Org},
		{"Project", "project", cfg.Project},
		{"PAT", "pat", cfg.PAT},
		{"Query ID", "query_id", cfg.QueryID},
		{"Assignee", "assignee", cfg.Assignee},
		{"Team", "team", cfg.Team},
	}
	for _, k := range adoKeys {
		ti := textinput.New()
		ti.SetValue(k.value)
		ti.CharLimit = 256
		ti.Width = 60
		fields = append(fields, settingsField{
			label:   k.label,
			key:     k.key,
			section: "ado",
			input:   ti,
		})
	}

	// Summary settings
	sumKeys := []struct {
		label, key, value string
	}{
		{"Days", "summary.days", strconv.Itoa(cfg.Summary.Days)},
		{"Repos", "summary.repos", strings.Join(cfg.Summary.Repos, ",")},
		{"Template", "summary.template", cfg.Summary.Template},
		{"Author", "summary.author", cfg.Summary.Author},
	}
	for _, k := range sumKeys {
		ti := textinput.New()
		ti.SetValue(k.value)
		ti.CharLimit = 512
		ti.Width = 60
		fields = append(fields, settingsField{
			label:   k.label,
			key:     k.key,
			section: "summary",
			input:   ti,
		})
	}

	// LLM settings
	llmKeys := []struct {
		label, key, value string
	}{
		{"Provider", "llm.provider", cfg.LLM.Provider},
		{"Model", "llm.model", cfg.LLM.Model},
		{"API Key Env", "llm.api_key_env", cfg.LLM.APIKeyEnv},
		{"Base URL", "llm.base_url", cfg.LLM.BaseURL},
		{"Max Tokens", "llm.max_tokens", strconv.Itoa(cfg.LLM.MaxTokens)},
	}
	for _, k := range llmKeys {
		ti := textinput.New()
		ti.SetValue(k.value)
		ti.CharLimit = 256
		ti.Width = 60
		fields = append(fields, settingsField{
			label:   k.label,
			key:     k.key,
			section: "llm",
			input:   ti,
		})
	}

	return settingsModel{
		fields: fields,
		cfg:    cfg,
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
		cfg := m.cfg

		for _, f := range m.fields {
			val := f.input.Value()
			switch f.key {
			// ADO
			case "org":
				cfg.Org = val
			case "project":
				cfg.Project = val
			case "pat":
				cfg.PAT = val
			case "query_id":
				cfg.QueryID = val
			case "assignee":
				cfg.Assignee = val
			case "team":
				cfg.Team = val
			// Summary
			case "summary.days":
				if n, err := strconv.Atoi(val); err == nil {
					cfg.Summary.Days = n
				}
			case "summary.repos":
				if val != "" {
					cfg.Summary.Repos = strings.Split(val, ",")
					for i := range cfg.Summary.Repos {
						cfg.Summary.Repos[i] = strings.TrimSpace(cfg.Summary.Repos[i])
					}
				}
			case "summary.template":
				cfg.Summary.Template = val
			case "summary.author":
				cfg.Summary.Author = val
			// LLM
			case "llm.provider":
				cfg.LLM.Provider = val
			case "llm.model":
				cfg.LLM.Model = val
			case "llm.api_key_env":
				cfg.LLM.APIKeyEnv = val
			case "llm.base_url":
				cfg.LLM.BaseURL = val
			case "llm.max_tokens":
				if n, err := strconv.Atoi(val); err == nil {
					cfg.LLM.MaxTokens = n
				}
			}
		}

		if err := config.Save(cfg); err != nil {
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

	sectionHeader := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	prevSection := ""

	for i, f := range m.fields {
		if f.section != prevSection {
			if prevSection != "" {
				b.WriteString("\n")
			}
			switch f.section {
			case "ado":
				b.WriteString(sectionHeader.Render("  ADO Connection"))
			case "summary":
				b.WriteString(sectionHeader.Render("  Summary"))
			case "llm":
				b.WriteString(sectionHeader.Render("  LLM"))
			}
			b.WriteString("\n")
			prevSection = f.section
		}

		label := fmt.Sprintf("%-*s", labelWidth, f.label)
		cursor := "  "
		if i == m.cursor {
			cursor = "> "
		}

		if m.editing && i == m.cursor {
			b.WriteString(fmt.Sprintf("%s%s : %s\n", cursor, label, f.input.View()))
		} else {
			value := f.input.Value()
			if f.key == "pat" {
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
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("42")).MarginTop(1).Render("\n  Saved to " + config.ConfigPath()))
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
