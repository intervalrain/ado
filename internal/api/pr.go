package api

import (
	"fmt"
)

type Repository struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type reposResult struct {
	Value []Repository `json:"value"`
}

type PullRequest struct {
	ID            int          `json:"pullRequestId"`
	Title         string       `json:"title"`
	Status        string       `json:"status"`
	CreatedBy     Identity     `json:"createdBy"`
	SourceRefName string       `json:"sourceRefName"`
	TargetRefName string       `json:"targetRefName"`
	Reviewers     []PRReviewer `json:"reviewers"`
	Repository    Repository   `json:"repository"`
	AutoCompleteSetBy *Identity `json:"autoCompleteSetBy,omitempty"`
}

type PRReviewer struct {
	ID          string `json:"id"`
	DisplayName string `json:"displayName"`
	Vote        int    `json:"vote"`
	IsRequired  bool   `json:"isRequired"`
}

// VoteLabel returns a human-readable label for the reviewer's vote.
func (r PRReviewer) VoteLabel() string {
	switch {
	case r.Vote == 10:
		return "Approved"
	case r.Vote == 5:
		return "Approved w/ suggestions"
	case r.Vote == 0:
		return "No vote"
	case r.Vote == -5:
		return "Waiting"
	case r.Vote == -10:
		return "Rejected"
	default:
		return fmt.Sprintf("Vote(%d)", r.Vote)
	}
}

// RequiredLabel returns "Required" or "Optional".
func (r PRReviewer) RequiredLabel() string {
	if r.IsRequired {
		return "Required"
	}
	return "Optional"
}

type pullRequestsResult struct {
	Value []PullRequest `json:"value"`
}

// ListRepositories returns all Git repositories in the project.
func (c *Client) ListRepositories() ([]Repository, error) {
	url := fmt.Sprintf(
		"%s/%s/_apis/git/repositories?api-version=7.1",
		c.BaseURL(), c.Project(),
	)
	var result reposResult
	if err := c.get(url, &result); err != nil {
		return nil, err
	}
	return result.Value, nil
}

// ListPullRequests returns active pull requests for a repository.
func (c *Client) ListPullRequests(repoID string) ([]PullRequest, error) {
	url := fmt.Sprintf(
		"%s/%s/_apis/git/repositories/%s/pullrequests?searchCriteria.status=active&api-version=7.1",
		c.BaseURL(), c.Project(), repoID,
	)
	var result pullRequestsResult
	if err := c.get(url, &result); err != nil {
		return nil, err
	}
	return result.Value, nil
}

// PRWebURL builds the browser URL for a pull request.
func (c *Client) PRWebURL(repoName string, prID int) string {
	return fmt.Sprintf("%s/%s/_git/%s/pullrequest/%d",
		c.BaseURL(), c.Project(), repoName, prID)
}

// ResolveMyIdentity resolves the current user's identity and caches it.
func (c *Client) ResolveMyIdentity() (*IdentitySearchResult, error) {
	return c.SearchIdentity(c.cfg.Assignee)
}

// ListMyCreatedPullRequests returns active PRs created by the current user.
func (c *Client) ListMyCreatedPullRequests() ([]PullRequest, error) {
	me, err := c.ResolveMyIdentity()
	if err != nil {
		return nil, fmt.Errorf("resolve own identity: %w", err)
	}

	u := fmt.Sprintf(
		"%s/%s/_apis/git/pullrequests?searchCriteria.status=active&searchCriteria.creatorId=%s&api-version=7.1",
		c.BaseURL(), c.Project(), me.ID,
	)
	var result pullRequestsResult
	if err := c.get(u, &result); err != nil {
		return nil, err
	}
	return result.Value, nil
}

// ListMyReviewPullRequests returns active PRs where the current user is a reviewer.
func (c *Client) ListMyReviewPullRequests() ([]PullRequest, error) {
	me, err := c.ResolveMyIdentity()
	if err != nil {
		return nil, fmt.Errorf("resolve own identity: %w", err)
	}

	u := fmt.Sprintf(
		"%s/%s/_apis/git/pullrequests?searchCriteria.status=active&searchCriteria.reviewerId=%s&api-version=7.1",
		c.BaseURL(), c.Project(), me.ID,
	)
	var result pullRequestsResult
	if err := c.get(u, &result); err != nil {
		return nil, err
	}
	return result.Value, nil
}

// ListMyPullRequests returns active PRs where the current user is a required reviewer.
func (c *Client) ListMyPullRequests() ([]PullRequest, error) {
	me, err := c.ResolveMyIdentity()
	if err != nil {
		return nil, fmt.Errorf("resolve own identity: %w", err)
	}

	u := fmt.Sprintf(
		"%s/%s/_apis/git/pullrequests?searchCriteria.status=active&searchCriteria.reviewerId=%s&api-version=7.1",
		c.BaseURL(), c.Project(), me.ID,
	)
	var result pullRequestsResult
	if err := c.get(u, &result); err != nil {
		return nil, err
	}

	// Filter: only keep PRs where I am a required reviewer
	var filtered []PullRequest
	for _, pr := range result.Value {
		for _, r := range pr.Reviewers {
			if r.IsRequired && r.ID == me.ID {
				filtered = append(filtered, pr)
				break
			}
		}
	}
	return filtered, nil
}

// IdentitySearchResult represents an identity from the identity picker API.
type IdentitySearchResult struct {
	ID          string `json:"localId"`
	DisplayName string `json:"displayName"`
	SignInAddress string `json:"signInAddress"`
}

type identitySearchResponse struct {
	Results []struct {
		Identities []IdentitySearchResult `json:"identities"`
	} `json:"results"`
}

// SearchIdentity finds a user identity by display name or email prefix.
func (c *Client) SearchIdentity(query string) (*IdentitySearchResult, error) {
	u := fmt.Sprintf(
		"%s/_apis/identitypicker/identities?api-version=7.1-preview.1",
		c.BaseURL(),
	)
	body := map[string]any{
		"query":              query,
		"identityTypes":      []string{"user"},
		"operationScopes":    []string{"ims", "source"},
		"options":            map[string]any{"MinResults": 1, "MaxResults": 5},
	}
	var resp identitySearchResponse
	if err := c.post(u, body, &resp); err != nil {
		return nil, fmt.Errorf("identity search: %w", err)
	}
	if len(resp.Results) == 0 || len(resp.Results[0].Identities) == 0 {
		return nil, fmt.Errorf("no identity found for %q", query)
	}
	return &resp.Results[0].Identities[0], nil
}

// CreatePullRequestInput holds the parameters for creating a PR.
type CreatePullRequestInput struct {
	RepoID      string
	Title       string
	Description string
	SourceBranch string
	TargetBranch string
}

// CreatePullRequest creates a new pull request.
func (c *Client) CreatePullRequest(input CreatePullRequestInput) (*PullRequest, error) {
	u := fmt.Sprintf(
		"%s/%s/_apis/git/repositories/%s/pullrequests?api-version=7.1",
		c.BaseURL(), c.Project(), input.RepoID,
	)
	body := map[string]any{
		"sourceRefName": "refs/heads/" + input.SourceBranch,
		"targetRefName": "refs/heads/" + input.TargetBranch,
		"title":         input.Title,
		"description":   input.Description,
	}
	var pr PullRequest
	if err := c.post(u, body, &pr); err != nil {
		return nil, err
	}
	return &pr, nil
}

// AddPRReviewer adds a reviewer to a pull request.
func (c *Client) AddPRReviewer(repoID string, prID int, reviewerID string, isRequired bool) error {
	u := fmt.Sprintf(
		"%s/%s/_apis/git/repositories/%s/pullrequests/%d/reviewers/%s?api-version=7.1",
		c.BaseURL(), c.Project(), repoID, prID, reviewerID,
	)
	body := map[string]any{
		"vote":       0,
		"isRequired": isRequired,
	}
	return c.put(u, body, nil)
}

// SetAutoComplete enables auto-complete on a PR (merges when all policies pass).
func (c *Client) SetAutoComplete(repoID string, prID int, creatorID string) error {
	u := fmt.Sprintf(
		"%s/%s/_apis/git/repositories/%s/pullrequests/%d?api-version=7.1",
		c.BaseURL(), c.Project(), repoID, prID,
	)
	ops := map[string]any{
		"autoCompleteSetBy": map[string]string{"id": creatorID},
		"completionOptions": map[string]any{
			"deleteSourceBranch": true,
			"mergeStrategy":      "squash",
		},
	}
	return c.patchJSON(u, ops, nil)
}
