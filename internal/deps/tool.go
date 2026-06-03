package deps

import (
	"bufio"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

type Tool struct {
	Name        string   // logical name shown to the user
	binNames    []string // candidate executables, in priority order
	versionArgs []string

	// install holds OS-specific install commands keyed by GOOS
	// ("darwin", "linux", "windows").
	install map[string]string

	// Populated by lookup.
	Found   bool
	Bin     string
	Path    string
	Version string
}

func (t *Tool) lookup() {
	for _, name := range t.binNames {
		if p, err := exec.LookPath(name); err == nil {
			t.Found = true
			t.Bin = name
			t.Path = p
			t.Version = probeVersion(name, t.versionArgs)
			if t.Bin == "main" {
				t.Version = strings.TrimSpace(t.Version + " (legacy 'main' binary; rename to whisper-cli)")
			}
			return
		}
	}
}

func probeVersion(bin string, args []string) string {
	if len(args) == 0 {
		return ""
	}
	out, err := exec.Command(bin, args...).Output()
	if err != nil {
		return ""
	}
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	if sc.Scan() {
		return strings.TrimSpace(sc.Text())
	}
	return ""
}

// InstallHint returns the OS-appropriate install command(s).
func (t *Tool) InstallHint() string {
	if h, ok := t.install[runtime.GOOS]; ok {
		return h
	}
	return "see the project README for install instructions on your platform"
}

// Require returns a clear, OS-correct error if the tool is missing.
func (t *Tool) Require() error {
	if t.Found {
		return nil
	}
	return fmt.Errorf("%s not found in PATH.\n  Install it with: %s", t.Name, t.InstallHint())
}
