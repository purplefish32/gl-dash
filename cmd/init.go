package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
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

		reader := bufio.NewReader(os.Stdin)

		// Prompt for GitLab URL
		fmt.Print("GitLab URL (press Enter for https://gitlab.com): ")
		urlInput, _ := reader.ReadString('\n')
		urlInput = strings.TrimSpace(urlInput)
		if urlInput == "" {
			urlInput = "https://gitlab.com"
		}

		// Prompt for token (hidden input)
		fmt.Print("GitLab personal access token: ")
		tokenBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			return fmt.Errorf("reading token: %w", err)
		}
		token := strings.TrimSpace(string(tokenBytes))

		// Build config with provided values
		config := defaultConfig
		if urlInput != "https://gitlab.com" {
			config = strings.Replace(config, "# baseUrl: https://gitlab.com", "baseUrl: "+urlInput, 1)
		}
		if token != "" {
			config = strings.Replace(config, "# token: your-token-here", "token: "+token, 1)
		}

		if err := os.WriteFile(configPath, []byte(config), 0600); err != nil {
			return fmt.Errorf("writing config: %w", err)
		}

		fmt.Printf("Config created at %s\n", configPath)
		if token == "" {
			fmt.Println("\nNo token provided. Set it later:")
			fmt.Println("  export GITLAB_TOKEN=your-token")
			fmt.Println("  or edit: $EDITOR " + configPath)
		}
		fmt.Println("\nRun gl-dash to start!")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
}
