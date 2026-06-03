package summarize

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/rursache/vid-summary-cli/internal/proc"
)

// claudeBackend shells out to the Claude Code CLI in print mode, using the
// user's existing OAuth login (no API key). Tools are disabled so it behaves as
// a pure text generator.
type claudeBackend struct {
	model string
}

func (c *claudeBackend) Name() string { return "claude" }

func (c *claudeBackend) Available() error {
	if _, err := exec.LookPath("claude"); err != nil {
		return fmt.Errorf("claude CLI not found in PATH.\n  Install Claude Code, then run: claude login")
	}
	return nil
}

func (c *claudeBackend) Generate(ctx context.Context, prompt string) (string, error) {
	args := []string{"-p", prompt, "--output-format", "text", "--tools", ""}
	if c.model != "" {
		args = append(args, "--model", c.model)
	}

	res, err := proc.Run(ctx, proc.RunOpts{Name: "claude", Args: args, Label: "claude -p"})
	if err != nil {
		return "", err
	}
	out := strings.TrimSpace(res.Stdout)
	if out == "" {
		return "", fmt.Errorf("claude returned empty output\n%s", proc.Tail(res.Stderr, 10))
	}
	return out, nil
}
