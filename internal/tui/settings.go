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

type settingsTab struct {
	label   string
	section string
}

var settingsTabs = []settingsTab{
	{"ADO", "ado"},
	{"Summary", "summary"},
	{"LLM", "llm"},
}

type settingsModel struct {
	fields    []settingsField
	cursor    int
	activeTab int
	editing   bool
	saved     bool
	err       error
	cfg       *config.Config

	// Repos list sub-editor
	repos        []string
	reposCursor  int
	reposEditing bool
	reposAdding  bool
	inReposList  bool // true when navigating inside the repos sub-list

	// Model profile switcher
	profiles        []string
	profilesCursor  int
	inProfilesList  bool
	currentProfile  string
	profileSwitched bool
	pendingDelete   string // profile name awaiting a second press to confirm

	// Active wizard (add/edit); nil when not open.
	wizard *profileWizard

	// Directory browser
	dirPicker   dirPickerModel
	browsingDir bool // true when the directory picker is active
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
		{"Repos", "summary.repos", ""}, // placeholder — rendered as sub-list
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

	// LLM settings — "Profile" is a sub-list (like repos); the rest are text inputs
	llmKeys := []struct {
		label, key, value string
	}{
		{"Profile", "llm.profile", ""}, // rendered as sub-list
		{"Provider", "llm.provider", cfg.LLM.Provider},
		{"Model", "llm.model", cfg.LLM.Model},
		{"API Key", "llm.api_key", cfg.LLM.APIKey},
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

	repos := make([]string, len(cfg.Summary.Repos))
	copy(repos, cfg.Summary.Repos)

	m := settingsModel{
		fields:         fields,
		cfg:            cfg,
		repos:          repos,
		profiles:       config.ListModelProfiles(),
		currentProfile: config.CurrentModel(),
	}
	m.cursor = m.firstFieldOfTab(0)
	return m
}

// firstFieldOfTab returns the index of the first field in the given tab,
// or 0 if no field matches (defensive, shouldn't happen in practice).
func (m settingsModel) firstFieldOfTab(tabIdx int) int {
	section := settingsTabs[tabIdx].section
	for i, f := range m.fields {
		if f.section == section {
			return i
		}
	}
	return 0
}

// lastFieldOfTab returns the index of the last field in the given tab.
func (m settingsModel) lastFieldOfTab(tabIdx int) int {
	section := settingsTabs[tabIdx].section
	last := 0
	for i, f := range m.fields {
		if f.section == section {
			last = i
		}
	}
	return last
}

func (m settingsModel) update(msg tea.Msg) (settingsModel, tea.Cmd) {
	// Wizard consumes all events while open, except completion messages below.
	if m.wizard != nil {
		if done, ok := msg.(profileWizardDoneMsg); ok {
			m.wizard = nil
			m.profiles = config.ListModelProfiles()
			switch done.action {
			case "added", "updated":
				if done.current {
					_ = config.SetCurrentModel(done.name)
					m.currentProfile = done.name
					m.profileSwitched = true
					if p, err := config.LoadModelProfile(done.name); err == nil {
						m.syncLLMFields(p)
					}
				}
				// Cursor to newly saved profile.
				for i, n := range m.profiles {
					if n == done.name {
						m.profilesCursor = i
						break
					}
				}
			}
			return m, nil
		}
		w, cmd := m.wizard.update(msg)
		m.wizard = &w
		return m, cmd
	}
	switch msg := msg.(type) {
	case settingsSavedMsg:
		m.saved = true
		m.editing = false
		m.reposEditing = false
		m.reposAdding = false
		return m, nil
	case dirSelectedMsg:
		if m.reposAdding && len(msg.paths) > 0 {
			// Multi-select: append all selected paths (dedup)
			existing := make(map[string]bool, len(m.repos))
			for _, r := range m.repos {
				existing[r] = true
			}
			for _, p := range msg.paths {
				if !existing[p] {
					m.repos = append(m.repos, p)
				}
			}
			m.reposCursor = len(m.repos) - 1
		} else if m.reposAdding && msg.path != "" {
			m.repos = append(m.repos, msg.path)
			m.reposCursor = len(m.repos) - 1
		} else if m.reposEditing {
			m.repos[m.reposCursor] = msg.path
		}
		m.browsingDir = false
		m.reposAdding = false
		m.reposEditing = false
		return m, m.saveRepos()
	case dirCancelledMsg:
		m.browsingDir = false
		m.reposAdding = false
		m.reposEditing = false
		return m, nil
	case tea.KeyMsg:
		if m.browsingDir {
			var cmd tea.Cmd
			m.dirPicker, cmd = m.dirPicker.Update(msg)
			return m, cmd
		}
		if m.inReposList {
			return m.updateReposList(msg)
		}
		if m.inProfilesList {
			return m.updateProfilesList(msg)
		}
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
	first := m.firstFieldOfTab(m.activeTab)
	last := m.lastFieldOfTab(m.activeTab)
	switch msg.String() {
	case "tab", "shift+tab", "right", "left":
		m.saved = false
		delta := 1
		if s := msg.String(); s == "shift+tab" || s == "left" {
			delta = -1
		}
		m.activeTab = (m.activeTab + delta + len(settingsTabs)) % len(settingsTabs)
		m.cursor = m.firstFieldOfTab(m.activeTab)
		return m, nil
	case "up", "k":
		m.saved = false
		if m.cursor > first {
			m.cursor--
		}
	case "down", "j":
		m.saved = false
		if m.cursor < last {
			m.cursor++
		}
	case "enter":
		m.saved = false
		if m.fields[m.cursor].key == "summary.repos" {
			m.inReposList = true
			m.reposCursor = 0
			return m, nil
		}
		if m.fields[m.cursor].key == "llm.profile" {
			m.inProfilesList = true
			m.profiles = config.ListModelProfiles()
			m.currentProfile = config.CurrentModel()
			m.profilesCursor = 0
			for i, n := range m.profiles {
				if n == m.currentProfile {
					m.profilesCursor = i
					break
				}
			}
			return m, nil
		}
		m.editing = true
		m.fields[m.cursor].input.Focus()
		return m, m.fields[m.cursor].input.Cursor.BlinkCmd()
	}
	return m, nil
}

func (m settingsModel) updateProfilesList(msg tea.KeyMsg) (settingsModel, tea.Cmd) {
	key := msg.String()

	// Any key except the repeat-press clears a pending delete.
	if m.pendingDelete != "" && key != "x" && key != "d" {
		m.pendingDelete = ""
	}

	maxIdx := len(m.profiles) - 1
	if maxIdx < 0 {
		// Empty list: allow `a` to add, `esc` to close.
		switch key {
		case "a":
			w := newProfileWizard(pwModeAdd, nil)
			m.wizard = &w
			return m, w.name.Cursor.BlinkCmd()
		case "esc":
			m.inProfilesList = false
		}
		return m, nil
	}
	switch key {
	case "up", "k":
		if m.profilesCursor > 0 {
			m.profilesCursor--
		}
	case "down", "j":
		if m.profilesCursor < maxIdx {
			m.profilesCursor++
		}
	case "enter":
		name := m.profiles[m.profilesCursor]
		p, err := config.LoadModelProfile(name)
		if err != nil {
			m.err = err
			return m, nil
		}
		if err := config.SetCurrentModel(name); err != nil {
			m.err = err
			return m, nil
		}
		m.currentProfile = name
		m.profileSwitched = true
		m.syncLLMFields(p)
		m.inProfilesList = false
	case "a":
		w := newProfileWizard(pwModeAdd, nil)
		m.wizard = &w
		return m, w.name.Cursor.BlinkCmd()
	case "e":
		name := m.profiles[m.profilesCursor]
		p, err := config.LoadModelProfile(name)
		if err != nil {
			m.err = err
			return m, nil
		}
		w := newProfileWizard(pwModeEdit, p)
		m.wizard = &w
		return m, w.name.Cursor.BlinkCmd()
	case "x", "d":
		name := m.profiles[m.profilesCursor]
		if m.pendingDelete != name {
			m.pendingDelete = name
			return m, nil
		}
		// Second press confirms.
		if err := config.RemoveModelProfile(name); err != nil {
			m.err = err
			m.pendingDelete = ""
			return m, nil
		}
		if m.currentProfile == name {
			_ = config.SetCurrentModel("")
			m.currentProfile = ""
		}
		m.pendingDelete = ""
		m.profiles = config.ListModelProfiles()
		if m.profilesCursor >= len(m.profiles) && m.profilesCursor > 0 {
			m.profilesCursor--
		}
	case "esc":
		m.inProfilesList = false
	}
	return m, nil
}

// syncLLMFields mirrors a profile's values into the llm.* text inputs so the
// UI reflects what Load() will see next launch.
func (m *settingsModel) syncLLMFields(p *config.ModelProfile) {
	for i := range m.fields {
		switch m.fields[i].key {
		case "llm.provider":
			m.fields[i].input.SetValue(p.Provider)
			m.cfg.LLM.Provider = p.Provider
		case "llm.model":
			m.fields[i].input.SetValue(p.Model)
			m.cfg.LLM.Model = p.Model
		case "llm.api_key":
			m.fields[i].input.SetValue(p.APIKey)
			m.cfg.LLM.APIKey = p.APIKey
		case "llm.base_url":
			m.fields[i].input.SetValue(p.BaseURL)
			m.cfg.LLM.BaseURL = p.BaseURL
		case "llm.max_tokens":
			if p.MaxTokens > 0 {
				m.fields[i].input.SetValue(strconv.Itoa(p.MaxTokens))
				m.cfg.LLM.MaxTokens = p.MaxTokens
			}
		}
	}
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

func (m settingsModel) updateReposList(msg tea.KeyMsg) (settingsModel, tea.Cmd) {
	// Browsing the repos list
	maxIdx := len(m.repos) // last index is the [+ Add] button
	switch msg.String() {
	case "up", "k":
		if m.reposCursor > 0 {
			m.reposCursor--
		}
	case "down", "j":
		if m.reposCursor < maxIdx {
			m.reposCursor++
		}
	case "enter":
		if m.reposCursor == len(m.repos) {
			// [+ Add] — open multi-select directory browser
			m.reposAdding = true
			m.browsingDir = true
			m.dirPicker = newMultiDirPicker("")
			return m, nil
		}
		// Edit existing — open single-select directory browser
		m.reposEditing = true
		m.browsingDir = true
		m.dirPicker = newDirPicker(m.repos[m.reposCursor])
		return m, nil
	case "a":
		m.reposAdding = true
		m.browsingDir = true
		m.dirPicker = newMultiDirPicker("")
		return m, nil
	case "x", "d":
		if m.reposCursor < len(m.repos) && len(m.repos) > 0 {
			m.repos = append(m.repos[:m.reposCursor], m.repos[m.reposCursor+1:]...)
			if m.reposCursor >= len(m.repos) && m.reposCursor > 0 {
				m.reposCursor--
			}
			return m, m.saveRepos()
		}
	case "esc":
		m.inReposList = false
		return m, nil
	}
	return m, nil
}

func (m settingsModel) saveRepos() tea.Cmd {
	return func() tea.Msg {
		m.cfg.Summary.Repos = make([]string, len(m.repos))
		copy(m.cfg.Summary.Repos, m.repos)
		if err := config.Save(m.cfg); err != nil {
			return errMsg{err}
		}
		return settingsSavedMsg{}
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
			// Summary
			case "summary.days":
				if n, err := strconv.Atoi(val); err == nil {
					cfg.Summary.Days = n
				}
			case "summary.repos":
				// handled by saveRepos(), skip here
			case "llm.profile":
				// handled by updateProfilesList(), skip here
			case "summary.template":
				cfg.Summary.Template = val
			case "summary.author":
				cfg.Summary.Author = val
			// LLM
			case "llm.provider":
				cfg.LLM.Provider = val
			case "llm.model":
				cfg.LLM.Model = val
			case "llm.api_key":
				cfg.LLM.APIKey = val
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
	if m.wizard != nil {
		return m.wizard.view()
	}
	if m.browsingDir {
		return m.dirPicker.View()
	}

	var b strings.Builder

	b.WriteString(titleStyle.Render("Settings"))
	b.WriteString("\n")
	b.WriteString(m.renderTabBar())
	b.WriteString("\n\n")

	// Only consider visible fields for layout.
	activeSection := settingsTabs[m.activeTab].section
	labelWidth := 0
	for _, f := range m.fields {
		if f.section != activeSection {
			continue
		}
		if len(f.label) > labelWidth {
			labelWidth = len(f.label)
		}
	}

	dimStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	addStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("42"))

	for i, f := range m.fields {
		if f.section != activeSection {
			continue
		}

		// Special rendering for model profile switcher
		if f.key == "llm.profile" {
			label := fmt.Sprintf("%-*s", labelWidth, f.label)
			cursor := "  "
			if i == m.cursor && !m.inReposList && !m.inProfilesList {
				cursor = "> "
			}
			style := lipgloss.NewStyle()
			if i == m.cursor && !m.inReposList && !m.inProfilesList {
				style = style.Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57"))
			}
			summary := "(inline)"
			if m.currentProfile != "" {
				summary = m.currentProfile
				if p, err := config.LoadModelProfile(m.currentProfile); err == nil {
					summary = fmt.Sprintf("%s · %s / %s", m.currentProfile, p.Provider, p.Model)
				}
			}
			b.WriteString(style.Render(fmt.Sprintf("%s%s : %s", cursor, label, summary)))
			b.WriteString("\n")

			if m.inProfilesList {
				pad := strings.Repeat(" ", labelWidth+5)
				if len(m.profiles) == 0 {
					b.WriteString(pad + dimStyle.Render("  (no profiles — press a to add)") + "\n")
				}
				warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
				for j, n := range m.profiles {
					rc := "  "
					if j == m.profilesCursor {
						rc = "> "
					}
					rs := lipgloss.NewStyle()
					if j == m.profilesCursor {
						rs = rs.Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57"))
					}
					marker := "  "
					if n == m.currentProfile {
						marker = "* "
					}
					line := rc + marker + n
					if p, err := config.LoadModelProfile(n); err == nil {
						line += dimStyle.Render(fmt.Sprintf("  (%s / %s)", p.Provider, p.Model))
					}
					if n == m.pendingDelete {
						line += "  " + warnStyle.Render("press x again to delete")
					}
					b.WriteString(pad + rs.Render(line) + "\n")
				}
			}
			continue
		}

		// Special rendering for repos list
		if f.key == "summary.repos" {
			label := fmt.Sprintf("%-*s", labelWidth, f.label)
			cursor := "  "
			if i == m.cursor && !m.inReposList {
				cursor = "> "
			}
			style := lipgloss.NewStyle()
			if i == m.cursor && !m.inReposList {
				style = style.Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57"))
			}
			if len(m.repos) == 0 {
				b.WriteString(style.Render(fmt.Sprintf("%s%s : (none)", cursor, label)))
				b.WriteString("\n")
			} else {
				b.WriteString(style.Render(fmt.Sprintf("%s%s :", cursor, label)))
				b.WriteString("\n")
			}

			if m.inReposList {
				pad := strings.Repeat(" ", labelWidth+5)
				for j, repo := range m.repos {
					rc := "  "
					if j == m.reposCursor {
						rc = "> "
					}
					rs := lipgloss.NewStyle()
					if j == m.reposCursor {
						rs = rs.Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57"))
					}
					b.WriteString(pad + rs.Render(rc+repo) + "\n")
				}
				// [+ Add] button
				ac := "  "
				if m.reposCursor == len(m.repos) {
					ac = "> "
				}
				as := lipgloss.NewStyle()
				if m.reposCursor == len(m.repos) {
					as = as.Foreground(lipgloss.Color("229")).Background(lipgloss.Color("57"))
				}
				b.WriteString(pad + as.Render(ac+addStyle.Render("[+ Add]")) + "\n")
			} else if len(m.repos) > 0 {
				pad := strings.Repeat(" ", labelWidth+5)
				for _, repo := range m.repos {
					b.WriteString(pad + dimStyle.Render("  "+repo) + "\n")
				}
			}
			continue
		}

		label := fmt.Sprintf("%-*s", labelWidth, f.label)
		cursor := "  "
		if i == m.cursor && !m.inReposList {
			cursor = "> "
		}

		if m.editing && i == m.cursor {
			fmt.Fprintf(&b, "%s%s : %s\n", cursor, label, f.input.View())
		} else {
			value := f.input.Value()
			if f.key == "pat" || f.key == "llm.api_key" {
				value = maskPAT(value)
			}
			style := lipgloss.NewStyle()
			if i == m.cursor && !m.inReposList {
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

	if m.inReposList {
		b.WriteString(helpStyle.Render("\n  ↑↓: navigate  enter: browse  a: add  x: delete  esc: back"))
	} else if m.inProfilesList {
		b.WriteString(helpStyle.Render("\n  ↑↓: navigate  enter: activate  a: add  e: edit  x: delete (2×)  esc: back"))
	} else {
		b.WriteString(helpStyle.Render("\n  ↑↓: navigate  ←→/tab: switch tab  enter: edit  esc: back"))
	}
	return b.String()
}

func (m settingsModel) renderTabBar() string {
	active := lipgloss.NewStyle().
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(true).
		Padding(0, 2)
	idle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Padding(0, 2)

	var parts []string
	for i, t := range settingsTabs {
		if i == m.activeTab {
			parts = append(parts, active.Render(t.label))
		} else {
			parts = append(parts, idle.Render(t.label))
		}
	}
	return "  " + strings.Join(parts, " ")
}

func maskPAT(pat string) string {
	if len(pat) <= 8 {
		return "****"
	}
	return pat[:4] + strings.Repeat("*", len(pat)-8) + pat[len(pat)-4:]
}
