package transcribe

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rursache/vid-summary-cli/internal/proc"
)

type Segment struct {
	StartMS int64  `json:"start_ms"`
	EndMS   int64  `json:"end_ms"`
	Text    string `json:"text"`
}

type Result struct {
	Text     string    `json:"text"`
	Language string    `json:"language"`
	Segments []Segment `json:"segments"`
}

type Options struct {
	ModelPath string
	Language  string // "auto" or a language code
	Threads   int    // 0 = whisper default (4 on Apple Silicon)
	BeamSize  int    // 0 = whisper default (5); set 1 for greedy
	Verbose   bool
}

// whisperJSON mirrors the relevant subset of whisper-cli's -oj output.
type whisperJSON struct {
	Result struct {
		Language string `json:"language"`
	} `json:"result"`
	Transcription []struct {
		Offsets struct {
			From int64 `json:"from"`
			To   int64 `json:"to"`
		} `json:"offsets"`
		Text string `json:"text"`
	} `json:"transcription"`
}

// Run transcribes a 16 kHz mono WAV with whisper-cli and parses the JSON output.
func Run(ctx context.Context, bin, wav, tmpDir string, opts Options) (Result, error) {
	outBase := filepath.Join(tmpDir, "transcript")
	args := []string{"-m", opts.ModelPath, "-f", wav, "-oj", "-of", outBase}
	if opts.Language != "" {
		args = append(args, "-l", opts.Language)
	}
	if opts.Threads > 0 {
		args = append(args, "-t", strconv.Itoa(opts.Threads))
	}
	if opts.BeamSize > 0 {
		args = append(args, "-bs", strconv.Itoa(opts.BeamSize), "-bo", strconv.Itoa(opts.BeamSize))
	}

	if _, err := proc.Run(ctx, proc.RunOpts{Name: bin, Args: args, StreamStderr: opts.Verbose, Label: "whisper-cli"}); err != nil {
		return Result{}, err
	}

	jsonPath := outBase + ".json"
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		return Result{}, fmt.Errorf("read whisper json %s: %w", jsonPath, err)
	}
	return parse(data)
}

func parse(data []byte) (Result, error) {
	var raw whisperJSON
	if err := json.Unmarshal(data, &raw); err != nil {
		return Result{}, fmt.Errorf("parse whisper json: %w", err)
	}

	res := Result{Language: raw.Result.Language}
	var b strings.Builder
	for _, seg := range raw.Transcription {
		text := strings.TrimSpace(seg.Text)
		if text == "" {
			continue
		}
		res.Segments = append(res.Segments, Segment{StartMS: seg.Offsets.From, EndMS: seg.Offsets.To, Text: text})
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(text)
	}
	res.Text = b.String()
	if res.Text == "" {
		return res, fmt.Errorf("transcription produced no text")
	}
	return res, nil
}
