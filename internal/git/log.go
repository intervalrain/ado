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
func CollectLogs(repoPath string, days int, author string) ([]CommitLog, error) {
	since := fmt.Sprintf("--since=%d days ago", days)
	args := []string{"-C", repoPath, "log", since, "--format=%H|%an|%aI|%s|%b%x00"}
	if author != "" {
		args = append(args, "--author="+author)
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
func CollectAllLogs(repoPaths []string, days int, author string) ([]CommitLog, []error) {
	var all []CommitLog
	var errs []error

	for _, p := range repoPaths {
		logs, err := CollectLogs(p, days, author)
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
