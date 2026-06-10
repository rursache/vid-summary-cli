package extract

import (
	"net/url"
	"os"
	"strings"
)

// IsURL reports whether input should be treated as a remote URL (handled by
// yt-dlp) rather than a local file path. A local file always wins.
func IsURL(input string) bool {
	if _, err := os.Stat(input); err == nil {
		return false
	}
	u, err := url.Parse(input)
	if err != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

// IsYouTube reports whether the URL points at YouTube, where captions can be
// fetched directly instead of transcribing the audio
func IsYouTube(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	for _, prefix := range []string{"www.", "m.", "music."} {
		host = strings.TrimPrefix(host, prefix)
	}
	switch host {
	case "youtube.com", "youtu.be", "youtube-nocookie.com":
		return true
	}
	return false
}
