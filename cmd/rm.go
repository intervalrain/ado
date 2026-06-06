package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/rainhu/ado/internal/features/remove"
	"github.com/spf13/cobra"
)

var rmYes bool

var rmCmd = &cobra.Command{
	Use:     "rm <id> [id...]",
	Aliases: []string{"delete"},
	Short:   "Delete work items (send to recycle bin)",
	Long: `Delete one or more work items. They are sent to the project recycle
bin and can be restored from the Azure DevOps web UI.

Prompts for confirmation unless --yes is given.

Examples:
  ado rm 1234
  ado rm 1234 5678 9012
  ado rm 1234 --yes`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		ids, err := parseIDs(args)
		if err != nil {
			return err
		}

		if !rmYes {
			fmt.Printf("Delete %d work item(s) %v? This sends them to the recycle bin. [y/N] ", len(ids), ids)
			reader := bufio.NewReader(os.Stdin)
			line, _ := reader.ReadString('\n')
			ans := strings.ToLower(strings.TrimSpace(line))
			if ans != "y" && ans != "yes" {
				fmt.Println("Cancelled.")
				return nil
			}
		}

		req := &remove.RemoveWorkItemRequest{IDs: ids}
		return mediator.Send(context.Background(), req, os.Stdout)
	},
}

func init() {
	rmCmd.Flags().BoolVarP(&rmYes, "yes", "y", false, "skip the confirmation prompt")
	rootCmd.AddCommand(rmCmd)
}

// parseIDs converts positional args to work item IDs, erroring on the first
// non-numeric value.
func parseIDs(args []string) ([]int, error) {
	ids := make([]int, 0, len(args))
	for _, a := range args {
		id, err := strconv.Atoi(a)
		if err != nil {
			return nil, fmt.Errorf("invalid work item id %q", a)
		}
		ids = append(ids, id)
	}
	return ids, nil
}
