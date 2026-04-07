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

type patchOp struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	Value any    `json:"value"`
}

func (c *Client) UpdateWorkItemField(id int, fieldPath string, value any) error {
	url := fmt.Sprintf(
		"%s/%s/_apis/wit/workitems/%d?api-version=7.1",
		c.BaseURL(), c.Project(), id,
	)
	ops := []patchOp{{Op: "replace", Path: "/fields/" + fieldPath, Value: value}}
	return c.patch(url, ops, nil)
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
