package show

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/rainhu/ado/internal/api"
	"github.com/rainhu/ado/internal/cqrs"
	"github.com/rainhu/ado/internal/util"
)

const RequestName = "ShowWorkItem"

type ShowWorkItemRequest struct {
	IDs  []int
	JSON bool
}

func (r *ShowWorkItemRequest) RequestName() string { return RequestName }

type ShowWorkItemHandler struct {
	client *api.Client
}

func NewShowWorkItemHandler(client *api.Client) *ShowWorkItemHandler {
	return &ShowWorkItemHandler{client: client}
}

type jsonItem struct {
	ID                 int      `json:"id"`
	Type               string   `json:"type"`
	State              string   `json:"state"`
	Title              string   `json:"title"`
	Tags               string   `json:"tags"`
	AssignedTo         string   `json:"assignedTo"`
	IterationPath      string   `json:"iterationPath"`
	Estimate           *float64 `json:"estimate"`
	Remaining          *float64 `json:"remaining"`
	Description        string   `json:"description"`
	ReproSteps         string   `json:"reproSteps,omitempty"`
	AcceptanceCriteria string   `json:"acceptanceCriteria,omitempty"`
	Error              string   `json:"error,omitempty"`
}

func (h *ShowWorkItemHandler) Handle(ctx context.Context, req cqrs.Request, w io.Writer) error {
	r := req.(*ShowWorkItemRequest)

	if r.JSON {
		return h.writeJSON(r.IDs, w)
	}

	var firstErr error
	for i, id := range r.IDs {
		if i > 0 {
			fmt.Fprint(w, "\n")
		}
		wi, err := h.client.GetWorkItem(id)
		if err != nil {
			fmt.Fprintf(w, "#%d: error: %v\n", id, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		printWorkItem(w, wi)
	}
	return firstErr
}

func printWorkItem(w io.Writer, wi *api.WorkItem) {
	f := wi.Fields
	fmt.Fprintf(w, "%s #%d: %s\n", f.WorkItemType, wi.ID, f.Title)

	printField := func(label, value string) {
		if value != "" {
			fmt.Fprintf(w, "%-11s %s\n", label+":", value)
		}
	}
	printField("State", f.State)
	printField("Assigned", f.AssignedTo.DisplayName)
	printField("Iteration", f.IterationPath)
	printField("Tags", f.Tags)
	if f.OriginalEstimate > 0 {
		printField("Estimate", fmt.Sprintf("%.1f", f.OriginalEstimate))
	}
	if f.RemainingWork > 0 {
		printField("Remaining", fmt.Sprintf("%.1f", f.RemainingWork))
	}

	printSection(w, "Description", f.Description)
	printSection(w, "Repro Steps", f.ReproSteps)
	printSection(w, "Acceptance Criteria", f.AcceptanceCriteria)
}

func printSection(w io.Writer, title, html string) {
	text := util.HTMLToText(html)
	if text == "" {
		return
	}
	fmt.Fprintf(w, "\n%s:\n", title)
	for line := range strings.SplitSeq(text, "\n") {
		fmt.Fprintf(w, "  %s\n", line)
	}
}

func (h *ShowWorkItemHandler) writeJSON(ids []int, w io.Writer) error {
	items := make([]jsonItem, 0, len(ids))
	for _, id := range ids {
		wi, err := h.client.GetWorkItem(id)
		if err != nil {
			items = append(items, jsonItem{ID: id, Error: err.Error()})
			continue
		}
		f := wi.Fields
		item := jsonItem{
			ID:                 wi.ID,
			Type:               f.WorkItemType,
			State:              f.State,
			Title:              f.Title,
			Tags:               f.Tags,
			AssignedTo:         f.AssignedTo.DisplayName,
			IterationPath:      f.IterationPath,
			Description:        util.HTMLToText(f.Description),
			ReproSteps:         util.HTMLToText(f.ReproSteps),
			AcceptanceCriteria: util.HTMLToText(f.AcceptanceCriteria),
		}
		if f.OriginalEstimate > 0 {
			est := f.OriginalEstimate
			item.Estimate = &est
		}
		if f.RemainingWork > 0 {
			rem := f.RemainingWork
			item.Remaining = &rem
		}
		items = append(items, item)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(items)
}
