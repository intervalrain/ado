package query

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/rainhu/ado/internal/api"
	"github.com/rainhu/ado/internal/cqrs"
	"github.com/rainhu/ado/internal/util"
)

const RequestName = "GetQuery"

type GetQueryRequest struct {
	QueryID string
	JSON    bool
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

type jsonItem struct {
	ID            int      `json:"id"`
	Type          string   `json:"type"`
	State         string   `json:"state"`
	Title         string   `json:"title"`
	Tags          string   `json:"tags"`
	AssignedTo    string   `json:"assignedTo"`
	IterationPath string   `json:"iterationPath"`
	Estimate      *float64 `json:"estimate"`
	Remaining     *float64 `json:"remaining"`
	Error         string   `json:"error,omitempty"`
}

func (h *GetQueryHandler) Handle(ctx context.Context, req cqrs.Request, w io.Writer) error {
	r := req.(*GetQueryRequest)

	result, err := h.client.RunQuery(r.QueryID)
	if err != nil {
		return fmt.Errorf("run query: %w", err)
	}

	if r.JSON {
		return h.writeJSON(result, w)
	}

	// Collect all rows
	headers := row{"Tags", "ID", "Type", "State", "Title", "Assigned To", "Estimate", "Remaining"}
	rows := []row{headers}

	for _, fetched := range h.fetchAll(result.WorkItems) {
		wi, err := fetched.wi, fetched.err
		if err != nil {
			rows = append(rows, row{id: fmt.Sprintf("%d", fetched.id), typ: fmt.Sprintf("(error: %v)", err)})
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

type fetchResult struct {
	id  int
	wi  *api.WorkItem
	err error
}

// fetchAll retrieves work items concurrently (bounded), preserving query order.
func (h *GetQueryHandler) fetchAll(refs []api.WorkItemRef) []fetchResult {
	results := make([]fetchResult, len(refs))
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup
	for i, ref := range refs {
		wg.Add(1)
		go func(i, id int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			wi, err := h.client.GetWorkItem(id)
			results[i] = fetchResult{id: id, wi: wi, err: err}
		}(i, ref.ID)
	}
	wg.Wait()
	return results
}

func (h *GetQueryHandler) writeJSON(result *api.QueryResult, w io.Writer) error {
	items := make([]jsonItem, 0, len(result.WorkItems))
	for _, fetched := range h.fetchAll(result.WorkItems) {
		wi, err := fetched.wi, fetched.err
		if err != nil {
			items = append(items, jsonItem{ID: fetched.id, Error: err.Error()})
			continue
		}
		item := jsonItem{
			ID:            wi.ID,
			Type:          wi.Fields.WorkItemType,
			State:         wi.Fields.State,
			Title:         wi.Fields.Title,
			Tags:          wi.Fields.Tags,
			AssignedTo:    wi.Fields.AssignedTo.DisplayName,
			IterationPath: wi.Fields.IterationPath,
		}
		if wi.Fields.OriginalEstimate > 0 {
			est := wi.Fields.OriginalEstimate
			item.Estimate = &est
		}
		if wi.Fields.RemainingWork > 0 {
			rem := wi.Fields.RemainingWork
			item.Remaining = &rem
		}
		items = append(items, item)
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(items)
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
