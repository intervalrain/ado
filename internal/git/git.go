package git

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// CurrentBranch returns the current git branch name.
func CurrentBranch() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD").Output()
	if err != nil {
		return "", fmt.Errorf("not in a git repository or git not available: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// RepoName returns the repository name derived from the remote origin URL.
func RepoName() (string, error) {
	out, err := exec.Command("git", "remote", "get-url", "origin").Output()
	if err != nil {
		return "", fmt.Errorf("no git remote 'origin' found: %w", err)
	}
	url := strings.TrimSpace(string(out))
	// Handle both SSH and HTTPS URLs
	// HTTPS: https://dev.azure.com/org/project/_git/repo
	// SSH: git@ssh.dev.azure.com:v3/org/project/repo
	name := filepath.Base(url)
	name = strings.TrimSuffix(name, ".git")
	return name, nil
}

// DefaultBranch returns the default branch (main or master).
func DefaultBranch() string {
	out, err := exec.Command("git", "symbolic-ref", "refs/remotes/origin/HEAD", "--short").Output()
	if err != nil {
		return "main"
	}
	// output is like "origin/main"
	parts := strings.SplitN(strings.TrimSpace(string(out)), "/", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return "main"
}
