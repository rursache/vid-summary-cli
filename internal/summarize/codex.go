package summarize

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/rursache/vid-summary-cli/internal/proc"
)

// codexBackend shells out to the Codex CLI's non-interactive exec mode in
// --yolo mode (bypasses sandbox + approvals, the documented alias for
// --dangerously-bypass-approvals-and-sandbox) so it never stalls on a prompt.
// --skip-git-repo-check lets it run in any directory. stdin is left nil
// (/dev/null) to dodge the non-TTY hang bug; the final message lands on stdout.
type codexBackend struct {
	model string
}

func (c *codexBackend) Name() string { return "codex" }

// codexBin resolves the executable, tolerating the winget Windows packaging bug
// (#11283) that installs codex-x86_64-pc-windows-msvc.exe instead of codex.exe.
func codexBin() (string, bool) {
	for _, n := range []string{"codex", "codex-x86_64-pc-windows-msvc"} {
		if p, err := exec.LookPath(n); err == nil {
			return p, true
		}
	}
	return "", false
}

func (c *codexBackend) Available() error {
	if _, ok := codexBin(); !ok {
		return fmt.Errorf("codex CLI not found in PATH.\n  Install Codex CLI, then run: codex login")
	}
	return nil
}

func (c *codexBackend) Generate(ctx context.Context, prompt string) (string, error) {
	bin, ok := codexBin()
	if !ok {
		bin = "codex"
	}
	args := []string{"exec", "--yolo", "--skip-git-repo-check", "--ephemeral"}
	if c.model != "" {
		args = append(args, "--model", c.model)
	}
	args = append(args, prompt)

	res, err := proc.Run(ctx, proc.RunOpts{Name: bin, Args: args, Label: "codex exec"})
	if err != nil {
		return "", err
	}
	out := strings.TrimSpace(res.Stdout)
	if out == "" {
		return "", fmt.Errorf("codex returned empty output\n%s", proc.Tail(res.Stderr, 10))
	}
	return out, nil
}
