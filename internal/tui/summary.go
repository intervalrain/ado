package tui

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

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
	summaryStepSelectCommits
	summaryStepGenerating
	summaryStepActions
	summaryStepViewing
	summaryStepSavePrompt
	summaryStepSaved
)

type summaryModel struct {
	client    *api.Client
	llmClient llm.Client
	cfg       *configpkg.Config

	step           summaryStep
	err            error
	commits        []git.CommitLog
	commitSelected []bool
	workItems      []tmpl.WorkItemSummary
	report         string
	suggestions    []tmpl.ItemSuggestion
	selected       []bool // checkbox state for suggestions

	cursor     int
	scrollOff  int
	viewHeight int

	// Save state
	savePath    string
	saveMsg     string
	editingPath bool
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
type saveResultMsg struct {
	path string
	err  error
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

		// Only include selected commits
		var chosen []git.CommitLog
		for i, c := range m.commits {
			if i < len(m.commitSelected) && m.commitSelected[i] {
				chosen = append(chosen, c)
			}
		}

		data := tmpl.TemplateData{
			DateRange: tmpl.FormatDateRange(m.cfg.Summary.Days),
			Commits:   chosen,
			WorkItems: m.workItems,
		}
		system, _, err := tmpl.LoadSystemPrompt(m.cfg.Summary.Template, data.DateRange)
		if err != nil {
			return summaryErrMsg{err: err}
		}
		userMsg := tmpl.BuildUserMessage(data)

		resp, err := m.llmClient.Complete(context.Background(), system, []llm.Message{
			{Role: "user", Content: userMsg},
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

func (m summaryModel) saveReport() tea.Cmd {
	path := m.savePath
	content := m.report
	return func() tea.Msg {
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			return saveResultMsg{err: err}
		}
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			return saveResultMsg{err: err}
		}
		return saveResultMsg{path: path}
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
		m.commitSelected = make([]bool, len(msg.commits))
		for i := range m.commitSelected {
			m.commitSelected[i] = true // default all selected
		}
		commitsReady = true
		if itemsReady {
			m.step = summaryStepSelectCommits
			m.cursor = 0
		}
		return m, nil

	case workItemsMsg:
		m.workItems = msg.items
		itemsReady = true
		if commitsReady {
			m.step = summaryStepSelectCommits
			m.cursor = 0
		}
		return m, nil

	case llmResultMsg:
		m.report = msg.report
		m.suggestions = msg.suggestions
		m.selected = make([]bool, len(msg.suggestions))
		for i := range m.selected {
			m.selected[i] = true
		}
		m.cursor = 0
		if len(m.suggestions) > 0 {
			m.step = summaryStepActions
		} else {
			m.step = summaryStepViewing
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

	case saveResultMsg:
		if msg.err != nil {
			m.saveMsg = fmt.Sprintf("✗ save failed: %v", msg.err)
		} else {
			m.saveMsg = fmt.Sprintf("✓ saved to %s", msg.path)
			m.savePath = msg.path
		}
		m.step = summaryStepSaved
		return m, nil

	case tea.KeyMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m summaryModel) handleKey(msg tea.KeyMsg) (summaryModel, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}

	switch m.step {
	case summaryStepSelectCommits:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.commits)-1 {
				m.cursor++
			}
		case " ":
			if m.cursor < len(m.commitSelected) {
				m.commitSelected[m.cursor] = !m.commitSelected[m.cursor]
			}
		case "a":
			for i := range m.commitSelected {
				m.commitSelected[i] = true
			}
		case "n":
			for i := range m.commitSelected {
				m.commitSelected[i] = false
			}
		case "enter":
			m.step = summaryStepGenerating
			return m, m.callLLM()
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
		case " ":
			m.selected[m.cursor] = !m.selected[m.cursor]
		case "a":
			for i := range m.selected {
				m.selected[i] = true
			}
		case "n":
			for i := range m.selected {
				m.selected[i] = false
			}
		case "enter":
			return m, m.resolveItems()
		case "s":
			m.step = summaryStepViewing
		}

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
		case "s":
			m.savePath = defaultReportPath(m.cfg.Summary.Output)
			m.step = summaryStepSavePrompt
			m.editingPath = false
		}

	case summaryStepSaved:
		if msg.String() == "enter" && m.savePath != "" {
			_ = openInFileManager(filepath.Dir(m.savePath))
		}

	case summaryStepSavePrompt:
		if m.editingPath {
			switch msg.String() {
			case "enter":
				m.editingPath = false
			case "backspace":
				if len(m.savePath) > 0 {
					m.savePath = m.savePath[:len(m.savePath)-1]
				}
			case "esc":
				m.editingPath = false
			default:
				if len(msg.String()) == 1 {
					m.savePath += msg.String()
				}
			}
		} else {
			switch msg.String() {
			case "y", "enter":
				return m, m.saveReport()
			case "e":
				m.editingPath = true
			case "n", "esc":
				m.step = summaryStepViewing
			}
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

	case summaryStepSelectCommits:
		b.WriteString(fmt.Sprintf("\n  Select commits to include (%d found, %d work item(s) loaded)\n\n", len(m.commits), len(m.workItems)))
		start, end := visibleWindow(m.cursor, len(m.commits), m.viewHeight)
		for i := start; i < end; i++ {
			c := m.commits[i]
			check := "[ ]"
			if m.commitSelected[i] {
				check = "[x]"
			}
			line := fmt.Sprintf("  %s [%s] %s %s (%s, %s)", check, c.Repo, c.Hash, truncate(c.Subject, 60), c.Author, c.Date.Format("2006-01-02"))
			if i == m.cursor {
				b.WriteString(selectedStyle.Render(line))
			} else {
				b.WriteString(itemStyle.Render(line))
			}
			b.WriteString("\n")
		}
		b.WriteString(helpStyle.Render("  ↑↓/jk: navigate  space: toggle  a: all  n: none  enter: continue  esc: back"))

	case summaryStepGenerating:
		selectedCount := 0
		for _, v := range m.commitSelected {
			if v {
				selectedCount++
			}
		}
		b.WriteString(fmt.Sprintf("\n  Using %d commit(s), %d work item(s)\n", selectedCount, len(m.workItems)))
		b.WriteString("  Generating summary with LLM...\n")

	case summaryStepActions:
		b.WriteString(fmt.Sprintf("\n  ADO Work Items — suggested state changes (%d)\n\n", len(m.suggestions)))
		for i, s := range m.suggestions {
			check := "[ ]"
			if m.selected[i] {
				check = "[x]"
			}
			// Find the matching work item title
			title := ""
			for _, wi := range m.workItems {
				if wi.ID == s.ID {
					title = truncate(wi.Title, 50)
					break
				}
			}
			line := fmt.Sprintf("  %s #%d %s → %s  %s", check, s.ID, title, strings.ToUpper(s.Action), truncate(s.Reason, 60))
			if i == m.cursor {
				b.WriteString(selectedStyle.Render(line))
			} else {
				b.WriteString(itemStyle.Render(line))
			}
			b.WriteString("\n")
		}
		b.WriteString(helpStyle.Render("  space: toggle  a: all  n: none  enter: apply  s: skip to report  esc: back"))

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
			b.WriteString(helpStyle.Render("  ↑↓/jk: scroll  s: save report  esc: back"))
		}

	case summaryStepSavePrompt:
		b.WriteString("\n  Save report?\n\n")
		if m.editingPath {
			b.WriteString(fmt.Sprintf("  Path: %s_\n\n", m.savePath))
			b.WriteString(helpStyle.Render("  type to edit  enter: confirm  esc: cancel"))
		} else {
			b.WriteString(fmt.Sprintf("  Path: %s\n\n", m.savePath))
			b.WriteString(helpStyle.Render("  y/enter: save  e: edit path  n/esc: cancel"))
		}

	case summaryStepSaved:
		b.WriteString("\n  " + m.saveMsg + "\n\n")
		b.WriteString(helpStyle.Render("  enter: open folder  esc: back to menu"))
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

// defaultReportPath builds a default save path with a date suffix.
// If output is a directory (no file extension), the file is named
// summary-YYYY-MM-DD.md inside it. If output is a file path, the date is
// inserted as a suffix before the extension (e.g. report.md -> report-YYYY-MM-DD.md).
func defaultReportPath(output string) string {
	date := time.Now().Format("2006-01-02")
	if output == "" {
		return fmt.Sprintf("summary-%s.md", date)
	}
	if ext := filepath.Ext(output); ext != "" {
		base := strings.TrimSuffix(output, ext)
		return fmt.Sprintf("%s-%s%s", base, date, ext)
	}
	return filepath.Join(output, fmt.Sprintf("summary-%s.md", date))
}

// openInFileManager opens the given path in the OS file manager (non-blocking).
func openInFileManager(path string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("explorer", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	return cmd.Start()
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

func visibleWindow(cursor, total, height int) (int, int) {
	if height <= 0 {
		height = 10
	}
	if total <= height {
		return 0, total
	}
	start := cursor - height/2
	if start < 0 {
		start = 0
	}
	end := start + height
	if end > total {
		end = total
		start = end - height
	}
	return start, end
}
