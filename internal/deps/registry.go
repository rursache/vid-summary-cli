package deps

func ytdlp() *Tool {
	return &Tool{
		Name:        "yt-dlp",
		binNames:    []string{"yt-dlp"},
		versionArgs: []string{"--version"},
		install: map[string]string{
			"darwin":  "brew install yt-dlp",
			"linux":   "yay -S yt-dlp  (Arch)  |  sudo apt install yt-dlp  (Debian/Ubuntu)  |  pipx install yt-dlp",
			"windows": "winget install yt-dlp.yt-dlp  |  scoop install yt-dlp",
		},
	}
}

func ffmpeg() *Tool {
	return &Tool{
		Name:        "ffmpeg",
		binNames:    []string{"ffmpeg"},
		versionArgs: []string{"-version"},
		install: map[string]string{
			"darwin":  "brew install ffmpeg",
			"linux":   "sudo pacman -S ffmpeg  (Arch)  |  sudo apt install ffmpeg  (Debian/Ubuntu)  |  sudo dnf install ffmpeg  (Fedora)",
			"windows": "winget install Gyan.FFmpeg  |  scoop install ffmpeg",
		},
	}
}

// whisper detects the modern "whisper-cli" binary and falls back to the legacy
// "main" name. The brew formula is "whisper-cpp", not "whisper-cli".
func whisper() *Tool {
	return &Tool{
		Name:        "whisper-cli",
		binNames:    []string{"whisper-cli", "main"},
		versionArgs: nil, // whisper-cli has no stable --version flag
		install: map[string]string{
			"darwin":  "brew install whisper-cpp",
			"linux":   "yay -S whisper.cpp  (AUR)  |  or build from source: github.com/ggml-org/whisper.cpp",
			"windows": "scoop install whisper-cpp  |  or build from source: github.com/ggml-org/whisper.cpp",
		},
	}
}

func YtDlp() *Tool   { t := ytdlp(); t.lookup(); return t }
func FFmpeg() *Tool  { t := ffmpeg(); t.lookup(); return t }
func Whisper() *Tool { t := whisper(); t.lookup(); return t }

// All returns every tool with lookup performed, for doctor output.
func All() []*Tool {
	tools := []*Tool{ytdlp(), ffmpeg(), whisper()}
	for _, t := range tools {
		t.lookup()
	}
	return tools
}
