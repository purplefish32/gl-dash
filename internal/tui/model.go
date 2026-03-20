package tui

import (
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	zone "github.com/lrstanley/bubblezone/v2"

	"github.com/purplefish32/gl-dash/internal/config"
	"github.com/purplefish32/gl-dash/internal/data"
	"github.com/purplefish32/gl-dash/internal/git"
)

type tickMsg time.Time

type viewType int

const (
	viewMRs viewType = iota
	viewIssues
	viewTodos
	viewCount // sentinel for cycling
)

type statusMsg struct{ text string }

type sectionMRsMsg struct {
	index    int
	mrs      []data.MergeRequest
	page     int
	pageInfo data.PageInfo
}

type sectionIssuesMsg struct {
	index    int
	issues   []data.Issue
	page     int
	pageInfo data.PageInfo
}

type sectionTodosMsg struct {
	index int
	todos []data.Todo
}

type sectionErrMsg struct {
	view  viewType
	index int
	err   error
}

type sidebarDataMsg struct {
	notes     []data.MRNote
	commits   []data.MRCommit
	pipelines []data.MRPipeline
	changes   []data.MRChange
	tab       sidebarTab
	err       error
}

type todoDoneMsg struct{ id int }
type allTodosDoneMsg struct{}
type projectResolvedMsg struct {
	id  int
	err error
}

type searchProjectResolvedMsg struct {
	id  int
	err error
}

type pickerItemsMsg struct {
	items []pickerItem
}

type Model struct {
	cfg           *config.Config
	client        *data.Client
	view          viewType
	mrSections    []section
	issSections   []section
	todoSections  []section
	issuesLoaded  bool
	todosLoaded   bool
	activeTab     int
	showSidebar   bool
	showHelp      bool
	sidebar       sidebarState
	searching     bool
	searchInput   string
	activePicker  *picker
	statusText    string
	width         int
	height        int
	// Smart repo filtering
	projectPath   string // detected from git remote (e.g. "group/project")
	projectID     int    // resolved GitLab project ID
	repoFilter    bool   // whether repo filtering is active
}

func (m Model) activeSections() []section {
	switch m.view {
	case viewMRs:
		return m.mrSections
	case viewIssues:
		return m.issSections
	case viewTodos:
		return m.todoSections
	}
	return nil
}

func (m *Model) activeSection() *section {
	switch m.view {
	case viewMRs:
		return &m.mrSections[m.activeTab]
	case viewIssues:
		return &m.issSections[m.activeTab]
	case viewTodos:
		return &m.todoSections[m.activeTab]
	}
	return &m.mrSections[0]
}

// contentHeight returns the number of item rows that fit in the viewport.
// Accounts for: header(2) + tabs(2) + col header(2) + search(0-1) + footer(2) = ~8 fixed rows.
const cardHeight = 4 // lines per card (line1 + title + metadata + blank)

func (m Model) contentHeight() int {
	fixed := 10 // title(3) + blank + viewbar(1) + blank + tabs(2) + footer(2)
	s := m.activeSection()
	if m.searching || s.searchQuery != "" {
		fixed++
	}
	h := m.height - fixed
	if h < 1 {
		h = 1
	}
	// Return number of items that fit
	return h / cardHeight
}

func (m Model) selectedURL() string {
	sections := m.activeSections()
	if m.activeTab >= len(sections) {
		return ""
	}
	return sections[m.activeTab].selectedURL()
}

func (m Model) selectedTodo() *data.Todo {
	if m.view != viewTodos {
		return nil
	}
	s := m.todoSections[m.activeTab]
	todos := s.visibleTodos()
	if len(todos) == 0 || s.cursor >= len(todos) {
		return nil
	}
	return &todos[s.cursor]
}

func openBrowser(url string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", url)
		case "linux":
			cmd = exec.Command("xdg-open", url)
		default:
			cmd = exec.Command("open", url)
		}
		if err := cmd.Run(); err != nil {
			return statusMsg{text: fmt.Sprintf("Failed to open browser: %v", err)}
		}
		return statusMsg{text: "Opened in browser"}
	}
}

func copyToClipboard(text string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("pbcopy")
		case "linux":
			cmd = exec.Command("xclip", "-selection", "clipboard")
		default:
			return statusMsg{text: "Clipboard not supported on this OS"}
		}
		cmd.Stdin = strings.NewReader(text)
		if err := cmd.Run(); err != nil {
			return statusMsg{text: "Failed to copy to clipboard"}
		}
		return statusMsg{text: "Copied URL to clipboard"}
	}
}

func gitlabHost(baseURL string) string {
	// Extract host from URL like "https://gitlab.com"
	u, err := url.Parse(baseURL)
	if err != nil {
		return "gitlab.com"
	}
	return u.Hostname()
}

func NewModel(cfg *config.Config, client *data.Client) Model {
	mrSections := make([]section, len(cfg.Sections.MergeRequests))
	for i, sc := range cfg.Sections.MergeRequests {
		mrSections[i] = newSection(sc)
	}

	issSections := make([]section, len(cfg.Sections.Issues))
	for i, sc := range cfg.Sections.Issues {
		issSections[i] = newSection(sc)
	}

	todoSections := []section{
		newSection(config.SectionConfig{Title: "Pending", Filter: "pending", Limit: 50}),
		newSection(config.SectionConfig{Title: "Done", Filter: "done", Limit: 20}),
	}

	projectPath := git.DetectProjectPath(gitlabHost(cfg.GitLab.BaseURL))

	return Model{
		cfg:          cfg,
		client:       client,
		view:         viewMRs,
		mrSections:   mrSections,
		issSections:  issSections,
		todoSections: todoSections,
		showSidebar:  false,
		projectPath:  projectPath,
	}
}

func (m Model) Init() tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(m.mrSections)+2)

	// If launched from a git repo and smartFilter is enabled (default: true)
	smartFilter := m.cfg.SmartFilter == nil || *m.cfg.SmartFilter
	if m.projectPath != "" && smartFilter {
		client := m.client
		path := m.projectPath
		cmds = append(cmds, func() tea.Msg {
			id, err := client.ResolveProjectID(path)
			return projectResolvedMsg{id: id, err: err}
		})
	} else {
		for i := range m.mrSections {
			cmds = append(cmds, m.fetchMRSection(i))
		}
	}

	if m.cfg.RefreshMinutes > 0 {
		cmds = append(cmds, m.tickCmd())
	}
	return tea.Batch(cmds...)
}

func (m Model) tickCmd() tea.Cmd {
	return tea.Tick(time.Duration(m.cfg.RefreshMinutes)*time.Minute, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func (m Model) activeProjectID() int {
	if m.repoFilter {
		return m.projectID
	}
	return 0
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case sectionMRsMsg:
		if msg.index < len(m.mrSections) {
			s := &m.mrSections[msg.index]
			s.loading = false
			s.hasMore = msg.pageInfo.HasMore
			s.page = msg.page
			if msg.page > 1 {
				s.mrs = append(s.mrs, msg.mrs...)
			} else {
				s.mrs = msg.mrs
			}
			if s.searchQuery != "" && !s.searchParsed.hasOverrides {
				s.applyFilter()
			}
		}

	case sectionIssuesMsg:
		if msg.index < len(m.issSections) {
			s := &m.issSections[msg.index]
			s.loading = false
			s.hasMore = msg.pageInfo.HasMore
			s.page = msg.page
			if msg.page > 1 {
				s.issues = append(s.issues, msg.issues...)
			} else {
				s.issues = msg.issues
			}
			if s.searchQuery != "" && !s.searchParsed.hasOverrides {
				s.applyFilter()
			}
		}

	case sectionTodosMsg:
		if msg.index < len(m.todoSections) {
			s := &m.todoSections[msg.index]
			s.loading = false
			s.todos = msg.todos
			if s.searchQuery != "" && !s.searchParsed.hasOverrides {
				s.applyFilter()
			}
		}

	case sectionErrMsg:
		switch msg.view {
		case viewMRs:
			if msg.index < len(m.mrSections) {
				m.mrSections[msg.index].loading = false
				m.mrSections[msg.index].err = msg.err
			}
		case viewIssues:
			if msg.index < len(m.issSections) {
				m.issSections[msg.index].loading = false
				m.issSections[msg.index].err = msg.err
			}
		case viewTodos:
			if msg.index < len(m.todoSections) {
				m.todoSections[msg.index].loading = false
				m.todoSections[msg.index].err = msg.err
			}
		}

	case commandDoneMsg:
		if msg.err != nil {
			m.statusText = fmt.Sprintf("%s failed: %v", msg.action, msg.err)
		} else {
			m.statusText = fmt.Sprintf("%s: done", msg.action)
		}

	case sidebarDataMsg:
		m.sidebar.loadErr = msg.err
		switch msg.tab {
		case tabDiscussion:
			m.sidebar.notes = msg.notes
		case tabCommits:
			m.sidebar.commits = msg.commits
		case tabPipeline:
			m.sidebar.pipelines = msg.pipelines
		case tabChanges:
			m.sidebar.changes = msg.changes
		}

	case todoDoneMsg:
		m.statusText = "Todo marked as done"
		// Refresh pending todos
		m.todoSections[0].loading = true
		return m, m.fetchTodoSection(0)

	case allTodosDoneMsg:
		m.statusText = "All todos marked as done"
		for i := range m.todoSections {
			m.todoSections[i].loading = true
		}
		cmds := make([]tea.Cmd, len(m.todoSections))
		for i := range m.todoSections {
			cmds[i] = m.fetchTodoSection(i)
		}
		return m, tea.Batch(cmds...)

	case tickMsg:
		// Auto-refresh active view silently, then restart timer
		cmds := []tea.Cmd{m.tickCmd()}
		switch m.view {
		case viewMRs:
			for i := range m.mrSections {
				cmds = append(cmds, m.fetchMRSection(i))
			}
		case viewIssues:
			if m.issuesLoaded {
				for i := range m.issSections {
					cmds = append(cmds, m.fetchIssueSection(i))
				}
			}
		case viewTodos:
			if m.todosLoaded {
				for i := range m.todoSections {
					cmds = append(cmds, m.fetchTodoSection(i))
				}
			}
		}
		return m, tea.Batch(cmds...)

	case projectResolvedMsg:
		if msg.err != nil {
			m.statusText = fmt.Sprintf("Failed to resolve project: %v", msg.err)
			m.repoFilter = false
		} else {
			m.projectID = msg.id
			m.repoFilter = true
			m.statusText = fmt.Sprintf("Filtered to %s", m.projectPath)
			return m, m.refreshAll()
		}

	case pickerItemsMsg:
		if m.activePicker != nil {
			m.activePicker.mergeItems(msg.items)
			m.activePicker.applyFilter()
		}

	case searchProjectResolvedMsg:
		if msg.err != nil {
			m.statusText = fmt.Sprintf("Project not found: %v", msg.err)
			m.activeSection().searchParsed.project = ""
		} else {
			return m, m.refreshCurrentSection()
		}

	case statusMsg:
		m.statusText = msg.text

	case tea.KeyMsg:
		m.statusText = ""

		// Help overlay - dismiss on any key
		if m.showHelp {
			switch msg.String() {
			case "ctrl+c":
				return m, tea.Quit
			default:
				m.showHelp = false
			}
			return m, nil
		}

		// Picker mode (inside search)
		if m.activePicker != nil {
			switch msg.String() {
			case "esc":
				// Close picker, stay in search mode
				m.activePicker = nil
			case "enter":
				// Insert selection into search input
				selected := m.activePicker.selected()
				m.searchInput += selected + " "
				m.activePicker = nil
			case "ctrl+c":
				return m, tea.Quit
			case "down", "ctrl+j", "ctrl+n":
				if m.activePicker.cursor < len(m.activePicker.filtered)-1 {
					m.activePicker.cursor++
				}
			case "up", "ctrl+k", "ctrl+p":
				if m.activePicker.cursor > 0 {
					m.activePicker.cursor--
				}
			case "backspace":
				if len(m.activePicker.query) > 0 {
					m.activePicker.query = m.activePicker.query[:len(m.activePicker.query)-1]
					m.activePicker.applyFilter()
				} else {
					// Close picker if query is empty
					m.activePicker = nil
				}
			default:
				key := msg.String()
				if len(key) == 1 {
					// If picker is "!" and user types another "!", switch to target branch
					if m.activePicker.prefix == "!" && key == "!" && m.activePicker.query == "" {
						m.searchInput = m.searchInput[:len(m.searchInput)-1] + "!!"
						p := newPicker("!!")
						items := m.collectBranches(false)
						p.setItems(items)
						m.activePicker = &p
						return m, nil
					}
					m.activePicker.query += key
					m.activePicker.applyFilter()
					// For user picker, also search the API for more results
					if m.activePicker.prefix == "@" && len(m.activePicker.query) >= 2 {
						client := m.client
						query := m.activePicker.query
						return m, func() tea.Msg {
							candidates, err := client.FetchUsers(query)
							if err != nil || len(candidates) == 0 {
								return nil
							}
							items := make([]pickerItem, 0, len(candidates))
							for _, c := range candidates {
								// Filter out bots
								if strings.HasPrefix(c.Value, "project_") || strings.HasPrefix(c.Value, "group_") {
									continue
								}
								items = append(items, pickerItem{value: c.Value, display: c.Display})
							}
							return pickerItemsMsg{items: items}
						}
					}
				}
			}
			return m, nil
		}

		// Search mode
		if m.searching {
			switch msg.String() {
			case "esc":
				m.searching = false
				m.searchInput = ""
				m.activeSection().clearFilter()
				return m, m.refreshCurrentSection()
			case "enter":
				m.searching = false
				if m.searchInput != "" {
					cmd := m.executeSearch(m.searchInput)
					return m, cmd
				} else {
					// Empty input — clear filter and refetch with base config
					m.activeSection().clearFilter()
					return m, m.refreshCurrentSection()
				}
			case "backspace":
				if len(m.searchInput) > 0 {
					m.searchInput = m.searchInput[:len(m.searchInput)-1]
				}
			case "ctrl+c":
				return m, tea.Quit
			default:
				key := msg.String()
				if key == "space" {
					key = " "
				}
				if len(key) == 1 {
					m.searchInput += key
					// Check if we should open a picker
					if cmd := m.maybeOpenPicker(key); cmd != nil {
						return m, cmd
					}
				}
			}
			return m, nil
		}

		// Modal mode — scroll, tab switch, close, quit
		if m.showSidebar {
			switch msg.String() {
			case "q", "ctrl+c":
				return m, tea.Quit
			case "esc", "p":
				m.showSidebar = false
				m.sidebar.scrollY = 0
			case "]", "s", "tab", "l", "right":
				if m.view == viewMRs {
					m.sidebar.activeTab = (m.sidebar.activeTab + 1) % tabCount
					m.sidebar.scrollY = 0
					return m, m.fetchSidebarData()
				}
			case "[", "shift+tab", "h", "left":
				if m.view == viewMRs {
					m.sidebar.activeTab = (m.sidebar.activeTab - 1 + tabCount) % tabCount
					m.sidebar.scrollY = 0
					return m, m.fetchSidebarData()
				}
			case "j", "down":
				m.sidebar.scrollY++
			case "k", "up":
				if m.sidebar.scrollY > 0 {
					m.sidebar.scrollY--
				}
			case "ctrl+d":
				m.sidebar.scrollY += 10
			case "ctrl+u":
				if m.sidebar.scrollY > 10 {
					m.sidebar.scrollY -= 10
				} else {
					m.sidebar.scrollY = 0
				}
			case "g", "home":
				m.sidebar.scrollY = 0
			case "G", "end":
				m.sidebar.scrollY = 999 // will be clamped by render
			case "o":
				// Open context-appropriate GitLab URL based on active tab
				s := m.activeSections()[m.activeTab]
				mrs := s.visibleMRs()
				if m.view == viewMRs && len(mrs) > 0 && s.cursor < len(mrs) {
					mr := mrs[s.cursor]
					parts := strings.Split(mr.WebURL, "/-/")
					if len(parts) > 0 {
						var url string
						switch m.sidebar.activeTab {
						case tabOverview, tabDiscussion:
							url = mr.WebURL
						case tabCommits:
							url = parts[0] + "/-/merge_requests/" + fmt.Sprintf("%d", mr.IID) + "/commits"
						case tabPipeline:
							url = parts[0] + "/-/pipelines"
						case tabChanges:
							url = parts[0] + "/-/merge_requests/" + fmt.Sprintf("%d", mr.IID) + "/diffs"
						}
						if url != "" {
							return m, openBrowser(url)
						}
					}
				} else if url := m.selectedURL(); url != "" {
					return m, openBrowser(url)
				}
			}
			return m, nil
		}

		// Normal mode
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "/":
			m.searching = true
			s := m.activeSection()
			if s.searchQuery != "" {
				m.searchInput = s.searchQuery
			} else {
				m.searchInput = ""
			}
		case "s":
			return m, m.switchView()
		case "tab", "l", "right":
			return m, m.switchView()
		case "shift+tab", "h", "left":
			m.view = (m.view - 1 + viewCount) % viewCount
			m.activeTab = 0
			return m, m.ensureViewLoaded()
		case "j", "down":
			s := m.activeSection()
			max := s.itemCount() - 1
			if s.cursor < max {
				s.cursor++
				s.ensureCursorVisible(m.contentHeight())
				if m.showSidebar && m.sidebar.activeTab != tabOverview {
					return m, m.fetchSidebarData()
				}
			} else if s.hasMore && !s.loading {
				return m, m.loadMoreCurrentSection()
			}
		case "k", "up":
			s := m.activeSection()
			if s.cursor > 0 {
				s.cursor--
				s.ensureCursorVisible(m.contentHeight())
				if m.showSidebar && m.sidebar.activeTab != tabOverview {
					return m, m.fetchSidebarData()
				}
			}
		case "g", "home":
			s := m.activeSection()
			s.cursor = 0
			s.ensureCursorVisible(m.contentHeight())
		case "G", "end":
			s := m.activeSection()
			if s.itemCount() > 0 {
				s.cursor = s.itemCount() - 1
				s.ensureCursorVisible(m.contentHeight())
			}
		case "ctrl+d":
			s := m.activeSection()
			half := m.contentHeight() / 2
			s.cursor += half
			if s.cursor >= s.itemCount() {
				s.cursor = s.itemCount() - 1
			}
			s.ensureCursorVisible(m.contentHeight())
		case "ctrl+u":
			s := m.activeSection()
			half := m.contentHeight() / 2
			s.cursor -= half
			if s.cursor < 0 {
				s.cursor = 0
			}
			s.ensureCursorVisible(m.contentHeight())
		case "o":
			if url := m.selectedURL(); url != "" {
				return m, openBrowser(url)
			}
		case "y":
			if url := m.selectedURL(); url != "" {
				return m, copyToClipboard(url)
			}
		case "d":
			if m.view == viewTodos {
				if todo := m.selectedTodo(); todo != nil {
					client := m.client
					id := todo.ID
					return m, func() tea.Msg {
						client.MarkTodoAsDone(id)
						return todoDoneMsg{id: id}
					}
				}
			}
		case "D":
			if m.view == viewTodos {
				client := m.client
				return m, func() tea.Msg {
					client.MarkAllTodosAsDone()
					return allTodosDoneMsg{}
				}
			}
		case "t":
			if m.projectPath == "" {
				m.statusText = "Not in a GitLab repository"
			} else if m.repoFilter {
				// Toggle off
				m.repoFilter = false
				m.statusText = "Showing all projects"
				return m, m.refreshAll()
			} else if m.projectID > 0 {
				// Already resolved, toggle on
				m.repoFilter = true
				m.statusText = fmt.Sprintf("Filtered to %s", m.projectPath)
				return m, m.refreshAll()
			} else {
				// Need to resolve project ID first
				m.statusText = fmt.Sprintf("Resolving %s...", m.projectPath)
				client := m.client
				path := m.projectPath
				return m, func() tea.Msg {
					id, err := client.ResolveProjectID(path)
					return projectResolvedMsg{id: id, err: err}
				}
			}
		case "?":
			m.showHelp = !m.showHelp
		case "v":
			if m.view == viewMRs {
				s := m.activeSections()[m.activeTab]
				mrs := s.visibleMRs()
				if len(mrs) > 0 && s.cursor < len(mrs) {
					mr := mrs[s.cursor]
					m.statusText = fmt.Sprintf("Reviewing %s...", mr.SourceBranch)
					return m, m.executeReview(mr)
				}
			}
		case "L":
			m.statusText = "Launching local tool..."
			return m, m.executeLocal()
		case "p", "enter":
			m.showSidebar = true
			if m.view == viewMRs {
				return m, m.fetchSidebarData()
			}
		case "r":
			return m, m.refreshCurrentSection()
		case "R":
			return m, m.refreshAll()
		}

	case tea.MouseWheelMsg:
		// Scroll modal if open
		if m.showSidebar {
			if msg.Button == tea.MouseWheelUp {
				if m.sidebar.scrollY > 0 {
					m.sidebar.scrollY--
				}
			} else if msg.Button == tea.MouseWheelDown {
				m.sidebar.scrollY++
			}
			return m, nil
		}
		s := m.activeSection()
		if msg.Button == tea.MouseWheelUp {
			if s.cursor > 0 {
				s.cursor--
				s.ensureCursorVisible(m.contentHeight())
			}
		} else if msg.Button == tea.MouseWheelDown {
			max := s.itemCount() - 1
			if s.cursor < max {
				s.cursor++
				s.ensureCursorVisible(m.contentHeight())
			} else if s.hasMore && !s.loading {
				return m, m.loadMoreCurrentSection()
			}
		}

	case tea.MouseClickMsg:
		if msg.Button == tea.MouseLeft {
			// Check view switcher zones
			for v := viewType(0); v < viewCount; v++ {
				if zone.Get(fmt.Sprintf("view-%d", v)).InBounds(msg) {
					if v != m.view {
						m.view = v
						m.activeTab = 0
						return m, m.ensureViewLoaded()
					}
					break
				}
			}

			// Check section tab zones
			for i := range m.activeSections() {
				if zone.Get(fmt.Sprintf("tab-%d", i)).InBounds(msg) {
					m.activeTab = i
					return m, nil
				}
			}

			// Check sidebar tab zones
			if m.showSidebar && m.view == viewMRs {
				for t := sidebarTab(0); t < tabCount; t++ {
					if zone.Get(fmt.Sprintf("sidebar-%d", t)).InBounds(msg) {
						if t != m.sidebar.activeTab {
							m.sidebar.activeTab = t
							return m, m.fetchSidebarData()
						}
						break
					}
				}
			}

			// Check file change clicks in sidebar
			if m.showSidebar && m.sidebar.activeTab == tabChanges && m.sidebar.changes != nil {
				for _, ch := range m.sidebar.changes {
					if zone.Get("file-"+ch.NewPath).InBounds(msg) {
						s := m.activeSections()[m.activeTab]
						mrs := s.visibleMRs()
						if len(mrs) > 0 && s.cursor < len(mrs) {
							mr := mrs[s.cursor]
							parts := strings.Split(mr.WebURL, "/-/")
							if len(parts) > 0 {
								fileURL := fmt.Sprintf("%s/-/blob/%s/%s", parts[0], mr.SourceBranch, ch.NewPath)
								return m, openBrowser(fileURL)
							}
						}
						break
					}
				}
			}

			// Check pipeline clicks in sidebar
			if m.showSidebar && m.sidebar.activeTab == tabPipeline && m.sidebar.pipelines != nil {
				for _, p := range m.sidebar.pipelines {
					if zone.Get(fmt.Sprintf("pipeline-%d", p.ID)).InBounds(msg) {
						s := m.activeSections()[m.activeTab]
						mrs := s.visibleMRs()
						if len(mrs) > 0 && s.cursor < len(mrs) {
							mr := mrs[s.cursor]
							parts := strings.Split(mr.WebURL, "/-/")
							if len(parts) > 0 {
								pipeURL := fmt.Sprintf("%s/-/pipelines/%d", parts[0], p.ID)
								return m, openBrowser(pipeURL)
							}
						}
						break
					}
				}
			}

			// Check commit hash clicks in sidebar
			if m.showSidebar && m.sidebar.activeTab == tabCommits && m.sidebar.commits != nil {
				for _, cm := range m.sidebar.commits {
					if zone.Get("commit-"+cm.ID).InBounds(msg) {
						// Build commit URL from MR's WebURL
						s := m.activeSections()[m.activeTab]
						mrs := s.visibleMRs()
						if len(mrs) > 0 && s.cursor < len(mrs) {
							mr := mrs[s.cursor]
							// WebURL: https://host/group/project/-/merge_requests/123
							// Commit URL: https://host/group/project/-/commit/SHA
							parts := strings.Split(mr.WebURL, "/-/")
							if len(parts) > 0 {
								commitURL := parts[0] + "/-/commit/" + cm.ID
								return m, openBrowser(commitURL)
							}
						}
						break
					}
				}
			}

			// Card click (Y-based) — skip if modal is open
			if !m.showSidebar {
				headerLines := 6
				if m.searching || m.activeSection().searchQuery != "" {
					headerLines++
				}
				clickY := msg.Y - headerLines
				if clickY >= 0 {
					s := m.activeSection()
					clickedIdx := s.offset + (clickY / cardHeight)
					if clickedIdx >= 0 && clickedIdx < s.itemCount() {
						s.cursor = clickedIdx
					}
				}
			}
		}
	}

	return m, nil
}
