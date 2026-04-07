package pr

import (
	"context"
	"fmt"
	"io"
	"strings"
	"github.com/rainhu/ado/internal/api"
	"github.com/rainhu/ado/internal/cqrs"
	"github.com/rainhu/ado/internal/util"
)

const ListRequestName = "ListMyPRs"

type ListMyPRsRequest struct{}

func (r *ListMyPRsRequest) RequestName() string { return ListRequestName }

type ListMyPRsHandler struct {
	client *api.Client
}

func NewListMyPRsHandler(client *api.Client) *ListMyPRsHandler {
	return &ListMyPRsHandler{client: client}
}

type prRow struct {
	id, title, branch, creator, repo, required, optional string
}

func (r prRow) columns() []string {
	return []string{r.id, r.title, r.branch, r.creator, r.repo, r.required, r.optional}
}

func (h *ListMyPRsHandler) Handle(ctx context.Context, req cqrs.Request, w io.Writer) error {
	prs, err := h.client.ListMyPullRequests()
	if err != nil {
		return fmt.Errorf("list my PRs: %w", err)
	}

	if len(prs) == 0 {
		fmt.Fprintln(w, "No active pull requests assigned to you.")
		return nil
	}

	fmt.Fprintf(w, "Found %d active pull request(s) assigned to you:\n\n", len(prs))

	headers := prRow{"ID", "Title", "Branch", "Creator", "Repo", "Required", "Optional"}
	rows := []prRow{headers}

	for _, pr := range prs {
		src := strings.TrimPrefix(pr.SourceRefName, "refs/heads/")
		tgt := strings.TrimPrefix(pr.TargetRefName, "refs/heads/")

		req, opt := splitReviewers(pr.Reviewers)
		rows = append(rows, prRow{
			id:       fmt.Sprintf("#%d", pr.ID),
			title:    pr.Title,
			branch:   fmt.Sprintf("%s → %s", src, tgt),
			creator:  pr.CreatedBy.DisplayName,
			repo:     pr.Repository.Name,
			required: req,
			optional: opt,
		})
	}

	// Calculate max display width per column
	colCount := 7
	widths := make([]int, colCount)
	for _, r := range rows {
		for i, c := range r.columns() {
			dw := util.DisplayWidth(c)
			if dw > widths[i] {
				widths[i] = dw
			}
		}
	}

	// Print header
	printRow(w, widths, rows[0])

	// Print separator
	var sep strings.Builder
	for i, width := range widths {
		if i > 0 {
			sep.WriteString("  ")
		}
		sep.WriteString(strings.Repeat("-", width))
	}
	sep.WriteString("\n")
	fmt.Fprint(w, sep.String())

	// Print data rows
	for _, r := range rows[1:] {
		printRow(w, widths, r)
	}

	return nil
}

func printRow(w io.Writer, widths []int, r prRow) {
	cols := r.columns()
	for i, c := range cols {
		if i > 0 {
			fmt.Fprint(w, "  ")
		}
		fmt.Fprint(w, util.PadRight(c, widths[i]))
	}
	fmt.Fprint(w, "\n")
}

func splitReviewers(reviewers []api.PRReviewer) (required, optional string) {
	var req, opt []string
	for _, r := range reviewers {
		label := fmt.Sprintf("%s:%s", r.DisplayName, shortVote(r.Vote))
		if r.IsRequired {
			req = append(req, label)
		} else {
			opt = append(opt, label)
		}
	}
	return strings.Join(req, " "), strings.Join(opt, " ")
}

func reviewSummary(reviewers []api.PRReviewer) string {
	if len(reviewers) == 0 {
		return "no reviewers"
	}
	var parts []string
	for _, r := range reviewers {
		parts = append(parts, fmt.Sprintf("%s:%s", r.DisplayName, shortVote(r.Vote)))
	}
	return strings.Join(parts, " ")
}

func shortVote(vote int) string {
	switch {
	case vote >= 5:
		return "✓"
	case vote == -5:
		return "⏳"
	case vote == -10:
		return "✗"
	default:
		return "○"
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
