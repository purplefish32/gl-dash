package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

const defaultConfig = `# gl-dash configuration

gitlab:
  # GitLab instance URL (can also use GITLAB_URL env var)
  # baseUrl: https://gitlab.com

  # Personal access token (can also use GITLAB_TOKEN env var)
  # token: your-token-here

sections:
  mergeRequests:
    - title: Merge Requests
      filter: all
      limit: 50

  issues:
    - title: Issues
      filter: all
      limit: 50

# Auto-refresh interval in minutes (0 to disable)
refreshMinutes: 5

# Review command (press v on an MR). Template variables:
#   {{.IID}}, {{.MrNumber}}, {{.SourceBranch}}, {{.TargetBranch}},
#   {{.ProjectPath}}, {{.Author}}, {{.Title}}, {{.WebURL}}
# reviewCommand: "tmux new-window -n 'MR-{{.MrNumber}}' 'wt switch mr:{{.MrNumber}} && claude /review'"

# Map project paths to local directories (for review in multi-repo setups)
# repoPaths:
#   "group/project": "~/Code/project"

# Use / in the app to filter with prefixes:
#   @username      - filter by author
#   @r:username    - filter by reviewer
#   @a:username    - filter by assignee
#   #group/project - filter by project
#   ~label         - filter by label
#   !branch        - filter by source branch
#   !!branch       - filter by target branch
#   text           - search in title/description
# Press t to toggle filtering to current git project
`

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a default configuration file",
	RunE: func(cmd *cobra.Command, args []string) error {
		home, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("getting home directory: %w", err)
		}

		configDir := filepath.Join(home, ".config", "gl-dash")
		configPath := filepath.Join(configDir, "config.yml")

		if _, err := os.Stat(configPath); err == nil {
			return fmt.Errorf("config already exists at %s — edit it directly or delete to reinitialize", configPath)
		}

		if err := os.MkdirAll(configDir, 0755); err != nil {
			return fmt.Errorf("creating config directory: %w", err)
		}

		if err := os.WriteFile(configPath, []byte(defaultConfig), 0644); err != nil {
			return fmt.Errorf("writing config: %w", err)
		}

		fmt.Printf("Config created at %s\n", configPath)
		fmt.Println("\nNext steps:")
		fmt.Println("  1. Set your GitLab token:  export GITLAB_TOKEN=your-token")
		fmt.Println("  2. Set your GitLab URL:    export GITLAB_URL=https://gitlab.example.com")
		fmt.Println("  3. Edit the config:        $EDITOR " + configPath)
		fmt.Println("  4. Run:                    gl-dash")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
