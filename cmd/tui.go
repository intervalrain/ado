package cmd

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/rainhu/ado/internal/api"
	"github.com/rainhu/ado/internal/tui"
	"github.com/spf13/cobra"
)

var tuiQueryID string

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Interactive TUI for browsing work items",
	RunE: func(cmd *cobra.Command, args []string) error {
		id := tuiQueryID
		if id == "" {
			id = cfg.QueryID
		}

		client := api.NewClient(cfg)
		m := tui.NewModel(client, id, llmClient, cfg)
		p := tea.NewProgram(m)
		_, err := p.Run()
		return err
	},
}

func init() {
	tuiCmd.Flags().StringVarP(&tuiQueryID, "id", "i", "", "query ID (overrides ADO_QUERY_ID)")
	rootCmd.AddCommand(tuiCmd)
}
