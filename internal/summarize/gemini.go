package summarize

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/rursache/vid-summary-cli/internal/proc"
)

// geminiBackend shells out to the Gemini CLI in non-interactive (-p) mode with
// --yolo so it never blocks on approvals. It uses the user's existing login
// (no API key). The answer is printed to stdout.
type geminiBackend struct {
	model string
}

func (g *geminiBackend) Name() string { return "gemini" }

func (g *geminiBackend) Available() error {
	if _, err := exec.LookPath("gemini"); err != nil {
		return fmt.Errorf("gemini CLI not found in PATH.\n  Install Gemini CLI, then sign in once by running: gemini")
	}
	return nil
}

func (g *geminiBackend) Generate(ctx context.Context, prompt string) (string, error) {
	args := []string{"-p", prompt, "--yolo", "--output-format", "text"}
	if g.model != "" {
		args = append(args, "-m", g.model)
	}

	res, err := proc.Run(ctx, proc.RunOpts{Name: "gemini", Args: args, Label: "gemini -p"})
	if err != nil {
		return "", err
	}
	out := strings.TrimSpace(res.Stdout)
	if out == "" {
		return "", fmt.Errorf("gemini returned empty output\n%s", proc.Tail(res.Stderr, 10))
	}
	return out, nil
}
