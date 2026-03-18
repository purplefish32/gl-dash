package data

import (
	"fmt"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

type MRNote struct {
	Author    string
	Body      string
	CreatedAt string
	System    bool
}

type MRCommit struct {
	ID        string
	ShortID   string
	Title     string
	Author    string
	CreatedAt string
}

type MRPipeline struct {
	ID     int
	Status string
	Ref    string
	Jobs   []MRJob
}

type MRJob struct {
	Name   string
	Stage  string
	Status string
}

type MRChange struct {
	OldPath string
	NewPath string
	Diff    string
	NewFile bool
	Deleted bool
	Renamed bool
}

func (c *Client) FetchMRNotes(projectID, mrIID int) ([]MRNote, error) {
	sort := "asc"
	opts := &gitlab.ListMergeRequestNotesOptions{
		Sort:        &sort,
		ListOptions: gitlab.ListOptions{PerPage: 50},
	}

	notes, _, err := c.gl.Notes.ListMergeRequestNotes(int64(projectID), int64(mrIID), opts)
	if err != nil {
		return nil, fmt.Errorf("fetching MR notes: %w", err)
	}

	result := make([]MRNote, 0, len(notes))
	for _, n := range notes {
		note := MRNote{
			Body:   n.Body,
			System: n.System,
		}
		if n.Author.Username != "" {
			note.Author = n.Author.Username
		}
		if n.CreatedAt != nil {
			note.CreatedAt = n.CreatedAt.Format("2006-01-02 15:04")
		}
		result = append(result, note)
	}
	return result, nil
}

func (c *Client) FetchMRCommits(projectID, mrIID int) ([]MRCommit, error) {
	opts := &gitlab.GetMergeRequestCommitsOptions{
		ListOptions: gitlab.ListOptions{PerPage: 50},
	}

	commits, _, err := c.gl.MergeRequests.GetMergeRequestCommits(int64(projectID), int64(mrIID), opts)
	if err != nil {
		return nil, fmt.Errorf("fetching MR commits: %w", err)
	}

	result := make([]MRCommit, 0, len(commits))
	for _, cm := range commits {
		c := MRCommit{
			ID:      cm.ID,
			ShortID: cm.ShortID,
			Title:   cm.Title,
		}
		if cm.AuthorName != "" {
			c.Author = cm.AuthorName
		}
		if cm.CreatedAt != nil {
			c.CreatedAt = cm.CreatedAt.Format("2006-01-02 15:04")
		}
		result = append(result, c)
	}
	return result, nil
}

func (c *Client) FetchMRPipelines(projectID, mrIID int) ([]MRPipeline, error) {
	pipelines, _, err := c.gl.MergeRequests.ListMergeRequestPipelines(int64(projectID), int64(mrIID))
	if err != nil {
		return nil, fmt.Errorf("fetching MR pipelines: %w", err)
	}

	result := make([]MRPipeline, 0, len(pipelines))
	for _, p := range pipelines {
		result = append(result, MRPipeline{
			ID:     int(p.ID),
			Status: p.Status,
			Ref:    p.Ref,
		})
	}
	return result, nil
}

func (c *Client) FetchMRChanges(projectID, mrIID int) ([]MRChange, error) {
	opts := &gitlab.ListMergeRequestDiffsOptions{
		ListOptions: gitlab.ListOptions{PerPage: 100},
	}

	diffs, _, err := c.gl.MergeRequests.ListMergeRequestDiffs(int64(projectID), int64(mrIID), opts)
	if err != nil {
		return nil, fmt.Errorf("fetching MR changes: %w", err)
	}

	result := make([]MRChange, 0, len(diffs))
	for _, d := range diffs {
		result = append(result, MRChange{
			OldPath: d.OldPath,
			NewPath: d.NewPath,
			Diff:    d.Diff,
			NewFile: d.NewFile,
			Deleted: d.DeletedFile,
			Renamed: d.RenamedFile,
		})
	}
	return result, nil
}
