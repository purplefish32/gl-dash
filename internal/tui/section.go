package tui

import (
	"strings"

	"github.com/purplefish32/gl-dash/internal/config"
	"github.com/purplefish32/gl-dash/internal/data"
)

type section struct {
	config  config.SectionConfig
	mrs     []data.MergeRequest
	issues  []data.Issue
	todos   []data.Todo
	loading bool
	err     error
	cursor  int
	offset  int // scroll offset (first visible row index)
	page    int // current page (for pagination)
	hasMore bool // whether there are more pages to load

	// Search state
	searchQuery    string
	searchParsed   searchOverride // parsed search with API overrides
	filteredMRs    []data.MergeRequest
	filteredIssues []data.Issue
	filteredTodos  []data.Todo
}

// effectiveConfig returns the section config with any active search overrides applied.
func (s section) effectiveConfig() config.SectionConfig {
	if s.searchParsed.hasOverrides {
		return s.searchParsed.toSectionConfig(s.config)
	}
	return s.config
}

func newSection(cfg config.SectionConfig) section {
	return section{
		config:  cfg,
		loading: true,
	}
}

func (s *section) applyFilter() {
	q := strings.ToLower(s.searchQuery)
	if q == "" {
		s.filteredMRs = nil
		s.filteredIssues = nil
		s.filteredTodos = nil
		return
	}

	if len(s.mrs) > 0 {
		s.filteredMRs = nil
		for _, mr := range s.mrs {
			if strings.Contains(strings.ToLower(mr.Title), q) ||
				strings.Contains(strings.ToLower(mr.Author), q) ||
				strings.Contains(strings.ToLower(mr.SourceBranch), q) ||
				strings.Contains(strings.ToLower(mr.TargetBranch), q) {
				s.filteredMRs = append(s.filteredMRs, mr)
			}
		}
	}

	if len(s.issues) > 0 {
		s.filteredIssues = nil
		for _, issue := range s.issues {
			if strings.Contains(strings.ToLower(issue.Title), q) ||
				strings.Contains(strings.ToLower(issue.Author), q) {
				s.filteredIssues = append(s.filteredIssues, issue)
			}
		}
	}

	if len(s.todos) > 0 {
		s.filteredTodos = nil
		for _, todo := range s.todos {
			if strings.Contains(strings.ToLower(todo.TargetTitle), q) ||
				strings.Contains(strings.ToLower(todo.Author), q) ||
				strings.Contains(strings.ToLower(todo.Project), q) ||
				strings.Contains(strings.ToLower(todo.Action), q) {
				s.filteredTodos = append(s.filteredTodos, todo)
			}
		}
	}

	s.cursor = 0
}

// ensureCursorVisible adjusts the scroll offset so the cursor is visible
// within the given viewport height.
func (s *section) ensureCursorVisible(viewportHeight int) {
	if viewportHeight <= 0 {
		return
	}
	// Scroll down if cursor is below viewport
	if s.cursor >= s.offset+viewportHeight {
		s.offset = s.cursor - viewportHeight + 1
	}
	// Scroll up if cursor is above viewport
	if s.cursor < s.offset {
		s.offset = s.cursor
	}
	// Clamp offset
	maxOffset := s.itemCount() - viewportHeight
	if maxOffset < 0 {
		maxOffset = 0
	}
	if s.offset > maxOffset {
		s.offset = maxOffset
	}
	if s.offset < 0 {
		s.offset = 0
	}
}

func (s *section) clearFilter() {
	s.searchQuery = ""
	s.searchParsed = searchOverride{}
	s.filteredMRs = nil
	s.filteredIssues = nil
	s.filteredTodos = nil
	s.cursor = 0
	s.page = 1
	s.hasMore = false
}

func (s section) visibleMRs() []data.MergeRequest {
	if s.searchQuery != "" && s.filteredMRs != nil {
		return s.filteredMRs
	}
	return s.mrs
}

func (s section) visibleIssues() []data.Issue {
	if s.searchQuery != "" && s.filteredIssues != nil {
		return s.filteredIssues
	}
	return s.issues
}

func (s section) visibleTodos() []data.Todo {
	if s.searchQuery != "" && s.filteredTodos != nil {
		return s.filteredTodos
	}
	return s.todos
}

func (s section) itemCount() int {
	return len(s.visibleMRs()) + len(s.visibleIssues()) + len(s.visibleTodos())
}

func (s section) selectedURL() string {
	mrs := s.visibleMRs()
	if len(mrs) > 0 && s.cursor < len(mrs) {
		return mrs[s.cursor].WebURL
	}
	issues := s.visibleIssues()
	if len(issues) > 0 && s.cursor < len(issues) {
		return issues[s.cursor].WebURL
	}
	todos := s.visibleTodos()
	if len(todos) > 0 && s.cursor < len(todos) {
		return todos[s.cursor].TargetURL
	}
	return ""
}
