package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/rainhu/ado/internal/features/show"
	"github.com/spf13/cobra"
)

var showJSON bool

var showCmd = &cobra.Command{
	Use:   "show <id> [id...]",
	Short: "Show full details of work items, including description",
	Long: `Show the full details of one or more work items: title, state, assignee,
iteration, tags, estimate/remaining, and rich-text fields (Description,
Repro Steps for bugs, Acceptance Criteria for user stories) rendered as
plain text.

Examples:
  ado show 1234
  ado show 1234 5678
  ado show 1234 --json`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ids := make([]int, 0, len(args))
		for _, a := range args {
			id, err := strconv.Atoi(a)
			if err != nil {
				return fmt.Errorf("invalid work item id %q", a)
			}
			ids = append(ids, id)
		}

		req := &show.ShowWorkItemRequest{IDs: ids, JSON: showJSON}
		return mediator.Send(context.Background(), req, os.Stdout)
	},
}

func init() {
	showCmd.Flags().BoolVar(&showJSON, "json", false, "output work items as JSON")
	rootCmd.AddCommand(showCmd)
}
