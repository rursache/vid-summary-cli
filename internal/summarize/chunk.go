package summarize

import (
	"strings"

	"github.com/rursache/vid-summary-cli/internal/transcribe"
)

// chunkSegments splits the transcript into windows under maxChars, respecting
// segment boundaries and carrying overlap between windows. Falls back to raw
// text slicing when segments are unavailable.
func chunkSegments(segs []transcribe.Segment, full string, maxChars, overlap int) []string {
	if len(segs) == 0 {
		return chunkText(full, maxChars, overlap)
	}

	var chunks []string
	var cur strings.Builder
	for _, s := range segs {
		if cur.Len() > 0 && cur.Len()+len(s.Text)+1 > maxChars {
			chunk := cur.String()
			chunks = append(chunks, chunk)
			cur.Reset()
			if overlap > 0 && len(chunk) > overlap {
				cur.WriteString(chunk[len(chunk)-overlap:])
				cur.WriteByte(' ')
			}
		}
		if cur.Len() > 0 {
			cur.WriteByte(' ')
		}
		cur.WriteString(s.Text)
	}
	if cur.Len() > 0 {
		chunks = append(chunks, cur.String())
	}
	return chunks
}

func chunkText(s string, maxChars, overlap int) []string {
	if len(s) <= maxChars {
		return []string{s}
	}
	var chunks []string
	step := maxChars - overlap
	if step <= 0 {
		step = maxChars
	}
	for start := 0; start < len(s); start += step {
		end := start + maxChars
		if end > len(s) {
			end = len(s)
		}
		chunks = append(chunks, s[start:end])
		if end == len(s) {
			break
		}
	}
	return chunks
}
