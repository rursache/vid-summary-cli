package summarize

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/rursache/vid-summary-cli/internal/proc"
)

// grokBackend shells out to the Grok Build CLI in single-turn (-p) mode,
// using the user's existing login (no API key). Tools and subagents are
// disabled so it behaves as a pure text generator.
type grokBackend struct {
	model string
}

func (g *grokBackend) Name() string { return "grok" }

func (g *grokBackend) Available() error {
	if _, err := exec.LookPath("grok"); err != nil {
		return fmt.Errorf("grok CLI not found in PATH.\n  Install Grok Build, then run: grok login")
	}
	return nil
}

func (g *grokBackend) Generate(ctx context.Context, prompt string) (string, error) {
	// -p / --single: headless one-shot to stdout
	// --tools "": no tool use (pure summary)
	// --no-subagents / --no-plan / --disable-web-search: keep it non-interactive
	// --max-turns 1: one model response then exit
	args := []string{
		"-p", prompt,
		"--output-format", "plain",
		"--tools", "",
		"--no-subagents",
		"--no-plan",
		"--disable-web-search",
		"--max-turns", "1",
	}
	if g.model != "" {
		args = append(args, "-m", g.model)
	}

	res, err := proc.Run(ctx, proc.RunOpts{Name: "grok", Args: args, Label: "grok -p"})
	if err != nil {
		return "", err
	}
	out := strings.TrimSpace(res.Stdout)
	if out == "" {
		return "", fmt.Errorf("grok returned empty output\n%s", proc.Tail(res.Stderr, 10))
	}
	return out, nil
}
