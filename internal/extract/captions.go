package extract

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/rursache/vid-summary-cli/internal/proc"
	"github.com/rursache/vid-summary-cli/internal/transcribe"
)

// subTrack lists the formats one caption track is available in
type subTrack []struct {
	Ext string `json:"ext"`
}

// captionInfo is the probed caption inventory of a video. Auto track keys:
// "en" (plain ASR, often vtt-only), "en-orig" (original language ASR) and
// "ro-en" (auto-translated, target-source)
type captionInfo struct {
	manual   map[string]subTrack
	auto     map[string]subTrack
	origLang string
}

// Captions fetches a YouTube caption track with yt-dlp (no media download)
// and parses it into a transcript. Preference: manual subs in lang, auto subs
// in lang (incl auto-translated), then English, then the original language.
// lang may be empty, meaning the video's own language
func Captions(ctx context.Context, ytdlpBin, rawURL, tmpDir, lang string, verbose bool) (transcribe.Result, error) {
	info, err := probeCaptions(ctx, ytdlpBin, rawURL)
	if err != nil {
		return transcribe.Result{}, err
	}

	target := lang
	if target == "" {
		target = info.origLang
	}
	if target == "" {
		target = "en"
	}

	cands := pickTracks(info, target)
	if len(cands) == 0 {
		return transcribe.Result{}, fmt.Errorf("no captions available")
	}

	// a track can still fail to download (YouTube rate-limits the translated
	// timedtext endpoint with 429s), so fall through the ranked candidates
	var lastErr error
	for _, c := range cands {
		if c.lang != target {
			fmt.Fprintf(os.Stderr, "No %q captions; using track %q instead\n", target, c.key)
		} else if verbose {
			fmt.Fprintf(os.Stderr, "Using caption track %q (auto: %v)\n", c.key, c.isAuto)
		}
		path, err := fetchTrack(ctx, ytdlpBin, rawURL, tmpDir, c.key, c.isAuto, verbose)
		if err != nil {
			lastErr = err
			fmt.Fprintf(os.Stderr, "Caption track %q failed (%s)\n", c.key, proc.Tail(err.Error(), 1))
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return transcribe.Result{}, fmt.Errorf("read captions %s: %w", path, err)
		}
		res, err := parseJSON3(data)
		if err != nil {
			lastErr = err
			continue
		}
		res.Language = c.lang
		return res, nil
	}
	return transcribe.Result{}, lastErr
}

// probeCaptions lists available caption tracks and the video language in a
// single yt-dlp metadata call
func probeCaptions(ctx context.Context, ytdlpBin, rawURL string) (captionInfo, error) {
	args := []string{
		"--skip-download", "--no-playlist",
		"--quiet", "--no-warnings",
		// the write flags force the extractor to expand the full caption
		// inventory (incl auto-translated tracks); --print implies simulate,
		// so no files are written
		"--write-subs", "--write-auto-subs",
		"--print", "%(subtitles)j\n%(automatic_captions)j\n%(language)s",
		rawURL,
	}
	res, err := proc.Run(ctx, proc.RunOpts{Name: ytdlpBin, Args: args, Label: "yt-dlp"})
	if err != nil {
		return captionInfo{}, err
	}

	lines := strings.Split(strings.TrimSpace(res.Stdout), "\n")
	if len(lines) < 3 {
		return captionInfo{}, fmt.Errorf("unexpected yt-dlp caption metadata output")
	}
	var info captionInfo
	if err := json.Unmarshal([]byte(lines[0]), &info.manual); err != nil {
		return captionInfo{}, fmt.Errorf("parse subtitles metadata: %w", err)
	}
	if err := json.Unmarshal([]byte(lines[1]), &info.auto); err != nil {
		return captionInfo{}, fmt.Errorf("parse auto captions metadata: %w", err)
	}

	info.origLang = lines[2]
	if info.origLang == "NA" || info.origLang == "none" || info.origLang == "null" {
		info.origLang = ""
	}
	if info.origLang == "" {
		// derive it from the original ASR track key ("<lang>-orig") if present
		for k := range info.auto {
			if l, found := strings.CutSuffix(k, "-orig"); found {
				info.origLang = l
				break
			}
		}
	}
	return info, nil
}

// candidate is a downloadable caption track; lang is the language the
// transcript will actually be in
type candidate struct {
	key    string
	isAuto bool
	lang   string
}

// pickTracks ranks the caption tracks to try: manual then auto in the target
// language, then English, then the video's original language
func pickTracks(info captionInfo, target string) []candidate {
	langs := []string{target}
	if target != "en" {
		langs = append(langs, "en")
	}
	if o := info.origLang; o != "" && o != target && o != "en" {
		langs = append(langs, o)
	}

	var cands []candidate
	seen := map[string]bool{}
	add := func(key string, isAuto bool, lang string) {
		if !seen[key] {
			seen[key] = true
			cands = append(cands, candidate{key: key, isAuto: isAuto, lang: lang})
		}
	}
	for _, lang := range langs {
		if k, found := matchLang(info.manual, lang, info.origLang); found {
			add(k, false, lang)
		}
		if k, found := matchLang(info.auto, lang, info.origLang); found {
			add(k, true, lang)
		}
	}
	return cands
}

// matchLang finds the best json3-capable track for lang: exact, original ASR
// (lang-orig), auto-translated from the original (lang-<orig>), translated
// from English, then any other variant like en-US or ro-<src>
func matchLang(tracks map[string]subTrack, lang, orig string) (string, bool) {
	for _, k := range []string{lang, lang + "-orig", lang + "-" + orig, lang + "-en"} {
		if hasJSON3(tracks[k]) {
			return k, true
		}
	}
	keys := make([]string, 0, len(tracks))
	for k := range tracks {
		keys = append(keys, k)
	}
	sort.Strings(keys) // deterministic pick across runs
	for _, k := range keys {
		if strings.HasPrefix(k, lang+"-") && hasJSON3(tracks[k]) {
			return k, true
		}
	}
	return "", false
}

func hasJSON3(t subTrack) bool {
	for _, f := range t {
		if f.Ext == "json3" {
			return true
		}
	}
	return false
}

// fetchTrack downloads exactly one caption track and returns the file path
func fetchTrack(ctx context.Context, ytdlpBin, rawURL, tmpDir, key string, isAuto, verbose bool) (string, error) {
	writeFlag := "--write-subs"
	if isAuto {
		writeFlag = "--write-auto-subs"
	}
	tmpl := filepath.Join(tmpDir, "captions.%(ext)s")
	args := []string{
		"--skip-download", "--no-playlist",
		writeFlag,
		"--sub-langs", key,
		"--sub-format", "json3",
		"--quiet", "--no-warnings",
		"--restrict-filenames",
		"-o", tmpl,
		rawURL,
	}
	if _, err := proc.Run(ctx, proc.RunOpts{Name: ytdlpBin, Args: args, StreamStderr: verbose, Label: "yt-dlp"}); err != nil {
		return "", err
	}

	// exact-path stat only: a glob fallback could pick up a stale file left
	// behind by a previously failed candidate in the same run dir
	path := filepath.Join(tmpDir, "captions."+key+".json3")
	if st, err := os.Stat(path); err != nil || st.Size() == 0 {
		return "", fmt.Errorf("yt-dlp wrote no caption file for track %q", key)
	}
	return path, nil
}

// json3Doc mirrors the subset of YouTube's json3 caption format we consume.
// Auto captions carry word-level segs; manual ones usually one seg per event
type json3Doc struct {
	Events []struct {
		TStartMs    int64 `json:"tStartMs"`
		DDurationMs int64 `json:"dDurationMs"`
		Segs        []struct {
			UTF8 string `json:"utf8"`
		} `json:"segs"`
	} `json:"events"`
}

func parseJSON3(data []byte) (transcribe.Result, error) {
	var doc json3Doc
	if err := json.Unmarshal(data, &doc); err != nil {
		return transcribe.Result{}, fmt.Errorf("parse json3 captions: %w", err)
	}

	var res transcribe.Result
	var b strings.Builder
	for _, ev := range doc.Events {
		var sb strings.Builder
		for _, seg := range ev.Segs {
			sb.WriteString(seg.UTF8)
		}
		text := strings.Join(strings.Fields(sb.String()), " ")
		if text == "" {
			continue
		}
		res.Segments = append(res.Segments, transcribe.Segment{
			StartMS: ev.TStartMs,
			EndMS:   ev.TStartMs + ev.DDurationMs,
			Text:    text,
		})
		if b.Len() > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(text)
	}
	res.Text = b.String()
	if res.Text == "" {
		return res, fmt.Errorf("captions track is empty")
	}
	return res, nil
}
