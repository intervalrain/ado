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
)

var prCmd = &cobra.Command{
	Use:   "pr [title]",
	Short: "List my pull requests, or create a new one",
	Long: `Without arguments: list all active PRs assigned to you.
With a title: create a new PR from the current branch.

Examples:
  ado pr
  ado pr "Add login feature" -n main -r "John Doe" -o "Jane Doe" --auto-complete
  ado pr "Fix bug" -d "Fixes issue #123"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			req := &pr.ListMyPRsRequest{}
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

func init() {
	prCmd.Flags().StringVarP(&prBranch, "branch", "n", "", "target branch (default: repo default branch)")
	prCmd.Flags().StringVarP(&prDesc, "desc", "d", "", "PR description")
	prCmd.Flags().StringVarP(&prReviewer, "reviewer", "r", "", "required reviewer (display name or email)")
	prCmd.Flags().StringVarP(&prOptional, "optional", "o", "", "optional reviewer (display name or email)")
	prCmd.Flags().BoolVar(&prAutoComplete, "auto-complete", false, "set auto-complete (squash merge, delete source branch)")
	rootCmd.AddCommand(prCmd)
}
