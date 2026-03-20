package tui

import (
	tea "charm.land/bubbletea/v2"
)

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
