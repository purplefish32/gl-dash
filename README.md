# gl-dash

A terminal UI dashboard for GitLab. Inspired by [gh-dash](https://github.com/dlvhdr/gh-dash).

![Go](https://img.shields.io/badge/Go-1.25+-00ADD8?logo=go&logoColor=white)
![GitLab](https://img.shields.io/badge/GitLab-API%20v4-FC6D26?logo=gitlab&logoColor=white)

## Features

- **Three views**: Merge Requests, Issues, Todos — switch with `s`
- **Card-style layout** with status emoji, branch arrows, pipeline status, relative timestamps
- **Smart search** with prefix syntax and fuzzy picker:
  - `@username` — filter by author (with fuzzy autocomplete)
  - `@r:username` — filter by reviewer
  - `@a:username` — filter by assignee
  - `#group/project` — filter by project
  - `~label` — filter by label
  - `!branch` — filter by source branch
  - `!!branch` — filter by target branch
- **Sidebar with tabs**: Overview, Discussion, Commits, Pipeline, Changes
- **Smart repo filtering**: auto-detects current GitLab project from git remotes
- **Configurable review command**: launch code reviews in tmux with Claude Code
- **Auto-refresh** with configurable interval
- **Pagination**: auto-loads more results when scrolling to bottom

### MR Status Indicators

| Icon | Status |
|------|--------|
| 📝 | Draft |
| ✅ | Mergeable |
| 👀 | Not approved |
| ⚡ | Needs rebase |
| ❌ | Has conflicts |
| 🔄 | CI running / must pass |
| 💬 | Unresolved discussions |
| ✏️ | Requested changes |
| 🚫 | Blocked |
| ⏳ | Checking |

## Install

```bash
go install gl-dash@latest
```

Or build from source:

```bash
git clone <repo-url>
cd gl-dash
go build -o gl-dash .
```

## Quick Start

```bash
# Set your GitLab token
export GITLAB_TOKEN=your-personal-access-token

# For self-hosted GitLab
export GITLAB_URL=https://gitlab.your-company.com

# Create default config
gl-dash init

# Launch (recommended: inside tmux)
gl-dash
```

## Configuration

Config file: `~/.config/gl-dash/config.yml`

```yaml
gitlab:
  baseUrl: https://gitlab.your-company.com
  token: your-token-here  # or use GITLAB_TOKEN env var

sections:
  mergeRequests:
    - title: Merge Requests
      filter: all
      limit: 50

  issues:
    - title: Issues
      filter: all
      limit: 50

# Auto-refresh interval in minutes (0 to disable)
refreshMinutes: 5

# Auto-filter to current git project on launch (default: true)
# smartFilter: false

# Review command (press v on an MR)
# Template variables: {{.IID}}, {{.MrNumber}}, {{.SourceBranch}},
#   {{.TargetBranch}}, {{.ProjectPath}}, {{.Author}}, {{.Title}}, {{.WebURL}}
reviewCommand: "tmux new-window -n 'MR-{{.MrNumber}}' 'wt switch mr:{{.MrNumber}} && claude /review'"

# Local tool command (press L)
localCommand: "tmux new-window -n 'lazygit' 'lazygit'"

# Map project paths to local directories
repoPaths:
  "group/project": "~/Code/project"
```

### Advanced Section Filters

```yaml
sections:
  mergeRequests:
    # Use filter shorthand
    - title: My MRs
      filter: author
      limit: 20

    # Or explicit API fields
    - title: Bug Fixes
      scope: all
      labels: [bug]
      targetBranch: main
      limit: 30

    - title: Team MRs
      scope: all
      authorUsername: colleague
      limit: 20
```

## Keybindings

### Navigation
| Key | Action |
|-----|--------|
| `j` / `k` | Move cursor down / up |
| `g` / `G` | Jump to top / bottom |
| `Ctrl+d` / `Ctrl+u` | Page down / up |
| `h` / `l` | Switch section tab |
| `s` | Cycle view (MRs / Issues / Todos) |

### Actions
| Key | Action |
|-----|--------|
| `o` / `Enter` | Open in browser |
| `y` | Copy URL to clipboard |
| `v` | Review MR (run reviewCommand) |
| `L` | Local tool (run localCommand) |
| `d` | Mark todo as done |
| `D` | Mark all todos as done |
| `t` | Toggle repo filter |

### Sidebar
| Key | Action |
|-----|--------|
| `p` | Toggle sidebar |
| `]` / `[` | Next / prev sidebar tab |

### Search
| Key | Action |
|-----|--------|
| `/` | Open search |
| `Enter` | Execute search |
| `Esc` | Clear filter |
| `?` | Toggle help overlay |

### Search Prefixes
| Prefix | Filter |
|--------|--------|
| `@username` | Author |
| `@r:username` | Reviewer |
| `@a:username` | Assignee |
| `#group/project` | Project |
| `~label` | Label |
| `!branch` | Source branch |
| `!!branch` | Target branch |
| `text` | Title / description |

Prefixes open a fuzzy picker — type to narrow, `Ctrl+j`/`Ctrl+k` to navigate, `Enter` to select.

## Tech Stack

- [Bubble Tea v2](https://charm.land/bubbletea) — TUI framework
- [Lipgloss v2](https://charm.land/lipgloss) — Terminal styling
- [Glamour v2](https://charm.land/glamour) — Markdown rendering
- [go-gitlab](https://gitlab.com/gitlab-org/api/client-go) — GitLab API client
- [Cobra](https://github.com/spf13/cobra) — CLI framework

## License

MIT
