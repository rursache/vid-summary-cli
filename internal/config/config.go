package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	// EnvHome overrides the application home directory (handy for tests).
	EnvHome = "VID_SUMMARY_CLI_HOME"

	DefaultModel    = "large-v3-turbo-q8_0"
	DefaultLanguage = "auto"
	DefaultBackend  = "codex"
)

// defaultPrompt is shared by all summarizer backends. <DETAIL> is replaced with
// a level-specific instruction (see --detail) and <TRANSCRIPTION> with the
// (chunk of) transcript text at summary time.
const defaultPrompt = "Read and summarize the following text by responding only with the summary itself. <DETAIL> The text is a full transcription of a video:: <TRANSCRIPTION>"

const DefaultDetail = "medium"

type Config struct {
	Model      string           `yaml:"model"`
	Language   string           `yaml:"language"`
	Threads    int              `yaml:"threads"`
	BeamSize   int              `yaml:"beam_size"` // 0 = whisper default; set 1 for greedy (faster)
	Summarizer SummarizerConfig `yaml:"summarizer"`
	Chunking   ChunkingConfig   `yaml:"chunking"`
}

type SummarizerConfig struct {
	Backend string `yaml:"backend"` // claude | codex | gemini | grok
	Model   string `yaml:"model"`   // optional CLI model override; empty = CLI default
	Detail  string `yaml:"detail"`  // short | medium | long
	Prompt  string `yaml:"prompt"`
}

type ChunkingConfig struct {
	MaxChars     int `yaml:"max_chars"`
	OverlapChars int `yaml:"overlap_chars"`
}

func Defaults() Config {
	return Config{
		Model:    DefaultModel,
		Language: DefaultLanguage,
		Threads:  0,
		Summarizer: SummarizerConfig{
			Backend: DefaultBackend,
			Model:   "",
			Detail:  DefaultDetail,
			Prompt:  defaultPrompt,
		},
		Chunking: ChunkingConfig{
			MaxChars:     12000,
			OverlapChars: 500,
		},
	}
}

// Home resolves the application directory, creating it (0755) if missing.
// Matches the sibling *-cli tools: ~/.config/vid-summary-cli/.
func Home() (string, error) {
	dir := os.Getenv(EnvHome)
	if dir == "" {
		h, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home: %w", err)
		}
		dir = filepath.Join(h, ".config", "vid-summary-cli")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create %s: %w", dir, err)
	}
	return dir, nil
}

// ModelsDir returns the directory holding downloaded models. It is the app
// directory itself, so models sit alongside config.yaml.
func ModelsDir() (string, error) { return Home() }

// TmpDir returns the scratch subdirectory for per-run artifacts.
func TmpDir() (string, error) {
	home, err := Home()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(home, "tmp")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create %s: %w", dir, err)
	}
	return dir, nil
}

func Path() (string, error) {
	home, err := Home()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "config.yaml"), nil
}

// Load reads config.yaml, generating a default file on first run if absent.
// Missing fields fall back to defaults.
func Load() (Config, error) {
	cfg := Defaults()
	path, err := Path()
	if err != nil {
		return cfg, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if err := writeDefault(path); err != nil {
			return cfg, err
		}
		return cfg, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("read config: %w", err)
	}

	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("parse %s: %w", path, err)
	}
	applyFallbacks(&cfg)
	return cfg, nil
}

func applyFallbacks(c *Config) {
	d := Defaults()
	if c.Model == "" {
		c.Model = d.Model
	}
	if c.Language == "" {
		c.Language = d.Language
	}
	if c.Summarizer.Backend == "" {
		c.Summarizer.Backend = d.Summarizer.Backend
	}
	if c.Summarizer.Detail == "" {
		c.Summarizer.Detail = d.Summarizer.Detail
	}
	if c.Summarizer.Prompt == "" {
		c.Summarizer.Prompt = d.Summarizer.Prompt
	}
	if c.Chunking.MaxChars <= 0 {
		c.Chunking.MaxChars = d.Chunking.MaxChars
	}
	if c.Chunking.OverlapChars < 0 {
		c.Chunking.OverlapChars = d.Chunking.OverlapChars
	}
}

func writeDefault(path string) error {
	data, err := yaml.Marshal(Defaults())
	if err != nil {
		return err
	}
	header := "# vid-summary-cli configuration\n" +
		"# summarizer.backend uses your local claude/codex/gemini/grok CLI login; no API key is stored here.\n"
	if err := os.WriteFile(path, append([]byte(header), data...), 0o644); err != nil {
		return fmt.Errorf("write default config: %w", err)
	}
	return nil
}
