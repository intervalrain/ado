package cmd

import (
	"fmt"
	"strings"

	"github.com/rainhu/ado/internal/config"
	"github.com/spf13/cobra"
)

var modelCmd = &cobra.Command{
	Use:     "model",
	Aliases: []string{"models"},
	Short:   "Manage LLM model profiles (provider + model + API key)",
	Long: `Model profiles let you save multiple LLM configurations and switch between
them quickly without editing ~/.ado/config.yaml.

Profiles are stored as YAML files under ~/.ado/models/<name>.yaml, and the
active profile is tracked in ~/.ado/models/current.txt.`,
}

func init() {
	modelCmd.AddCommand(modelLsCmd())
	modelCmd.AddCommand(modelSelectCmd())
	modelCmd.AddCommand(modelAddCmd())
	modelCmd.AddCommand(modelRmCmd())
	modelCmd.AddCommand(modelCurrentCmd())
	rootCmd.AddCommand(modelCmd)
}

func modelLsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List model profiles",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			names := config.ListModelProfiles()
			if len(names) == 0 {
				fmt.Println("No model profiles configured. Run `ado model add <name> <provider> <model>` to create one.")
				return nil
			}
			cur := config.CurrentModel()

			type row struct{ name, provider, model, desc string }
			rows := make([]row, 0, len(names))
			nameW, provW, modelW, descW := len("Name"), len("Provider"), len("Model"), len("Description")
			for _, n := range names {
				p, err := config.LoadModelProfile(n)
				if err != nil {
					continue
				}
				display := n
				if n == cur {
					display += "*"
				}
				desc := p.Description
				if desc == "" && p.BaseURL != "" {
					desc = p.BaseURL
				}
				r := row{display, p.Provider, p.Model, desc}
				if len(r.name) > nameW {
					nameW = len(r.name)
				}
				if len(r.provider) > provW {
					provW = len(r.provider)
				}
				if len(r.model) > modelW {
					modelW = len(r.model)
				}
				if len(r.desc) > descW {
					descW = len(r.desc)
				}
				rows = append(rows, r)
			}

			titleW := nameW + provW + modelW + descW + 9 // 3 separators of " │ "
			title := "Model Profiles"
			pad := titleW - len(title)
			lpad := pad / 2
			rpad := pad - lpad

			fmt.Printf("╭%s╮\n", strings.Repeat("─", titleW+2))
			fmt.Printf("│ %s%s%s │\n", strings.Repeat(" ", lpad), title, strings.Repeat(" ", rpad))
			fmt.Printf("├%s┬%s┬%s┬%s┤\n",
				strings.Repeat("─", nameW+2), strings.Repeat("─", provW+2),
				strings.Repeat("─", modelW+2), strings.Repeat("─", descW+2))
			fmt.Printf("│ %-*s │ %-*s │ %-*s │ %-*s │\n",
				nameW, "Name", provW, "Provider", modelW, "Model", descW, "Description")
			fmt.Printf("├%s┼%s┼%s┼%s┤\n",
				strings.Repeat("─", nameW+2), strings.Repeat("─", provW+2),
				strings.Repeat("─", modelW+2), strings.Repeat("─", descW+2))
			for _, r := range rows {
				fmt.Printf("│ %-*s │ %-*s │ %-*s │ %-*s │\n",
					nameW, r.name, provW, r.provider, modelW, r.model, descW, r.desc)
			}
			fmt.Printf("╰%s┴%s┴%s┴%s╯\n",
				strings.Repeat("─", nameW+2), strings.Repeat("─", provW+2),
				strings.Repeat("─", modelW+2), strings.Repeat("─", descW+2))
			return nil
		},
	}
}

func modelSelectCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "select <name>",
		Short: "Select the active model profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			name := args[0]
			p, err := config.LoadModelProfile(name)
			if err != nil {
				return fmt.Errorf("profile %q not found", name)
			}
			if err := config.SetCurrentModel(name); err != nil {
				return err
			}
			fmt.Printf("Selected model %q (%s / %s)\n", name, p.Provider, p.Model)
			return nil
		},
	}
}

func modelAddCmd() *cobra.Command {
	var (
		description string
		apiKey      string
		baseURL     string
		maxTokens   int
	)
	cmd := &cobra.Command{
		Use:   "add <name> <provider> <model>",
		Short: "Add a model profile",
		Long: `Add a new model profile. Provider is one of: claude, openai, gemini, ollama.

claude / openai / gemini require --api-key. ollama only needs --base-url
(defaults to http://localhost:11434).

Examples:
  ado model add sonnet claude claude-sonnet-4-20250514 \
    --api-key sk-ant-... -d "Anthropic default"

  ado model add gpt4 openai gpt-4o-mini --api-key sk-...

  ado model add gemini-flash gemini gemini-2.5-flash --api-key ...

  ado model add local ollama llama3.2 \
    --base-url http://localhost:11434`,
		Args: cobra.ExactArgs(3),
		RunE: func(_ *cobra.Command, args []string) error {
			p := &config.ModelProfile{
				Name:        args[0],
				Provider:    args[1],
				Model:       args[2],
				Description: description,
				APIKey:      apiKey,
				BaseURL:     baseURL,
				MaxTokens:   maxTokens,
			}
			if err := config.SaveModelProfile(p); err != nil {
				return err
			}
			if config.CurrentModel() == "" {
				_ = config.SetCurrentModel(p.Name)
			}
			fmt.Printf("Added model %q (%s / %s)\n", p.Name, p.Provider, p.Model)
			return nil
		},
	}
	cmd.Flags().StringVarP(&description, "description", "d", "", "Profile description")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key (required for claude/openai/gemini)")
	cmd.Flags().StringVar(&baseURL, "base-url", "", "Base URL (ollama only; defaults to http://localhost:11434)")
	cmd.Flags().IntVar(&maxTokens, "max-tokens", 0, "Max output tokens (default 4096 when unset)")
	return cmd
}

func modelRmCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <name>",
		Short: "Remove a model profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			name := args[0]
			if _, err := config.LoadModelProfile(name); err != nil {
				return fmt.Errorf("profile %q not found", name)
			}
			if err := config.RemoveModelProfile(name); err != nil {
				return err
			}
			if config.CurrentModel() == name {
				_ = config.SetCurrentModel("")
			}
			fmt.Printf("Removed model %q\n", name)
			return nil
		},
	}
}

func modelCurrentCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "current",
		Short: "Print the active model profile",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			cur := config.CurrentModel()
			if cur == "" {
				fmt.Println("(none — using inline llm: section from ~/.ado/config.yaml)")
				return nil
			}
			p, err := config.LoadModelProfile(cur)
			if err != nil {
				return fmt.Errorf("active profile %q unreadable: %w", cur, err)
			}
			fmt.Printf("%s\n  provider:   %s\n  model:      %s\n", cur, p.Provider, p.Model)
			if p.BaseURL != "" {
				fmt.Printf("  base_url:   %s\n", p.BaseURL)
			}
			if p.APIKey != "" {
				fmt.Printf("  api_key:    %s\n", maskSecret(p.APIKey))
			}
			if p.Description != "" {
				fmt.Printf("  description: %s\n", p.Description)
			}
			return nil
		},
	}
}

func maskSecret(s string) string {
	n := len(s)
	if n == 0 {
		return ""
	}
	if n <= 8 {
		return strings.Repeat("*", n)
	}
	return s[:4] + strings.Repeat("*", n-8) + s[n-4:]
}
