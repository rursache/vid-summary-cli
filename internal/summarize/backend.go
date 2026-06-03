package summarize

import (
	"context"
	"fmt"
	"strings"
)

// Backend is a single non-interactive LLM call. Implementations shell out to a
// local CLI (claude, codex) using its existing login; no API keys are handled.
// prompt is the fully built prompt (template with the transcript substituted).
type Backend interface {
	Name() string
	Available() error
	Generate(ctx context.Context, prompt string) (string, error)
}

type Options struct {
	Prompt       string
	Detail       string // short | medium | long
	MaxChars     int
	OverlapChars int
}

const reducePrompt = "The following are summaries of consecutive chunks of one transcript. " +
	"Combine them into a single coherent summary, preserving the original prompt's intent. Return only the summary."

func New(backend, model string) (Backend, error) {
	switch strings.ToLower(backend) {
	case "claude":
		return &claudeBackend{model: model}, nil
	case "codex":
		return &codexBackend{model: model}, nil
	case "gemini":
		return &geminiBackend{model: model}, nil
	default:
		return nil, fmt.Errorf("unknown summarizer backend %q (want \"claude\", \"codex\", or \"gemini\")", backend)
	}
}
