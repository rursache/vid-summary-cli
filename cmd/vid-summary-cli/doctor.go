package main

import (
	"fmt"

	"github.com/rursache/vid-summary-cli/internal/config"
	"github.com/rursache/vid-summary-cli/internal/deps"
	"github.com/rursache/vid-summary-cli/internal/model"
	"github.com/rursache/vid-summary-cli/internal/summarize"
	"github.com/spf13/cobra"
)

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check external dependencies and config",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			out := cmd.OutOrStdout()

			fmt.Fprintln(out, "Dependencies:")
			for _, t := range deps.All() {
				if t.Found {
					ver := t.Version
					if ver == "" {
						ver = "found"
					}
					fmt.Fprintf(out, "  [ok]   %-12s %s (%s)\n", t.Name, ver, t.Path)
				} else {
					fmt.Fprintf(out, "  [miss] %-12s install: %s\n", t.Name, t.InstallHint())
				}
			}

			cfg, err := config.Load()
			if err != nil {
				return err
			}
			home, _ := config.Home()
			path, _ := config.Path()
			fmt.Fprintln(out, "\nConfig:")
			fmt.Fprintf(out, "  home:       %s\n", home)
			fmt.Fprintf(out, "  file:       %s\n", path)
			fmt.Fprintf(out, "  model:      %s\n", cfg.Model)
			fmt.Fprintf(out, "  language:   %s\n", cfg.Language)
			fmt.Fprintf(out, "  backend:    %s\n", cfg.Summarizer.Backend)
			llm := cfg.Summarizer.Model
			if llm == "" {
				llm = "(CLI default)"
			}
			fmt.Fprintf(out, "  llm-model:  %s\n", llm)

			fmt.Fprintln(out, "\nSummarizer backend:")
			if b, err := summarize.New(cfg.Summarizer.Backend, cfg.Summarizer.Model); err != nil {
				fmt.Fprintf(out, "  [miss] %v\n", err)
			} else if err := b.Available(); err != nil {
				fmt.Fprintf(out, "  [miss] %v\n", err)
			} else {
				fmt.Fprintf(out, "  [ok]   %s CLI available\n", b.Name())
			}

			fmt.Fprintln(out, "\nModel cache:")
			for _, name := range model.KnownNames() {
				if _, exists, _ := model.Local(name); exists {
					fmt.Fprintf(out, "  [cached] %s\n", name)
				}
			}
			if _, exists, _ := model.Local(cfg.Model); !exists {
				fmt.Fprintf(out, "  default model %q not yet downloaded (will fetch on first run)\n", cfg.Model)
			}
			return nil
		},
	}
}
