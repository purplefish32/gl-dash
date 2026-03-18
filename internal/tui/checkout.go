package tui

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"text/template"

	tea "charm.land/bubbletea/v2"

	"github.com/purplefish32/gl-dash/internal/data"
)

type commandDoneMsg struct {
	action string
	output string
	err    error
}

type commandVars struct {
	IID          int
	MrNumber     int    // alias for IID
	SourceBranch string
	TargetBranch string
	ProjectPath  string
	Author       string
	Title        string
	WebURL       string
}

func varsFromMR(mr data.MergeRequest) commandVars {
	return commandVars{
		IID:          mr.IID,
		MrNumber:     mr.IID,
		SourceBranch: mr.SourceBranch,
		TargetBranch: mr.TargetBranch,
		ProjectPath:  mr.Project,
		Author:       mr.Author,
		Title:        mr.Title,
		WebURL:       mr.WebURL,
	}
}

func (m *Model) executeCommand(action, cmdTemplate string, mr data.MergeRequest) tea.Cmd {
	vars := varsFromMR(mr)

	tmpl, err := template.New(action).Parse(cmdTemplate)
	if err != nil {
		return func() tea.Msg {
			return commandDoneMsg{action: action, err: fmt.Errorf("invalid %s command template: %w", action, err)}
		}
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, vars); err != nil {
		return func() tea.Msg {
			return commandDoneMsg{action: action, err: fmt.Errorf("executing %s template: %w", action, err)}
		}
	}

	cmdStr := buf.String()
	workDir := m.resolveRepoPath(mr.Project)

	return func() tea.Msg {
		cmd := exec.Command("sh", "-c", cmdStr)
		cmd.Dir = workDir

		output, err := cmd.CombinedOutput()
		if err != nil {
			return commandDoneMsg{
				action: action,
				output: string(output),
				err:    fmt.Errorf("%s: %w", string(output), err),
			}
		}
		return commandDoneMsg{action: action, output: string(output)}
	}
}

func (m *Model) executeLocal() tea.Cmd {
	cmdTemplate := m.cfg.LocalCommand
	if cmdTemplate == "" {
		return func() tea.Msg {
			return commandDoneMsg{
				action: "local",
				err:    fmt.Errorf("no localCommand configured in ~/.config/gl-dash/config.yml"),
			}
		}
	}

	workDir := m.resolveRepoPath(m.projectPath)
	return func() tea.Msg {
		cmd := exec.Command("sh", "-c", cmdTemplate)
		cmd.Dir = workDir

		output, err := cmd.CombinedOutput()
		if err != nil {
			return commandDoneMsg{
				action: "local",
				output: string(output),
				err:    fmt.Errorf("%s: %w", string(output), err),
			}
		}
		return commandDoneMsg{action: "local", output: string(output)}
	}
}

func (m *Model) executeReview(mr data.MergeRequest) tea.Cmd {
	cmdTemplate := m.cfg.ReviewCommand
	if cmdTemplate == "" {
		return func() tea.Msg {
			return commandDoneMsg{
				action: "review",
				err:    fmt.Errorf("no reviewCommand configured in ~/.config/gl-dash/config.yml"),
			}
		}
	}
	return m.executeCommand("review", cmdTemplate, mr)
}

func (m *Model) resolveRepoPath(projectPath string) string {
	if m.cfg.RepoPaths != nil {
		if path, ok := m.cfg.RepoPaths[projectPath]; ok {
			if len(path) > 0 && path[0] == '~' {
				if home, err := os.UserHomeDir(); err == nil {
					path = home + path[1:]
				}
			}
			return path
		}
	}

	dir, _ := os.Getwd()
	return dir
}
