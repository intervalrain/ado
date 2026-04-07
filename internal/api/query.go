package api

import (
	"fmt"
)

type QueryResult struct {
	WorkItems []WorkItemRef `json:"workItems"`
}

type WorkItemRef struct {
	ID  int    `json:"id"`
	URL string `json:"url"`
}

type WorkItem struct {
	ID     int            `json:"id"`
	Fields WorkItemFields `json:"fields"`
}

type WorkItemFields struct {
	Title            string   `json:"System.Title"`
	State            string   `json:"System.State"`
	AssignedTo       Identity `json:"System.AssignedTo"`
	WorkItemType     string   `json:"System.WorkItemType"`
	Tags             string   `json:"System.Tags"`
	IterationPath    string   `json:"System.IterationPath"`
	OriginalEstimate float64  `json:"Microsoft.VSTS.Scheduling.OriginalEstimate"`
	RemainingWork    float64  `json:"Microsoft.VSTS.Scheduling.RemainingWork"`
}

type Identity struct {
	DisplayName string `json:"displayName"`
}

func (c *Client) RunQuery(queryID string) (*QueryResult, error) {
	url := fmt.Sprintf("%s/%s/_apis/wit/wiql/%s?api-version=7.1", c.BaseURL(), c.Project(), queryID)
	var result QueryResult
	if err := c.get(url, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

type PatchOp struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value"`
}

func (c *Client) UpdateWorkItemField(id int, fieldPath string, value any) error {
	url := fmt.Sprintf(
		"%s/%s/_apis/wit/workitems/%d?api-version=7.1",
		c.BaseURL(), c.Project(), id,
	)
	ops := []PatchOp{{Op: "replace", Path: "/fields/" + fieldPath, Value: value}}
	return c.patch(url, ops, nil)
}

func (c *Client) CreateWorkItem(workItemType string, fields []PatchOp) (*WorkItem, error) {
	url := fmt.Sprintf(
		"%s/%s/_apis/wit/workitems/$%s?api-version=7.1",
		c.BaseURL(), c.Project(), workItemType,
	)
	var wi WorkItem
	if err := c.patch(url, fields, &wi); err != nil {
		return nil, err
	}
	return &wi, nil
}

type Iteration struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Path string `json:"path"`
}

type iterationsResult struct {
	Value []Iteration `json:"value"`
}

type teamResult struct {
	Value []struct {
		Name string `json:"name"`
	} `json:"value"`
}

// GetCurrentIteration returns the current sprint iteration path.
// Uses ADO_TEAM if set, otherwise fetches the default team from the project.
func (c *Client) GetCurrentIteration() (string, error) {
	team := c.cfg.Team
	if team == "" {
		// Auto-discover: list project teams and use the first one (default team)
		teamURL := fmt.Sprintf(
			"%s/_apis/projects/%s/teams?$top=1&api-version=7.1",
			c.BaseURL(), c.Project(),
		)
		var tr teamResult
		if err := c.get(teamURL, &tr); err != nil {
			return "", fmt.Errorf("failed to list teams: %w", err)
		}
		if len(tr.Value) == 0 {
			return "", fmt.Errorf("no teams found in project")
		}
		team = tr.Value[0].Name
	}
	url := fmt.Sprintf(
		"%s/%s/%s/_apis/work/teamsettings/iterations?$timeframe=current&api-version=7.1",
		c.BaseURL(), c.Project(), team,
	)
	var result iterationsResult
	if err := c.get(url, &result); err != nil {
		return "", err
	}
	if len(result.Value) == 0 {
		return "", fmt.Errorf("no current iteration found")
	}
	return result.Value[0].Path, nil
}

func (c *Client) GetWorkItem(id int) (*WorkItem, error) {
	url := fmt.Sprintf(
		"%s/%s/_apis/wit/workitems/%d?$expand=all&api-version=7.1",
		c.BaseURL(), c.Project(), id,
	)
	var wi WorkItem
	if err := c.get(url, &wi); err != nil {
		return nil, err
	}
	return &wi, nil
}
