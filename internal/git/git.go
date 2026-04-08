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

// HasRemoteBranch checks if a branch exists on the remote.
func HasRemoteBranch(branch string) bool {
	err := exec.Command("git", "ls-remote", "--exit-code", "--heads", "origin", branch).Run()
	return err == nil
}

// PushBranch pushes the current branch to the remote.
func PushBranch(branch string) error {
	cmd := exec.Command("git", "push", "-u", "origin", branch)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git push failed: %s: %w", strings.TrimSpace(string(out)), err)
	}
	return nil
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
