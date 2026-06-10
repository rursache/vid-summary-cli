package pipeline

import (
	"context"
	"fmt"
	"os"

	"github.com/rursache/vid-summary-cli/internal/config"
	"github.com/rursache/vid-summary-cli/internal/deps"
	"github.com/rursache/vid-summary-cli/internal/extract"
	"github.com/rursache/vid-summary-cli/internal/model"
	"github.com/rursache/vid-summary-cli/internal/proc"
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
	Detail   string
	Prompt   string
	Format   string // summary | summary+transcript | json
	KeepTemp bool
	Verbose  bool
}

// Run executes the full pipeline sequentially: acquire -> normalize ->
// transcribe -> summarize, with fail-fast dependency checks up front.
// YouTube URLs take a fast path: fetch the caption track instead of
// downloading and transcribing the audio, falling back to whisper when no
// captions exist
func Run(ctx context.Context, cfg config.Config, opts Options) (Result, error) {
	isURL := extract.IsURL(opts.Input)
	isYouTube := isURL && extract.IsYouTube(opts.Input)

	ffmpeg := deps.FFmpeg()
	whisper := deps.Whisper()
	// The YouTube captions path needs neither ffmpeg/whisper nor a model, so
	// for YouTube these checks move to the transcription fallback
	requireTranscription := func() (string, error) {
		if err := ffmpeg.Require(); err != nil {
			return "", err
		}
		if err := whisper.Require(); err != nil {
			return "", err
		}
		return model.Ensure(opts.Model)
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

	var modelPath string
	if !isYouTube {
		if modelPath, err = requireTranscription(); err != nil {
			return Result{}, err
		}
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

	lang := opts.Language
	if lang == "auto" {
		lang = "" // let whisper auto-detect / use the video's own language
	}

	var tr transcribe.Result
	gotCaptions := false
	if isYouTube {
		fmt.Fprintln(os.Stderr, "Fetching YouTube captions with yt-dlp...")
		if ctr, capErr := extract.Captions(ctx, ytdlp.Bin, opts.Input, runDir, lang, opts.Verbose); capErr == nil {
			tr = ctr
			gotCaptions = true
		} else {
			fmt.Fprintf(os.Stderr, "Captions unavailable (%s); falling back to audio transcription\n", proc.Tail(capErr.Error(), 1))
		}
	}

	if !gotCaptions {
		if isYouTube {
			// deferred fail-fast checks for the fallback path
			if modelPath, err = requireTranscription(); err != nil {
				return Result{}, err
			}
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
		tr, err = transcribe.Run(ctx, whisper.Bin, wav, runDir, transcribe.Options{
			ModelPath: modelPath,
			Language:  lang,
			Threads:   opts.Threads,
			BeamSize:  opts.BeamSize,
			Verbose:   opts.Verbose,
		})
		if err != nil {
			return Result{}, err
		}
	}

	fmt.Fprintf(os.Stderr, "Summarizing with %s...\n", backend.Name())
	summary, err := summarize.Summarize(ctx, backend, tr, summarize.Options{
		Prompt:       opts.Prompt,
		Detail:       opts.Detail,
		MaxChars:     cfg.Chunking.MaxChars,
		OverlapChars: cfg.Chunking.OverlapChars,
	})
	if err != nil {
		return Result{}, err
	}

	res := Result{Summary: summary, Language: tr.Language}
	if gotCaptions {
		res.Source = "captions"
	} else {
		res.Source = "whisper"
	}
	switch opts.Format {
	case "summary+transcript":
		res.Transcript = tr.Text
	case "json":
		res.Transcript = tr.Text
		res.Segments = tr.Segments
	}
	return res, nil
}
