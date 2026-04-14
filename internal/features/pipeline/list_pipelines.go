package pipeline

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/rainhu/ado/internal/api"
	"github.com/rainhu/ado/internal/cqrs"
	"github.com/rainhu/ado/internal/util"
)

const ListRequestName = "ListPipelines"

type ListPipelinesRequest struct {
	DefinitionID int // 0 means list all definitions with latest build
	Top          int // number of recent builds to show (used with DefinitionID)
}

func (r *ListPipelinesRequest) RequestName() string { return ListRequestName }

type ListPipelinesHandler struct {
	client *api.Client
}

func NewListPipelinesHandler(client *api.Client) *ListPipelinesHandler {
	return &ListPipelinesHandler{client: client}
}

type pipelineRow struct {
	pipeline, build, branch, status, result, duration, triggeredBy, stages string
}

func (r pipelineRow) columns() []string {
	return []string{r.pipeline, r.build, r.branch, r.status, r.result, r.duration, r.triggeredBy, r.stages}
}

func (h *ListPipelinesHandler) Handle(ctx context.Context, req cqrs.Request, w io.Writer) error {
	r := req.(*ListPipelinesRequest)

	if r.DefinitionID > 0 {
		return h.showBuilds(w, r.DefinitionID, r.Top)
	}
	return h.showDefinitions(w)
}

func (h *ListPipelinesHandler) showDefinitions(w io.Writer) error {
	defs, err := h.client.ListPipelineDefinitions()
	if err != nil {
		return err
	}
	if len(defs) == 0 {
		fmt.Fprintln(w, "No pipeline definitions found.")
		return nil
	}

	// Collect definition IDs and fetch latest builds
	ids := make([]int, len(defs))
	for i, d := range defs {
		ids[i] = d.ID
	}

	latest, err := h.client.GetLatestBuilds(ids)
	if err != nil {
		return err
	}

	// Fetch stage progress for running builds
	for defID, b := range latest {
		if strings.EqualFold(b.Status, "inProgress") {
			if sp, err := h.client.GetBuildTimeline(b.ID); err == nil && sp != nil {
				b.Stages = sp
				latest[defID] = b
			}
		}
	}

	fmt.Fprintf(w, "Found %d pipeline(s):\n\n", len(defs))

	headers := pipelineRow{"Pipeline", "Build", "Branch", "Status", "Result", "Duration", "Triggered By", "Stages"}
	rows := []pipelineRow{headers}

	for _, def := range defs {
		row := pipelineRow{pipeline: def.Name}
		if b, ok := latest[def.ID]; ok {
			row.build = fmt.Sprintf("#%d", b.ID)
			row.branch = b.BranchName()
			row.status = b.StatusLabel()
			row.result = fmt.Sprintf("%s %s", b.ResultIcon(), b.ResultLabel())
			row.duration = b.Duration()
			row.triggeredBy = b.RequestedBy
			row.stages = b.StagesLabel()
		} else {
			row.build = "-"
			row.branch = "-"
			row.status = "-"
			row.result = "-"
			row.duration = "-"
			row.triggeredBy = "-"
			row.stages = "-"
		}
		rows = append(rows, row)
	}

	printTable(w, rows)
	return nil
}

func (h *ListPipelinesHandler) showBuilds(w io.Writer, definitionID int, top int) error {
	if top <= 0 {
		top = 5
	}

	builds, err := h.client.ListBuilds([]int{definitionID}, top)
	if err != nil {
		return err
	}
	if len(builds) == 0 {
		fmt.Fprintf(w, "No builds found for definition %d.\n", definitionID)
		return nil
	}

	fmt.Fprintf(w, "Recent builds for %s (definition %d):\n\n", builds[0].DefinitionName, definitionID)

	headers := pipelineRow{"Pipeline", "Build", "Branch", "Status", "Result", "Duration", "Triggered By", "Stages"}
	rows := []pipelineRow{headers}

	for _, b := range builds {
		// Fetch stage progress for running builds
		if strings.EqualFold(b.Status, "inProgress") {
			if sp, err := h.client.GetBuildTimeline(b.ID); err == nil && sp != nil {
				b.Stages = sp
			}
		}
		rows = append(rows, pipelineRow{
			pipeline:    b.DefinitionName,
			build:       fmt.Sprintf("#%d", b.ID),
			branch:      b.BranchName(),
			status:      b.StatusLabel(),
			result:      fmt.Sprintf("%s %s", b.ResultIcon(), b.ResultLabel()),
			duration:    b.Duration(),
			triggeredBy: b.RequestedBy,
			stages:      b.StagesLabel(),
		})
	}

	printTable(w, rows)
	return nil
}

func printTable(w io.Writer, rows []pipelineRow) {
	colCount := 8
	widths := make([]int, colCount)
	for _, r := range rows {
		for i, c := range r.columns() {
			dw := util.DisplayWidth(c)
			if dw > widths[i] {
				widths[i] = dw
			}
		}
	}

	// Print header
	printPipelineRow(w, widths, rows[0])

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
		printPipelineRow(w, widths, r)
	}
}

func printPipelineRow(w io.Writer, widths []int, r pipelineRow) {
	cols := r.columns()
	for i, c := range cols {
		if i > 0 {
			fmt.Fprint(w, "  ")
		}
		fmt.Fprint(w, util.PadRight(c, widths[i]))
	}
	fmt.Fprint(w, "\n")
}
