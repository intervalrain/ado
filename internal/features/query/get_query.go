package query

import (
	"context"
	"fmt"
	"io"
	"strings"

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

	// Calculate max width per column
	widths := [8]int{}
	for _, r := range rows {
		cols := r.columns()
		for i, c := range cols {
			if len(c) > widths[i] {
				widths[i] = len(c)
			}
		}
	}

	// Build format string
	fmtParts := make([]string, 8)
	for i, w := range widths {
		fmtParts[i] = fmt.Sprintf("%%-%ds", w)
	}
	lineFmt := strings.Join(fmtParts, "  ") + "\n"

	fmt.Fprintf(w, "Found %d work items\n\n", len(result.WorkItems)-0)

	// Print header
	h.printRow(w, lineFmt, rows[0])

	// Print separator
	seps := make([]string, 8)
	for i, width := range widths {
		seps[i] = strings.Repeat("-", width)
	}
	fmt.Fprintf(w, lineFmt, str2iface(seps)...)

	// Print data rows
	for _, r := range rows[1:] {
		h.printRow(w, lineFmt, r)
	}

	return nil
}

func (r row) columns() []string {
	return []string{r.tags, r.id, r.typ, r.state, r.title, r.assignee, r.estimate, r.remaining}
}

func (h *GetQueryHandler) printRow(w io.Writer, fmtStr string, r row) {
	fmt.Fprintf(w, fmtStr, str2iface(r.columns())...)
}

func str2iface(ss []string) []interface{} {
	out := make([]interface{}, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}
