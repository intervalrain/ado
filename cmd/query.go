package cmd

import (
	"context"
	"os"

	"github.com/rainhu/ado/internal/features/query"
	"github.com/spf13/cobra"
)

var queryID string

var queryCmd = &cobra.Command{
	Use:   "query",
	Short: "Run a saved query and list work items",
	RunE: func(cmd *cobra.Command, args []string) error {
		id := queryID
		if id == "" {
			id = cfg.QueryID
		}

		req := &query.GetQueryRequest{QueryID: id}
		return mediator.Send(context.Background(), req, os.Stdout)
	},
}

func init() {
	queryCmd.Flags().StringVarP(&queryID, "id", "i", "", "query ID (overrides ADO_QUERY_ID)")
	rootCmd.AddCommand(queryCmd)
}
