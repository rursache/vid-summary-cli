package summarize

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/rursache/vid-summary-cli/internal/transcribe"
)

// Placeholder is replaced with the transcript (or chunk) inside the prompt
// template. If a template omits it, the text is appended instead.
const Placeholder = "<TRANSCRIPTION>"

func buildPrompt(template, text string) string {
	if strings.Contains(template, Placeholder) {
		return strings.ReplaceAll(template, Placeholder, text)
	}
	return template + "\n\n" + text
}

// Summarize runs a single-pass summary for short transcripts, or map-reduce for
// long ones that would exceed the model context.
func Summarize(ctx context.Context, b Backend, tr transcribe.Result, opts Options) (string, error) {
	if len(tr.Text) <= opts.MaxChars {
		return b.Generate(ctx, buildPrompt(opts.Prompt, tr.Text))
	}

	chunks := chunkSegments(tr.Segments, tr.Text, opts.MaxChars, opts.OverlapChars)
	fmt.Fprintf(os.Stderr, "Transcript is long; map-reduce over %d chunks\n", len(chunks))

	summaries := make([]string, 0, len(chunks))
	for i, c := range chunks {
		fmt.Fprintf(os.Stderr, "  summarizing chunk %d/%d\n", i+1, len(chunks))
		s, err := b.Generate(ctx, buildPrompt(opts.Prompt, c))
		if err != nil {
			return "", fmt.Errorf("map chunk %d: %w", i+1, err)
		}
		summaries = append(summaries, strings.TrimSpace(s))
	}

	return reduce(ctx, b, summaries, opts)
}

// reduce folds chunk summaries into one, recursing if the combined text still
// exceeds the chunk budget.
func reduce(ctx context.Context, b Backend, summaries []string, opts Options) (string, error) {
	combined := strings.Join(summaries, "\n\n")
	if len(combined) <= opts.MaxChars || len(summaries) == 1 {
		return b.Generate(ctx, reducePrompt+"\n\n"+combined)
	}

	groups := groupByBudget(summaries, opts.MaxChars)
	next := make([]string, 0, len(groups))
	for _, g := range groups {
		s, err := b.Generate(ctx, reducePrompt+"\n\n"+strings.Join(g, "\n\n"))
		if err != nil {
			return "", fmt.Errorf("reduce: %w", err)
		}
		next = append(next, strings.TrimSpace(s))
	}
	return reduce(ctx, b, next, opts)
}

func groupByBudget(items []string, maxChars int) [][]string {
	var groups [][]string
	var cur []string
	curLen := 0
	for _, it := range items {
		if curLen > 0 && curLen+len(it) > maxChars {
			groups = append(groups, cur)
			cur, curLen = nil, 0
		}
		cur = append(cur, it)
		curLen += len(it) + 2
	}
	if len(cur) > 0 {
		groups = append(groups, cur)
	}
	return groups
}
