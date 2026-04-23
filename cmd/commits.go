package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/rainhu/ado/internal/git"
	"github.com/spf13/cobra"
)

var (
	commitsDays   int
	commitsRepos  string
	commitsAuthor string
	commitsRaw    bool
)

var commitsCmd = &cobra.Command{
	Use:   "commits",
	Short: "List git commits that would be fed to `ado summary`",
	Long: `Runs the same commit-collection logic that summary uses, and prints
the commits without invoking the LLM. Useful for verifying which commits
the tool sees across your configured repos.

Flags mirror summary defaults from ~/.ado/config.yaml and can be overridden.`,
	Example: `  ado commits                           # use config defaults
  ado commits -d 14                    # look back 14 days
  ado commits -r /path/to/repo,/path2  # override repo list
  ado commits -a "Rain Hu"             # override author filter
  ado commits --raw                    # machine-readable one-per-line`,
	RunE: func(cmd *cobra.Command, args []string) error {
		days := commitsDays
		if days == 0 {
			days = cfg.Summary.Days
		}
		if days == 0 {
			days = 7
		}

		var repos []string
		if commitsRepos != "" {
			repos = strings.Split(commitsRepos, ",")
		} else {
			repos = cfg.Summary.Repos
		}
		if len(repos) == 0 {
			return fmt.Errorf("no repos configured — set summary.repos in ~/.ado/config.yaml or pass --repos")
		}

		author := commitsAuthor
		if author == "" {
			author = cfg.Summary.Author
		}

		// Header
		if !commitsRaw {
			fmt.Fprintf(os.Stdout, "Scanning %d repo(s), past %d day(s)", len(repos), days)
			if author != "" {
				fmt.Fprintf(os.Stdout, ", author=%q", author)
			}
			fmt.Fprintln(os.Stdout)
			for _, r := range repos {
				fmt.Fprintf(os.Stdout, "  · %s\n", r)
			}
			fmt.Fprintln(os.Stdout)
		}

		commits, errs := git.CollectAllLogs(repos, days, author)
		for _, err := range errs {
			fmt.Fprintf(os.Stderr, "  warning: %v\n", err)
		}

		if len(commits) == 0 {
			if !commitsRaw {
				fmt.Fprintln(os.Stdout, "(no commits found)")
			}
			return nil
		}

		if commitsRaw {
			for _, c := range commits {
				fmt.Fprintf(os.Stdout, "%s\t%s\t%s\t%s\t%s\n",
					c.Date.Format("2006-01-02"), c.Repo, c.Hash, c.Author, c.Subject)
			}
			return nil
		}

		// Column widths
		dateW, repoW, hashW, authW := 10, len("Repo"), len("Hash"), len("Author")
		for _, c := range commits {
			if n := len(c.Repo); n > repoW {
				repoW = n
			}
			if n := len(c.Hash); n > hashW {
				hashW = n
			}
			if n := len(c.Author); n > authW {
				authW = n
			}
		}

		fmt.Fprintf(os.Stdout, "%-*s  %-*s  %-*s  %-*s  %s\n",
			dateW, "Date", repoW, "Repo", hashW, "Hash", authW, "Author", "Subject")
		fmt.Fprintf(os.Stdout, "%s  %s  %s  %s  %s\n",
			strings.Repeat("-", dateW), strings.Repeat("-", repoW),
			strings.Repeat("-", hashW), strings.Repeat("-", authW),
			strings.Repeat("-", 40))
		for _, c := range commits {
			fmt.Fprintf(os.Stdout, "%-*s  %-*s  %-*s  %-*s  %s\n",
				dateW, c.Date.Format("2006-01-02"),
				repoW, c.Repo,
				hashW, c.Hash,
				authW, c.Author,
				c.Subject)
		}

		fmt.Fprintf(os.Stdout, "\n%d commit(s) across %d repo(s)\n", len(commits), distinctRepos(commits))
		return nil
	},
}

func distinctRepos(commits []git.CommitLog) int {
	seen := make(map[string]bool)
	for _, c := range commits {
		seen[c.Repo] = true
	}
	return len(seen)
}

func init() {
	commitsCmd.Flags().IntVarP(&commitsDays, "days", "d", 0, "number of days to look back (default from config or 7)")
	commitsCmd.Flags().StringVarP(&commitsRepos, "repos", "r", "", "comma-separated repo paths (overrides config)")
	commitsCmd.Flags().StringVarP(&commitsAuthor, "author", "a", "", "author filter (overrides summary.author in config)")
	commitsCmd.Flags().BoolVar(&commitsRaw, "raw", false, "tab-separated one-line-per-commit output")
	rootCmd.AddCommand(commitsCmd)
}
