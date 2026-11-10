package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/rainhu/ado/internal/features/pr"
	"github.com/spf13/cobra"
)

var (
	prBranch     string
	prDesc       string
	prReviewer   string
	prOptional   string
	prAutoComplete bool

	prAssigned bool
	prCreated  bool
	prRequired bool
	prRepo     string

	prUpdate int
	prTitle  string

	prShow int
	prJSON bool
)

var prCmd = &cobra.Command{
	Use:   "pr [title]",
	Short: "List pull requests, or create a new one",
	Long: `Without arguments: list active PRs. By default lists PRs where you are a
required reviewer; use a category flag to change the set:
  --required   PRs where you are a required reviewer (default)
  --assigned   PRs where you are any reviewer
  --created    PRs you created
  --repo NAME  active PRs in a specific repository

With a title: create a new PR from the current branch.

With --show <id>: print one PR in full — status, branches, reviewers and
their votes, linked work items and the description. Works from any directory.

With --update <id>: edit an existing PR in the current repo. Only the fields you
pass are changed, so updating a description leaves the title alone.

Examples:
  ado pr
  ado pr --created
  ado pr --assigned
  ado pr --repo my-service
  ado pr --show 7308
  ado pr --show 7308 --json
  ado pr "Add login feature" -n main -r "John Doe" -o "Jane Doe" --auto-complete
  ado pr "Fix bug" -d "Fixes issue #123"
  ado pr --update 7308 -d "$(cat pr.md)"
  ado pr --update 7308 --title "fix(api): new title"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if prShow > 0 {
			if prUpdate > 0 || len(args) > 0 {
				return fmt.Errorf("--show cannot be combined with --update or a title")
			}
			req := &pr.ShowPRRequest{PRID: prShow, JSON: prJSON}
			return mediator.Send(context.Background(), req, os.Stdout)
		}
		if prJSON {
			return fmt.Errorf("--json is only supported with --show")
		}

		if prUpdate > 0 {
			title := prTitle
			if title == "" && len(args) > 0 {
				title = args[0]
			}
			req := &pr.UpdatePRRequest{
				PRID:        prUpdate,
				Title:       title,
				Description: prDesc,
			}
			return mediator.Send(context.Background(), req, os.Stdout)
		}

		if len(args) == 0 {
			req := &pr.ListMyPRsRequest{Category: prListCategory()}
			if req.Category == pr.PRRepo {
				req.RepoName = prRepo
			}
			return mediator.Send(context.Background(), req, os.Stdout)
		}

		req := &pr.CreatePRRequest{
			Title:        args[0],
			TargetBranch: prBranch,
			Description:  prDesc,
			Reviewer:     prReviewer,
			OptReviewer:  prOptional,
			AutoComplete: prAutoComplete,
		}
		return mediator.Send(context.Background(), req, os.Stdout)
	},
}

// prListCategory resolves the listing category from the mutually-exclusive
// category flags. Precedence: --repo > --created > --assigned > --required.
func prListCategory() pr.PRCategory {
	switch {
	case prRepo != "":
		return pr.PRRepo
	case prCreated:
		return pr.PRCreated
	case prAssigned:
		return pr.PRAssigned
	default:
		return pr.PRRequired
	}
}

func init() {
	prCmd.Flags().StringVarP(&prBranch, "branch", "n", "", "target branch (default: repo default branch)")
	prCmd.Flags().StringVarP(&prDesc, "desc", "d", "", "PR description")
	prCmd.Flags().StringVarP(&prReviewer, "reviewer", "r", "", "required reviewer (display name or email)")
	prCmd.Flags().StringVarP(&prOptional, "optional", "o", "", "optional reviewer (display name or email)")
	prCmd.Flags().BoolVar(&prAutoComplete, "auto-complete", false, "set auto-complete (squash merge, delete source branch)")
	prCmd.Flags().IntVar(&prUpdate, "update", 0, "update the PR with this ID instead of creating one")
	prCmd.Flags().StringVar(&prTitle, "title", "", "new title, used with --update")
	prCmd.Flags().IntVar(&prShow, "show", 0, "show full details of the PR with this ID")
	prCmd.Flags().BoolVar(&prJSON, "json", false, "output as JSON, used with --show")

	// Listing category flags (no args).
	prCmd.Flags().BoolVar(&prAssigned, "assigned", false, "list PRs where you are any reviewer")
	prCmd.Flags().BoolVar(&prCreated, "created", false, "list PRs you created")
	prCmd.Flags().BoolVar(&prRequired, "required", false, "list PRs where you are a required reviewer (default)")
	prCmd.Flags().StringVar(&prRepo, "repo", "", "list active PRs in the named repository")
	rootCmd.AddCommand(prCmd)
}
