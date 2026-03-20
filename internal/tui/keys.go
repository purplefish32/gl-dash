package tui

import (
	tea "charm.land/bubbletea/v2"
)

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
