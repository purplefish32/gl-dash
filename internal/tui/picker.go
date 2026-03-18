package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

type pickerItem struct {
	value   string // what gets inserted into search (e.g. "jdoe")
	display string // what's shown in the picker (e.g. "jdoe (Jane Doe)")
}

type picker struct {
	prefix   string       // the trigger prefix ("@", "#", "~")
	query    string       // typed characters after the prefix
	items    []pickerItem // all candidates
	filtered []pickerItem // filtered by query
	cursor   int
	loading  bool
}

func newPicker(prefix string) picker {
	return picker{
		prefix:  prefix,
		loading: true,
	}
}

func (p *picker) setItems(items []pickerItem) {
	p.items = items
	p.loading = false
	p.applyFilter()
}

func (p *picker) mergeItems(items []pickerItem) {
	seen := make(map[string]bool)
	for _, item := range p.items {
		seen[item.value] = true
	}
	for _, item := range items {
		if !seen[item.value] {
			p.items = append(p.items, item)
			seen[item.value] = true
		}
	}
}

func (p *picker) applyFilter() {
	if p.query == "" {
		p.filtered = p.items
		p.cursor = 0
		return
	}

	q := strings.ToLower(p.query)
	p.filtered = nil
	for _, item := range p.items {
		if strings.Contains(strings.ToLower(item.display), q) ||
			strings.Contains(strings.ToLower(item.value), q) {
			p.filtered = append(p.filtered, item)
		}
	}
	p.cursor = 0
}

func (p *picker) selected() string {
	if len(p.filtered) == 0 || p.cursor >= len(p.filtered) {
		return p.query // fallback to raw input
	}
	return p.filtered[p.cursor].value
}

var (
	pickerBorderStyle = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#FF6B00")).
		Padding(0, 1)

	pickerSelectedStyle = lipgloss.NewStyle().
		Background(lipgloss.Color("#FF6B00")).
		Foreground(lipgloss.Color("#000000"))

	pickerItemStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#CCCCCC"))

	pickerQueryStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FF6B00")).
		Bold(true)

	pickerHintStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666")).
		Italic(true)
)

func (p picker) render(width, maxHeight int) string {
	pickerW := width / 2
	if pickerW < 30 {
		pickerW = 30
	}
	if pickerW > 60 {
		pickerW = 60
	}

	var lines []string

	// Header with query
	header := pickerQueryStyle.Render(p.prefix) + pickerItemStyle.Render(p.query) + "▏"
	lines = append(lines, header)

	if p.loading {
		lines = append(lines, pickerHintStyle.Render("  Loading..."))
	} else if p.query == "" {
		lines = append(lines, pickerHintStyle.Render("  Type to search..."))
	} else if len(p.filtered) == 0 {
		lines = append(lines, pickerHintStyle.Render("  No matches"))
	} else {
		// Show items, scrolled to keep cursor visible
		visibleCount := maxHeight - 4
		if visibleCount < 3 {
			visibleCount = 3
		}
		if visibleCount > len(p.filtered) {
			visibleCount = len(p.filtered)
		}

		start := 0
		if p.cursor >= visibleCount {
			start = p.cursor - visibleCount + 1
		}
		end := start + visibleCount
		if end > len(p.filtered) {
			end = len(p.filtered)
			start = end - visibleCount
			if start < 0 {
				start = 0
			}
		}

		for i := start; i < end; i++ {
			item := p.filtered[i]
			display := item.display
			if len(display) > pickerW-4 {
				display = display[:pickerW-5] + "…"
			}

			if i == p.cursor {
				lines = append(lines, pickerSelectedStyle.Render("  "+display))
			} else {
				lines = append(lines, pickerItemStyle.Render("  "+display))
			}
		}

		if len(p.filtered) > visibleCount {
			lines = append(lines, pickerHintStyle.Render(
				strings.Repeat(" ", 2)+"↑↓ or ctrl+j/k to navigate"))
		}
	}

	lines = append(lines, pickerHintStyle.Render("  enter: select  esc: cancel"))

	content := strings.Join(lines, "\n")
	return pickerBorderStyle.Width(pickerW).Render(content)
}
