package cmd

import (
	"github.com/rainhu/ado/internal/api"
	"github.com/rainhu/ado/internal/behaviors"
	"github.com/rainhu/ado/internal/config"
	"github.com/rainhu/ado/internal/cqrs"
	"github.com/rainhu/ado/internal/features/create"
	"github.com/rainhu/ado/internal/features/pipeline"
	"github.com/rainhu/ado/internal/features/pr"
	"github.com/rainhu/ado/internal/features/query"
	"github.com/rainhu/ado/internal/features/summary"
	"github.com/rainhu/ado/internal/llm"
	"github.com/spf13/cobra"
)

var (
	mediator  *cqrs.Mediator
	cfg       *config.Config
	llmClient llm.Client
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

		// TUI can start without org/pat so users can configure via Settings
		if cmd.Name() != "tui" {
			if err := cfg.Validate(); err != nil {
				return err
			}
		}

		client := api.NewClient(cfg)
		mediator = cqrs.NewMediator()

		// Register behaviors (pipeline)
		mediator.Use(&behaviors.LoggingBehavior{})

		// Register handlers
		mediator.Register(query.RequestName, query.NewGetQueryHandler(client))
		mediator.Register(create.RequestName, create.NewCreateWorkItemHandler(client))
		mediator.Register(pr.ListRequestName, pr.NewListMyPRsHandler(client))
		mediator.Register(pr.CreateRequestName, pr.NewCreatePRHandler(client))
		mediator.Register(pipeline.ListRequestName, pipeline.NewListPipelinesHandler(client))

		// LLM client (non-fatal if API key missing)
		apiKey := cfg.LLM.ResolvedAPIKey()
		if apiKey != "" {
			llmClient, _ = llm.New(llm.Config{
				Provider:  cfg.LLM.Provider,
				Model:     cfg.LLM.Model,
				APIKey:    apiKey,
				BaseURL:   cfg.LLM.BaseURL,
				MaxTokens: cfg.LLM.MaxTokens,
			})
		}
		mediator.Register(summary.GenerateRequestName, summary.NewGenerateSummaryHandler(client, llmClient, cfg))
		mediator.Register(summary.ResolveRequestName, summary.NewResolveSummaryItemsHandler(client))

		return nil
	},
}

func Execute() error {
	return rootCmd.Execute()
}
