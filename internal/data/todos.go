package data

import (
	"fmt"

	gitlab "gitlab.com/gitlab-org/api/client-go"
)

type Todo struct {
	ID         int
	Action     string
	TargetType string
	TargetTitle string
	TargetURL  string
	Body       string
	State      string
	Author     string
	Project    string
	CreatedAt  string
}

func formatAction(action gitlab.TodoAction) string {
	switch action {
	case gitlab.TodoAssigned:
		return "assigned"
	case gitlab.TodoMentioned:
		return "mentioned"
	case gitlab.TodoBuildFailed:
		return "build failed"
	case gitlab.TodoMarked:
		return "marked"
	case gitlab.TodoApprovalRequired:
		return "approval required"
	case gitlab.TodoDirectlyAddressed:
		return "addressed"
	default:
		return string(action)
	}
}

func formatTargetType(tt gitlab.TodoTargetType) string {
	switch tt {
	case gitlab.TodoTargetMergeRequest:
		return "MR"
	case gitlab.TodoTargetIssue:
		return "Issue"
	default:
		return string(tt)
	}
}

func (c *Client) FetchTodos(state string, limit int) ([]Todo, error) {
	opts := &gitlab.ListTodosOptions{
		State: &state,
		ListOptions: gitlab.ListOptions{
			PerPage: int64(limit),
		},
	}

	todos, _, err := c.gl.Todos.ListTodos(opts)
	if err != nil {
		return nil, fmt.Errorf("fetching todos: %w", err)
	}

	result := make([]Todo, 0, len(todos))
	for _, todo := range todos {
		t := Todo{
			ID:         int(todo.ID),
			Action:     formatAction(todo.ActionName),
			TargetType: formatTargetType(todo.TargetType),
			TargetURL:  todo.TargetURL,
			Body:       todo.Body,
			State:      todo.State,
		}

		if todo.Author != nil {
			t.Author = todo.Author.Username
		}
		if todo.Project != nil {
			t.Project = todo.Project.PathWithNamespace
		}
		if todo.Target != nil {
			t.TargetTitle = todo.Target.Title
		}
		if todo.CreatedAt != nil {
			t.CreatedAt = todo.CreatedAt.Format("2006-01-02 15:04")
		}

		result = append(result, t)
	}

	return result, nil
}

func (c *Client) MarkTodoAsDone(id int) error {
	_, err := c.gl.Todos.MarkTodoAsDone(int64(id))
	return err
}

func (c *Client) MarkAllTodosAsDone() error {
	_, err := c.gl.Todos.MarkAllTodosAsDone()
	return err
}
