package summary

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
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

// RenderPrompt renders the template with the given data and returns the LLM prompt.
func RenderPrompt(templatePath string, data TemplateData) (string, error) {
	tmplStr := defaultTemplate

	if templatePath != "" {
		content, err := os.ReadFile(templatePath)
		if err != nil {
			if !os.IsNotExist(err) {
				return "", fmt.Errorf("read template %s: %w", templatePath, err)
			}
			// File doesn't exist — fall through to default
		} else {
			tmplStr = string(content)
		}
	}

	tmpl, err := template.New("summary").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}

	return buf.String(), nil
}

// FormatDateRange returns a human-readable date range string.
func FormatDateRange(days int) string {
	end := time.Now()
	start := end.AddDate(0, 0, -days)
	return fmt.Sprintf("%s ~ %s", start.Format("2006-01-02"), end.Format("2006-01-02"))
}
