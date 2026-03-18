package data

import (
	"fmt"
	"strings"

	"github.com/purplefish32/gl-dash/internal/config"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

// extractProject gets a short project name from References or WebURL.
func extractProject(refs *gitlab.IssueReferences, webURL string) string {
	if refs != nil && refs.Full != "" {
		// Full is like "group/subgroup/project!123" or "group/project#45"
		full := refs.Full
		// Remove the MR/issue reference suffix (!123 or #45)
		for i := len(full) - 1; i >= 0; i-- {
			if full[i] == '!' || full[i] == '#' {
				return full[:i]
			}
		}
		return full
	}
	// Fallback: parse from WebURL like https://host/group/project/-/merge_requests/123
	if webURL != "" {
		parts := strings.Split(webURL, "/-/")
		if len(parts) > 0 {
			// parts[0] is like https://host/group/subgroup/project
			urlPath := parts[0]
			// Remove https://host/
			if idx := strings.Index(urlPath, "//"); idx >= 0 {
				urlPath = urlPath[idx+2:]
			}
			if idx := strings.Index(urlPath, "/"); idx >= 0 {
				return urlPath[idx+1:]
			}
		}
	}
	return ""
}

type Client struct {
	gl *gitlab.Client
}

func NewClient(cfg *config.Config) (*Client, error) {
	opts := []gitlab.ClientOptionFunc{}
	if cfg.GitLab.BaseURL != "https://gitlab.com" {
		opts = append(opts, gitlab.WithBaseURL(cfg.GitLab.BaseURL))
	}

	gl, err := gitlab.NewClient(cfg.GitLab.Token, opts...)
	if err != nil {
		return nil, fmt.Errorf("creating GitLab client: %w", err)
	}

	return &Client{gl: gl}, nil
}

// ResolveProjectID looks up the numeric project ID from a path like "group/project".
func (c *Client) ResolveProjectID(projectPath string) (int, error) {
	project, _, err := c.gl.Projects.GetProject(projectPath, nil)
	if err != nil {
		return 0, fmt.Errorf("resolving project %q: %w", projectPath, err)
	}
	return int(project.ID), nil
}

type UserInfo struct {
	Username string
	Name     string
}

type MergeRequest struct {
	IID            int
	ProjectID      int
	Title          string
	Description    string
	Author         string
	AuthorName     string
	Project        string
	Assignees      []UserInfo
	Reviewers      []UserInfo
	Labels         []string
	TargetBranch   string
	SourceBranch   string
	State          string
	Draft          bool
	WebURL         string
	UpdatedAt      string
	CreatedAt      string
	Pipeline       string
	MergeStatus    string
	HasConflicts   bool
	ChangesCount   string
	UserNotesCount int
	Upvotes        int
	Downvotes      int
}

// resolveScope maps the shorthand "filter" field to a GitLab API scope,
// or uses the explicit "scope" field from config if set.
func resolveScope(sc config.SectionConfig) string {
	if sc.Scope != "" {
		return sc.Scope
	}
	switch sc.Filter {
	case "author":
		return "created_by_me"
	case "reviewer", "assignee":
		return "assigned_to_me"
	default:
		return "all"
	}
}

func resolveState(sc config.SectionConfig) string {
	if sc.State != "" {
		return sc.State
	}
	return "opened"
}

type PageInfo struct {
	NextPage   int
	TotalItems int
	HasMore    bool
}

func (c *Client) FetchMergeRequests(sc config.SectionConfig, projectID int, page int) ([]MergeRequest, PageInfo, error) {
	scope := resolveScope(sc)
	state := resolveState(sc)
	limit := sc.Limit
	if limit <= 0 {
		limit = 20
	}
	if page <= 0 {
		page = 1
	}

	var mrs []*gitlab.BasicMergeRequest
	var resp *gitlab.Response
	var err error

	if projectID > 0 {
		opts := &gitlab.ListProjectMergeRequestsOptions{
			Scope: &scope,
			State: &state,
			ListOptions: gitlab.ListOptions{
				PerPage: int64(limit),
				Page:    int64(page),
			},
		}
		applyMRFiltersProject(opts, sc)
		mrs, resp, err = c.gl.MergeRequests.ListProjectMergeRequests(int64(projectID), opts)
	} else {
		opts := &gitlab.ListMergeRequestsOptions{
			Scope: &scope,
			State: &state,
			ListOptions: gitlab.ListOptions{
				PerPage: int64(limit),
				Page:    int64(page),
			},
		}
		applyMRFilters(opts, sc)
		mrs, resp, err = c.gl.MergeRequests.ListMergeRequests(opts)
	}

	if err != nil {
		return nil, PageInfo{}, fmt.Errorf("fetching merge requests: %w", err)
	}

	pi := PageInfo{}
	if resp != nil {
		pi.NextPage = int(resp.NextPage)
		pi.TotalItems = int(resp.TotalItems)
		pi.HasMore = resp.NextPage > 0
	}

	return convertMRs(mrs), pi, nil
}

func applyMRFilters(opts *gitlab.ListMergeRequestsOptions, sc config.SectionConfig) {
	if sc.AuthorUsername != "" {
		opts.AuthorUsername = &sc.AuthorUsername
	}
	if sc.ReviewerUsername != "" {
		opts.ReviewerUsername = &sc.ReviewerUsername
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
	if sc.SourceBranch != "" {
		opts.SourceBranch = &sc.SourceBranch
	}
	if sc.TargetBranch != "" {
		opts.TargetBranch = &sc.TargetBranch
	}
	if sc.Draft != nil {
		opts.Draft = sc.Draft
	}
}

func applyMRFiltersProject(opts *gitlab.ListProjectMergeRequestsOptions, sc config.SectionConfig) {
	if sc.AuthorUsername != "" {
		opts.AuthorUsername = &sc.AuthorUsername
	}
	if sc.ReviewerUsername != "" {
		opts.ReviewerUsername = &sc.ReviewerUsername
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
	if sc.SourceBranch != "" {
		opts.SourceBranch = &sc.SourceBranch
	}
	if sc.TargetBranch != "" {
		opts.TargetBranch = &sc.TargetBranch
	}
	if sc.Draft != nil {
		opts.Draft = sc.Draft
	}
}

func convertMRs(mrs []*gitlab.BasicMergeRequest) []MergeRequest {
	result := make([]MergeRequest, 0, len(mrs))
	for _, mr := range mrs {
		m := MergeRequest{
			IID:            int(mr.IID),
			ProjectID:      int(mr.ProjectID),
			Title:          mr.Title,
			Description:    mr.Description,
			Project:        extractProject(mr.References, mr.WebURL),
			TargetBranch:   mr.TargetBranch,
			SourceBranch:   mr.SourceBranch,
			State:          mr.State,
			Draft:          mr.Draft,
			WebURL:         mr.WebURL,
			MergeStatus:    mr.DetailedMergeStatus,
			HasConflicts:   mr.HasConflicts,
			UserNotesCount: int(mr.UserNotesCount),
			Upvotes:        int(mr.Upvotes),
			Downvotes:      int(mr.Downvotes),
			Labels:         []string(mr.Labels),
		}

		if mr.Author != nil {
			m.Author = mr.Author.Username
			m.AuthorName = mr.Author.Name
		}

		for _, a := range mr.Assignees {
			if a != nil {
				m.Assignees = append(m.Assignees, UserInfo{Username: a.Username, Name: a.Name})
			}
		}

		for _, r := range mr.Reviewers {
			if r != nil {
				m.Reviewers = append(m.Reviewers, UserInfo{Username: r.Username, Name: r.Name})
			}
		}

		if mr.UpdatedAt != nil {
			m.UpdatedAt = mr.UpdatedAt.Format("2006-01-02 15:04")
		}
		if mr.CreatedAt != nil {
			m.CreatedAt = mr.CreatedAt.Format("2006-01-02 15:04")
		}

		result = append(result, m)
	}
	return result
}
