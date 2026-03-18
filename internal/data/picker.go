package data

import (
	gitlab "gitlab.com/gitlab-org/api/client-go"
)

type PickerCandidate struct {
	Value   string
	Display string
}

func (c *Client) FetchUsers(search string) ([]PickerCandidate, error) {
	opts := &gitlab.ListUsersOptions{
		ListOptions: gitlab.ListOptions{PerPage: 50},
		Active:      gitlab.Ptr(true),
	}
	if search != "" {
		opts.Search = &search
	}

	users, _, err := c.gl.Users.ListUsers(opts)
	if err != nil {
		return nil, err
	}

	result := make([]PickerCandidate, 0, len(users))
	for _, u := range users {
		display := u.Username
		if u.Name != "" {
			display += " (" + u.Name + ")"
		}
		result = append(result, PickerCandidate{
			Value:   u.Username,
			Display: display,
		})
	}
	return result, nil
}

func (c *Client) FetchProjects(search string) ([]PickerCandidate, error) {
	opts := &gitlab.ListProjectsOptions{
		ListOptions: gitlab.ListOptions{PerPage: 30},
		Membership:  gitlab.Ptr(true),
		OrderBy:     gitlab.Ptr("last_activity_at"),
	}
	if search != "" {
		opts.Search = &search
	}

	projects, _, err := c.gl.Projects.ListProjects(opts)
	if err != nil {
		return nil, err
	}

	result := make([]PickerCandidate, 0, len(projects))
	for _, p := range projects {
		result = append(result, PickerCandidate{
			Value:   p.PathWithNamespace,
			Display: p.PathWithNamespace,
		})
	}
	return result, nil
}

func (c *Client) FetchLabels(projectID int) ([]PickerCandidate, error) {
	if projectID <= 0 {
		return nil, nil
	}

	opts := &gitlab.ListLabelsOptions{
		ListOptions: gitlab.ListOptions{PerPage: 50},
	}

	labels, _, err := c.gl.Labels.ListLabels(int64(projectID), opts)
	if err != nil {
		return nil, err
	}

	result := make([]PickerCandidate, 0, len(labels))
	for _, l := range labels {
		result = append(result, PickerCandidate{
			Value:   l.Name,
			Display: l.Name,
		})
	}
	return result, nil
}
