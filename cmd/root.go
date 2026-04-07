package cmd

import (
	"github.com/rainhu/ado/internal/api"
	"github.com/rainhu/ado/internal/behaviors"
	"github.com/rainhu/ado/internal/config"
	"github.com/rainhu/ado/internal/cqrs"
	"github.com/rainhu/ado/internal/features/query"
	"github.com/spf13/cobra"
)

var (
	mediator *cqrs.Mediator
	cfg      *config.Config
)

var rootCmd = &cobra.Command{
	Use:   "ado",
	Short: "Azure DevOps CLI tool",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error
		cfg, err = config.Load()
		if err != nil {
			return err
		}

		client := api.NewClient(cfg)
		mediator = cqrs.NewMediator()

		// Register behaviors (pipeline)
		mediator.Use(&behaviors.LoggingBehavior{})

		// Register handlers
		mediator.Register(query.RequestName, query.NewGetQueryHandler(client))

		return nil
	},
}

func Execute() error {
	return rootCmd.Execute()
}
