package summary

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"regexp"

	"github.com/rainhu/ado/internal/api"
	configpkg "github.com/rainhu/ado/internal/config"
	"github.com/rainhu/ado/internal/cqrs"
	"github.com/rainhu/ado/internal/git"
	"github.com/rainhu/ado/internal/llm"
	tmpl "github.com/rainhu/ado/internal/summary"
)

const GenerateRequestName = "GenerateSummary"

type GenerateSummaryRequest struct {
	Days     int
	Repos    []string
	Template string
	Author   string
}

func (r *GenerateSummaryRequest) RequestName() string { return GenerateRequestName }

// GenerateSummaryResult holds the output so callers (TUI) can access structured data.
type GenerateSummaryResult struct {
	Report      string
	Suggestions []tmpl.ItemSuggestion
}

type GenerateSummaryHandler struct {
	client    *api.Client
	llmClient llm.Client
	cfg       *configpkg.Config
}

func NewGenerateSummaryHandler(client *api.Client, llmClient llm.Client, cfg *configpkg.Config) *GenerateSummaryHandler {
	return &GenerateSummaryHandler{client: client, llmClient: llmClient, cfg: cfg}
}

func (h *GenerateSummaryHandler) Handle(ctx context.Context, req cqrs.Request, w io.Writer) error {
	r := req.(*GenerateSummaryRequest)

	if h.llmClient == nil {
		return fmt.Errorf("LLM is not configured — set api_key_env in ~/.ado/config.yaml or ANTHROPIC_API_KEY env var")
	}

	// Resolve parameters with defaults from config
	days := r.Days
	if days == 0 {
		days = h.cfg.Summary.Days
	}
	repos := r.Repos
	if len(repos) == 0 {
		repos = h.cfg.Summary.Repos
	}
	templatePath := r.Template
	if templatePath == "" {
		templatePath = h.cfg.Summary.Template
	}
	author := r.Author
	if author == "" {
		author = h.cfg.Summary.Author
	}

	// Step 1: Collect git logs
	fmt.Fprintf(w, "Collecting git commits from %d repo(s) for the past %d days...\n", len(repos), days)
	commits, errs := git.CollectAllLogs(repos, days, author)
	for _, err := range errs {
		fmt.Fprintf(w, "  warning: %v\n", err)
	}
	fmt.Fprintf(w, "  found %d commit(s)\n", len(commits))

	// Step 2: Fetch ADO work items
	fmt.Fprintf(w, "Fetching ADO work items...\n")
	workItems, err := h.fetchWorkItems()
	if err != nil {
		fmt.Fprintf(w, "  warning: could not fetch work items: %v\n", err)
	} else {
		fmt.Fprintf(w, "  found %d work item(s)\n", len(workItems))
	}

	if len(commits) == 0 && len(workItems) == 0 {
		fmt.Fprintf(w, "\nNo commits or work items found. Nothing to summarize.\n")
		return nil
	}

	// Step 3: Render template
	data := tmpl.TemplateData{
		DateRange: tmpl.FormatDateRange(days),
		Commits:   commits,
		WorkItems: workItems,
	}
	prompt, err := tmpl.RenderPrompt(templatePath, data)
	if err != nil {
		return fmt.Errorf("render template: %w", err)
	}

	// Step 4: Call LLM
	fmt.Fprintf(w, "Generating summary with LLM...\n\n")
	resp, err := h.llmClient.Complete(ctx, []llm.Message{
		{Role: "user", Content: prompt},
	})
	if err != nil {
		return fmt.Errorf("LLM call: %w", err)
	}

	// Step 5: Parse and output
	report, suggestions := parseResponse(resp.Content)
	fmt.Fprintln(w, report)

	if len(suggestions) > 0 {
		fmt.Fprintf(w, "\n--- Suggested Actions ---\n")
		for _, s := range suggestions {
			fmt.Fprintf(w, "  #%d → %s: %s\n", s.ID, s.Action, s.Reason)
		}
	}

	fmt.Fprintf(w, "\n(tokens: %d in / %d out)\n", resp.Usage.InputTokens, resp.Usage.OutputTokens)
	return nil
}

func (h *GenerateSummaryHandler) fetchWorkItems() ([]tmpl.WorkItemSummary, error) {
	queryID := h.client.Config().QueryID
	if queryID == "" {
		return nil, nil
	}

	result, err := h.client.RunQuery(queryID)
	if err != nil {
		return nil, err
	}

	var items []tmpl.WorkItemSummary
	for _, ref := range result.WorkItems {
		wi, err := h.client.GetWorkItem(ref.ID)
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
	return items, nil
}

var suggestionsRe = regexp.MustCompile("(?s)```suggestions\\s*\\n(.+?)```")

func parseResponse(content string) (string, []tmpl.ItemSuggestion) {
	matches := suggestionsRe.FindStringSubmatch(content)
	if len(matches) < 2 {
		return content, nil
	}

	var suggestions []tmpl.ItemSuggestion
	if err := json.Unmarshal([]byte(matches[1]), &suggestions); err != nil {
		return content, nil
	}

	// Remove the suggestions block from the report
	report := suggestionsRe.ReplaceAllString(content, "")
	return report, suggestions
}
