# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

gl-dash is a terminal UI dashboard for GitLab (inspired by gh-dash), built in Go using Bubble Tea. It displays merge requests, issues, and todos in a TUI with sidebar detail views.

## Commands

```bash
# Build
go build -o gl-dash .

# Run (requires GITLAB_TOKEN env var, optionally GITLAB_URL)
./gl-dash

# Generate default config
./gl-dash init
```

No test suite or linter is currently configured.

## Architecture

```
cmd/              # Cobra CLI commands (root.go, init.go)
internal/
  config/         # YAML config loading (~/.config/gl-dash/config.yml) + env var overrides
  data/           # GitLab API client layer - fetch/convert MRs, issues, todos, details
  git/            # Git remote detection for smart project filtering
  tui/            # Bubble Tea TUI
    model.go      # Main Model - all state, Update(), View() (~2000 lines)
    section.go    # Section abstraction (tab with cursor, pagination, filtered items)
    sidebar.go    # Sidebar rendering (Overview/Discussion/Commits/Pipeline/Changes tabs)
    search.go     # Search query parser with prefix syntax (@user, #project, ~label, !branch)
    picker.go     # Fuzzy picker UI for users/projects/labels
    checkout.go   # Template-based command execution (review/local commands)
```

## Key Patterns

- **Bubble Tea architecture**: Single `Model` struct with `Init()`, `Update()`, `View()`. Messages like `sectionMRsMsg`, `sidebarDataMsg` drive async data loading.
- **Section abstraction**: Each tab (MR list, issue list, etc.) is a `section` with its own cursor, pagination (`PageInfo`), loading state, and search filtering.
- **Search prefix syntax**: `@user` (author), `@r:user` (reviewer), `@a:user` (assignee), `#group/project` (project), `~label` (label), `!branch`/`!!branch` (source/target branch). Parsed in `searchOverride` and applied at API level.
- **Template commands**: `reviewCommand` and `localCommand` in config use Go templates with MR fields (`{{.IID}}`, `{{.SourceBranch}}`, `{{.ProjectPath}}`, etc.).
- **Smart filtering**: Auto-detects current GitLab project from git remotes and filters sections to it. Toggle with `t` key.
- **Data layer**: Each entity type has a fetch function that calls the GitLab API with pagination, then converts API types to simpler internal structs.
