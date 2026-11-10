package pr

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/rainhu/ado/internal/api"
	"github.com/rainhu/ado/internal/cqrs"
)

const ShowRequestName = "ShowPR"

// ShowPRRequest displays the full details of one pull request. The PR is looked
// up by ID across the whole project, so it does not need to be run from the
// PR's repository.
type ShowPRRequest struct {
	PRID int
	JSON bool
}

func (r *ShowPRRequest) RequestName() string { return ShowRequestName }

type ShowPRHandler struct {
	client *api.Client
}

func NewShowPRHandler(client *api.Client) *ShowPRHandler {
	return &ShowPRHandler{client: client}
}

// prDetail is everything the show view renders: the PR itself plus the data
// that needs extra calls to collect.
type prDetail struct {
	PR        *api.PullRequest
	WorkItems []string
	URL       string
}

func (h *ShowPRHandler) Handle(ctx context.Context, req cqrs.Request, w io.Writer) error {
	r := req.(*ShowPRRequest)
	if r.PRID <= 0 {
		return fmt.Errorf("invalid PR id %d", r.PRID)
	}

	p, err := h.client.GetPullRequest(r.PRID)
	if err != nil {
		return fmt.Errorf("get PR #%d: %w", r.PRID, err)
	}

	d := prDetail{PR: p, URL: h.client.PRWebURL(p.Repository.Name, p.ID)}

	// Linked work items are secondary information: show the PR even if this
	// lookup fails, and say so rather than silently printing nothing.
	ids, wiErr := h.client.ListPRWorkItemIDs(p.Repository.ID, p.ID)
	if wiErr == nil {
		d.WorkItems = ids
	}

	if r.JSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(toJSONPR(d))
	}

	printPRDetail(w, d)
	if wiErr != nil {
		fmt.Fprintf(w, "\n(could not load linked work items: %v)\n", wiErr)
	}
	return nil
}

func printPRDetail(w io.Writer, d prDetail) {
	p := d.PR
	fmt.Fprintf(w, "PR #%d: %s\n", p.ID, p.Title)

	field := func(label, value string) {
		if value != "" {
			fmt.Fprintf(w, "%-12s %s\n", label+":", value)
		}
	}

	status := p.Status
	if p.IsDraft {
		status += " (draft)"
	}
	field("Status", status)
	field("Repo", p.Repository.Name)
	field("Author", p.CreatedBy.DisplayName)
	field("Created", formatDate(p.CreationDate))
	field("Closed", formatDate(p.ClosedDate))
	field("Branch", fmt.Sprintf("%s -> %s", shortRef(p.SourceRefName), shortRef(p.TargetRefName)))
	field("Merge", p.MergeStatus)
	if p.AutoCompleteSetBy != nil {
		field("Auto-comp.", "set by "+p.AutoCompleteSetBy.DisplayName)
	}
	if len(d.WorkItems) > 0 {
		refs := make([]string, len(d.WorkItems))
		for i, id := range d.WorkItems {
			refs[i] = "#" + id
		}
		field("Work items", strings.Join(refs, ", "))
	}

	if len(p.Reviewers) > 0 {
		nameW := 0
		for _, r := range p.Reviewers {
			nameW = max(nameW, len(r.DisplayName))
		}
		fmt.Fprint(w, "\nReviewers:\n")
		for _, r := range p.Reviewers {
			fmt.Fprintf(w, "  %-*s  %-8s  %s\n", nameW, r.DisplayName, r.RequiredLabel(), r.VoteLabel())
		}
	}

	if desc := strings.TrimSpace(p.Description); desc != "" {
		fmt.Fprint(w, "\nDescription:\n")
		for line := range strings.SplitSeq(desc, "\n") {
			fmt.Fprintf(w, "  %s\n", strings.TrimRight(line, " \t\r"))
		}
	}

	fmt.Fprintf(w, "\n%s\n", d.URL)
}

type jsonReviewer struct {
	Name     string `json:"name"`
	Vote     string `json:"vote"`
	Required bool   `json:"required"`
}

type jsonPR struct {
	ID           int            `json:"id"`
	Title        string         `json:"title"`
	Status       string         `json:"status"`
	IsDraft      bool           `json:"isDraft"`
	Repository   string         `json:"repository"`
	Author       string         `json:"author"`
	CreationDate string         `json:"creationDate,omitempty"`
	ClosedDate   string         `json:"closedDate,omitempty"`
	SourceBranch string         `json:"sourceBranch"`
	TargetBranch string         `json:"targetBranch"`
	MergeStatus  string         `json:"mergeStatus,omitempty"`
	AutoComplete bool           `json:"autoComplete"`
	WorkItems    []string       `json:"workItems"`
	Reviewers    []jsonReviewer `json:"reviewers"`
	Description  string         `json:"description"`
	URL          string         `json:"url"`
}

func toJSONPR(d prDetail) jsonPR {
	p := d.PR
	revs := make([]jsonReviewer, 0, len(p.Reviewers))
	for _, r := range p.Reviewers {
		revs = append(revs, jsonReviewer{Name: r.DisplayName, Vote: r.VoteLabel(), Required: r.IsRequired})
	}
	wis := d.WorkItems
	if wis == nil {
		wis = []string{}
	}
	return jsonPR{
		ID:           p.ID,
		Title:        p.Title,
		Status:       p.Status,
		IsDraft:      p.IsDraft,
		Repository:   p.Repository.Name,
		Author:       p.CreatedBy.DisplayName,
		CreationDate: formatRFC3339(p.CreationDate),
		ClosedDate:   formatRFC3339(p.ClosedDate),
		SourceBranch: shortRef(p.SourceRefName),
		TargetBranch: shortRef(p.TargetRefName),
		MergeStatus:  p.MergeStatus,
		AutoComplete: p.AutoCompleteSetBy != nil,
		WorkItems:    wis,
		Reviewers:    revs,
		Description:  p.Description,
		URL:          d.URL,
	}
}

func shortRef(ref string) string { return strings.TrimPrefix(ref, "refs/heads/") }

// formatDate renders a timestamp in local time; ADO uses year-1 zero dates
// (e.g. closedDate on an active PR), which render as empty.
func formatDate(t time.Time) string {
	if t.Year() <= 1 {
		return ""
	}
	return t.Local().Format("2006-01-02 15:04")
}

func formatRFC3339(t time.Time) string {
	if t.Year() <= 1 {
		return ""
	}
	return t.Format(time.RFC3339)
}
