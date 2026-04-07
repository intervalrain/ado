package query

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/rainhu/ado/internal/api"
	"github.com/rainhu/ado/internal/cqrs"
	"github.com/rainhu/ado/internal/util"
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

type row struct {
	tags, id, typ, state, title, assignee, estimate, remaining string
}

func (h *GetQueryHandler) Handle(ctx context.Context, req cqrs.Request, w io.Writer) error {
	r := req.(*GetQueryRequest)

	result, err := h.client.RunQuery(r.QueryID)
	if err != nil {
		return fmt.Errorf("run query: %w", err)
	}

	// Collect all rows
	headers := row{"Tags", "ID", "Type", "State", "Title", "Assigned To", "Estimate", "Remaining"}
	rows := []row{headers}

	for _, ref := range result.WorkItems {
		wi, err := h.client.GetWorkItem(ref.ID)
		if err != nil {
			rows = append(rows, row{id: fmt.Sprintf("%d", ref.ID), typ: fmt.Sprintf("(error: %v)", err)})
			continue
		}
		estimate := ""
		if wi.Fields.OriginalEstimate > 0 {
			estimate = fmt.Sprintf("%.1f", wi.Fields.OriginalEstimate)
		}
		remaining := ""
		if wi.Fields.RemainingWork > 0 {
			remaining = fmt.Sprintf("%.1f", wi.Fields.RemainingWork)
		}
		rows = append(rows, row{
			tags:      wi.Fields.Tags,
			id:        fmt.Sprintf("%d", wi.ID),
			typ:       wi.Fields.WorkItemType,
			state:     wi.Fields.State,
			title:     wi.Fields.Title,
			assignee:  wi.Fields.AssignedTo.DisplayName,
			estimate:  estimate,
			remaining: remaining,
		})
	}

	// Calculate max display width per column
	widths := [8]int{}
	for _, r := range rows {
		for i, c := range r.columns() {
			dw := util.DisplayWidth(c)
			if dw > widths[i] {
				widths[i] = dw
			}
		}
	}

	fmt.Fprintf(w, "Found %d work items\n\n", len(result.WorkItems))

	// Print header
	printQueryRow(w, widths[:], rows[0])

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
		printQueryRow(w, widths[:], r)
	}

	return nil
}

func (r row) columns() []string {
	return []string{r.tags, r.id, r.typ, r.state, r.title, r.assignee, r.estimate, r.remaining}
}

func printQueryRow(w io.Writer, widths []int, r row) {
	for i, c := range r.columns() {
		if i > 0 {
			fmt.Fprint(w, "  ")
		}
		fmt.Fprint(w, util.PadRight(c, widths[i]))
	}
	fmt.Fprint(w, "\n")
}
