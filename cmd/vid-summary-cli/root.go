package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/rursache/vid-summary-cli/internal/config"
	"github.com/rursache/vid-summary-cli/internal/pipeline"
	"github.com/rursache/vid-summary-cli/internal/summarize"
	"github.com/spf13/cobra"
)

type rootFlags struct {
	model    string
	language string
	threads  int
	beamSize int
	backend  string
	llmModel string
	detail   string
	prompt   string
	output   string
	format   string
	keepTemp bool
	verbose  bool
}

func newRootCmd() *cobra.Command {
	f := &rootFlags{}

	cmd := &cobra.Command{
		Use:   "vid-summary-cli <url|file>",
		Short: "Summarize a video via audio -> transcript -> AI summary",
		Long: "vid-summary-cli turns a video URL or local media file into a concise text summary,\n" +
			"using yt-dlp, ffmpeg and whisper.cpp for transcription and a local claude/codex CLI for the summary.",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runPipeline(cmd, args[0], f)
		},
	}

	pf := cmd.PersistentFlags()
	pf.StringVar(&f.model, "model", "", "whisper model (default from config)")
	pf.StringVar(&f.language, "language", "", "source language or \"auto\" (default from config)")
	pf.IntVar(&f.threads, "threads", 0, "whisper-cli threads (0 = whisper default)")
	pf.IntVar(&f.beamSize, "beam-size", 0, "whisper beam size (0 = default 5; 1 = greedy/faster)")
	pf.StringVar(&f.backend, "backend", "", "summarizer backend: claude | codex | gemini (default from config)")
	pf.StringVar(&f.llmModel, "llm-model", "", "LLM model name for the summarizer CLI")
	pf.StringVar(&f.detail, "detail", "", "summary detail level: short | medium | long (default from config)")
	pf.StringVar(&f.prompt, "prompt", "", "path to a custom prompt template")
	pf.StringVar(&f.output, "output", "", "write summary to file (default: stdout)")
	pf.StringVar(&f.format, "format", "summary", "output: summary | summary+transcript | json")
	pf.BoolVar(&f.keepTemp, "keep-temp", false, "do not delete temp artifacts")
	pf.BoolVarP(&f.verbose, "verbose", "v", false, "verbose logging")

	cmd.AddCommand(newModelsCmd(), newDoctorCmd(), newVersionCmd())
	return cmd
}

func runPipeline(cmd *cobra.Command, input string, f *rootFlags) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	// An explicit --backend is used as-is (and errors later if not installed).
	// Otherwise auto-detect: prefer the configured backend, then codex/claude/gemini.
	backend := f.backend
	if backend == "" {
		backend = summarize.Detect(cfg.Summarizer.Backend)
		if backend == "" {
			return fmt.Errorf("no summarizer CLI found in PATH. Install one of: %s", strings.Join(summarize.Known(), ", "))
		}
		if backend != cfg.Summarizer.Backend {
			fmt.Fprintf(os.Stderr, "summarizer %q not found; using %q\n", cfg.Summarizer.Backend, backend)
		}
	}

	opts := pipeline.Options{
		Input:    input,
		Model:    pick(f.model, cfg.Model),
		Language: pick(f.language, cfg.Language),
		Threads:  pickInt(f.threads, cfg.Threads),
		BeamSize: pickInt(f.beamSize, cfg.BeamSize),
		Backend:  backend,
		LLMModel: pick(f.llmModel, cfg.Summarizer.Model),
		Detail:   pick(f.detail, cfg.Summarizer.Detail),
		Prompt:   cfg.Summarizer.Prompt,
		Format:   f.format,
		KeepTemp: f.keepTemp,
		Verbose:  f.verbose,
	}

	if f.prompt != "" {
		data, err := os.ReadFile(f.prompt)
		if err != nil {
			return fmt.Errorf("read prompt template: %w", err)
		}
		opts.Prompt = string(data)
	}

	switch opts.Detail {
	case "short", "medium", "long":
	default:
		return fmt.Errorf("invalid --detail %q (want short, medium, or long)", opts.Detail)
	}

	res, err := pipeline.Run(cmd.Context(), cfg, opts)
	if err != nil {
		return err
	}

	rendered, err := res.Render(f.format)
	if err != nil {
		return err
	}

	if f.output != "" {
		if err := os.WriteFile(f.output, []byte(rendered+"\n"), 0o644); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Wrote summary to %s\n", f.output)
		return nil
	}
	fmt.Fprintln(cmd.OutOrStdout(), rendered)
	return nil
}

func pick(flag, fallback string) string {
	if flag != "" {
		return flag
	}
	return fallback
}

func pickInt(flag, fallback int) int {
	if flag != 0 {
		return flag
	}
	return fallback
}
