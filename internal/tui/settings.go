package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rainhu/ado/internal/config"
	"gopkg.in/yaml.v3"
)

type settingsSavedMsg struct{}

type settingsField struct {
	label    string
	envKey   string // for .env fields
	yamlKey  string // for yaml fields
	section  string // "ado" or "summary"
	input    textinput.Model
}

type settingsModel struct {
	fields  []settingsField
	cursor  int
	editing bool
	saved   bool
	err     error
	envPath string
	sumCfg  *config.SummaryConfig
}

func newSettingsModel(cfg *config.Config, sumCfg *config.SummaryConfig) settingsModel {
	var fields []settingsField

	// ADO connection settings (.env)
	adoKeys := []struct {
		label, envKey, value string
	}{
		{"Org", "ADO_ORG", cfg.Org},
		{"Project", "ADO_PROJECT", cfg.Project},
		{"PAT", "ADO_PAT", cfg.PAT},
		{"Query ID", "ADO_QUERY_ID", cfg.QueryID},
		{"Assignee", "ADO_ASSIGNEE", cfg.Assignee},
		{"Team", "ADO_TEAM", cfg.Team},
	}
	for _, k := range adoKeys {
		ti := textinput.New()
		ti.SetValue(k.value)
		ti.CharLimit = 256
		ti.Width = 60
		fields = append(fields, settingsField{
			label:   k.label,
			envKey:  k.envKey,
			section: "ado",
			input:   ti,
		})
	}

	// Summary settings (~/.ado/config.yaml)
	if sumCfg != nil {
		sumKeys := []struct {
			label, yamlKey, value string
		}{
			{"Days", "summary.days", strconv.Itoa(sumCfg.Summary.Days)},
			{"Repos", "summary.repos", strings.Join(sumCfg.Summary.Repos, ",")},
			{"Template", "summary.template", sumCfg.Summary.Template},
			{"Author", "summary.author", sumCfg.Summary.Author},
			{"LLM Provider", "llm.provider", sumCfg.LLM.Provider},
			{"LLM Model", "llm.model", sumCfg.LLM.Model},
			{"LLM API Key Env", "llm.api_key_env", sumCfg.LLM.APIKeyEnv},
			{"LLM Base URL", "llm.base_url", sumCfg.LLM.BaseURL},
			{"LLM Max Tokens", "llm.max_tokens", strconv.Itoa(sumCfg.LLM.MaxTokens)},
		}
		for _, k := range sumKeys {
			ti := textinput.New()
			ti.SetValue(k.value)
			ti.CharLimit = 512
			ti.Width = 60
			fields = append(fields, settingsField{
				label:   k.label,
				yamlKey: k.yamlKey,
				section: "summary",
				input:   ti,
			})
		}
	}

	return settingsModel{
		fields:  fields,
		envPath: cfg.EnvPath(),
		sumCfg:  sumCfg,
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
		// Save .env fields
		var envLines []string
		for _, f := range m.fields {
			if f.section == "ado" {
				envLines = append(envLines, fmt.Sprintf("%s=%s", f.envKey, f.input.Value()))
			}
		}
		content := strings.Join(envLines, "\n") + "\n"
		if err := os.WriteFile(m.envPath, []byte(content), 0600); err != nil {
			return errMsg{err}
		}

		// Save summary config fields to ~/.ado/config.yaml
		if m.sumCfg != nil {
			if err := m.saveSummaryConfig(); err != nil {
				return errMsg{err}
			}
		}

		return settingsSavedMsg{}
	}
}

func (m settingsModel) saveSummaryConfig() error {
	cfg := m.sumCfg

	for _, f := range m.fields {
		if f.section != "summary" {
			continue
		}
		val := f.input.Value()
		switch f.yamlKey {
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

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}

	dir := filepath.Join(os.Getenv("HOME"), ".ado")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create ~/.ado: %w", err)
	}

	path := filepath.Join(dir, "config.yaml")
	return os.WriteFile(path, data, 0644)
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
		// Section headers
		if f.section != prevSection {
			if prevSection != "" {
				b.WriteString("\n")
			}
			switch f.section {
			case "ado":
				b.WriteString(sectionHeader.Render("  ADO Connection (.env)"))
			case "summary":
				b.WriteString(sectionHeader.Render("  Summary Config (~/.ado/config.yaml)"))
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
		b.WriteString(lipgloss.NewStyle().Foreground(lipgloss.Color("42")).MarginTop(1).Render("\n  Saved"))
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
