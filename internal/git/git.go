package git

import (
	"os/exec"
	"regexp"
	"strings"
)

// DetectProjectPath tries to determine the GitLab project path (e.g. "group/project")
// from the current directory's git remotes. Checks origin first, then any other remote.
// Returns empty string if not in a git repo or no GitLab remote found.
func DetectProjectPath(gitlabHost string) string {
	// Try origin first, then fall back to any remote
	for _, remote := range []string{"origin", ""} {
		path := projectPathFromRemote(remote, gitlabHost)
		if path != "" {
			return path
		}
	}
	return ""
}

func projectPathFromRemote(remoteName, gitlabHost string) string {
	args := []string{"remote", "get-url"}
	if remoteName != "" {
		args = append(args, remoteName)
	} else {
		// Get first remote
		out, err := exec.Command("git", "remote").Output()
		if err != nil {
			return ""
		}
		remotes := strings.Fields(string(out))
		if len(remotes) == 0 {
			return ""
		}
		// Skip origin since we already tried it
		for _, r := range remotes {
			if r != "origin" {
				args = append(args, r)
				break
			}
		}
		if len(args) == 2 {
			return ""
		}
	}

	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return ""
	}

	url := strings.TrimSpace(string(out))
	return parseProjectPath(url, gitlabHost)
}

func parseProjectPath(url, gitlabHost string) string {
	// SSH: git@gitlab.com:group/subgroup/project.git
	sshPattern := regexp.MustCompile(`@` + regexp.QuoteMeta(gitlabHost) + `:(.+?)(?:\.git)?$`)
	if m := sshPattern.FindStringSubmatch(url); len(m) > 1 {
		return m[1]
	}

	// HTTPS: https://gitlab.com/group/subgroup/project.git
	httpsPattern := regexp.MustCompile(`https?://` + regexp.QuoteMeta(gitlabHost) + `/(.+?)(?:\.git)?$`)
	if m := httpsPattern.FindStringSubmatch(url); len(m) > 1 {
		return m[1]
	}

	return ""
}
