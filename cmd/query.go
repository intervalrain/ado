package cmd

import (
	"context"
	"os"

	"github.com/rainhu/ado/internal/features/query"
	"github.com/spf13/cobra"
)

var (
	queryID   string
	queryJSON bool
)

var queryCmd = &cobra.Command{
	Use:   "query",
	Short: "Run a saved query and list work items",
	RunE: func(cmd *cobra.Command, args []string) error {
		id := queryID
		if id == "" {
			id = cfg.QueryID
		}

		req := &query.GetQueryRequest{QueryID: id, JSON: queryJSON}
		return mediator.Send(context.Background(), req, os.Stdout)
	},
}

func init() {
	queryCmd.Flags().StringVarP(&queryID, "id", "i", "", "query ID (overrides ADO_QUERY_ID)")
	queryCmd.Flags().BoolVar(&queryJSON, "json", false, "output work items as JSON")
	rootCmd.AddCommand(queryCmd)
}
