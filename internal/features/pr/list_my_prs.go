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

// PRCategory selects which set of pull requests to list.
type PRCategory int

const (
	// PRRequired lists PRs where the current user is a required reviewer (default).
	PRRequired PRCategory = iota
	// PRAssigned lists PRs where the current user is any reviewer.
	PRAssigned
	// PRCreated lists PRs created by the current user.
	PRCreated
	// PRRepo lists active PRs in a specific repository.
	PRRepo
)

type ListMyPRsRequest struct {
	Category PRCategory
	RepoName string // used when Category == PRRepo
}

func (r *ListMyPRsRequest) RequestName() string { return ListRequestName }

type ListMyPRsHandler struct {
	client *api.Client
}

func NewListMyPRsHandler(client *api.Client) *ListMyPRsHandler {
	return &ListMyPRsHandler{client: client}
}

// fetch returns the PRs for the request's category, plus a human-readable
// description of the set for the header line.
func (h *ListMyPRsHandler) fetch(req *ListMyPRsRequest) ([]api.PullRequest, string, error) {
	switch req.Category {
	case PRAssigned:
		prs, err := h.client.ListMyReviewPullRequests()
		return prs, "assigned to you for review", err
	case PRCreated:
		prs, err := h.client.ListMyCreatedPullRequests()
		return prs, "created by you", err
	case PRRepo:
		repoID, repoName, err := h.resolveRepo(req.RepoName)
		if err != nil {
			return nil, "", err
		}
		prs, err := h.client.ListPullRequests(repoID)
		return prs, fmt.Sprintf("in repo %q", repoName), err
	default:
		prs, err := h.client.ListMyPullRequests()
		return prs, "where you are a required reviewer", err
	}
}

// resolveRepo matches a repo name (case-insensitive exact, then substring)
// against the project's repositories.
func (h *ListMyPRsHandler) resolveRepo(name string) (id, resolved string, err error) {
	repos, err := h.client.ListRepositories()
	if err != nil {
		return "", "", fmt.Errorf("list repositories: %w", err)
	}
	q := strings.TrimSpace(name)
	if q == "" {
		return "", "", fmt.Errorf("--repo requires a repository name")
	}
	var matches []api.Repository
	for _, r := range repos {
		if strings.EqualFold(r.Name, q) {
			return r.ID, r.Name, nil
		}
	}
	ql := strings.ToLower(q)
	for _, r := range repos {
		if strings.Contains(strings.ToLower(r.Name), ql) {
			matches = append(matches, r)
		}
	}
	switch len(matches) {
	case 0:
		return "", "", fmt.Errorf("no repository matching %q", q)
	case 1:
		return matches[0].ID, matches[0].Name, nil
	default:
		var names []string
		for _, r := range matches {
			names = append(names, r.Name)
		}
		return "", "", fmt.Errorf("%q is ambiguous, matches: %s", q, strings.Join(names, ", "))
	}
}

type prRow struct {
	id, title, branch, creator, repo, required, optional string
}

func (r prRow) columns() []string {
	return []string{r.id, r.title, r.branch, r.creator, r.repo, r.required, r.optional}
}

func (h *ListMyPRsHandler) Handle(ctx context.Context, req cqrs.Request, w io.Writer) error {
	r := req.(*ListMyPRsRequest)

	prs, desc, err := h.fetch(r)
	if err != nil {
		return fmt.Errorf("list PRs: %w", err)
	}

	if len(prs) == 0 {
		fmt.Fprintf(w, "No active pull requests %s.\n", desc)
		return nil
	}

	fmt.Fprintf(w, "Found %d active pull request(s) %s:\n\n", len(prs), desc)

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
