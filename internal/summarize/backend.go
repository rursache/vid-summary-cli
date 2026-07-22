package summarize

import (
	"context"
	"fmt"
	"strings"
)

// Backend is a single non-interactive LLM call. Implementations shell out to a
// local CLI (claude, codex, gemini, grok) using its existing login; no API keys
// are handled. prompt is the fully built prompt (template with the transcript
// substituted).
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

// knownBackends is the auto-detect preference order when no explicit backend
// is requested.
var knownBackends = []string{"codex", "claude", "gemini", "grok"}

func Known() []string { return knownBackends }

func New(backend, model string) (Backend, error) {
	switch strings.ToLower(backend) {
	case "claude":
		return &claudeBackend{model: model}, nil
	case "codex":
		return &codexBackend{model: model}, nil
	case "gemini":
		return &geminiBackend{model: model}, nil
	case "grok":
		return &grokBackend{model: model}, nil
	default:
		return nil, fmt.Errorf("unknown summarizer backend %q (want \"claude\", \"codex\", \"gemini\", or \"grok\")", backend)
	}
}

// Installed reports whether a backend's CLI is present in PATH.
func Installed(name string) bool {
	b, err := New(name, "")
	return err == nil && b.Available() == nil
}

// Detect returns the first installed backend, trying preferred first and then
// the known order. Returns "" if none are installed.
func Detect(preferred string) string {
	seen := map[string]bool{}
	for _, name := range append([]string{preferred}, knownBackends...) {
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		if Installed(name) {
			return name
		}
	}
	return ""
}
