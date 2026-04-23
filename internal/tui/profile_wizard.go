package tui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rainhu/ado/internal/config"
	"github.com/rainhu/ado/internal/llm"
)

type profileWizardStep int

const (
	pwStepName profileWizardStep = iota
	pwStepProvider
	pwStepModel
	pwStepCredential // API Key for claude/openai/gemini; Base URL for ollama
	pwStepMaxTokens
	pwStepDescription
	pwStepConfirm
)

type profileWizardMode int

const (
	pwModeAdd profileWizardMode = iota
	pwModeEdit
)

type testState int

const (
	testIdle testState = iota
	testRunning
	testFailed
)

// Wizard outcomes — sent to the settings model when the wizard closes.
type profileWizardDoneMsg struct {
	name    string
	action  string // "added" | "updated" | "cancelled"
	current bool   // true if profile should become the active one
}

// Internal messages for the async connection test.
type testOkMsg struct{}
type testFailedMsg struct{ err error }

type profileWizard struct {
	mode         profileWizardMode
	step         profileWizardStep
	originalName string // for edit mode

	name    textinput.Model
	prov    textinput.Model
	model   textinput.Model
	apiKey  textinput.Model // used for claude/openai/gemini
	baseURL textinput.Model // used for ollama
	maxTok  textinput.Model
	desc    textinput.Model

	test   testState
	errMsg string
}

// isOllama reports whether the currently-selected provider is ollama.
func (w profileWizard) isOllama() bool {
	return strings.TrimSpace(w.prov.Value()) == "ollama"
}

// credentialInput returns the textinput shown at pwStepCredential for the
// currently-selected provider.
func (w *profileWizard) credentialInput() *textinput.Model {
	if w.isOllama() {
		return &w.baseURL
	}
	return &w.apiKey
}

func newProfileWizard(mode profileWizardMode, existing *config.ModelProfile) profileWizard {
	mk := func(placeholder string, width int) textinput.Model {
		ti := textinput.New()
		ti.Placeholder = placeholder
		ti.CharLimit = 256
		ti.Width = width
		return ti
	}

	apiKey := mk("paste your API key", 60)
	apiKey.EchoMode = textinput.EchoPassword
	apiKey.EchoCharacter = '•'

	w := profileWizard{
		mode:    mode,
		name:    mk("e.g. sonnet / gpt4 / gemini-flash / local", 40),
		prov:    mk("claude | openai | gemini | ollama", 40),
		model:   mk("model id", 50),
		apiKey:  apiKey,
		baseURL: mk("http://localhost:11434", 60),
		maxTok:  mk("4096", 10),
		desc:    mk("optional", 60),
	}
	if existing != nil {
		w.originalName = existing.Name
		w.name.SetValue(existing.Name)
		w.prov.SetValue(existing.Provider)
		w.model.SetValue(existing.Model)
		w.apiKey.SetValue(existing.APIKey)
		w.baseURL.SetValue(existing.BaseURL)
		if existing.MaxTokens > 0 {
			w.maxTok.SetValue(strconv.Itoa(existing.MaxTokens))
		}
		w.desc.SetValue(existing.Description)
	}
	w.focusStep()
	return w
}

func (w *profileWizard) focusStep() {
	for _, ti := range []*textinput.Model{&w.name, &w.prov, &w.model, &w.apiKey, &w.baseURL, &w.maxTok, &w.desc} {
		ti.Blur()
	}
	switch w.step {
	case pwStepName:
		w.name.Focus()
	case pwStepProvider:
		w.prov.Focus()
	case pwStepModel:
		w.model.Focus()
	case pwStepCredential:
		w.credentialInput().Focus()
	case pwStepMaxTokens:
		w.maxTok.Focus()
	case pwStepDescription:
		w.desc.Focus()
	}
}

// blinkCmd returns the cursor blink command for the currently-focused input,
// or nil when the wizard is past the input steps (Confirm).
func (w profileWizard) blinkCmd() tea.Cmd {
	switch w.step {
	case pwStepName:
		return w.name.Cursor.BlinkCmd()
	case pwStepProvider:
		return w.prov.Cursor.BlinkCmd()
	case pwStepModel:
		return w.model.Cursor.BlinkCmd()
	case pwStepCredential:
		if w.isOllama() {
			return w.baseURL.Cursor.BlinkCmd()
		}
		return w.apiKey.Cursor.BlinkCmd()
	case pwStepMaxTokens:
		return w.maxTok.Cursor.BlinkCmd()
	case pwStepDescription:
		return w.desc.Cursor.BlinkCmd()
	}
	return nil
}

// buildProfile assembles the in-progress profile (may have empty values).
func (w profileWizard) buildProfile() *config.ModelProfile {
	mt, _ := strconv.Atoi(strings.TrimSpace(w.maxTok.Value()))
	// ollama uses Base URL only; other providers use API Key only.
	var apiKey, baseURL string
	if w.isOllama() {
		baseURL = strings.TrimSpace(w.baseURL.Value())
	} else {
		apiKey = strings.TrimSpace(w.apiKey.Value())
	}
	return &config.ModelProfile{
		Name:        strings.TrimSpace(w.name.Value()),
		Provider:    strings.TrimSpace(w.prov.Value()),
		Model:       strings.TrimSpace(w.model.Value()),
		APIKey:      apiKey,
		BaseURL:     baseURL,
		MaxTokens:   mt,
		Description: strings.TrimSpace(w.desc.Value()),
	}
}

// validateStep returns an error if the value at the current step is invalid.
func (w profileWizard) validateStep() error {
	switch w.step {
	case pwStepName:
		v := strings.TrimSpace(w.name.Value())
		if v == "" {
			return fmt.Errorf("name is required")
		}
		// Filename-safe: letters, digits, and `. - _ :` (colon is common in
		// ollama-style tags like "qwen2.5:7b").
		for _, r := range v {
			if !((r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' || r == ':') {
				return fmt.Errorf("name may only contain letters, digits, . : - _")
			}
		}
		// For add mode, reject names that already exist.
		if w.mode == pwModeAdd {
			if _, err := config.LoadModelProfile(v); err == nil {
				return fmt.Errorf("profile %q already exists", v)
			}
		}
	case pwStepProvider:
		v := strings.TrimSpace(w.prov.Value())
		switch v {
		case "claude", "openai", "gemini", "ollama":
		default:
			return fmt.Errorf("provider must be one of: claude, openai, gemini, ollama")
		}
	case pwStepModel:
		if strings.TrimSpace(w.model.Value()) == "" {
			return fmt.Errorf("model is required")
		}
	case pwStepCredential:
		if w.isOllama() {
			// Base URL optional — default http://localhost:11434 is used when empty.
		} else if strings.TrimSpace(w.apiKey.Value()) == "" {
			return fmt.Errorf("API key is required for provider %q", strings.TrimSpace(w.prov.Value()))
		}
	case pwStepMaxTokens:
		v := strings.TrimSpace(w.maxTok.Value())
		if v != "" {
			if _, err := strconv.Atoi(v); err != nil {
				return fmt.Errorf("max_tokens must be a number")
			}
		}
	}
	return nil
}

func (w profileWizard) update(msg tea.Msg) (profileWizard, tea.Cmd) {
	switch msg := msg.(type) {
	case testOkMsg:
		w.test = testIdle
		// Save the profile — validation was done on the way in.
		p := w.buildProfile()
		// On edit with a renamed profile, remove the old file.
		if w.mode == pwModeEdit && w.originalName != "" && w.originalName != p.Name {
			_ = config.RemoveModelProfile(w.originalName)
		}
		if err := config.SaveModelProfile(p); err != nil {
			w.test = testFailed
			w.errMsg = err.Error()
			return w, nil
		}
		action := "added"
		if w.mode == pwModeEdit {
			action = "updated"
		}
		return w, func() tea.Msg {
			return profileWizardDoneMsg{name: p.Name, action: action, current: true}
		}
	case testFailedMsg:
		w.test = testFailed
		w.errMsg = msg.err.Error()
		return w, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			// step back; at first step, cancel wizard
			if w.test == testRunning {
				return w, nil
			}
			if w.step == pwStepName {
				return w, func() tea.Msg {
					return profileWizardDoneMsg{action: "cancelled"}
				}
			}
			w.test = testIdle
			w.errMsg = ""
			w.step--
			w.focusStep()
			return w, w.blinkCmd()
		case "enter":
			if w.test == testRunning {
				return w, nil
			}
			if w.step == pwStepConfirm {
				if err := w.validateAll(); err != nil {
					w.test = testFailed
					w.errMsg = err.Error()
					return w, nil
				}
				w.test = testRunning
				w.errMsg = ""
				return w, runConnectionTest(w.buildProfile())
			}
			if err := w.validateStep(); err != nil {
				w.test = testFailed
				w.errMsg = err.Error()
				return w, nil
			}
			w.test = testIdle
			w.errMsg = ""
			w.step++
			w.focusStep()
			return w, w.blinkCmd()
		}
	}

	// Forward key events to the focused input for the current step.
	var cmd tea.Cmd
	switch w.step {
	case pwStepName:
		w.name, cmd = w.name.Update(msg)
	case pwStepProvider:
		w.prov, cmd = w.prov.Update(msg)
	case pwStepModel:
		w.model, cmd = w.model.Update(msg)
	case pwStepCredential:
		if w.isOllama() {
			w.baseURL, cmd = w.baseURL.Update(msg)
		} else {
			w.apiKey, cmd = w.apiKey.Update(msg)
		}
	case pwStepMaxTokens:
		w.maxTok, cmd = w.maxTok.Update(msg)
	case pwStepDescription:
		w.desc, cmd = w.desc.Update(msg)
	}
	return w, cmd
}

func (w profileWizard) validateAll() error {
	// w is a value receiver, so mutating w.step here is local.
	for s := pwStepName; s <= pwStepDescription; s++ {
		w.step = s
		if err := w.validateStep(); err != nil {
			return err
		}
	}
	return nil
}

// runConnectionTest builds an llm.Config from the profile and fires a minimal
// probe. Emits testOkMsg or testFailedMsg.
func runConnectionTest(p *config.ModelProfile) tea.Cmd {
	return func() tea.Msg {
		err := llm.TestConnection(llm.Config{
			Provider:  p.Provider,
			Model:     p.Model,
			APIKey:    p.APIKey,
			BaseURL:   p.BaseURL,
			MaxTokens: p.MaxTokens,
		})
		if err != nil {
			return testFailedMsg{err: err}
		}
		return testOkMsg{}
	}
}

var (
	pwHeader = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("99"))
	pwLabel  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214"))
	pwHint   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	pwErr    = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	pwRun    = lipgloss.NewStyle().Foreground(lipgloss.Color("228"))
)

func (w profileWizard) view() string {
	var b strings.Builder

	title := "Add Model Profile"
	if w.mode == pwModeEdit {
		title = fmt.Sprintf("Edit Model Profile · %s", w.originalName)
	}
	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	// progress breadcrumb — the credential step's label depends on provider
	credLabel := "API Key"
	if w.isOllama() {
		credLabel = "Base URL"
	}
	steps := []string{"Name", "Provider", "Model", credLabel, "Max Tokens", "Description", "Test & Save"}
	crumbs := make([]string, len(steps))
	for i, s := range steps {
		if i == int(w.step) {
			crumbs[i] = pwLabel.Render(s)
		} else if i < int(w.step) {
			crumbs[i] = pwHint.Render(s + " ✓")
		} else {
			crumbs[i] = pwHint.Render(s)
		}
	}
	b.WriteString("  " + strings.Join(crumbs, pwHint.Render(" › ")))
	b.WriteString("\n\n")

	switch w.step {
	case pwStepName:
		writeField(&b, "Name", w.name.View(), "letters, digits, . : - _ only")
	case pwStepProvider:
		writeField(&b, "Provider", w.prov.View(), "one of: claude, openai, gemini, ollama")
	case pwStepModel:
		writeField(&b, "Model", w.model.View(), "e.g. claude-sonnet-4-20250514, gpt-4o-mini, gemini-2.5-flash, llama3.2")
	case pwStepCredential:
		if w.isOllama() {
			writeField(&b, "Base URL", w.baseURL.View(), "leave empty to use http://localhost:11434")
		} else {
			writeField(&b, "API Key", w.apiKey.View(), "stored in plaintext at ~/.ado/models/<name>.yaml")
		}
	case pwStepMaxTokens:
		writeField(&b, "Max Tokens", w.maxTok.View(), "leave empty to use default (4096)")
	case pwStepDescription:
		writeField(&b, "Description", w.desc.View(), "optional reminder shown in the list")
	case pwStepConfirm:
		p := w.buildProfile()
		b.WriteString(pwHeader.Render("  Review"))
		b.WriteString("\n\n")
		kv := func(k, v string) {
			if v == "" {
				v = pwHint.Render("(empty)")
			}
			fmt.Fprintf(&b, "    %-14s %s\n", pwLabel.Render(k), v)
		}
		kv("Name", p.Name)
		kv("Provider", p.Provider)
		kv("Model", p.Model)
		if w.isOllama() {
			baseURL := p.BaseURL
			if baseURL == "" {
				baseURL = "http://localhost:11434 (default)"
			}
			kv("Base URL", baseURL)
		} else {
			kv("API Key", maskAPIKey(p.APIKey))
		}
		if p.MaxTokens > 0 {
			kv("Max Tokens", strconv.Itoa(p.MaxTokens))
		} else {
			kv("Max Tokens", "")
		}
		kv("Description", p.Description)
		b.WriteString("\n")
		switch w.test {
		case testRunning:
			b.WriteString("  " + pwRun.Render("⋯ Testing connection…"))
		case testFailed:
			b.WriteString("  " + pwErr.Render("✗ "+w.errMsg))
		default:
			b.WriteString("  " + pwHint.Render("Press Enter to test connection and save."))
		}
		b.WriteString("\n")
	}

	// Step-level error (non-confirm steps)
	if w.step != pwStepConfirm && w.test == testFailed && w.errMsg != "" {
		b.WriteString("\n  " + pwErr.Render("✗ "+w.errMsg) + "\n")
	}

	b.WriteString(helpStyle.Render("\n  enter: next  esc: back  (esc at first step cancels)"))
	return b.String()
}

func maskAPIKey(key string) string {
	n := len(key)
	if n == 0 {
		return ""
	}
	if n <= 8 {
		return strings.Repeat("•", n)
	}
	return key[:4] + strings.Repeat("•", n-8) + key[n-4:]
}

func writeField(b *strings.Builder, label, view, hint string) {
	b.WriteString(pwLabel.Render("  "+label) + "\n")
	b.WriteString("  " + view + "\n")
	if hint != "" {
		b.WriteString("  " + pwHint.Render(hint) + "\n")
	}
}
