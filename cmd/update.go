package cmd

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/rainhu/ado/internal/features/update"
	"github.com/spf13/cobra"
)

var (
	updateTitle     string
	updateState     string
	updateTags      string
	updateEstimate  float64
	updateRemaining float64
)

var updateCmd = &cobra.Command{
	Use:   "update <id>",
	Short: "Update fields of a work item (title, state, tags, estimate, remaining)",
	Long: `Update one or more fields of an existing work item.

Only the flags you pass are changed; everything else is left untouched.
Mirrors the inline-editable columns in the query TUI.

Examples:
  ado update 1234 --state Active
  ado update 1234 --title "New title" --est 4
  ado update 1234 --tags "frontend; urgent" --remaining 2`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id, err := strconv.Atoi(args[0])
		if err != nil {
			return fmt.Errorf("invalid work item id %q", args[0])
		}

		req := &update.UpdateWorkItemRequest{ID: id}
		if cmd.Flags().Changed("title") {
			req.Title = &updateTitle
		}
		if cmd.Flags().Changed("state") {
			req.State = &updateState
		}
		if cmd.Flags().Changed("tags") {
			req.Tags = &updateTags
		}
		if cmd.Flags().Changed("est") {
			req.Estimate = &updateEstimate
		}
		if cmd.Flags().Changed("remaining") {
			req.Remaining = &updateRemaining
		}

		return mediator.Send(context.Background(), req, os.Stdout)
	},
}

func init() {
	updateCmd.Flags().StringVarP(&updateTitle, "title", "T", "", "new title")
	updateCmd.Flags().StringVarP(&updateState, "state", "s", "", "new state (e.g. New, Active, Closed)")
	updateCmd.Flags().StringVar(&updateTags, "tags", "", "tags (semicolon-separated; replaces existing)")
	updateCmd.Flags().Float64VarP(&updateEstimate, "est", "e", 0, "original estimate (hours)")
	updateCmd.Flags().Float64Var(&updateRemaining, "remaining", 0, "remaining work (hours)")
	rootCmd.AddCommand(updateCmd)
}
