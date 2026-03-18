package tui

import (
	"fmt"
	"strings"

	glamour "charm.land/glamour/v2"
	"charm.land/lipgloss/v2"
	zone "github.com/lrstanley/bubblezone/v2"

	"github.com/purplefish32/gl-dash/internal/data"
)

type sidebarTab int

const (
	tabOverview sidebarTab = iota
	tabDiscussion
	tabCommits
	tabPipeline
	tabChanges
	tabCount
)

func (t sidebarTab) label() string {
	switch t {
	case tabOverview:
		return "Overview"
	case tabDiscussion:
		return "Discussion"
	case tabCommits:
		return "Commits"
	case tabPipeline:
		return "Pipeline"
	case tabChanges:
		return "Changes"
	}
	return ""
}

// sidebarState holds cached data for sidebar tabs
type sidebarState struct {
	activeTab  sidebarTab
	scrollY    int // vertical scroll offset for modal content
	// Cached data per MR (keyed by projectID:mrIID)
	cachedKey  string
	notes      []data.MRNote
	commits    []data.MRCommit
	pipelines  []data.MRPipeline
	changes    []data.MRChange
	loadingTab sidebarTab
	loadErr    error
}

func (ss *sidebarState) cacheKey(mr data.MergeRequest) string {
	return fmt.Sprintf("%d:%d", mr.ProjectID, mr.IID)
}

func (ss *sidebarState) isCached(mr data.MergeRequest, tab sidebarTab) bool {
	if ss.cachedKey != ss.cacheKey(mr) {
		return false
	}
	switch tab {
	case tabDiscussion:
		return ss.notes != nil
	case tabCommits:
		return ss.commits != nil
	case tabPipeline:
		return ss.pipelines != nil
	case tabChanges:
		return ss.changes != nil
	}
	return true // overview doesn't need extra data
}

func (ss *sidebarState) clearCache() {
	ss.cachedKey = ""
	ss.notes = nil
	ss.commits = nil
	ss.pipelines = nil
	ss.changes = nil
	ss.loadErr = nil
}

var (
	sidebarBorderStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#555555")).
		Padding(1, 2)

	sidebarTitleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FF6B00"))

	sidebarLabelStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#888888"))

	sidebarValueStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#CCCCCC"))

	sidebarTabActiveStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FF6B00"))

	sidebarTabInactiveStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#555555"))

	pipelineSuccessStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#00CC00"))

	pipelineFailedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF0000"))

	pipelineRunningStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFAA00"))

	pipelinePendingStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#888888"))

	labelTagStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#000000")).
			Background(lipgloss.Color("#FF6B00")).
			Padding(0, 1)

	diffAddStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00CC00"))

	diffDelStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FF0000"))

	diffFileStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFAA00"))

	noteAuthorStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF6B00"))

	noteSystemStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#555555")).
			Italic(true)

	commitHashStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFAA00"))
)

func renderMRSidebar(mr data.MergeRequest, ss *sidebarState, width, height int) string {
	contentWidth := width - 6

	var parts []string

	// Tab bar
	var tabs []string
	for t := sidebarTab(0); t < tabCount; t++ {
		if t == ss.activeTab {
			tabs = append(tabs, zone.Mark(fmt.Sprintf("sidebar-%d", t), sidebarTabActiveStyle.Render(t.label())))
		} else {
			tabs = append(tabs, zone.Mark(fmt.Sprintf("sidebar-%d", t), sidebarTabInactiveStyle.Render(t.label())))
		}
	}
	parts = append(parts, strings.Join(tabs, " "))
	parts = append(parts, sidebarLabelStyle.Render(strings.Repeat("─", contentWidth)))

	// Tab content
	switch ss.activeTab {
	case tabOverview:
		parts = append(parts, renderMROverview(mr, contentWidth)...)
	case tabDiscussion:
		if ss.loadErr != nil {
			parts = append(parts, errorStyle.Render(fmt.Sprintf("Error: %v", ss.loadErr)))
		} else if ss.notes == nil {
			parts = append(parts, sidebarValueStyle.Render("Loading..."))
		} else {
			parts = append(parts, renderMRDiscussion(ss.notes, contentWidth)...)
		}
	case tabCommits:
		if ss.loadErr != nil {
			parts = append(parts, errorStyle.Render(fmt.Sprintf("Error: %v", ss.loadErr)))
		} else if ss.commits == nil {
			parts = append(parts, sidebarValueStyle.Render("Loading..."))
		} else {
			parts = append(parts, renderMRCommits(ss.commits, contentWidth)...)
		}
	case tabPipeline:
		if ss.loadErr != nil {
			parts = append(parts, errorStyle.Render(fmt.Sprintf("Error: %v", ss.loadErr)))
		} else if ss.pipelines == nil {
			parts = append(parts, sidebarValueStyle.Render("Loading..."))
		} else {
			parts = append(parts, renderMRPipelines(ss.pipelines)...)
		}
	case tabChanges:
		if ss.loadErr != nil {
			parts = append(parts, errorStyle.Render(fmt.Sprintf("Error: %v", ss.loadErr)))
		} else if ss.changes == nil {
			parts = append(parts, sidebarValueStyle.Render("Loading..."))
		} else {
			parts = append(parts, renderMRChanges(ss.changes, contentWidth)...)
		}
	}

	// Apply scroll offset (skip tab bar which is first 2 lines)
	if len(parts) > 2 && ss.scrollY > 0 {
		contentParts := parts[2:]
		if ss.scrollY < len(contentParts) {
			parts = append(parts[:2], contentParts[ss.scrollY:]...)
		}
	}

	// Scroll indicator
	allLines := strings.Split(strings.Join(parts, "\n"), "\n")
	if len(allLines) > height-4 {
		pct := 0
		totalContent := len(allLines)
		if totalContent > 0 {
			pct = (ss.scrollY * 100) / totalContent
		}
		scrollHint := helpStyle.Render(fmt.Sprintf(" %d%%", pct))
		parts[0] = parts[0] + "  " + scrollHint
	}

	return sidebarBorderStyle.
		Width(width - 2).
		Height(height - 2).
		Render(strings.Join(parts, "\n"))
}

func renderMROverview(mr data.MergeRequest, contentWidth int) []string {
	var parts []string

	title := mr.Title
	if mr.Draft {
		title = "Draft: " + title
	}
	parts = append(parts, sidebarTitleStyle.Width(contentWidth).Render(title))
	parts = append(parts, "")

	parts = append(parts, renderField("Author", mr.Author))
	parts = append(parts, renderField("Branch", mr.SourceBranch+" → "+mr.TargetBranch))

	if len(mr.Assignees) > 0 {
		parts = append(parts, renderField("Assignees", joinUsernames(mr.Assignees)))
	}
	if len(mr.Reviewers) > 0 {
		parts = append(parts, renderField("Reviewers", joinUsernames(mr.Reviewers)))
	}

	if mr.Pipeline != "" {
		parts = append(parts, renderField("Pipeline", renderPipelineStatus(mr.Pipeline)))
	}

	stats := []string{}
	if mr.ChangesCount != "" {
		stats = append(stats, fmt.Sprintf("%s changes", mr.ChangesCount))
	}
	if mr.UserNotesCount > 0 {
		stats = append(stats, fmt.Sprintf("%d comments", mr.UserNotesCount))
	}
	if mr.HasConflicts {
		stats = append(stats, errorStyle.Render("has conflicts"))
	}
	if len(stats) > 0 {
		parts = append(parts, renderField("Stats", strings.Join(stats, "  ")))
	}

	parts = append(parts, renderField("Created", mr.CreatedAt))
	parts = append(parts, renderField("Updated", mr.UpdatedAt))

	parts = append(parts, renderLabelsAndDescription(mr.Labels, mr.Description, contentWidth)...)

	return parts
}

func renderMRDiscussion(notes []data.MRNote, contentWidth int) []string {
	var parts []string

	// Count non-system notes
	userNotes := 0
	for _, n := range notes {
		if !n.System {
			userNotes++
		}
	}

	if userNotes == 0 {
		parts = append(parts, sidebarValueStyle.Render("No comments"))
		return parts
	}

	parts = append(parts, sidebarLabelStyle.Render(fmt.Sprintf("%d comments", userNotes)))
	parts = append(parts, "")

	for _, note := range notes {
		if note.System {
			// Compact system note — dimmed, single line
			body := note.Body
			if len(body) > contentWidth-20 {
				body = body[:contentWidth-23] + "…"
			}
			parts = append(parts, noteSystemStyle.Render(
				"  \uf0e2 "+body+"  "+note.CreatedAt))
		} else {
			// User comment — author header + markdown body
			parts = append(parts, noteAuthorStyle.Render("\uf075 "+note.Author)+
				sidebarLabelStyle.Render("  "+relativeTime(note.CreatedAt)))
			parts = append(parts, "")
			parts = append(parts, renderMarkdown(note.Body, contentWidth))
			parts = append(parts, "")
		}
	}

	return parts
}

func renderMRCommits(commits []data.MRCommit, contentWidth int) []string {
	var parts []string

	if len(commits) == 0 {
		parts = append(parts, sidebarValueStyle.Render("No commits"))
		return parts
	}

	parts = append(parts, sidebarLabelStyle.Render(fmt.Sprintf("%d commits", len(commits))))
	parts = append(parts, "")

	for _, cm := range commits {
		title := cm.Title
		if len(title) > contentWidth-12 {
			title = title[:contentWidth-15] + "…"
		}
		hash := zone.Mark("commit-"+cm.ID, commitHashStyle.Render(cm.ShortID))
		parts = append(parts, hash+"  "+sidebarValueStyle.Render(title))
		parts = append(parts, sidebarLabelStyle.Render("  "+cm.Author+"  "+relativeTime(cm.CreatedAt)))
		parts = append(parts, "")
	}

	return parts
}

func renderMRPipelines(pipelines []data.MRPipeline) []string {
	var parts []string

	if len(pipelines) == 0 {
		parts = append(parts, sidebarValueStyle.Render("No pipelines"))
		return parts
	}

	for _, p := range pipelines {
		status := renderPipelineStatus(p.Status)
		pipeID := zone.Mark(fmt.Sprintf("pipeline-%d", p.ID), cardIIDStyle.Render(fmt.Sprintf("#%d", p.ID)))
		parts = append(parts, fmt.Sprintf("%s  %s  %s",
			pipeID, status, sidebarLabelStyle.Render(p.Ref)))
	}

	return parts
}

func renderMRChanges(changes []data.MRChange, contentWidth int) []string {
	var parts []string

	if len(changes) == 0 {
		parts = append(parts, sidebarValueStyle.Render("No changes"))
		return parts
	}

	parts = append(parts, sidebarLabelStyle.Render(fmt.Sprintf("%d files changed", len(changes))))
	parts = append(parts, "")

	for _, ch := range changes {
		prefix := "M"
		if ch.NewFile {
			prefix = "A"
		} else if ch.Deleted {
			prefix = "D"
		} else if ch.Renamed {
			prefix = "R"
		}

		path := ch.NewPath
		if len(path) > contentWidth-4 {
			path = "…" + path[len(path)-contentWidth+5:]
		}

		var prefixStyled string
		switch prefix {
		case "A":
			prefixStyled = diffAddStyle.Render(prefix)
		case "D":
			prefixStyled = diffDelStyle.Render(prefix)
		default:
			prefixStyled = sidebarLabelStyle.Render(prefix)
		}

		fileLink := zone.Mark("file-"+ch.NewPath, diffFileStyle.Render(path))
		parts = append(parts, prefixStyled+"  "+fileLink)
	}

	return parts
}

func renderIssueSidebar(issue data.Issue, width, height int) string {
	contentWidth := width - 6

	var parts []string

	title := issue.Title
	if issue.Confidential {
		title = "[Confidential] " + title
	}
	parts = append(parts, sidebarTitleStyle.Width(contentWidth).Render(title))
	parts = append(parts, "")

	parts = append(parts, renderField("Author", issue.Author))

	if len(issue.Assignees) > 0 {
		parts = append(parts, renderField("Assignees", joinUsernames(issue.Assignees)))
	}

	stats := []string{}
	if issue.UserNotesCount > 0 {
		stats = append(stats, fmt.Sprintf("%d comments", issue.UserNotesCount))
	}
	if issue.Upvotes > 0 {
		stats = append(stats, fmt.Sprintf("%d upvotes", issue.Upvotes))
	}
	if issue.Downvotes > 0 {
		stats = append(stats, fmt.Sprintf("%d downvotes", issue.Downvotes))
	}
	if len(stats) > 0 {
		parts = append(parts, renderField("Stats", strings.Join(stats, "  ")))
	}

	parts = append(parts, renderField("Created", issue.CreatedAt))
	parts = append(parts, renderField("Updated", issue.UpdatedAt))

	parts = append(parts, renderLabelsAndDescription(issue.Labels, issue.Description, contentWidth)...)

	return sidebarBorderStyle.
		Width(width - 2).
		Height(height - 2).
		Render(strings.Join(parts, "\n"))
}

func renderTodoSidebar(todo data.Todo, width, height int) string {
	contentWidth := width - 6

	var parts []string

	parts = append(parts, sidebarTitleStyle.Width(contentWidth).Render(todo.TargetTitle))
	parts = append(parts, "")

	parts = append(parts, renderField("Type", todo.TargetType))
	parts = append(parts, renderField("Action", todo.Action))
	parts = append(parts, renderField("From", todo.Author))
	parts = append(parts, renderField("Project", todo.Project))
	parts = append(parts, renderField("State", todo.State))
	parts = append(parts, renderField("Created", todo.CreatedAt))

	if todo.Body != "" {
		parts = append(parts, "")
		parts = append(parts, sidebarLabelStyle.Render("Body"))
		parts = append(parts, renderMarkdown(todo.Body, contentWidth))
	}

	return sidebarBorderStyle.
		Width(width - 2).
		Height(height - 2).
		Render(strings.Join(parts, "\n"))
}

func renderLabelsAndDescription(labels []string, description string, contentWidth int) []string {
	var parts []string

	if len(labels) > 0 {
		parts = append(parts, "")
		var tags []string
		for _, l := range labels {
			tags = append(tags, labelTagStyle.Render(l))
		}
		parts = append(parts, sidebarLabelStyle.Render("Labels  ")+strings.Join(tags, " "))
	}

	if description != "" {
		parts = append(parts, "")
		parts = append(parts, sidebarLabelStyle.Render("Description"))
		parts = append(parts, renderMarkdown(description, contentWidth))
	}

	return parts
}

func joinUsernames(users []data.UserInfo) string {
	names := make([]string, len(users))
	for i, u := range users {
		names[i] = u.Username
	}
	return strings.Join(names, ", ")
}

func renderField(label, value string) string {
	return sidebarLabelStyle.Render(fmt.Sprintf("%-10s", label)) + "  " + sidebarValueStyle.Render(value)
}

func renderPipelineStatus(status string) string {
	switch status {
	case "success":
		return pipelineSuccessStyle.Render("passed")
	case "failed":
		return pipelineFailedStyle.Render("failed")
	case "running":
		return pipelineRunningStyle.Render("running")
	case "pending":
		return pipelinePendingStyle.Render("pending")
	case "canceled":
		return pipelinePendingStyle.Render("canceled")
	case "skipped":
		return pipelinePendingStyle.Render("skipped")
	default:
		return sidebarValueStyle.Render(status)
	}
}

func renderMarkdown(md string, width int) string {
	r, err := glamour.NewTermRenderer(
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return md
	}

	rendered, err := r.Render(md)
	if err != nil {
		return md
	}

	return strings.TrimSpace(rendered)
}
