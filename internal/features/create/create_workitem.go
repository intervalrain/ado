package create

import (
	"context"
	"fmt"
	"io"

	"github.com/rainhu/ado/internal/api"
	"github.com/rainhu/ado/internal/cqrs"
)

const RequestName = "CreateWorkItem"

type CreateWorkItemRequest struct {
	Title       string
	Type        string
	Description string
	Estimate    float64
	Tags        string
	ParentID    int
}

func (r *CreateWorkItemRequest) RequestName() string { return RequestName }

type CreateWorkItemHandler struct {
	client *api.Client
}

func NewCreateWorkItemHandler(client *api.Client) *CreateWorkItemHandler {
	return &CreateWorkItemHandler{client: client}
}

func (h *CreateWorkItemHandler) Handle(ctx context.Context, req cqrs.Request, w io.Writer) error {
	r := req.(*CreateWorkItemRequest)

	ops := []api.PatchOp{
		{Op: "add", Path: "/fields/System.Title", Value: r.Title},
	}

	if r.Tags != "" {
		ops = append(ops, api.PatchOp{Op: "add", Path: "/fields/System.Tags", Value: r.Tags})
	}

	if r.Description != "" {
		ops = append(ops, api.PatchOp{Op: "add", Path: "/fields/System.Description", Value: r.Description})
	}

	if r.Estimate > 0 {
		ops = append(ops,
			api.PatchOp{Op: "add", Path: "/fields/Microsoft.VSTS.Scheduling.OriginalEstimate", Value: r.Estimate},
			api.PatchOp{Op: "add", Path: "/fields/Microsoft.VSTS.Scheduling.RemainingWork", Value: r.Estimate},
		)
	}

	// Set current iteration if team is configured
	if iterPath, err := h.client.GetCurrentIteration(); err == nil {
		ops = append(ops, api.PatchOp{Op: "add", Path: "/fields/System.IterationPath", Value: iterPath})
	}

	// Assign to configured user if available
	if h.client.Config().Assignee != "" {
		ops = append(ops, api.PatchOp{Op: "add", Path: "/fields/System.AssignedTo", Value: h.client.Config().Assignee})
	}

	// Link to parent work item (Hierarchy-Reverse = parent)
	if r.ParentID > 0 {
		parentURL := fmt.Sprintf("%s/%s/_apis/wit/workItems/%d",
			h.client.BaseURL(), h.client.Project(), r.ParentID)
		ops = append(ops, api.PatchOp{
			Op:   "add",
			Path: "/relations/-",
			Value: map[string]string{
				"rel": "System.LinkTypes.Hierarchy-Reverse",
				"url": parentURL,
			},
		})
	}

	wi, err := h.client.CreateWorkItem(r.Type, ops)
	if err != nil {
		return fmt.Errorf("create work item: %w", err)
	}

	if r.ParentID > 0 {
		fmt.Fprintf(w, "Created %s #%d: %s (parent: #%d)\n", r.Type, wi.ID, wi.Fields.Title, r.ParentID)
	} else {
		fmt.Fprintf(w, "Created %s #%d: %s\n", r.Type, wi.ID, wi.Fields.Title)
	}
	return nil
}