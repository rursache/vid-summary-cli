package extract

import (
	"context"
	"path/filepath"

	"github.com/rursache/vid-summary-cli/internal/proc"
)

// Normalize resamples any audio/video input to 16 kHz mono signed-16 PCM WAV,
// the format whisper.cpp requires. It selects the first audio stream and drops
// video so cover art or multi-track inputs don't break the conversion.
func Normalize(ctx context.Context, ffmpegBin, input, tmpDir string, verbose bool) (string, error) {
	out := filepath.Join(tmpDir, "audio.wav")
	loglevel := "error"
	if verbose {
		loglevel = "info"
	}
	args := []string{
		"-nostdin",
		"-hide_banner",
		"-loglevel", loglevel,
		"-i", input,
		"-vn",
		"-map", "0:a:0",
		"-ar", "16000",
		"-ac", "1",
		"-c:a", "pcm_s16le",
		"-y", out,
	}
	if _, err := proc.Run(ctx, proc.RunOpts{Name: ffmpegBin, Args: args, StreamStderr: verbose, Label: "ffmpeg"}); err != nil {
		return "", err
	}
	return out, nil
}
