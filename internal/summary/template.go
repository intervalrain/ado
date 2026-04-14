package summary

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"strings"
	"text/template"
	"time"

	"github.com/rainhu/ado/internal/git"
)

//go:embed default_template.md
var defaultTemplate string

// TemplateData holds data passed to the report template.
type TemplateData struct {
	DateRange string
	Commits   []git.CommitLog
	WorkItems []WorkItemSummary
}

// WorkItemSummary is a simplified work item for template rendering.
type WorkItemSummary struct {
	ID         int
	Type       string
	Title      string
	State      string
	AssignedTo string
}

// ItemSuggestion represents an LLM-suggested action on a work item.
type ItemSuggestion struct {
	ID     int    `json:"id"`
	Action string `json:"action"`
	Reason string `json:"reason"`
}

// LoadSystemPrompt returns the system prompt (template content) and a source
// descriptor. If templatePath points to an existing file it is used; otherwise
// the embedded default_template.md is returned. The template may reference
// {{.DateRange}} but should not iterate over commits/work items — the data
// block is provided separately via BuildUserMessage.
func LoadSystemPrompt(templatePath string, dateRange string) (string, string, error) {
	tmplStr := defaultTemplate
	source := "default (embedded)"

	if templatePath != "" {
		content, err := os.ReadFile(templatePath)
		if err != nil {
			if !os.IsNotExist(err) {
				return "", "", fmt.Errorf("read template %s: %w", templatePath, err)
			}
			source = fmt.Sprintf("default (embedded; %s not found)", templatePath)
		} else {
			tmplStr = string(content)
			source = "file: " + templatePath
		}
	}

	tmpl, err := template.New("summary_system").Parse(tmplStr)
	if err != nil {
		return "", "", fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, map[string]string{"DateRange": dateRange}); err != nil {
		// If template has fields we didn't provide, fall back to raw content.
		return tmplStr, source, nil
	}

	header := "You MUST follow the template/format rules below exactly when producing the report.\n\n"
	return header + buf.String(), source, nil
}

// BuildUserMessage formats the commits and work items as the user-turn message.
func BuildUserMessage(data TemplateData) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Date range: %s\n\n", data.DateRange)

	fmt.Fprintf(&b, "## Git Commits (%d)\n", len(data.Commits))
	if len(data.Commits) == 0 {
		b.WriteString("(none)\n")
	} else {
		for _, c := range data.Commits {
			fmt.Fprintf(&b, "- [%s] %s %s (%s, %s)\n",
				c.Repo, c.Hash, c.Subject, c.Author, c.Date.Format("2006-01-02"))
		}
	}

	fmt.Fprintf(&b, "\n## Open Work Items (%d)\n", len(data.WorkItems))
	if len(data.WorkItems) == 0 {
		b.WriteString("(none)\n")
	} else {
		for _, w := range data.WorkItems {
			fmt.Fprintf(&b, "- #%d [%s] %s (%s) assigned to %s\n",
				w.ID, w.Type, w.Title, w.State, w.AssignedTo)
		}
	}

	b.WriteString("\nProduce the report now, following the format specified in the system prompt exactly.")
	return b.String()
}

// FormatDateRange returns a human-readable date range string.
func FormatDateRange(days int) string {
	end := time.Now()
	start := end.AddDate(0, 0, -days)
	return fmt.Sprintf("%s ~ %s", start.Format("2006-01-02"), end.Format("2006-01-02"))
}
