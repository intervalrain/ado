package pr

import (
	"context"
	"fmt"
	"io"

	"github.com/rainhu/ado/internal/api"
	"github.com/rainhu/ado/internal/cqrs"
)

const CommentRequestName = "CommentPR"

// CommentPRRequest posts one new comment thread on a pull request. The PR is
// looked up by ID across the project, so it works from any directory.
type CommentPRRequest struct {
	PRID    int
	Content string
}

func (r *CommentPRRequest) RequestName() string { return CommentRequestName }

type CommentPRHandler struct {
	client *api.Client
}

func NewCommentPRHandler(client *api.Client) *CommentPRHandler {
	return &CommentPRHandler{client: client}
}

func (h *CommentPRHandler) Handle(ctx context.Context, req cqrs.Request, w io.Writer) error {
	r := req.(*CommentPRRequest)
	if r.PRID <= 0 {
		return fmt.Errorf("invalid PR id %d", r.PRID)
	}
	p, err := h.client.GetPullRequest(r.PRID)
	if err != nil {
		return fmt.Errorf("get PR #%d: %w", r.PRID, err)
	}
	id, err := h.client.CreatePRThread(p.Repository.ID, p.ID, r.Content)
	if err != nil {
		return fmt.Errorf("comment on PR #%d: %w", r.PRID, err)
	}
	fmt.Fprintf(w, "Comment thread %d added to PR #%d (%s)\n%s\n",
		id, p.ID, p.Repository.Name, h.client.PRWebURL(p.Repository.Name, p.ID))
	return nil
}
