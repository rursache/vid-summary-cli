package pipeline

import (
	"encoding/json"

	"github.com/rursache/vid-summary-cli/internal/transcribe"
)

type Result struct {
	Summary    string               `json:"summary"`
	Language   string               `json:"language,omitempty"`
	Transcript string               `json:"transcript,omitempty"`
	Segments   []transcribe.Segment `json:"segments,omitempty"`
}

// Render formats the result for output in the requested format.
func (r Result) Render(format string) (string, error) {
	switch format {
	case "json":
		b, err := json.MarshalIndent(r, "", "  ")
		if err != nil {
			return "", err
		}
		return string(b), nil
	case "summary+transcript":
		return r.Summary + "\n\n--- TRANSCRIPT ---\n\n" + r.Transcript, nil
	default:
		return r.Summary, nil
	}
}
