package git

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// CommitLog represents a parsed git commit.
type CommitLog struct {
	Hash    string
	Author  string
	Date    time.Time
	Subject string
	Body    string
	Repo    string // repository directory name
}

// CollectLogs runs git log in the given repo directory for the past N days.
// Uses an ISO date for --since (some git builds mis-parse "N days ago") and
// --all so commits on any ref (feature branches, remotes) are considered.
// authors is a list of git author patterns; each becomes a separate --author
// arg, which git ORs together (a commit matches if any pattern matches). An
// empty list disables author filtering.
func CollectLogs(repoPath string, days int, authors []string) ([]CommitLog, error) {
	cutoff := time.Now().AddDate(0, 0, -days).Format("2006-01-02")
	args := []string{
		"-C", repoPath,
		"log", "--all",
		"--since=" + cutoff,
		"--format=%H|%an|%aI|%s|%b%x00",
	}
	for _, author := range authors {
		if author != "" {
			args = append(args, "--author="+author)
		}
	}

	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return nil, fmt.Errorf("git log in %s: %w", repoPath, err)
	}

	repoName := filepath.Base(repoPath)
	entries := strings.Split(strings.TrimSpace(string(out)), "\x00")

	var logs []CommitLog
	for _, entry := range entries {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		parts := strings.SplitN(entry, "|", 5)
		if len(parts) < 4 {
			continue
		}
		date, _ := time.Parse(time.RFC3339, parts[2])
		body := ""
		if len(parts) == 5 {
			body = strings.TrimSpace(parts[4])
		}
		logs = append(logs, CommitLog{
			Hash:    parts[0][:8], // short hash
			Author:  parts[1],
			Date:    date,
			Subject: parts[3],
			Body:    body,
			Repo:    repoName,
		})
	}
	return logs, nil
}

// CollectAllLogs collects git logs from multiple repos and returns them sorted by date (newest first).
func CollectAllLogs(repoPaths []string, days int, authors []string) ([]CommitLog, []error) {
	var all []CommitLog
	var errs []error

	for _, p := range repoPaths {
		logs, err := CollectLogs(p, days, authors)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		all = append(all, logs...)
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].Date.After(all[j].Date)
	})

	return all, errs
}
