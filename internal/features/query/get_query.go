package query

import (
	"context"
	"fmt"
	"io"

	"github.com/rainhu/ado/internal/api"
	"github.com/rainhu/ado/internal/cqrs"
)

const RequestName = "GetQuery"

type GetQueryRequest struct {
	QueryID string
}

func (r *GetQueryRequest) RequestName() string { return RequestName }

type GetQueryHandler struct {
	client *api.Client
}

func NewGetQueryHandler(client *api.Client) *GetQueryHandler {
	return &GetQueryHandler{client: client}
}

func (h *GetQueryHandler) Handle(ctx context.Context, req cqrs.Request, w io.Writer) error {
	r := req.(*GetQueryRequest)

	result, err := h.client.RunQuery(r.QueryID)
	if err != nil {
		return fmt.Errorf("run query: %w", err)
	}

	fmt.Fprintf(w, "Found %d work items\n\n", len(result.WorkItems))
	fmt.Fprintf(w, "%-8s %-14s %-12s %-20s %s\n", "ID", "Type", "State", "Assigned To", "Title")
	fmt.Fprintf(w, "%-8s %-14s %-12s %-20s %s\n", "------", "------------", "----------", "------------------", "-----")

	// Stream: fetch and print each work item one by one
	for _, ref := range result.WorkItems {
		wi, err := h.client.GetWorkItem(ref.ID)
		if err != nil {
			fmt.Fprintf(w, "%-8d (error: %v)\n", ref.ID, err)
			continue
		}

		fmt.Fprintf(w, "%-8d %-14s %-12s %-20s %s\n",
			wi.ID,
			wi.Fields.WorkItemType,
			wi.Fields.State,
			wi.Fields.AssignedTo.DisplayName,
			wi.Fields.Title,
		)
	}

	return nil
}
