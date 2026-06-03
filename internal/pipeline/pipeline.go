package pipeline

import (
	"context"
	"fmt"
	"os"

	"github.com/rursache/vid-summary-cli/internal/config"
	"github.com/rursache/vid-summary-cli/internal/deps"
	"github.com/rursache/vid-summary-cli/internal/extract"
	"github.com/rursache/vid-summary-cli/internal/model"
	"github.com/rursache/vid-summary-cli/internal/summarize"
	"github.com/rursache/vid-summary-cli/internal/transcribe"
)

type Options struct {
	Input    string
	Model    string
	Language string
	Threads  int
	BeamSize int
	Backend  string
	LLMModel string
	Prompt   string
	Format   string // summary | summary+transcript | json
	KeepTemp bool
	Verbose  bool
}

// Run executes the full pipeline sequentially: acquire -> normalize ->
// transcribe -> summarize, with fail-fast dependency checks up front.
func Run(ctx context.Context, cfg config.Config, opts Options) (Result, error) {
	isURL := extract.IsURL(opts.Input)

	ffmpeg := deps.FFmpeg()
	whisper := deps.Whisper()
	if err := ffmpeg.Require(); err != nil {
		return Result{}, err
	}
	if err := whisper.Require(); err != nil {
		return Result{}, err
	}

	var ytdlp *deps.Tool
	if isURL {
		ytdlp = deps.YtDlp()
		if err := ytdlp.Require(); err != nil {
			return Result{}, err
		}
	}

	backend, err := summarize.New(opts.Backend, opts.LLMModel)
	if err != nil {
		return Result{}, err
	}
	// Fail fast before a long transcription if the summarizer CLI is missing.
	if err := backend.Available(); err != nil {
		return Result{}, err
	}

	modelPath, err := model.Ensure(opts.Model)
	if err != nil {
		return Result{}, err
	}

	tmpRoot, err := config.TmpDir()
	if err != nil {
		return Result{}, err
	}
	runDir, err := os.MkdirTemp(tmpRoot, "run-")
	if err != nil {
		return Result{}, err
	}
	if opts.KeepTemp {
		fmt.Fprintf(os.Stderr, "Keeping temp dir: %s\n", runDir)
	} else {
		defer os.RemoveAll(runDir)
	}

	source := opts.Input
	if isURL {
		fmt.Fprintln(os.Stderr, "Acquiring audio with yt-dlp...")
		source, err = extract.Acquire(ctx, ytdlp.Bin, opts.Input, runDir, opts.Verbose)
		if err != nil {
			return Result{}, err
		}
	}

	fmt.Fprintln(os.Stderr, "Normalizing audio with ffmpeg...")
	wav, err := extract.Normalize(ctx, ffmpeg.Bin, source, runDir, opts.Verbose)
	if err != nil {
		return Result{}, err
	}

	fmt.Fprintln(os.Stderr, "Transcribing with whisper-cli...")
	lang := opts.Language
	if lang == "auto" {
		lang = "" // let whisper auto-detect
	}
	tr, err := transcribe.Run(ctx, whisper.Bin, wav, runDir, transcribe.Options{
		ModelPath: modelPath,
		Language:  lang,
		Threads:   opts.Threads,
		BeamSize:  opts.BeamSize,
		Verbose:   opts.Verbose,
	})
	if err != nil {
		return Result{}, err
	}

	fmt.Fprintf(os.Stderr, "Summarizing with %s...\n", backend.Name())
	summary, err := summarize.Summarize(ctx, backend, tr, summarize.Options{
		Prompt:       opts.Prompt,
		MaxChars:     cfg.Chunking.MaxChars,
		OverlapChars: cfg.Chunking.OverlapChars,
	})
	if err != nil {
		return Result{}, err
	}

	res := Result{Summary: summary, Language: tr.Language}
	switch opts.Format {
	case "summary+transcript":
		res.Transcript = tr.Text
	case "json":
		res.Transcript = tr.Text
		res.Segments = tr.Segments
	}
	return res, nil
}
