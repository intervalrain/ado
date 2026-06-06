package update

import (
	"context"
	"fmt"
	"io"

	"github.com/rainhu/ado/internal/api"
	"github.com/rainhu/ado/internal/cqrs"
)

const RequestName = "UpdateWorkItem"

// UpdateWorkItemRequest carries the fields to change. Nil pointers are left
// untouched, mirroring the editable columns in the query TUI
// (Tags, State, Title, Estimate, Remaining).
type UpdateWorkItemRequest struct {
	ID        int
	Title     *string
	State     *string
	Tags      *string
	Estimate  *float64
	Remaining *float64
}

func (r *UpdateWorkItemRequest) RequestName() string { return RequestName }

type UpdateWorkItemHandler struct {
	client *api.Client
}

func NewUpdateWorkItemHandler(client *api.Client) *UpdateWorkItemHandler {
	return &UpdateWorkItemHandler{client: client}
}

func (h *UpdateWorkItemHandler) Handle(ctx context.Context, req cqrs.Request, w io.Writer) error {
	r := req.(*UpdateWorkItemRequest)

	var ops []api.PatchOp
	var changed []string
	add := func(path string, value any, label string) {
		ops = append(ops, api.PatchOp{Op: "replace", Path: "/fields/" + path, Value: value})
		changed = append(changed, label)
	}

	if r.Title != nil {
		add("System.Title", *r.Title, "Title")
	}
	if r.State != nil {
		add("System.State", *r.State, "State")
	}
	if r.Tags != nil {
		add("System.Tags", *r.Tags, "Tags")
	}
	if r.Estimate != nil {
		add("Microsoft.VSTS.Scheduling.OriginalEstimate", *r.Estimate, "Estimate")
	}
	if r.Remaining != nil {
		add("Microsoft.VSTS.Scheduling.RemainingWork", *r.Remaining, "Remaining")
	}

	if len(ops) == 0 {
		return fmt.Errorf("nothing to update: provide at least one of --title, --state, --tags, --est, --remaining")
	}

	if err := h.client.UpdateWorkItemFields(r.ID, ops); err != nil {
		return fmt.Errorf("update work item #%d: %w", r.ID, err)
	}

	wi, err := h.client.GetWorkItem(r.ID)
	if err != nil {
		// Update succeeded; only the read-back failed.
		fmt.Fprintf(w, "Updated #%d (%v)\n", r.ID, changed)
		return nil
	}
	fmt.Fprintf(w, "Updated %s #%d: %s [%s]\n", wi.Fields.WorkItemType, wi.ID, wi.Fields.Title, wi.Fields.State)
	fmt.Fprintf(w, "Changed: %v\n", changed)
	return nil
}
