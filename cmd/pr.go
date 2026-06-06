package cmd

import (
	"context"
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

Examples:
  ado pr
  ado pr --created
  ado pr --assigned
  ado pr --repo my-service
  ado pr "Add login feature" -n main -r "John Doe" -o "Jane Doe" --auto-complete
  ado pr "Fix bug" -d "Fixes issue #123"`,
	RunE: func(cmd *cobra.Command, args []string) error {
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

	// Listing category flags (no args).
	prCmd.Flags().BoolVar(&prAssigned, "assigned", false, "list PRs where you are any reviewer")
	prCmd.Flags().BoolVar(&prCreated, "created", false, "list PRs you created")
	prCmd.Flags().BoolVar(&prRequired, "required", false, "list PRs where you are a required reviewer (default)")
	prCmd.Flags().StringVar(&prRepo, "repo", "", "list active PRs in the named repository")
	rootCmd.AddCommand(prCmd)
}
