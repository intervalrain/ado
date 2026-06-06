package remove

import (
	"context"
	"fmt"
	"io"

	"github.com/rainhu/ado/internal/api"
	"github.com/rainhu/ado/internal/cqrs"
)

const RequestName = "RemoveWorkItem"

// RemoveWorkItemRequest sends one or more work items to the project recycle bin.
type RemoveWorkItemRequest struct {
	IDs []int
}

func (r *RemoveWorkItemRequest) RequestName() string { return RequestName }

type RemoveWorkItemHandler struct {
	client *api.Client
}

func NewRemoveWorkItemHandler(client *api.Client) *RemoveWorkItemHandler {
	return &RemoveWorkItemHandler{client: client}
}

func (h *RemoveWorkItemHandler) Handle(ctx context.Context, req cqrs.Request, w io.Writer) error {
	r := req.(*RemoveWorkItemRequest)

	var failed int
	for _, id := range r.IDs {
		if err := h.client.DeleteWorkItem(id); err != nil {
			failed++
			fmt.Fprintf(w, "  #%d: failed: %v\n", id, err)
			continue
		}
		fmt.Fprintf(w, "  #%d: deleted (recycle bin)\n", id)
	}
	if failed > 0 {
		return fmt.Errorf("%d of %d delete(s) failed", failed, len(r.IDs))
	}
	return nil
}
