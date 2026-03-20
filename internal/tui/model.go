package tui

import (
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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

func (m *Model) fetchMRSection(index int) tea.Cmd {
	return m.fetchMRSectionPage(index, 1)
}

func (m *Model) fetchMRSectionPage(index int, page int) tea.Cmd {
	sc := m.mrSections[index].effectiveConfig()
	pid := m.activeProjectID()
	client := m.client
	return func() tea.Msg {
		mrs, pi, err := client.FetchMergeRequests(sc, pid, page)
		if err != nil {
			return sectionErrMsg{view: viewMRs, index: index, err: err}
		}
		return sectionMRsMsg{index: index, mrs: mrs, page: page, pageInfo: pi}
	}
}

func (m *Model) fetchIssueSection(index int) tea.Cmd {
	return m.fetchIssueSectionPage(index, 1)
}

func (m *Model) fetchIssueSectionPage(index int, page int) tea.Cmd {
	sc := m.issSections[index].effectiveConfig()
	pid := m.activeProjectID()
	client := m.client
	return func() tea.Msg {
		issues, pi, err := client.FetchIssues(sc, pid, page)
		if err != nil {
			return sectionErrMsg{view: viewIssues, index: index, err: err}
		}
		return sectionIssuesMsg{index: index, issues: issues, page: page, pageInfo: pi}
	}
}

func (m Model) fetchTodoSection(index int) tea.Cmd {
	sc := m.todoSections[index].config
	return func() tea.Msg {
		todos, err := m.client.FetchTodos(sc.Filter, sc.Limit)
		if err != nil {
			return sectionErrMsg{view: viewTodos, index: index, err: err}
		}
		return sectionTodosMsg{index: index, todos: todos}
	}
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

func (m *Model) ensureViewLoaded() tea.Cmd {
	switch m.view {
	case viewIssues:
		if !m.issuesLoaded {
			m.issuesLoaded = true
			cmds := make([]tea.Cmd, len(m.issSections))
			for i := range m.issSections {
				cmds[i] = m.fetchIssueSection(i)
			}
			return tea.Batch(cmds...)
		}
	case viewTodos:
		if !m.todosLoaded {
			m.todosLoaded = true
			cmds := make([]tea.Cmd, len(m.todoSections))
			for i := range m.todoSections {
				cmds[i] = m.fetchTodoSection(i)
			}
			return tea.Batch(cmds...)
		}
	}
	return nil
}

func (m *Model) switchView() tea.Cmd {
	m.view = (m.view + 1) % viewCount
	m.activeTab = 0
	return m.ensureViewLoaded()
}

func (m *Model) maybeOpenPicker(key string) tea.Cmd {
	// Only trigger on prefix characters when they're at the start of a token
	// (after a space or at the beginning of input)
	input := m.searchInput
	if len(input) < 1 {
		return nil
	}

	// Check if the last character is a trigger and it's the start of a new token
	lastChar := input[len(input)-1:]
	beforeLast := ""
	if len(input) > 1 {
		beforeLast = string(input[len(input)-2])
	}

	isTokenStart := len(input) == 1 || beforeLast == " "
	if !isTokenStart {
		return nil
	}

	switch lastChar {
	case "@":
		p := newPicker("@")
		m.activePicker = &p
		// Extract users from already-loaded MRs and issues
		items := m.collectUsers()
		p.setItems(items)
		m.activePicker = &p
		return nil
	case "#":
		p := newPicker("#")
		items := m.collectProjects()
		p.setItems(items)
		m.activePicker = &p
		return nil
	case "~":
		p := newPicker("~")
		items := m.collectLabels()
		p.setItems(items)
		m.activePicker = &p
		return nil
	case "!":
		// Check if this is "!!" (target branch) or "!" (source branch)
		if len(input) >= 2 && input[len(input)-2] == '!' {
			// "!!" — target branch picker, replace the first "!" in searchInput
			m.searchInput = input[:len(input)-2] + "!!"
			p := newPicker("!!")
			items := m.collectBranches(false)
			p.setItems(items)
			m.activePicker = &p
		} else {
			p := newPicker("!")
			items := m.collectBranches(true)
			p.setItems(items)
			m.activePicker = &p
		}
		return nil
	}

	return nil
}

func (m *Model) collectUsers() []pickerItem {
	seen := make(map[string]bool)
	var items []pickerItem

	addUser := func(username, name string) {
		if username == "" || seen[username] {
			return
		}
		seen[username] = true
		display := username
		if name != "" {
			display = username + " (" + name + ")"
		}
		items = append(items, pickerItem{value: username, display: display})
	}

	for _, s := range m.mrSections {
		for _, mr := range s.mrs {
			addUser(mr.Author, mr.AuthorName)
			for _, a := range mr.Assignees {
				addUser(a.Username, a.Name)
			}
			for _, r := range mr.Reviewers {
				addUser(r.Username, r.Name)
			}
		}
	}
	for _, s := range m.issSections {
		for _, issue := range s.issues {
			addUser(issue.Author, issue.AuthorName)
			for _, a := range issue.Assignees {
				addUser(a.Username, a.Name)
			}
		}
	}
	for _, s := range m.todoSections {
		for _, todo := range s.todos {
			addUser(todo.Author, "")
		}
	}

	return items
}

func (m *Model) collectProjects() []pickerItem {
	seen := make(map[string]bool)
	var items []pickerItem

	addProject := func(project string) {
		if project == "" || seen[project] {
			return
		}
		seen[project] = true
		items = append(items, pickerItem{value: project, display: project})
	}

	for _, s := range m.mrSections {
		for _, mr := range s.mrs {
			addProject(mr.Project)
		}
	}
	for _, s := range m.issSections {
		for _, issue := range s.issues {
			addProject(issue.Project)
		}
	}
	for _, s := range m.todoSections {
		for _, todo := range s.todos {
			addProject(todo.Project)
		}
	}

	return items
}

func (m *Model) collectLabels() []pickerItem {
	seen := make(map[string]bool)
	var items []pickerItem

	for _, s := range m.mrSections {
		for _, mr := range s.mrs {
			for _, l := range mr.Labels {
				if !seen[l] {
					seen[l] = true
					items = append(items, pickerItem{value: l, display: l})
				}
			}
		}
	}
	for _, s := range m.issSections {
		for _, issue := range s.issues {
			for _, l := range issue.Labels {
				if !seen[l] {
					seen[l] = true
					items = append(items, pickerItem{value: l, display: l})
				}
			}
		}
	}

	return items
}

func (m *Model) collectBranches(source bool) []pickerItem {
	seen := make(map[string]bool)
	var items []pickerItem

	for _, s := range m.mrSections {
		for _, mr := range s.mrs {
			var branch string
			if source {
				branch = mr.SourceBranch
			} else {
				branch = mr.TargetBranch
			}
			if branch != "" && !seen[branch] {
				seen[branch] = true
				items = append(items, pickerItem{value: branch, display: branch})
			}
		}
	}

	return items
}

func (m *Model) executeSearch(query string) tea.Cmd {
	parsed := parseSearch(query)
	s := m.activeSection()
	s.searchQuery = query
	s.searchParsed = parsed

	if !parsed.hasOverrides {
		// Plain text only — do client-side filtering
		s.applyFilter()
		return nil
	}

	// API-level search — clear stale client-side filter results
	s.filteredMRs = nil
	s.filteredIssues = nil
	s.filteredTodos = nil

	// If there's a project prefix, resolve it first
	if parsed.project != "" {
		client := m.client
		project := parsed.project
		return func() tea.Msg {
			id, err := client.ResolveProjectID(project)
			return searchProjectResolvedMsg{id: id, err: err}
		}
	}

	// No project prefix — re-fetch with overrides from the API
	s.loading = true

	// Build config directly from parsed search, not from section state
	sc := parsed.toSectionConfig(s.config)
	pid := m.activeProjectID()
	client := m.client
	idx := m.activeTab

	switch m.view {
	case viewMRs:
		return func() tea.Msg {
			mrs, pi, err := client.FetchMergeRequests(sc, pid, 1)
			if err != nil {
				return sectionErrMsg{view: viewMRs, index: idx, err: err}
			}
			return sectionMRsMsg{index: idx, mrs: mrs, page: 1, pageInfo: pi}
		}
	case viewIssues:
		return func() tea.Msg {
			issues, pi, err := client.FetchIssues(sc, pid, 1)
			if err != nil {
				return sectionErrMsg{view: viewIssues, index: idx, err: err}
			}
			return sectionIssuesMsg{index: idx, issues: issues, page: 1, pageInfo: pi}
		}
	case viewTodos:
		s.applyFilter()
	}
	return nil
}

func (m *Model) fetchSidebarData() tea.Cmd {
	if m.view != viewMRs {
		return nil
	}

	s := m.activeSections()[m.activeTab]
	mrs := s.visibleMRs()
	if len(mrs) == 0 || s.cursor >= len(mrs) {
		return nil
	}

	mr := mrs[s.cursor]
	tab := m.sidebar.activeTab

	// Overview doesn't need extra data
	if tab == tabOverview {
		return nil
	}

	// Check cache
	key := m.sidebar.cacheKey(mr)
	if m.sidebar.cachedKey != key {
		m.sidebar.clearCache()
		m.sidebar.cachedKey = key
	}
	if m.sidebar.isCached(mr, tab) {
		return nil
	}

	m.sidebar.loadErr = nil
	client := m.client
	pid := mr.ProjectID
	iid := mr.IID

	switch tab {
	case tabDiscussion:
		return func() tea.Msg {
			notes, err := client.FetchMRNotes(pid, iid)
			return sidebarDataMsg{tab: tabDiscussion, notes: notes, err: err}
		}
	case tabCommits:
		return func() tea.Msg {
			commits, err := client.FetchMRCommits(pid, iid)
			return sidebarDataMsg{tab: tabCommits, commits: commits, err: err}
		}
	case tabPipeline:
		return func() tea.Msg {
			pipelines, err := client.FetchMRPipelines(pid, iid)
			return sidebarDataMsg{tab: tabPipeline, pipelines: pipelines, err: err}
		}
	case tabChanges:
		return func() tea.Msg {
			changes, err := client.FetchMRChanges(pid, iid)
			return sidebarDataMsg{tab: tabChanges, changes: changes, err: err}
		}
	}
	return nil
}

func (m *Model) loadMoreCurrentSection() tea.Cmd {
	s := m.activeSection()
	if !s.hasMore {
		return nil
	}
	s.loading = true
	nextPage := s.page + 1

	switch m.view {
	case viewMRs:
		return m.fetchMRSectionPage(m.activeTab, nextPage)
	case viewIssues:
		return m.fetchIssueSectionPage(m.activeTab, nextPage)
	}
	return nil
}

func (m *Model) refreshCurrentSection() tea.Cmd {
	s := m.activeSection()
	s.loading = true
	s.err = nil

	switch m.view {
	case viewMRs:
		return m.fetchMRSection(m.activeTab)
	case viewIssues:
		return m.fetchIssueSection(m.activeTab)
	case viewTodos:
		return m.fetchTodoSection(m.activeTab)
	}
	return nil
}

func (m *Model) refreshAll() tea.Cmd {
	switch m.view {
	case viewMRs:
		cmds := make([]tea.Cmd, len(m.mrSections))
		for i := range m.mrSections {
			m.mrSections[i].loading = true
			m.mrSections[i].err = nil
			cmds[i] = m.fetchMRSection(i)
		}
		return tea.Batch(cmds...)
	case viewIssues:
		cmds := make([]tea.Cmd, len(m.issSections))
		for i := range m.issSections {
			m.issSections[i].loading = true
			m.issSections[i].err = nil
			cmds[i] = m.fetchIssueSection(i)
		}
		return tea.Batch(cmds...)
	case viewTodos:
		cmds := make([]tea.Cmd, len(m.todoSections))
		for i := range m.todoSections {
			m.todoSections[i].loading = true
			m.todoSections[i].err = nil
			cmds[i] = m.fetchTodoSection(i)
		}
		return tea.Batch(cmds...)
	}
	return nil
}

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
