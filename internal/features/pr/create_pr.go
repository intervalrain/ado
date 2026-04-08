package pr

import (
	"context"
	"fmt"
	"io"

	"github.com/rainhu/ado/internal/api"
	"github.com/rainhu/ado/internal/cqrs"
	"github.com/rainhu/ado/internal/git"
)

const CreateRequestName = "CreatePR"

type CreatePRRequest struct {
	Title        string
	TargetBranch string
	Description  string
	Reviewer     string
	OptReviewer  string
	AutoComplete bool
}

func (r *CreatePRRequest) RequestName() string { return CreateRequestName }

type CreatePRHandler struct {
	client *api.Client
}

func NewCreatePRHandler(client *api.Client) *CreatePRHandler {
	return &CreatePRHandler{client: client}
}

func (h *CreatePRHandler) Handle(ctx context.Context, req cqrs.Request, w io.Writer) error {
	r := req.(*CreatePRRequest)

	// Detect current branch
	srcBranch, err := git.CurrentBranch()
	if err != nil {
		return err
	}

	// Detect repo name and resolve repo ID
	repoName, err := git.RepoName()
	if err != nil {
		return err
	}

	repoID, err := h.resolveRepoID(repoName)
	if err != nil {
		return err
	}

	// Default target branch
	targetBranch := r.TargetBranch
	if targetBranch == "" {
		targetBranch = git.DefaultBranch()
	}

	// Ensure source branch is pushed to remote
	if !git.HasRemoteBranch(srcBranch) {
		fmt.Fprintf(w, "Pushing branch %s to origin...\n", srcBranch)
		if err := git.PushBranch(srcBranch); err != nil {
			return fmt.Errorf("push branch: %w", err)
		}
	}

	fmt.Fprintf(w, "Creating PR: %s → %s in %s\n", srcBranch, targetBranch, repoName)

	pr, err := h.client.CreatePullRequest(api.CreatePullRequestInput{
		RepoID:       repoID,
		Title:        r.Title,
		Description:  r.Description,
		SourceBranch: srcBranch,
		TargetBranch: targetBranch,
	})
	if err != nil {
		return fmt.Errorf("create PR: %w", err)
	}

	fmt.Fprintf(w, "Created PR #%d: %s\n", pr.ID, pr.Title)

	// Add required reviewer
	if r.Reviewer != "" {
		if err := h.addReviewer(w, repoID, pr.ID, r.Reviewer, true); err != nil {
			fmt.Fprintf(w, "Warning: failed to add required reviewer %q: %v\n", r.Reviewer, err)
		}
	}

	// Add optional reviewer
	if r.OptReviewer != "" {
		if err := h.addReviewer(w, repoID, pr.ID, r.OptReviewer, false); err != nil {
			fmt.Fprintf(w, "Warning: failed to add optional reviewer %q: %v\n", r.OptReviewer, err)
		}
	}

	// Set auto-complete
	if r.AutoComplete {
		identity, err := h.client.SearchIdentity(h.client.Config().Assignee)
		if err != nil {
			fmt.Fprintf(w, "Warning: failed to resolve identity for auto-complete: %v\n", err)
		} else {
			if err := h.client.SetAutoComplete(repoID, pr.ID, identity.ID); err != nil {
				fmt.Fprintf(w, "Warning: failed to set auto-complete: %v\n", err)
			} else {
				fmt.Fprintln(w, "Auto-complete enabled (squash merge, delete source branch)")
			}
		}
	}

	url := h.client.PRWebURL(repoName, pr.ID)
	fmt.Fprintf(w, "\n%s\n", url)

	return nil
}

func (h *CreatePRHandler) resolveRepoID(repoName string) (string, error) {
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

func (h *CreatePRHandler) addReviewer(w io.Writer, repoID string, prID int, name string, required bool) error {
	identity, err := h.client.SearchIdentity(name)
	if err != nil {
		return err
	}
	label := "optional"
	if required {
		label = "required"
	}
	if err := h.client.AddPRReviewer(repoID, prID, identity.ID, required); err != nil {
		return err
	}
	fmt.Fprintf(w, "Added %s reviewer: %s\n", label, identity.DisplayName)
	return nil
}
