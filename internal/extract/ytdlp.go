package extract

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rursache/vid-summary-cli/internal/proc"
)

// Acquire downloads the best audio stream for a URL into tmpDir and returns the
// produced file path. It asks yt-dlp to print the final path deterministically
// and falls back to globbing if that path is unusable.
func Acquire(ctx context.Context, ytdlpBin, rawURL, tmpDir string, verbose bool) (string, error) {
	tmpl := filepath.Join(tmpDir, "source.%(ext)s")
	args := []string{
		"-f", "bestaudio/best", // fall back to combined stream if no audio-only format
		"--no-playlist",
		"--no-write-info-json", "--no-write-thumbnail", // keep the temp dir clean
		"-N", "4", // parallel fragments for HLS/DASH
		"-R", "10", "--fragment-retries", "10",
		"--throttled-rate", "100K", // re-extract if a CDN throttles us
		"--quiet", "--no-warnings",
		"--restrict-filenames",
		"-o", tmpl,
		"--print", "after_move:%(filepath,_filename)s",
	}
	if verbose {
		args = append(args, "--progress")
	} else {
		args = append(args, "--no-progress")
	}
	args = append(args, rawURL)

	res, err := proc.Run(ctx, proc.RunOpts{Name: ytdlpBin, Args: args, StreamStderr: verbose, Label: "yt-dlp"})
	if err != nil {
		return "", err
	}

	if p := lastLine(res.Stdout); p != "" {
		if _, statErr := os.Stat(p); statErr == nil {
			return p, nil
		}
	}
	// Fallback: the printed path was missing or stale (known after_move edge case).
	matches, _ := filepath.Glob(filepath.Join(tmpDir, "source.*"))
	for _, m := range matches {
		if !strings.HasSuffix(m, ".part") {
			return m, nil
		}
	}
	return "", fmt.Errorf("yt-dlp produced no output file in %s", tmpDir)
}

func lastLine(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	lines := strings.Split(s, "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}
