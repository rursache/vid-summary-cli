// Package proc centralizes how the tool shells out to external binaries
// (yt-dlp, ffmpeg, whisper-cli, claude, codex): consistent stdout capture,
// stderr handling, and error wrapping with a stderr tail.
package proc

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type RunOpts struct {
	Name         string    // executable name or path
	Args         []string  //
	Stdin        io.Reader // nil -> child stdin is /dev/null
	StreamStderr bool      // also mirror child stderr to our stderr (verbose mode)
	Label        string    // prefix for wrapped errors; defaults to Name
}

type Result struct {
	Stdout string
	Stderr string
}

// Run executes the command, capturing stdout. On failure the returned error
// includes the command label and the tail of stderr.
func Run(ctx context.Context, o RunOpts) (Result, error) {
	cmd := exec.CommandContext(ctx, o.Name, o.Args...)
	cmd.Stdin = o.Stdin

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	if o.StreamStderr {
		cmd.Stderr = io.MultiWriter(&stderr, os.Stderr)
	} else {
		cmd.Stderr = &stderr
	}

	err := cmd.Run()
	res := Result{Stdout: stdout.String(), Stderr: stderr.String()}
	if err != nil {
		label := o.Label
		if label == "" {
			label = o.Name
		}
		return res, fmt.Errorf("%s: %w\n%s", label, err, Tail(res.Stderr, 10))
	}
	return res, nil
}

// Tail returns the last n non-empty-trimmed lines of s.
func Tail(s string, n int) string {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
