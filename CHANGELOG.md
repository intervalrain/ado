# Changelog

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
