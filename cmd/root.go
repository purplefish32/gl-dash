package cmd

import (
	"fmt"
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"

	"github.com/purplefish32/gl-dash/internal/config"
	"github.com/purplefish32/gl-dash/internal/data"
	"github.com/purplefish32/gl-dash/internal/tui"
)

var rootCmd = &cobra.Command{
	Use:   "gl-dash",
	Short: "A terminal UI dashboard for GitLab",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return err
		}

		client, err := data.NewClient(cfg)
		if err != nil {
			return err
		}

		m := tui.NewModel(cfg, client)
		p := tea.NewProgram(m)
		if _, err := p.Run(); err != nil {
			return fmt.Errorf("error running gl-dash: %w", err)
		}
		return nil
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
