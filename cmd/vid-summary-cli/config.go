package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/rursache/vid-summary-cli/internal/config"
	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Open the config file in your editor",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return editConfig()
		},
	}

	cmd.AddCommand(&cobra.Command{
		Use:   "path",
		Short: "Print the config file path",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			path, err := ensureConfig()
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), path)
			return nil
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "edit",
		Short: "Open the config file in your editor",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return editConfig()
		},
	})

	return cmd
}

// ensureConfig creates the default config on first use and returns its path.
func ensureConfig() (string, error) {
	if _, err := config.Load(); err != nil {
		return "", err
	}
	return config.Path()
}

func editConfig() error {
	path, err := ensureConfig()
	if err != nil {
		return err
	}
	name, args := editorCommand(path)
	ed := exec.Command(name, args...)
	ed.Stdin, ed.Stdout, ed.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := ed.Run(); err != nil {
		return fmt.Errorf("open editor (%s): %w", name, err)
	}
	return nil
}

// editorCommand resolves the editor to launch, honoring $VISUAL/$EDITOR (which
// may include flags, e.g. "code --wait") and falling back to the OS default.
func editorCommand(path string) (string, []string) {
	for _, env := range []string{"VISUAL", "EDITOR"} {
		if v := strings.TrimSpace(os.Getenv(env)); v != "" {
			parts := strings.Fields(v)
			return parts[0], append(parts[1:], path)
		}
	}
	switch runtime.GOOS {
	case "darwin":
		return "open", []string{path}
	case "windows":
		return "cmd", []string{"/c", "start", "", path}
	default:
		return "xdg-open", []string{path}
	}
}
