package data

import (
	"fmt"

	"github.com/purplefish32/gl-dash/internal/config"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

type Issue struct {
	IID            int
	Title          string
	Description    string
	Author         string
	AuthorName     string
	Project        string
	Assignees      []UserInfo
	Labels         []string
	State          string
	WebURL         string
	UpdatedAt      string
	CreatedAt      string
	UserNotesCount int
	Upvotes        int
	Downvotes      int
	Confidential   bool
}

func (c *Client) FetchIssues(sc config.SectionConfig, projectID int, page int) ([]Issue, PageInfo, error) {
	scope := resolveScope(sc)
	state := resolveState(sc)
	limit := sc.Limit
	if limit <= 0 {
		limit = 20
	}
	if page <= 0 {
		page = 1
	}

	var glIssues []*gitlab.Issue
	var resp *gitlab.Response
	var err error

	if projectID > 0 {
		opts := &gitlab.ListProjectIssuesOptions{
			Scope: &scope,
			State: &state,
			ListOptions: gitlab.ListOptions{
				PerPage: int64(limit),
				Page:    int64(page),
			},
		}
		applyIssueFiltersProject(opts, sc)
		glIssues, resp, err = c.gl.Issues.ListProjectIssues(int64(projectID), opts)
	} else {
		opts := &gitlab.ListIssuesOptions{
			Scope: &scope,
			State: &state,
			ListOptions: gitlab.ListOptions{
				PerPage: int64(limit),
				Page:    int64(page),
			},
		}
		applyIssueFilters(opts, sc)
		glIssues, resp, err = c.gl.Issues.ListIssues(opts)
	}

	if err != nil {
		return nil, PageInfo{}, fmt.Errorf("fetching issues: %w", err)
	}

	result := make([]Issue, 0, len(glIssues))
	for _, issue := range glIssues {
		i := Issue{
			IID:            int(issue.IID),
			Title:          issue.Title,
			Description:    issue.Description,
			Project:        extractProject(issue.References, issue.WebURL),
			State:          issue.State,
			WebURL:         issue.WebURL,
			UserNotesCount: int(issue.UserNotesCount),
			Upvotes:        int(issue.Upvotes),
			Downvotes:      int(issue.Downvotes),
			Confidential:   issue.Confidential,
			Labels:         []string(issue.Labels),
		}

		if issue.Author != nil {
			i.Author = issue.Author.Username
			i.AuthorName = issue.Author.Name
		}

		for _, a := range issue.Assignees {
			if a != nil {
				i.Assignees = append(i.Assignees, UserInfo{Username: a.Username, Name: a.Name})
			}
		}

		if issue.UpdatedAt != nil {
			i.UpdatedAt = issue.UpdatedAt.Format("2006-01-02 15:04")
		}
		if issue.CreatedAt != nil {
			i.CreatedAt = issue.CreatedAt.Format("2006-01-02 15:04")
		}

		result = append(result, i)
	}

	pi := PageInfo{}
	if resp != nil {
		pi.NextPage = int(resp.NextPage)
		pi.TotalItems = int(resp.TotalItems)
		pi.HasMore = resp.NextPage > 0
	}

	return result, pi, nil
}

func applyIssueFilters(opts *gitlab.ListIssuesOptions, sc config.SectionConfig) {
	if sc.AuthorUsername != "" {
		opts.AuthorUsername = &sc.AuthorUsername
	}
	if sc.AssigneeUsername != "" {
		opts.AssigneeUsername = &sc.AssigneeUsername
	}
	if len(sc.Labels) > 0 {
		labels := gitlab.LabelOptions(sc.Labels)
		opts.Labels = &labels
	}
	if sc.Milestone != "" {
		opts.Milestone = &sc.Milestone
	}
	if sc.Search != "" {
		opts.Search = &sc.Search
	}
}

func applyIssueFiltersProject(opts *gitlab.ListProjectIssuesOptions, sc config.SectionConfig) {
	if sc.AuthorUsername != "" {
		opts.AuthorUsername = &sc.AuthorUsername
	}
	if sc.AssigneeUsername != "" {
		opts.AssigneeUsername = &sc.AssigneeUsername
	}
	if len(sc.Labels) > 0 {
		labels := gitlab.LabelOptions(sc.Labels)
		opts.Labels = &labels
	}
	if sc.Milestone != "" {
		opts.Milestone = &sc.Milestone
	}
	if sc.Search != "" {
		opts.Search = &sc.Search
	}
}
