package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/rainhu/ado/internal/api"
	"github.com/rainhu/ado/internal/cqrs"
)

const RequestName = "UpdateWorkItem"

// UpdateWorkItemRequest carries the fields to change. Nil pointers are left
// untouched, mirroring the editable columns in the query TUI
// (Tags, State, Title, Estimate, Remaining).
type UpdateWorkItemRequest struct {
	ID          int
	Title       *string
	State       *string
	Tags        *string
	Description *string
	Estimate    *float64
	Remaining   *float64
	AssignedTo  *string // "" unassigns
	Area        *string
	Iteration   *string
	Comment     *string // appended to the discussion (System.History)
	Parent      *int    // 0 removes the current parent
	JSON        bool
}

func (r *UpdateWorkItemRequest) RequestName() string { return RequestName }

type UpdateWorkItemHandler struct {
	client *api.Client
}

func NewUpdateWorkItemHandler(client *api.Client) *UpdateWorkItemHandler {
	return &UpdateWorkItemHandler{client: client}
}

func (h *UpdateWorkItemHandler) Handle(ctx context.Context, req cqrs.Request, w io.Writer) error {
	r := req.(*UpdateWorkItemRequest)

	// Relations are only needed to re-parent, so the extra read happens only then.
	var current *api.WorkItem
	if r.Parent != nil {
		wi, err := h.client.GetWorkItem(r.ID)
		if err != nil {
			return fmt.Errorf("read work item #%d relations: %w", r.ID, err)
		}
		current = wi
	}

	itemURLBase := fmt.Sprintf("%s/%s/_apis/wit/workItems/", h.client.BaseURL(), h.client.Project())
	ops, changed, err := buildPatch(r, current, itemURLBase)
	if err != nil {
		return err
	}
	if len(ops) == 0 {
		fmt.Fprintf(w, "#%d already has parent #%d; nothing changed\n", r.ID, *r.Parent)
		return nil
	}

	if err := h.client.UpdateWorkItemFields(r.ID, ops); err != nil {
		return fmt.Errorf("update work item #%d: %w", r.ID, err)
	}

	wi, err := h.client.GetWorkItem(r.ID)
	if r.JSON {
		out := map[string]any{"id": r.ID, "changed": changed}
		if err == nil {
			out["type"] = wi.Fields.WorkItemType
			out["state"] = wi.Fields.State
			out["title"] = wi.Fields.Title
			out["remaining"] = wi.Fields.RemainingWork
			out["estimate"] = wi.Fields.OriginalEstimate
		} else {
			out["readBackError"] = err.Error()
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}
	if err != nil {
		// Update succeeded; only the read-back failed.
		fmt.Fprintf(w, "Updated #%d (%v)\n", r.ID, changed)
		return nil
	}
	fmt.Fprintf(w, "Updated %s #%d: %s [%s]\n", wi.Fields.WorkItemType, wi.ID, wi.Fields.Title, wi.Fields.State)
	fmt.Fprintf(w, "Changed: %v\n", changed)
	return nil
}

// buildPatch turns the request into JSON-patch ops. current carries the item's
// relations and is required only when r.Parent is set. itemURLBase ends with
// ".../_apis/wit/workItems/" and is used to build the parent link URL.
func buildPatch(r *UpdateWorkItemRequest, current *api.WorkItem, itemURLBase string) ([]api.PatchOp, []string, error) {
	var ops []api.PatchOp
	var changed []string
	set := func(op, path string, value any, label string) {
		ops = append(ops, api.PatchOp{Op: op, Path: path, Value: value})
		changed = append(changed, label)
	}
	field := func(name string, value any, label string) { set("replace", "/fields/"+name, value, label) }

	if r.Title != nil {
		field("System.Title", *r.Title, "Title")
	}
	if r.State != nil {
		field("System.State", *r.State, "State")
	}
	if r.Tags != nil {
		field("System.Tags", *r.Tags, "Tags")
	}
	if r.Description != nil {
		field("System.Description", *r.Description, "Description")
	}
	if r.Estimate != nil {
		field("Microsoft.VSTS.Scheduling.OriginalEstimate", *r.Estimate, "Estimate")
	}
	if r.Remaining != nil {
		field("Microsoft.VSTS.Scheduling.RemainingWork", *r.Remaining, "Remaining")
	}
	if r.AssignedTo != nil {
		field("System.AssignedTo", *r.AssignedTo, "AssignedTo")
	}
	if r.Area != nil {
		field("System.AreaPath", *r.Area, "Area")
	}
	if r.Iteration != nil {
		field("System.IterationPath", *r.Iteration, "Iteration")
	}
	if r.Comment != nil {
		set("add", "/fields/System.History", *r.Comment, "Comment")
	}

	if r.Parent != nil {
		if current == nil {
			return nil, nil, fmt.Errorf("re-parenting #%d needs its current relations", r.ID)
		}
		idx, curParent := -1, 0
		for i, rel := range current.Relations {
			if rel.Rel == api.ParentRel {
				idx, curParent = i, idFromURL(rel.URL)
				break
			}
		}
		want := *r.Parent
		switch {
		case want == curParent && want != 0:
			// already linked to this parent
		case want == 0 && idx < 0:
			// nothing to unlink
		default:
			if idx >= 0 {
				ops = append(ops, api.PatchOp{Op: "remove", Path: "/relations/" + strconv.Itoa(idx)})
			}
			if want > 0 {
				ops = append(ops, api.PatchOp{Op: "add", Path: "/relations/-", Value: map[string]string{
					"rel": api.ParentRel,
					"url": itemURLBase + strconv.Itoa(want),
				}})
			}
			changed = append(changed, "Parent")
		}
	}

	if len(ops) == 0 && r.Parent == nil {
		return nil, nil, fmt.Errorf("nothing to update: provide at least one of --title, --state, --tags, --desc, --est, --remaining, --assign, --area, --iteration, --comment, --parent")
	}
	return ops, changed, nil
}

// idFromURL extracts the work item ID from ".../_apis/wit/workItems/<id>".
func idFromURL(u string) int {
	i := strings.LastIndex(u, "/")
	id, _ := strconv.Atoi(u[i+1:])
	return id
}
