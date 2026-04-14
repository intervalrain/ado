package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/rainhu/ado/internal/api"
	configpkg "github.com/rainhu/ado/internal/config"
	"github.com/rainhu/ado/internal/git"
	"github.com/rainhu/ado/internal/llm"
	tmpl "github.com/rainhu/ado/internal/summary"
)

var (
	suggestionsPattern = regexp.MustCompile("(?s)```suggestions\\s*\\n(.+?)```")
	jsonUnmarshal      = json.Unmarshal
)

type summaryStep int

const (
	summaryStepCollecting summaryStep = iota
	summaryStepGenerating
	summaryStepViewing
	summaryStepActions
)

type summaryModel struct {
	client    *api.Client
	llmClient llm.Client
	cfg       *configpkg.Config

	step        summaryStep
	err         error
	commits     []git.CommitLog
	workItems   []tmpl.WorkItemSummary
	report      string
	suggestions []tmpl.ItemSuggestion
	selected    []bool // checkbox state for suggestions

	cursor     int
	scrollOff  int
	viewHeight int
}

// Messages for async operations
type commitsMsg struct {
	commits []git.CommitLog
	errs    []error
}
type workItemsMsg struct {
	items []tmpl.WorkItemSummary
}
type llmResultMsg struct {
	report      string
	suggestions []tmpl.ItemSuggestion
	usage       llm.Usage
}
type summaryErrMsg struct{ err error }
type resolveResultMsg struct {
	results []string
}

func newSummaryModel(client *api.Client, llmClient llm.Client, cfg *configpkg.Config) summaryModel {
	return summaryModel{
		client:     client,
		llmClient:  llmClient,
		cfg:        cfg,
		step:       summaryStepCollecting,
		viewHeight: 20,
	}
}

func (m summaryModel) init() tea.Cmd {
	return tea.Batch(m.fetchCommits(), m.fetchWorkItems())
}

func (m summaryModel) fetchCommits() tea.Cmd {
	return func() tea.Msg {
		commits, errs := git.CollectAllLogs(m.cfg.Summary.Repos, m.cfg.Summary.Days, m.cfg.Summary.Author)
		return commitsMsg{commits: commits, errs: errs}
	}
}

func (m summaryModel) fetchWorkItems() tea.Cmd {
	return func() tea.Msg {
		queryID := m.client.Config().QueryID
		if queryID == "" {
			return workItemsMsg{}
		}
		result, err := m.client.RunQuery(queryID)
		if err != nil {
			return workItemsMsg{}
		}
		var items []tmpl.WorkItemSummary
		for _, ref := range result.WorkItems {
			wi, err := m.client.GetWorkItem(ref.ID)
			if err != nil {
				continue
			}
			items = append(items, tmpl.WorkItemSummary{
				ID:         wi.ID,
				Type:       wi.Fields.WorkItemType,
				Title:      wi.Fields.Title,
				State:      wi.Fields.State,
				AssignedTo: wi.Fields.AssignedTo.DisplayName,
			})
		}
		return workItemsMsg{items: items}
	}
}

func (m summaryModel) callLLM() tea.Cmd {
	return func() tea.Msg {
		if m.llmClient == nil {
			return summaryErrMsg{err: fmt.Errorf("LLM not configured — set API key")}
		}

		data := tmpl.TemplateData{
			DateRange: tmpl.FormatDateRange(m.cfg.Summary.Days),
			Commits:   m.commits,
			WorkItems: m.workItems,
		}
		prompt, err := tmpl.RenderPrompt(m.cfg.Summary.Template, data)
		if err != nil {
			return summaryErrMsg{err: err}
		}

		resp, err := m.llmClient.Complete(context.Background(), []llm.Message{
			{Role: "user", Content: prompt},
		})
		if err != nil {
			return summaryErrMsg{err: err}
		}

		report, suggestions := parseResponseContent(resp.Content)
		return llmResultMsg{report: report, suggestions: suggestions, usage: resp.Usage}
	}
}

func (m summaryModel) resolveItems() tea.Cmd {
	return func() tea.Msg {
		var results []string
		for i, s := range m.suggestions {
			if !m.selected[i] {
				continue
			}
			state := "Closed"
			if s.Action == "resolve" {
				state = "Resolved"
			}
			err := m.client.UpdateWorkItemField(s.ID, "System.State", state)
			if err != nil {
				results = append(results, fmt.Sprintf("✗ #%d: %v", s.ID, err))
			} else {
				results = append(results, fmt.Sprintf("✓ #%d → %s", s.ID, state))
			}
		}
		return resolveResultMsg{results: results}
	}
}

// Track whether both async fetches completed
var commitsReady, itemsReady bool

func (m summaryModel) update(msg tea.Msg) (summaryModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.viewHeight = msg.Height - 6

	case commitsMsg:
		m.commits = msg.commits
		commitsReady = true
		if itemsReady {
			m.step = summaryStepGenerating
			return m, m.callLLM()
		}
		return m, nil

	case workItemsMsg:
		m.workItems = msg.items
		itemsReady = true
		if commitsReady {
			m.step = summaryStepGenerating
			return m, m.callLLM()
		}
		return m, nil

	case llmResultMsg:
		m.report = msg.report
		m.suggestions = msg.suggestions
		m.selected = make([]bool, len(msg.suggestions))
		// Pre-select all suggestions
		for i := range m.selected {
			m.selected[i] = true
		}
		m.step = summaryStepViewing
		if len(m.suggestions) > 0 {
			m.step = summaryStepActions
		}
		return m, nil

	case summaryErrMsg:
		m.err = msg.err
		m.step = summaryStepViewing
		return m, nil

	case resolveResultMsg:
		m.report += "\n\n--- Applied Actions ---\n" + strings.Join(msg.results, "\n")
		m.suggestions = nil
		m.step = summaryStepViewing
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m summaryModel) handleKey(msg tea.KeyMsg) (summaryModel, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	}

	switch m.step {
	case summaryStepViewing:
		switch msg.String() {
		case "up", "k":
			if m.scrollOff > 0 {
				m.scrollOff--
			}
		case "down", "j":
			lines := strings.Count(m.report, "\n")
			if m.scrollOff < lines-m.viewHeight+1 {
				m.scrollOff++
			}
		}

	case summaryStepActions:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.suggestions)-1 {
				m.cursor++
			}
		case " ": // toggle checkbox
			m.selected[m.cursor] = !m.selected[m.cursor]
		case "enter": // apply selected
			return m, m.resolveItems()
		case "tab": // switch to report view
			m.step = summaryStepViewing
		}
	}

	return m, nil
}

func (m summaryModel) view() string {
	var b strings.Builder

	title := titleStyle.Render("Summary Report")
	b.WriteString(title)
	b.WriteString("\n")

	switch m.step {
	case summaryStepCollecting:
		b.WriteString("\n  Collecting git commits and work items...\n")

	case summaryStepGenerating:
		b.WriteString(fmt.Sprintf("\n  Found %d commit(s), %d work item(s)\n", len(m.commits), len(m.workItems)))
		b.WriteString("  Generating summary with LLM...\n")

	case summaryStepViewing:
		if m.err != nil {
			b.WriteString(fmt.Sprintf("\n  Error: %v\n", m.err))
		} else {
			lines := strings.Split(m.report, "\n")
			end := m.scrollOff + m.viewHeight
			if end > len(lines) {
				end = len(lines)
			}
			start := m.scrollOff
			if start > len(lines) {
				start = len(lines)
			}
			for _, line := range lines[start:end] {
				b.WriteString("  " + line + "\n")
			}
			b.WriteString(helpStyle.Render("  ↑↓/jk: scroll  esc: back"))
		}

	case summaryStepActions:
		b.WriteString("\n  Suggested Actions (space: toggle, enter: apply, tab: view report)\n\n")
		for i, s := range m.suggestions {
			check := "[ ]"
			if m.selected[i] {
				check = "[x]"
			}
			line := fmt.Sprintf("  %s #%d %s: %s", check, s.ID, s.Action, s.Reason)
			if i == m.cursor {
				b.WriteString(selectedStyle.Render(line))
			} else {
				b.WriteString(itemStyle.Render(line))
			}
			b.WriteString("\n")
		}
		b.WriteString(helpStyle.Render("  space: toggle  enter: apply  tab: view report  esc: back"))
	}

	return b.String()
}

// parseResponseContent extracts report text and JSON suggestions from LLM response.
func parseResponseContent(content string) (string, []tmpl.ItemSuggestion) {
	matches := suggestionsPattern.FindStringSubmatch(content)
	if len(matches) < 2 {
		return content, nil
	}

	var suggestions []tmpl.ItemSuggestion
	if err := jsonUnmarshal([]byte(matches[1]), &suggestions); err != nil {
		return content, nil
	}

	report := suggestionsPattern.ReplaceAllString(content, "")
	return report, suggestions
}
