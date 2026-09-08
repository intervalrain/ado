package pr

import (
	"context"
	"fmt"
	"io"

	"github.com/rainhu/ado/internal/api"
	"github.com/rainhu/ado/internal/cqrs"
	"github.com/rainhu/ado/internal/git"
)

const UpdateRequestName = "UpdatePR"

// UpdatePRRequest edits an existing pull request. Only the non-empty fields are
// sent, so updating a description never disturbs the title.
type UpdatePRRequest struct {
	PRID        int
	Title       string
	Description string
}

func (r *UpdatePRRequest) RequestName() string { return UpdateRequestName }

type UpdatePRHandler struct {
	client *api.Client
}

func NewUpdatePRHandler(client *api.Client) *UpdatePRHandler {
	return &UpdatePRHandler{client: client}
}

func (h *UpdatePRHandler) Handle(ctx context.Context, req cqrs.Request, w io.Writer) error {
	r := req.(*UpdatePRRequest)

	if r.Title == "" && r.Description == "" {
		return fmt.Errorf("nothing to update: pass --title or --desc")
	}

	repoName, err := git.RepoName()
	if err != nil {
		return err
	}

	repoID, err := h.resolveRepoID(repoName)
	if err != nil {
		return err
	}

	updated, err := h.client.UpdatePullRequest(api.UpdatePullRequestInput{
		RepoID:      repoID,
		PRID:        r.PRID,
		Title:       r.Title,
		Description: r.Description,
	})
	if err != nil {
		return fmt.Errorf("update PR #%d: %w", r.PRID, err)
	}

	var changed []string
	if r.Title != "" {
		changed = append(changed, "title")
	}
	if r.Description != "" {
		changed = append(changed, "description")
	}
	fmt.Fprintf(w, "Updated PR #%d (%s): %s\n", updated.ID, joinWords(changed), updated.Title)
	fmt.Fprintf(w, "\n%s\n", h.client.PRWebURL(repoName, updated.ID))

	return nil
}

func (h *UpdatePRHandler) resolveRepoID(repoName string) (string, error) {
	repos, err := h.client.ListRepositories()
	if err != nil {
		return "", fmt.Errorf("list repos: %w", err)
	}
	for _, r := range repos {
		if r.Name == repoName {
			return r.ID, nil
		}
	}
	return "", fmt.Errorf("repository %q not found in project", repoName)
}

// joinWords renders a short list as "a", "a and b".
func joinWords(words []string) string {
	switch len(words) {
	case 0:
		return ""
	case 1:
		return words[0]
	default:
		return words[0] + " and " + joinWords(words[1:])
	}
}
