package cmd

import (
	"context"
	"os"
	"strings"

	"github.com/rainhu/ado/internal/features/summary"
	"github.com/spf13/cobra"
)

var (
	summaryDays  int
	summaryRepos string
	summaryTmpl  string
)

var summaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Generate a summary report from git commits and ADO work items",
	RunE: func(cmd *cobra.Command, args []string) error {
		var repos []string
		if summaryRepos != "" {
			repos = strings.Split(summaryRepos, ",")
		}

		req := &summary.GenerateSummaryRequest{
			Days:     summaryDays,
			Repos:    repos,
			Template: summaryTmpl,
		}
		return mediator.Send(context.Background(), req, os.Stdout)
	},
}

func init() {
	summaryCmd.Flags().IntVarP(&summaryDays, "days", "d", 0, "number of days to look back (default from config or 7)")
	summaryCmd.Flags().StringVarP(&summaryRepos, "repos", "r", "", "comma-separated repo paths (overrides config)")
	summaryCmd.Flags().StringVarP(&summaryTmpl, "template", "t", "", "path to report template (overrides config)")
	rootCmd.AddCommand(summaryCmd)
}
