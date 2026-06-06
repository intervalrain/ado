package move

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/rainhu/ado/internal/api"
	"github.com/rainhu/ado/internal/cqrs"
)

const RequestName = "MoveWorkItem"

// MoveWorkItemRequest moves one or more work items to an iteration.
// When Current is true the team's current sprint is used; otherwise Iteration
// is matched against the team's iterations by path, then name (case-insensitive
// exact, then substring).
type MoveWorkItemRequest struct {
	IDs       []int
	Iteration string
	Current   bool
}

func (r *MoveWorkItemRequest) RequestName() string { return RequestName }

type MoveWorkItemHandler struct {
	client *api.Client
}

func NewMoveWorkItemHandler(client *api.Client) *MoveWorkItemHandler {
	return &MoveWorkItemHandler{client: client}
}

func (h *MoveWorkItemHandler) Handle(ctx context.Context, req cqrs.Request, w io.Writer) error {
	r := req.(*MoveWorkItemRequest)

	path, name, err := h.resolvePath(r)
	if err != nil {
		return err
	}

	fmt.Fprintf(w, "Moving %d work item(s) to %s\n", len(r.IDs), name)
	var failed int
	for _, id := range r.IDs {
		if err := h.client.UpdateWorkItemField(id, "System.IterationPath", path); err != nil {
			failed++
			fmt.Fprintf(w, "  #%d: failed: %v\n", id, err)
			continue
		}
		fmt.Fprintf(w, "  #%d: moved\n", id)
	}
	if failed > 0 {
		return fmt.Errorf("%d of %d move(s) failed", failed, len(r.IDs))
	}
	return nil
}

// resolvePath returns the iteration path and a display name for the target.
func (h *MoveWorkItemHandler) resolvePath(r *MoveWorkItemRequest) (path, name string, err error) {
	if r.Current {
		p, err := h.client.GetCurrentIteration()
		if err != nil {
			return "", "", fmt.Errorf("resolve current iteration: %w", err)
		}
		return p, p, nil
	}

	iters, err := h.client.ListIterations()
	if err != nil {
		return "", "", fmt.Errorf("list iterations: %w", err)
	}

	q := strings.TrimSpace(r.Iteration)
	if q == "" {
		return "", "", fmt.Errorf("specify --iteration <name|path> or --current")
	}

	// Exact path match.
	for _, it := range iters {
		if it.Path == q {
			return it.Path, it.Path, nil
		}
	}
	// Case-insensitive exact name match.
	var matches []api.Iteration
	for _, it := range iters {
		if strings.EqualFold(it.Name, q) {
			matches = append(matches, it)
		}
	}
	// Fall back to substring match on name or path.
	if len(matches) == 0 {
		ql := strings.ToLower(q)
		for _, it := range iters {
			if strings.Contains(strings.ToLower(it.Name), ql) ||
				strings.Contains(strings.ToLower(it.Path), ql) {
				matches = append(matches, it)
			}
		}
	}

	switch len(matches) {
	case 0:
		return "", "", fmt.Errorf("no iteration matching %q (run with a valid name/path)", q)
	case 1:
		return matches[0].Path, matches[0].Path, nil
	default:
		var names []string
		for _, it := range matches {
			names = append(names, it.Path)
		}
		return "", "", fmt.Errorf("%q is ambiguous, matches:\n  %s", q, strings.Join(names, "\n  "))
	}
}
