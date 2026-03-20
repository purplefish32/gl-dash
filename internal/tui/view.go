package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	zone "github.com/lrstanley/bubblezone/v2"

	"github.com/purplefish32/gl-dash/internal/data"
)

var (
	viewActiveStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")).
		Background(lipgloss.Color("#FF6B00")).
		Padding(0, 1)

	viewInactiveStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Padding(0, 1)

	tabActiveStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FF6B00")).
		Padding(0, 2)

	tabInactiveStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Padding(0, 2)

	titleStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FF6B00"))

	colHeaderStyle = lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#888888"))

	separatorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#444444"))

	selectedRowStyle = lipgloss.NewStyle().
		Background(lipgloss.Color("#333333")).
		Foreground(lipgloss.Color("#FFFFFF"))

	draftRowStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888"))

	errorStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF0000"))

	helpStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666"))

	searchPromptStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF6B00")).
		Bold(true)

	searchInputStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFFFFF"))

	searchActiveStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888")).
		Italic(true)

	cardIIDStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFAA00")).
		Bold(true)

	cardTitleStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#EEEEEE")).
		Bold(true)

	cardMetaStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666"))

	cardLabelStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#000000")).
		Background(lipgloss.Color("#FF6B00")).
		Padding(0, 1)

	cardBranchStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#555555"))

	cardSelectedBar = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF6B00")).
		Bold(true)

	cardSelectedBg = lipgloss.NewStyle().
		Background(lipgloss.Color("#1a1a2e"))

	cardSepStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#2a2a2a"))

	todoActionStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFAA00"))

	todoProjectStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888"))
)

func (m Model) viewLabel(v viewType) string {
	switch v {
	case viewMRs:
		return "MRs"
	case viewIssues:
		return "Issues"
	case viewTodos:
		return "Todos"
	}
	return ""
}

func (m Model) View() tea.View {
	if m.width == 0 {
		return tea.NewView("")
	}

	listWidth := m.width

	var rows []string

	// Header with view switcher
	// App title
	appTitle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FF6B00")).
		Render("  ╔═╗╦  ┌┬┐┌─┐┌─┐┬ ┬") + "\n" +
		lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FF6B00")).
			Render("  ║ ╦║   ││├─┤└─┐├─┤") + "\n" +
		lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#CC5500")).
			Render("  ╚═╝╩═╝─┴┘┴ ┴└─┘┴ ┴")
	rows = append(rows, appTitle, "")

	// View switcher
	viewBar := "  "
	for v := viewType(0); v < viewCount; v++ {
		label := m.viewLabel(v)
		if v == m.view {
			viewBar += zone.Mark(fmt.Sprintf("view-%d", v), viewActiveStyle.Render(label))
		} else {
			viewBar += zone.Mark(fmt.Sprintf("view-%d", v), viewInactiveStyle.Render(label))
		}
		if v < viewCount-1 {
			viewBar += " "
		}
	}
	if m.repoFilter && m.projectPath != "" {
		viewBar += "  " + lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00CC00")).
			Bold(true).
			Render("["+m.projectPath+"]")
	}
	rows = append(rows, viewBar, "")

	// Section tabs
	sections := m.activeSections()
	var tabs []string
	for i, s := range sections {
		label := s.config.Title
		count := s.itemCount()
		if s.loading {
			label += " ..."
		} else {
			label += fmt.Sprintf(" (%d)", count)
		}

		if i == m.activeTab {
			tabs = append(tabs, zone.Mark(fmt.Sprintf("tab-%d", i), tabActiveStyle.Render(label)))
		} else {
			tabs = append(tabs, zone.Mark(fmt.Sprintf("tab-%d", i), tabInactiveStyle.Render(label)))
		}
	}
	rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, tabs...))
	rows = append(rows, separatorStyle.Render(strings.Repeat("─", listWidth)))

	// Search bar
	s := sections[m.activeTab]
	if m.searching {
		rows = append(rows, searchPromptStyle.Render("  / ")+searchInputStyle.Render(m.searchInput)+"_")
	} else if s.searchQuery != "" {
		hint := s.searchQuery
		if s.searchParsed.hasOverrides {
			hint = s.searchParsed.formatSearchHint()
		}
		filterLine := fmt.Sprintf("  filter: %s", hint)
		if s.loading {
			filterLine += "  " + lipgloss.NewStyle().Foreground(lipgloss.Color("#FFAA00")).Render("searching...")
		}
		rows = append(rows, searchActiveStyle.Render(filterLine))
	}

	// Content
	if s.loading {
		rows = append(rows, "  "+lipgloss.NewStyle().Foreground(lipgloss.Color("#FFAA00")).Render("Loading..."))
	} else if s.err != nil {
		rows = append(rows, "  "+errorStyle.Render(fmt.Sprintf("Error: %v", s.err)))
	} else if s.itemCount() == 0 {
		if s.searchQuery != "" {
			rows = append(rows, "  No results matching filter.")
		} else {
			switch m.view {
			case viewMRs:
				rows = append(rows, "  No merge requests found.")
			case viewIssues:
				rows = append(rows, "  No issues found.")
			case viewTodos:
				rows = append(rows, "  No todos found.")
			}
		}
	} else {
		switch m.view {
		case viewMRs:
			rows = append(rows, m.renderMRRows(s, listWidth)...)
		case viewIssues:
			rows = append(rows, m.renderIssueRows(s, listWidth)...)
		case viewTodos:
			rows = append(rows, m.renderTodoRows(s, listWidth)...)
		}
	}

	// Footer
	rows = append(rows, "")
	if m.statusText != "" {
		rows = append(rows, lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00CC00")).
			Render("  "+m.statusText))
	} else if m.searching {
		rows = append(rows, helpStyle.Render(
			"  @user  @r:reviewer  #project  ~label  !branch  !!target  text | enter: search  esc: cancel"))
	} else {
		var hints []string
		hints = append(hints, "j/k:nav", "/:search", "s:view")
		if s.searchQuery != "" {
			hints = append(hints, "esc:clear filter")
		}
		switch m.view {
		case viewMRs:
			hints = append(hints, "o:open", "p:preview", "v:review", "t:repo filter")
		case viewIssues:
			hints = append(hints, "o:open", "p:preview", "t:repo filter")
		case viewTodos:
			hints = append(hints, "o:open", "d:done", "D:all done")
		}
		hints = append(hints, "?:help", "q:quit")
		rows = append(rows, helpStyle.Render("  "+strings.Join(hints, "  ")))
	}

	listContent := strings.Join(rows, "\n")

	// Preview modal overlay
	if m.showSidebar && !s.loading && s.err == nil && s.itemCount() > 0 {
		modalW := m.width - 8
		if modalW > 120 {
			modalW = 120
		}
		modalH := m.height - 6
		var modal string
		switch m.view {
		case viewMRs:
			visMRs := s.visibleMRs()
			if s.cursor < len(visMRs) {
				modal = renderMRSidebar(visMRs[s.cursor], &m.sidebar, modalW, modalH)
			}
		case viewIssues:
			visIssues := s.visibleIssues()
			if s.cursor < len(visIssues) {
				modal = renderIssueSidebar(visIssues[s.cursor], modalW, modalH)
			}
		case viewTodos:
			visTodos := s.visibleTodos()
			if s.cursor < len(visTodos) {
				modal = renderTodoSidebar(visTodos[s.cursor], modalW, modalH)
			}
		}
		if modal != "" {
			listContent = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, modal)
		}
	}

	// Picker overlay — replace content area with picker
	if m.activePicker != nil {
		pickerView := m.activePicker.render(listWidth, m.height/2)
		// Re-build: keep header rows, replace content with picker
		var headerRows []string
		headerRows = append(headerRows, rows[:4]...) // header + tabs + separator
		if m.searching {
			headerRows = append(headerRows, searchPromptStyle.Render("  / ")+searchInputStyle.Render(m.searchInput)+"_")
		}
		headerRows = append(headerRows, "")
		headerRows = append(headerRows, pickerView)
		listContent = strings.Join(headerRows, "\n")
	}

	// Help overlay
	if m.showHelp {
		listContent = m.renderHelp()
	}

	v := tea.NewView(zone.Scan(listContent))
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (m Model) renderHelp() string {
	helpKeyStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FF6B00")).
		Width(14)

	helpDescStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#CCCCCC"))

	helpSectionStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FFFFFF")).
		MarginTop(1)

	line := func(key, desc string) string {
		return "  " + helpKeyStyle.Render(key) + helpDescStyle.Render(desc)
	}

	var lines []string
	lines = append(lines, titleStyle.Render("  gl-dash")+" "+helpStyle.Render("Keyboard Shortcuts"))
	lines = append(lines, "")

	lines = append(lines, helpSectionStyle.Render("  Navigation"))
	lines = append(lines, line("j / ↓", "Move cursor down"))
	lines = append(lines, line("k / ↑", "Move cursor up"))
	lines = append(lines, line("g / Home", "Jump to top"))
	lines = append(lines, line("G / End", "Jump to bottom"))
	lines = append(lines, line("Ctrl+d", "Page down"))
	lines = append(lines, line("Ctrl+u", "Page up"))
	lines = append(lines, line("h/l ←/→", "Switch section tab"))
	lines = append(lines, line("Tab", "Next section tab"))
	lines = append(lines, line("Shift+Tab", "Previous section tab"))
	lines = append(lines, line("s", "Cycle view (MRs/Issues/Todos)"))

	lines = append(lines, helpSectionStyle.Render("  Actions"))
	lines = append(lines, line("o / Enter", "Open in browser"))
	lines = append(lines, line("y", "Copy URL to clipboard"))
	lines = append(lines, line("/", "Search / filter"))

	lines = append(lines, helpSectionStyle.Render("  Search Prefixes"))
	lines = append(lines, line("@username", "Filter by author"))
	lines = append(lines, line("@r:username", "Filter by reviewer"))
	lines = append(lines, line("@a:username", "Filter by assignee"))
	lines = append(lines, line("#group/proj", "Filter by project"))
	lines = append(lines, line("~label", "Filter by label"))
	lines = append(lines, line("!branch", "Filter by source branch"))
	lines = append(lines, line("!!branch", "Filter by target branch"))
	lines = append(lines, line("text", "Search in title/description"))
	lines = append(lines, line("v", "Review MR (run reviewCommand)"))
	lines = append(lines, line("L", "Local tool (run localCommand)"))
	lines = append(lines, line("p", "Toggle preview sidebar"))
	lines = append(lines, line("] / [", "Next/prev sidebar tab (MRs only)"))
	lines = append(lines, line("r", "Refresh current tab"))
	lines = append(lines, line("R", "Refresh all tabs"))
	lines = append(lines, line("t", "Toggle repo filter (current git project)"))

	lines = append(lines, helpSectionStyle.Render("  Todos"))
	lines = append(lines, line("d", "Mark todo as done"))
	lines = append(lines, line("D", "Mark all todos as done"))

	lines = append(lines, helpSectionStyle.Render("  General"))
	lines = append(lines, line("?", "Toggle this help"))
	lines = append(lines, line("q / Ctrl+c", "Quit"))

	lines = append(lines, "")
	lines = append(lines, helpStyle.Render("  Press any key to close"))

	content := strings.Join(lines, "\n")

	overlay := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#FF6B00")).
		Padding(1, 2).
		Width(m.width - 10).
		Render(content)

	return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, overlay)
}

func relativeTime(ts string) string {
	t, err := time.Parse("2006-01-02 15:04", ts)
	if err != nil {
		return ts
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	case d < 30*24*time.Hour:
		return fmt.Sprintf("%dw ago", int(d.Hours()/(24*7)))
	default:
		return fmt.Sprintf("%dmo ago", int(d.Hours()/(24*30)))
	}
}

func shortProject(project string, maxLen int) string {
	if len(project) <= maxLen {
		return project
	}
	// Show last path segments that fit
	parts := strings.Split(project, "/")
	result := parts[len(parts)-1]
	for i := len(parts) - 2; i >= 0; i-- {
		candidate := parts[i] + "/" + result
		if len(candidate) > maxLen {
			break
		}
		result = candidate
	}
	return result
}

func truncLabels(labels []string, maxLen int) string {
	if len(labels) == 0 {
		return ""
	}
	result := strings.Join(labels, ",")
	if len(result) > maxLen {
		result = result[:maxLen-1] + "…"
	}
	return result
}

func mrStatusIcon(mr data.MergeRequest) string {
	if mr.Draft {
		return "📝"
	}
	switch mr.MergeStatus {
	case "mergeable":
		return "✅"
	case "not_approved":
		return "👀"
	case "need_rebase":
		return "⚡"
	case "has_conflicts", "conflict":
		return "❌"
	case "checking", "unchecked":
		return "⏳"
	case "ci_must_pass", "ci_still_running":
		return "🔄"
	case "discussions_not_resolved":
		return "💬"
	case "draft_status":
		return "📝"
	case "blocked_status":
		return "🚫"
	case "broken_status":
		return "💥"
	case "not_open":
		return "⬛"
	case "requested_changes":
		return "✏️"
	default:
		if mr.HasConflicts {
			return "❌"
		}
		return "🔵"
	}
}

var groupHeaderStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("#FF6B00")).
	Bold(true)

func (m Model) renderMRRows(s section, listWidth int) []string {
	var rows []string

	items := s.visibleMRs()
	viewH := m.contentHeight()
	end := s.offset + viewH
	if end > len(items) {
		end = len(items)
	}
	start := s.offset
	if start > len(items) {
		start = len(items)
	}

	maxTitleW := listWidth - 8
	if maxTitleW < 20 {
		maxTitleW = 20
	}

	lastProject := ""

	for idx := start; idx < end; idx++ {
		mr := items[idx]
		selected := idx == s.cursor

		// Group header
		if mr.Project != lastProject {
			lastProject = mr.Project
			proj := shortProject(mr.Project, listWidth-6)
			headerLine := "  " + groupHeaderStyle.Render(proj) + " " +
				cardSepStyle.Render(strings.Repeat("─", listWidth-len(proj)-4))
			rows = append(rows, headerLine, "")
		}

		// Line 1: status + IID + branch + pipeline dot
		pipelineDot := ""
		switch mr.Pipeline {
		case "success":
			pipelineDot = pipelineSuccessStyle.Render(" ●")
		case "failed":
			pipelineDot = pipelineFailedStyle.Render(" ●")
		case "running":
			pipelineDot = pipelineRunningStyle.Render(" ◐")
		case "pending":
			pipelineDot = pipelinePendingStyle.Render(" ○")
		}

		srcB := mr.SourceBranch
		tgtB := mr.TargetBranch

		statusIcon := mrStatusIcon(mr)
		iidStr := cardIIDStyle.Render(fmt.Sprintf("!%d", mr.IID))

		// Line 1: status + IID + title
		title := mr.Title
		if len(title) > maxTitleW {
			title = title[:maxTitleW-1] + "…"
		}
		line1content := fmt.Sprintf("%s  %s  %s%s", statusIcon, iidStr, cardTitleStyle.Render(title), pipelineDot)

		// Line 2: branch
		branchStr := cardBranchStyle.Render("\ue725 " + srcB + " → " + tgtB)
		line2content := branchStr

		// Line 3: Metadata with nerd font icons
		line3content := cardMetaStyle.Render("\uf007 "+mr.Author+" · \uf017 "+relativeTime(mr.UpdatedAt))
		if len(mr.Labels) > 0 {
			line3content += " "
			for _, l := range mr.Labels {
				line3content += " " + cardLabelStyle.Render(l)
			}
		}

		var line1, line2, line3 string
		if selected {
			bar := cardSelectedBar.Render("  ▌ ")
			line1 = bar + line1content
			line2 = bar + line2content
			line3 = bar + line3content
		} else {
			line1 = "    " + line1content
			line2 = "    " + line2content
			line3 = "    " + line3content
		}

		rows = append(rows, line1, line2, line3, "")
	}

	rows = append(rows, m.scrollIndicator(s, len(items))...)

	return rows
}

func (m Model) renderIssueRows(s section, listWidth int) []string {
	var rows []string

	items := s.visibleIssues()
	viewH := m.contentHeight()
	end := s.offset + viewH
	if end > len(items) {
		end = len(items)
	}
	start := s.offset
	if start > len(items) {
		start = len(items)
	}

	maxTitleW := listWidth - 8
	if maxTitleW < 20 {
		maxTitleW = 20
	}

	lastProject := ""

	for idx := start; idx < end; idx++ {
		issue := items[idx]
		selected := idx == s.cursor

		// Group header
		if issue.Project != lastProject {
			lastProject = issue.Project
			proj := shortProject(issue.Project, listWidth-6)
			headerLine := "  " + groupHeaderStyle.Render(proj) + " " +
				cardSepStyle.Render(strings.Repeat("─", listWidth-len(proj)-4))
			rows = append(rows, headerLine, "")
		}

		// Line 1: IID + comments
		iidStr := cardIIDStyle.Render(fmt.Sprintf("#%d", issue.IID))
		if issue.Confidential {
			iidStr += draftRowStyle.Render("  CONFIDENTIAL")
		}
		commentStr := ""
		if issue.UserNotesCount > 0 {
			commentStr = cardMetaStyle.Render(fmt.Sprintf("  \uf075 %d", issue.UserNotesCount))
		}
		line1content := iidStr + commentStr

		// Line 2: Title
		title := issue.Title
		if len(title) > maxTitleW {
			title = title[:maxTitleW-1] + "…"
		}
		line2content := cardTitleStyle.Render(title)

		// Line 3: Metadata with nerd font icons
		line3content := cardMetaStyle.Render("\uf007 "+issue.Author+" · \uf017 "+relativeTime(issue.UpdatedAt))
		if len(issue.Labels) > 0 {
			line3content += " "
			for _, l := range issue.Labels {
				line3content += " " + cardLabelStyle.Render(l)
			}
		}

		var line1, line2, line3 string
		if selected {
			bar := cardSelectedBar.Render("  ▌ ")
			line1 = bar + line1content
			line2 = bar + line2content
			line3 = bar + line3content
		} else {
			line1 = "    " + line1content
			line2 = "    " + line2content
			line3 = "    " + line3content
		}

		rows = append(rows, line1, line2, line3, "")
	}

	rows = append(rows, m.scrollIndicator(s, len(items))...)

	return rows
}

func (m Model) renderTodoRows(s section, listWidth int) []string {
	var rows []string

	items := s.visibleTodos()
	viewH := m.contentHeight()
	end := s.offset + viewH
	if end > len(items) {
		end = len(items)
	}
	start := s.offset
	if start > len(items) {
		start = len(items)
	}

	maxTitleW := listWidth - 8
	if maxTitleW < 20 {
		maxTitleW = 20
	}

	lastProject := ""

	for idx := start; idx < end; idx++ {
		todo := items[idx]
		selected := idx == s.cursor

		// Group header
		if todo.Project != lastProject {
			lastProject = todo.Project
			proj := shortProject(todo.Project, listWidth-6)
			headerLine := "  " + groupHeaderStyle.Render(proj) + " " +
				cardSepStyle.Render(strings.Repeat("─", listWidth-len(proj)-4))
			rows = append(rows, headerLine, "")
		}

		// Line 1: Type + Action
		typeStr := cardIIDStyle.Render(todo.TargetType)
		actionStr := todoActionStyle.Render(todo.Action)
		line1content := typeStr + "  " + actionStr

		// Line 2: Title
		title := todo.TargetTitle
		if len(title) > maxTitleW {
			title = title[:maxTitleW-1] + "…"
		}
		line2content := cardTitleStyle.Render(title)

		// Line 3: Metadata with nerd font icons
		line3content := cardMetaStyle.Render("\uf007 "+todo.Author+" · \uf017 "+relativeTime(todo.CreatedAt))

		var line1, line2, line3 string
		if selected {
			bar := cardSelectedBar.Render("  ▌ ")
			line1 = bar + line1content
			line2 = bar + line2content
			line3 = bar + line3content
		} else {
			line1 = "    " + line1content
			line2 = "    " + line2content
			line3 = "    " + line3content
		}

		rows = append(rows, line1, line2, line3, "")
	}

	rows = append(rows, m.scrollIndicator(s, len(items))...)

	return rows
}

func (m Model) scrollIndicator(s section, total int) []string {
	viewH := m.contentHeight()
	if total <= viewH && !s.hasMore {
		return nil
	}
	pos := s.cursor + 1
	indicator := fmt.Sprintf("  %d/%d", pos, total)
	if s.hasMore {
		indicator += " (more...)"
	}
	return []string{
		helpStyle.Render(indicator),
	}
}
