package tui

import (
	"strings"

	"github.com/purplefish32/gl-dash/internal/config"
)

// searchOverride represents parsed search prefixes that override API query params.
type searchOverride struct {
	author       string   // @username
	reviewer     string   // @r:username
	assignee     string   // @a:username
	project      string   // #group/project or project:group/project
	labels       []string // ~label
	srcBranch    string   // !branch (source branch)
	tgtBranch    string   // !!branch (target branch)
	textSearch   string   // plain text (title search)
	hasOverrides bool     // true if any API-level filter was parsed
}

// parseSearch parses a search query into API overrides and plain text.
//
// Prefix syntax:
//   - @username      → filter by author
//   - @r:username    → filter by reviewer
//   - @a:username    → filter by assignee
//   - #path/to/proj  → filter by project
//   - ~label         → filter by label (multiple allowed)
//   - !branch        → filter by source branch
//   - !!branch       → filter by target branch
//   - plain text     → API search parameter (title/description)
func parseSearch(query string) searchOverride {
	var so searchOverride
	var textParts []string

	tokens := strings.Fields(query)
	for _, token := range tokens {
		switch {
		case strings.HasPrefix(token, "@r:"):
			so.reviewer = strings.TrimPrefix(token, "@r:")
			so.hasOverrides = true
		case strings.HasPrefix(token, "@a:"):
			so.assignee = strings.TrimPrefix(token, "@a:")
			so.hasOverrides = true
		case strings.HasPrefix(token, "@"):
			so.author = strings.TrimPrefix(token, "@")
			so.hasOverrides = true
		case strings.HasPrefix(token, "#"):
			so.project = strings.TrimPrefix(token, "#")
			so.hasOverrides = true
		case strings.HasPrefix(token, "project:"):
			so.project = strings.TrimPrefix(token, "project:")
			so.hasOverrides = true
		case strings.HasPrefix(token, "~"):
			so.labels = append(so.labels, strings.TrimPrefix(token, "~"))
			so.hasOverrides = true
		case strings.HasPrefix(token, "!!"):
			so.tgtBranch = strings.TrimPrefix(token, "!!")
			so.hasOverrides = true
		case strings.HasPrefix(token, "!"):
			so.srcBranch = strings.TrimPrefix(token, "!")
			so.hasOverrides = true
		default:
			textParts = append(textParts, token)
		}
	}

	so.textSearch = strings.Join(textParts, " ")
	if so.textSearch != "" {
		so.hasOverrides = true
	}

	return so
}

// toSectionConfig creates a SectionConfig with the search overrides applied
// on top of a base config.
func (so searchOverride) toSectionConfig(base config.SectionConfig) config.SectionConfig {
	cfg := base

	// Any prefixed filter should override scope to "all" so results
	// aren't limited by the section's default scope (e.g. "created_by_me").
	if so.author != "" || so.reviewer != "" || so.assignee != "" ||
		len(so.labels) > 0 || so.srcBranch != "" || so.tgtBranch != "" || so.project != "" {
		cfg.Scope = "all"
		cfg.Filter = ""
	}

	if so.author != "" {
		cfg.AuthorUsername = so.author
	}

	if so.reviewer != "" {
		cfg.ReviewerUsername = so.reviewer
	}

	if so.assignee != "" {
		cfg.AssigneeUsername = so.assignee
	}

	if len(so.labels) > 0 {
		cfg.Labels = so.labels
	}

	if so.srcBranch != "" {
		cfg.SourceBranch = so.srcBranch
	}

	if so.tgtBranch != "" {
		cfg.TargetBranch = so.tgtBranch
	}

	if so.textSearch != "" {
		cfg.Search = so.textSearch
	}

	return cfg
}

// formatSearchHint returns a human-readable description of active search filters.
func (so searchOverride) formatSearchHint() string {
	var parts []string
	if so.author != "" {
		parts = append(parts, "author:"+so.author)
	}
	if so.reviewer != "" {
		parts = append(parts, "reviewer:"+so.reviewer)
	}
	if so.assignee != "" {
		parts = append(parts, "assignee:"+so.assignee)
	}
	if so.project != "" {
		parts = append(parts, "project:"+so.project)
	}
	for _, l := range so.labels {
		parts = append(parts, "label:"+l)
	}
	if so.srcBranch != "" {
		parts = append(parts, "src:"+so.srcBranch)
	}
	if so.tgtBranch != "" {
		parts = append(parts, "tgt:"+so.tgtBranch)
	}
	if so.textSearch != "" {
		parts = append(parts, "\""+so.textSearch+"\"")
	}
	return strings.Join(parts, " ")
}
