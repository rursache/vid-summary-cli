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
	Name         string       // executable name or path
	Args         []string     //
	Stdin        io.Reader    // nil -> child stdin is /dev/null
	StreamStderr bool         // also mirror child stderr to our stderr (verbose mode)
	OnStderrLine func(string) // optional callback per complete stderr line
	Label        string       // prefix for wrapped errors; defaults to Name
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

	writers := []io.Writer{&stderr}
	if o.StreamStderr {
		writers = append(writers, os.Stderr)
	}
	if o.OnStderrLine != nil {
		writers = append(writers, &lineWriter{onLine: o.OnStderrLine})
	}
	cmd.Stderr = io.MultiWriter(writers...)

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

// lineWriter calls onLine for each complete newline-terminated line written,
// buffering any trailing partial line.
type lineWriter struct {
	buf    []byte
	onLine func(string)
}

func (w *lineWriter) Write(p []byte) (int, error) {
	w.buf = append(w.buf, p...)
	for {
		i := bytes.IndexByte(w.buf, '\n')
		if i < 0 {
			break
		}
		w.onLine(string(w.buf[:i]))
		w.buf = w.buf[i+1:]
	}
	return len(p), nil
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
