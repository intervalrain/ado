# Changelog

## [v0.0.6] - 2026-04-23

### Added
- `ado model` command: manage LLM profiles (`add`/`ls`/`select`/`rm`/`current`); profiles stored under `~/.ado/models/`
- Gemini provider support in LLM layer (alongside Claude/OpenAI/Ollama)
- TUI profile wizard for creating model profiles from settings screen
- Query TUI: multi-select rows (Space / `a` to toggle all visible) and batch move to iteration (`m`)
- `ado commits` command — preview the commits that `ado summary` would feed to the LLM
- LICENSE (GPL-3.0) and project banner image

### Changed
- `resolveTeam()` helper extracted in `internal/api/query.go`; new `ListIterations()` API
- Filepicker and settings TUI refinements
- Removed stale `ADO_TEAM` guidance and `.ado_cache.json` from repo

## [v0.0.5] - 2026-04-16

### Added
- `/` filter in query TUI — live substring match across all cells, with count shown in title
- Query rows sorted by ID on load
- Per-row State coloring (active/new/closed/resolved/removed)

### Changed
- Query view now uses the custom renderer for all modes, so State colors render correctly without breaking column alignment

## [v0.0.4] - 2026-04-15

### Added
- Parent link support in `ado new` — `--parent/-p <id>` flag and new Parent step in TUI create wizard
- File-based logging across mediator, HTTP client, and TUI entry points (`internal/logging/logger.go`)

## [v0.0.3] - 2026-04-14

### Added
- Pipeline monitor feature in TUI (`internal/tui/pipeline.go`)
- Summary report save-to-file flow with editable path in TUI
- Press Enter on saved screen to open the report folder in OS file manager
- File picker component for selecting template/output paths (`internal/tui/filepicker.go`)
- LLM `Complete` signature now accepts a system prompt; Claude uses top-level `system` field, OpenAI prepends a `role:"system"` message
- Summary template split into system prompt (format rules) + user message (commits/work items data) so the LLM actually follows the template
- `Using template: <source>` log line to make template fallback behavior visible
- Extended settings screen with summary/LLM sections

### Changed
- Unified config into single `~/.ado/config.yaml` (replaces separate `.env` + template/output files)
- `default_template.md` rewritten as pure format instructions with required section structure and Traditional Chinese output

## [v0.0.2] - 2026-04-08

### Added
- Makefile with `build`, `install`, `cross`, `snapshot`, `release`, `clean` targets
- goreleaser configuration for multi-platform releases (linux/darwin/windows × amd64/arm64)
- Prerequisites and Quick Start sections in README

### Changed
- README restructured with bilingual quick start guide

## [v0.0.1] - 2026-04-07

### Added
- Azure DevOps CLI with CQRS + MediatR architecture
- `ado query` — list work items from saved queries
- `ado new` — create work items (task, bug, epic, issue, user story)
- `ado pr` — list and create pull requests with auto-complete support
- `ado tui` — interactive TUI with Bubble Tea
- TUI: inline cell editing, state picker, scrollable lists
- TUI: PR category menu (created by me, assigned to me, by repo)
- TUI: work item creation wizard with tag caching and iteration support
- TUI: settings screen for editing .env values
- Auto-push local branch to remote before creating PR
- Local cache for tags, favorite repos, and reviewer names
