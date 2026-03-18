package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	GitLab          GitLabConfig      `yaml:"gitlab"`
	Sections        SectionsConfig    `yaml:"sections"`
	RefreshMinutes  int               `yaml:"refreshMinutes"`
	SmartFilter     *bool             `yaml:"smartFilter"`
	ReviewCommand   string            `yaml:"reviewCommand"`
	LocalCommand    string            `yaml:"localCommand"`
	RepoPaths       map[string]string `yaml:"repoPaths"`
}

type GitLabConfig struct {
	Token   string `yaml:"token"`
	BaseURL string `yaml:"baseUrl"`
}

type SectionsConfig struct {
	MergeRequests []SectionConfig `yaml:"mergeRequests"`
	Issues        []SectionConfig `yaml:"issues"`
}

type SectionConfig struct {
	Title            string   `yaml:"title"`
	Filter           string   `yaml:"filter"`           // shorthand: "author", "reviewer", "assignee", "all"
	Limit            int      `yaml:"limit"`
	Scope            string   `yaml:"scope"`             // API scope: "created_by_me", "assigned_to_me", "all"
	State            string   `yaml:"state"`             // "opened", "closed", "merged", "all"
	AuthorUsername   string   `yaml:"authorUsername"`
	ReviewerUsername string   `yaml:"reviewerUsername"`
	AssigneeUsername string   `yaml:"assigneeUsername"`
	Labels           []string `yaml:"labels"`
	Milestone        string   `yaml:"milestone"`
	Search           string   `yaml:"search"`
	SourceBranch     string   `yaml:"sourceBranch"`
	TargetBranch     string   `yaml:"targetBranch"`
	Draft            *bool    `yaml:"draft"`
}

func defaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "gl-dash", "config.yml")
}

func Load() (*Config, error) {
	cfg := &Config{
		GitLab: GitLabConfig{
			BaseURL: "https://gitlab.com",
		},
		RefreshMinutes: 5,
		Sections: SectionsConfig{
			MergeRequests: []SectionConfig{
				{Title: "Merge Requests", Filter: "all", Limit: 50},
			},
			Issues: []SectionConfig{
				{Title: "Issues", Filter: "all", Limit: 50},
			},
		},
	}

	// Token from env takes priority
	if token := os.Getenv("GITLAB_TOKEN"); token != "" {
		cfg.GitLab.Token = token
	}

	if url := os.Getenv("GITLAB_URL"); url != "" {
		cfg.GitLab.BaseURL = url
	}

	// Try loading config file
	path := os.Getenv("GL_DASH_CONFIG")
	if path == "" {
		path = defaultConfigPath()
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if cfg.GitLab.Token == "" {
				return nil, fmt.Errorf("no GitLab token found. Set GITLAB_TOKEN env var or create %s", defaultConfigPath())
			}
			return cfg, nil
		}
		return nil, fmt.Errorf("reading config: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}

	// Env vars override config file
	if token := os.Getenv("GITLAB_TOKEN"); token != "" {
		cfg.GitLab.Token = token
	}
	if url := os.Getenv("GITLAB_URL"); url != "" {
		cfg.GitLab.BaseURL = url
	}

	if cfg.GitLab.Token == "" {
		return nil, fmt.Errorf("no GitLab token found. Set GITLAB_TOKEN env var or add gitlab.token to %s", path)
	}

	return cfg, nil
}
