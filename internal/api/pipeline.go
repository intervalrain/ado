package api

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// PipelineDefinition represents a pipeline (build definition) in Azure DevOps.
type PipelineDefinition struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	Revision int    `json:"revision"`
}

type pipelineDefinitionsResult struct {
	Value []PipelineDefinition `json:"value"`
}

// Build represents a single build execution.
type Build struct {
	ID             int        `json:"id"`
	DefinitionID   int        `json:"-"`
	DefinitionName string     `json:"-"`
	Status         string     `json:"status"`
	Result         string     `json:"result"`
	RequestedBy    string     `json:"-"`
	QueueTime      *time.Time `json:"queueTime"`
	StartTime      *time.Time `json:"startTime"`
	FinishTime     *time.Time `json:"finishTime"`
	SourceBranch   string     `json:"sourceBranch"`
	WebURL         string     `json:"-"`
	Stages         *StageProgress
}

// buildDTO is the raw JSON shape from the Azure DevOps builds API.
type buildDTO struct {
	ID         int        `json:"id"`
	Status     string     `json:"status"`
	Result     string     `json:"result"`
	QueueTime  *time.Time `json:"queueTime"`
	StartTime  *time.Time `json:"startTime"`
	FinishTime *time.Time `json:"finishTime"`
	SourceBranch string   `json:"sourceBranch"`
	RequestedBy  struct {
		DisplayName string `json:"displayName"`
	} `json:"requestedBy"`
	Definition struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"definition"`
	Links struct {
		Web struct {
			Href string `json:"href"`
		} `json:"web"`
	} `json:"_links"`
}

type buildsResult struct {
	Value []buildDTO `json:"value"`
}

// StageProgress describes the current stage execution progress within a running build.
type StageProgress struct {
	CurrentIndex    int
	Total           int
	CurrentStageName string
}

type timelineResponse struct {
	Records []timelineRecord `json:"records"`
}

type timelineRecord struct {
	Type   string `json:"type"`
	Name   string `json:"name"`
	Order  int    `json:"order"`
	State  string `json:"state"`
	Result string `json:"result"`
}

// StatusLabel returns a human-readable label for the build status.
func (b Build) StatusLabel() string {
	switch strings.ToLower(b.Status) {
	case "inprogress":
		return "Running"
	case "completed":
		return "Completed"
	case "cancelling":
		return "Cancelling"
	case "postponed":
		return "Postponed"
	case "notstarted":
		return "Not Started"
	case "none":
		return "None"
	default:
		return b.Status
	}
}

// ResultLabel returns a human-readable label for the build result.
func (b Build) ResultLabel() string {
	switch strings.ToLower(b.Result) {
	case "succeeded":
		return "Succeeded"
	case "partiallysucceeded":
		return "Partial"
	case "failed":
		return "Failed"
	case "canceled":
		return "Canceled"
	case "none", "":
		return "-"
	default:
		return b.Result
	}
}

// ResultIcon returns a status icon for the build result.
func (b Build) ResultIcon() string {
	switch strings.ToLower(b.Result) {
	case "succeeded":
		return "✓"
	case "partiallysucceeded":
		return "⚠"
	case "failed":
		return "✗"
	case "canceled":
		return "⊘"
	default:
		if strings.ToLower(b.Status) == "inprogress" {
			return "▶"
		}
		return "○"
	}
}

// Duration returns a formatted duration string for the build.
func (b Build) Duration() string {
	var start, end time.Time
	if b.StartTime != nil {
		start = *b.StartTime
	} else if b.QueueTime != nil {
		start = *b.QueueTime
	} else {
		return "-"
	}

	if b.FinishTime != nil {
		end = *b.FinishTime
	} else {
		end = time.Now()
	}

	d := end.Sub(start)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	}
	return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
}

// BranchName strips the refs/heads/ prefix from SourceBranch.
func (b Build) BranchName() string {
	return strings.TrimPrefix(b.SourceBranch, "refs/heads/")
}

// StagesLabel returns a formatted stage progress string.
func (b Build) StagesLabel() string {
	if b.Stages == nil {
		return "-"
	}
	return fmt.Sprintf("%d/%d %s", b.Stages.CurrentIndex, b.Stages.Total, b.Stages.CurrentStageName)
}

// ListPipelineDefinitions returns all build definitions in the project.
func (c *Client) ListPipelineDefinitions() ([]PipelineDefinition, error) {
	url := fmt.Sprintf(
		"%s/%s/_apis/build/definitions?api-version=7.1",
		c.BaseURL(), c.Project(),
	)
	var result pipelineDefinitionsResult
	if err := c.get(url, &result); err != nil {
		return nil, fmt.Errorf("list pipeline definitions: %w", err)
	}
	return result.Value, nil
}

// ListBuilds returns recent builds filtered by definition IDs.
func (c *Client) ListBuilds(definitionIDs []int, top int) ([]Build, error) {
	if len(definitionIDs) == 0 {
		return nil, nil
	}

	ids := make([]string, len(definitionIDs))
	for i, id := range definitionIDs {
		ids[i] = fmt.Sprintf("%d", id)
	}

	url := fmt.Sprintf(
		"%s/%s/_apis/build/builds?definitions=%s&$top=%d&queryOrder=queueTimeDescending&api-version=7.1",
		c.BaseURL(), c.Project(), strings.Join(ids, ","), top,
	)

	var result buildsResult
	if err := c.get(url, &result); err != nil {
		return nil, fmt.Errorf("list builds: %w", err)
	}

	builds := make([]Build, len(result.Value))
	for i, dto := range result.Value {
		builds[i] = mapBuild(dto)
	}
	return builds, nil
}

// GetLatestBuilds returns the latest build per definition ID.
func (c *Client) GetLatestBuilds(definitionIDs []int) (map[int]Build, error) {
	if len(definitionIDs) == 0 {
		return nil, nil
	}

	top := len(definitionIDs) * 2
	builds, err := c.ListBuilds(definitionIDs, top)
	if err != nil {
		return nil, err
	}

	latest := make(map[int]Build)
	for _, b := range builds {
		if existing, ok := latest[b.DefinitionID]; !ok || b.ID > existing.ID {
			latest[b.DefinitionID] = b
		}
	}
	return latest, nil
}

// GetBuildTimeline fetches the timeline for a build.
func (c *Client) GetBuildTimeline(buildID int) (*StageProgress, error) {
	url := fmt.Sprintf(
		"%s/%s/_apis/build/builds/%d/timeline?api-version=7.1",
		c.BaseURL(), c.Project(), buildID,
	)

	var result timelineResponse
	if err := c.get(url, &result); err != nil {
		return nil, fmt.Errorf("get build timeline: %w", err)
	}

	// Filter stage records and sort by order
	var stages []timelineRecord
	for _, r := range result.Records {
		if strings.EqualFold(r.Type, "Stage") {
			stages = append(stages, r)
		}
	}
	if len(stages) == 0 {
		return nil, nil
	}

	sort.Slice(stages, func(i, j int) bool {
		return stages[i].Order < stages[j].Order
	})

	// Find the in-progress stage with the lowest order
	for i, s := range stages {
		if strings.EqualFold(s.State, "inProgress") {
			return &StageProgress{
				CurrentIndex:    i + 1,
				Total:           len(stages),
				CurrentStageName: s.Name,
			}, nil
		}
	}

	return nil, nil
}

// BuildWebURL constructs the browser URL for a build.
func (c *Client) BuildWebURL(buildID int) string {
	return fmt.Sprintf("%s/%s/_build/results?buildId=%d",
		c.BaseURL(), c.Project(), buildID)
}

func mapBuild(dto buildDTO) Build {
	return Build{
		ID:             dto.ID,
		DefinitionID:   dto.Definition.ID,
		DefinitionName: dto.Definition.Name,
		Status:         dto.Status,
		Result:         dto.Result,
		RequestedBy:    dto.RequestedBy.DisplayName,
		QueueTime:      dto.QueueTime,
		StartTime:      dto.StartTime,
		FinishTime:     dto.FinishTime,
		SourceBranch:   dto.SourceBranch,
		WebURL:         dto.Links.Web.Href,
	}
}
