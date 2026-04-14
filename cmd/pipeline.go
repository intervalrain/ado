package cmd

import (
	"context"
	"os"

	"github.com/rainhu/ado/internal/features/pipeline"
	"github.com/spf13/cobra"
)

var (
	pipelineDefID int
	pipelineTop   int
)

var pipelineCmd = &cobra.Command{
	Use:   "pipeline",
	Short: "List pipeline definitions and their latest builds",
	Long: `Without flags: list all pipeline definitions with their latest build status.
With -i: show recent builds for a specific pipeline definition.

Examples:
  ado pipeline
  ado pipeline -i 42
  ado pipeline -i 42 -t 10`,
	RunE: func(cmd *cobra.Command, args []string) error {
		req := &pipeline.ListPipelinesRequest{
			DefinitionID: pipelineDefID,
			Top:          pipelineTop,
		}
		return mediator.Send(context.Background(), req, os.Stdout)
	},
}

func init() {
	pipelineCmd.Flags().IntVarP(&pipelineDefID, "id", "i", 0, "pipeline definition ID (show recent builds)")
	pipelineCmd.Flags().IntVarP(&pipelineTop, "top", "t", 5, "number of recent builds to show (used with -i)")
	rootCmd.AddCommand(pipelineCmd)
}
