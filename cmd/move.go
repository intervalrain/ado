package cmd

import (
	"context"
	"os"

	"github.com/rainhu/ado/internal/features/move"
	"github.com/spf13/cobra"
)

var (
	moveIteration string
	moveCurrent   bool
)

var moveCmd = &cobra.Command{
	Use:   "move <id> [id...]",
	Short: "Move work items to an iteration",
	Long: `Move one or more work items to a target iteration (sprint).

The iteration is matched by path, then by name (case-insensitive exact,
then substring). Use --current to move to the team's current sprint.

Examples:
  ado move 1234 --current
  ado move 1234 5678 --iteration "Sprint 12"
  ado move 1234 -i "MyProject\\Sprint 12"`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ids, err := parseIDs(args)
		if err != nil {
			return err
		}

		req := &move.MoveWorkItemRequest{
			IDs:       ids,
			Iteration: moveIteration,
			Current:   moveCurrent,
		}
		return mediator.Send(context.Background(), req, os.Stdout)
	},
}

func init() {
	moveCmd.Flags().StringVarP(&moveIteration, "iteration", "i", "", "target iteration name or path")
	moveCmd.Flags().BoolVar(&moveCurrent, "current", false, "move to the team's current sprint")
	rootCmd.AddCommand(moveCmd)
}
