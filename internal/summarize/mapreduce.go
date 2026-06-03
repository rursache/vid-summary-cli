package summarize

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/rursache/vid-summary-cli/internal/transcribe"
)

const (
	// Placeholder is replaced with the transcript (or chunk) text.
	Placeholder = "<TRANSCRIPTION>"
	// DetailPlaceholder is replaced with a level-specific instruction.
	DetailPlaceholder = "<DETAIL>"
)

func detailPhrase(level string) string {
	switch strings.ToLower(level) {
	case "short":
		return "Write a brief, high-level summary in a single short paragraph, capturing only the most essential points."
	case "long":
		return "Write a comprehensive, detailed summary that covers all topics, key points, supporting details, and conclusions."
	default: // medium
		return "Write a balanced summary of a few paragraphs covering the main topics, key points, and conclusions."
	}
}

// buildPrompt fills the <DETAIL> and <TRANSCRIPTION> placeholders. If a template
// omits one, the corresponding content is prepended/appended so custom prompts
// still work.
func buildPrompt(template, text, detail string) string {
	p := template
	phrase := detailPhrase(detail)
	if strings.Contains(p, DetailPlaceholder) {
		p = strings.ReplaceAll(p, DetailPlaceholder, phrase)
	} else {
		p = phrase + "\n\n" + p
	}
	if strings.Contains(p, Placeholder) {
		p = strings.ReplaceAll(p, Placeholder, text)
	} else {
		p = p + "\n\n" + text
	}
	return p
}

// Summarize runs a single-pass summary for short transcripts, or map-reduce for
// long ones that would exceed the model context.
func Summarize(ctx context.Context, b Backend, tr transcribe.Result, opts Options) (string, error) {
	if len(tr.Text) <= opts.MaxChars {
		return b.Generate(ctx, buildPrompt(opts.Prompt, tr.Text, opts.Detail))
	}

	chunks := chunkSegments(tr.Segments, tr.Text, opts.MaxChars, opts.OverlapChars)
	fmt.Fprintf(os.Stderr, "Transcript is long; map-reduce over %d chunks\n", len(chunks))

	summaries := make([]string, 0, len(chunks))
	for i, c := range chunks {
		fmt.Fprintf(os.Stderr, "  summarizing chunk %d/%d\n", i+1, len(chunks))
		s, err := b.Generate(ctx, buildPrompt(opts.Prompt, c, opts.Detail))
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
	instr := reducePrompt + " " + detailPhrase(opts.Detail)
	combined := strings.Join(summaries, "\n\n")
	if len(combined) <= opts.MaxChars || len(summaries) == 1 {
		return b.Generate(ctx, instr+"\n\n"+combined)
	}

	groups := groupByBudget(summaries, opts.MaxChars)
	next := make([]string, 0, len(groups))
	for _, g := range groups {
		s, err := b.Generate(ctx, instr+"\n\n"+strings.Join(g, "\n\n"))
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
