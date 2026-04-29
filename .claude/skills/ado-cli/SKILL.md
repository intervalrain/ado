---
name: ado-cli
description: Use the `ado` CLI to interact with Azure DevOps — query work items, create tasks/bugs/stories, manage pull requests, check pipeline status, generate summary reports, and more. Trigger this skill whenever the user mentions Azure DevOps, ADO, work items, sprint tasks, iterations, user stories, backlog, pull requests, pipelines, builds, or wants to create/query/update any ADO resource. Also trigger when the user says things like "what's assigned to me", "create a task", "check the build", "open a PR", "show my work items", or any request that involves project management on Azure DevOps — even if they don't explicitly say "ado".
---

# ado CLI — Azure DevOps from the Terminal

`ado` is a lightweight CLI (installed at `/usr/local/bin/ado`) for interacting with Azure DevOps. It supports both direct commands and an interactive TUI.

## Pre-flight: Is ado configured?

Before running any command, check whether the user has a working config:

```bash
ado query 2>&1 | head -3
```

If you see an error about missing org/pat, guide the user through setup:

1. Create `~/.ado/config.yaml`:
   ```yaml
   org: "Advantech-EBO"          # Just the org name, not the full URL
   project: "your-project"
   pat: "your-personal-access-token"
   query_id: "optional-default-query-id"
   assignee: "Display Name"
   ```
2. Or set environment variables: `ADO_ORG`, `ADO_PROJECT`, `ADO_PAT`, `ADO_QUERY_ID`, `ADO_ASSIGNEE`.

The `org` field accepts either a plain name (`Advantech-EBO`) or a full URL (`https://dev.azure.com/Advantech-EBO`).

---

## Commands Reference

### query — List work items from a saved query

```bash
ado query                    # Uses default query_id from config
ado query -i <query-id>     # Use a specific query ID
```

Output: a formatted table of work items (ID, Type, Title, State, Assigned To, etc.).

### new — Create a work item

```bash
ado new "<title>" [flags]
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--type` | `-t` | `task` | Type: `task`, `bug`, `epic`, `issue`, `user story` / `userstory` / `story` |
| `--desc` | `-d` | | Description text |
| `--est` | `-e` | `6` | Estimate in hours (sets both original and remaining) |
| `--tags` | | | Semicolon-separated tags, e.g. `"backend; urgent"` |
| `--parent` | `-p` | | Parent work item ID to link under |

Examples:
```bash
ado new "Fix login bug" --type bug --desc "Returns 500 on POST /login" --est 4
ado new "Implement caching layer" --type task --est 8 --tags "backend; perf"
ado new "Sub-task" --parent 12345 --est 2
ado new "Registration flow" --type story --desc "As a user I want to register"
```

### pr — Pull requests

**List PRs assigned to you:**
```bash
ado pr
```

**Create a PR from the current branch:**
```bash
ado pr "<title>" [flags]
```

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--branch` | `-n` | repo default | Target branch |
| `--desc` | `-d` | | PR description |
| `--reviewer` | `-r` | | Required reviewer (display name or email) |
| `--optional` | `-o` | | Optional reviewer |
| `--auto-complete` | | `false` | Auto-complete with squash merge + delete source |

Examples:
```bash
ado pr "Add auth middleware" -r "John Doe" --auto-complete
ado pr "Fix #789" -d "Closes issue 789" -n main
```

### pipeline — Pipeline status and builds

```bash
ado pipeline                  # List all pipeline definitions with latest status
ado pipeline -i <def-id>     # Show recent builds for a specific pipeline
ado pipeline -i 42 -t 10     # Show 10 recent builds
```

### commits — Preview commits for summary

```bash
ado commits                          # Use config defaults
ado commits -d 14                    # Look back 14 days
ado commits -r /path/to/repo        # Specific repos (comma-separated)
ado commits -a "Rain Hu"            # Filter by author
ado commits --raw                    # Machine-readable (tab-separated)
```

### summary — Generate a report from commits + work items

```bash
ado summary                              # Use config defaults
ado summary -d 14                        # Look back 14 days
ado summary -r /repo1,/repo2            # Specific repos
ado summary -t ~/.ado/template.md        # Custom template
```

Requires an LLM profile to be configured (see `model` below).

### model — Manage LLM profiles

```bash
ado model add <name> <provider> <model> [flags]   # Create profile
ado model ls                                        # List all profiles
ado model select <name>                             # Activate a profile
ado model current                                   # Show active profile
ado model rm <name>                                 # Delete profile
```

Providers: `claude`, `openai`, `gemini`, `ollama`.

Example:
```bash
ado model add sonnet claude claude-sonnet-4-20250514 --api-key sk-ant-...
ado model select sonnet
```

### tui — Interactive terminal UI

```bash
ado tui                      # Launch interactive mode
ado tui -i <query-id>       # Launch with specific query
```

The TUI has screens for Query (browse/edit work items), New (create wizard), Pull Requests, and Settings. Generally prefer direct CLI commands over TUI when acting on behalf of the user — the TUI is interactive and harder to automate.

---

## Usage Guidelines

- **Prefer CLI commands over TUI.** The TUI requires interactive input; CLI commands are scriptable and their output is easy to parse.
- **Quote titles and descriptions** that contain spaces.
- **Use `--raw` on `ado commits`** when you need to process the output programmatically.
- **Check `ado pipeline`** when the user asks about build status, CI failures, or deployment state.
- **Creating multiple work items**: run `ado new` once per item. You can batch them sequentially.
- **When creating a PR**: make sure the current branch has been pushed to the remote first.