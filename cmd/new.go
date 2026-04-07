package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/rainhu/ado/internal/features/create"
	"github.com/spf13/cobra"
)

var (
	newType     string
	newDesc     string
	newEstimate float64
	newTags     string
)

var validTypes = map[string]string{
	"task":       "Task",
	"bug":        "Bug",
	"epic":       "Epic",
	"issue":      "Issue",
	"user story": "User Story",
	"userstory":  "User Story",
	"story":      "User Story",
}

var newCmd = &cobra.Command{
	Use:   "new <title>",
	Short: "Create a new work item",
	Long: `Create a new work item in Azure DevOps.

Examples:
  ado new "Fix login bug"
  ado new "Add feature" --type bug
  ado new "Implement API" --desc "REST API for users" --est 8`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		title := strings.Join(args, " ")

		wiType, ok := validTypes[strings.ToLower(newType)]
		if !ok {
			return fmt.Errorf("invalid type %q, valid types: task, bug, epic, issue, user story", newType)
		}

		req := &create.CreateWorkItemRequest{
			Title:       title,
			Type:        wiType,
			Description: newDesc,
			Estimate:    newEstimate,
			Tags:        newTags,
		}
		return mediator.Send(context.Background(), req, os.Stdout)
	},
}

func init() {
	newCmd.Flags().StringVarP(&newType, "type", "t", "task", "work item type: task, bug, epic, issue, user story")
	newCmd.Flags().StringVarP(&newDesc, "desc", "d", "", "description")
	newCmd.Flags().Float64VarP(&newEstimate, "est", "e", 6, "original estimate (hours), also sets remaining work")
	newCmd.Flags().StringVar(&newTags, "tags", "", "tags (semicolon-separated)")
	rootCmd.AddCommand(newCmd)
}