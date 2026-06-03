package extract

import (
	"net/url"
	"os"
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
