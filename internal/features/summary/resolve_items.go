package summary

import (
	"context"
	"fmt"
	"io"

	"github.com/rainhu/ado/internal/api"
	"github.com/rainhu/ado/internal/cqrs"
)

const ResolveRequestName = "ResolveSummaryItems"

type ItemAction struct {
	ID     int
	State  string // "Closed", "Resolved", etc.
	Reason string
}

type ResolveSummaryItemsRequest struct {
	Items []ItemAction
}

func (r *ResolveSummaryItemsRequest) RequestName() string { return ResolveRequestName }

type ResolveSummaryItemsHandler struct {
	client *api.Client
}

func NewResolveSummaryItemsHandler(client *api.Client) *ResolveSummaryItemsHandler {
	return &ResolveSummaryItemsHandler{client: client}
}

func (h *ResolveSummaryItemsHandler) Handle(ctx context.Context, req cqrs.Request, w io.Writer) error {
	r := req.(*ResolveSummaryItemsRequest)

	for _, item := range r.Items {
		err := h.client.UpdateWorkItemField(item.ID, "System.State", item.State)
		if err != nil {
			fmt.Fprintf(w, "  ✗ #%d failed: %v\n", item.ID, err)
			continue
		}
		fmt.Fprintf(w, "  ✓ #%d → %s\n", item.ID, item.State)
	}
	return nil
}
