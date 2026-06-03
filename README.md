# vid-summary-cli

A cross-platform CLI that summarizes videos through a
**video → audio → transcript → AI → summary** pipeline.

`vid-summary-cli` is a single Go binary that orchestrates best-in-class external
tools (`yt-dlp`, `ffmpeg`, `whisper.cpp`) plus a local AI CLI (`claude`, `codex`,
or `gemini`) to turn a video — a URL or a local file — into a concise text summary.
The tool itself does no media decoding or transcription; it orchestrates.

- **Primary target:** macOS on Apple Silicon
- **Also supported:** Linux (x86_64 / arm64) and Windows

```
URL        → yt-dlp (bestaudio) ─┐
                                 ├→ ffmpeg (16kHz mono PCM) → whisper-cli (JSON) → AI summary
local file ──────────────────────┘
```

---

## Requirements

`vid-summary-cli` detects its dependencies at runtime and prints the exact
install command for your OS if one is missing. It never auto-installs anything.
The only artifact it downloads itself is the Whisper model.

### Transcription pipeline

| Dependency  | macOS                       | Linux                                                              | Windows                              |
|-------------|-----------------------------|--------------------------------------------------------------------|--------------------------------------|
| yt-dlp¹     | `brew install yt-dlp`       | `yay -S yt-dlp` · `sudo apt install yt-dlp` · `pipx install yt-dlp`| `winget install yt-dlp.yt-dlp` · `scoop install yt-dlp` |
| ffmpeg      | `brew install ffmpeg`       | `sudo pacman -S ffmpeg` · `sudo apt install ffmpeg` · `sudo dnf install ffmpeg` | `winget install Gyan.FFmpeg` · `scoop install ffmpeg` |
| whisper-cli | `brew install whisper-cpp`  | `yay -S whisper.cpp` (AUR) · or build from source                  | `scoop install whisper-cpp` · or build from source |

¹ `yt-dlp` is only required when the input is a URL. Local-file runs never need it.

> **whisper binary name:** the Homebrew `whisper-cpp` formula installs the
> `whisper-cli` binary (`whisper-cli.exe` on Windows). Older builds named it
> `main` — both are detected.

> **whisper GPU acceleration is a build-time choice.** Prebuilt packages are
> usually CPU-only. For GPU:
> - **macOS:** Metal is on by default — nothing to do.
> - **Linux + NVIDIA:** `yay -S whisper.cpp-cuda` (AUR), or build with `-DGGML_CUDA=1`.
> - **Linux + AMD/Intel:** `yay -S whisper.cpp-vulkan` (AUR), or build with `-DGGML_VULKAN=1`.
> - **Windows + NVIDIA:** [download](https://github.com/ggml-org/whisper.cpp/releases) the `whisper-cublas-*-bin-x64.zip` release.
> - **Windows + AMD/Intel:** build from source with `-DGGML_VULKAN=1`.
>
> On Fedora, full-codec `ffmpeg` needs RPM Fusion enabled.

### Summarizer (pick one)

The summary stage shells out to a local AI CLI using your existing login —
**no API keys are stored or required**.

- [`claude`](https://claude.com/claude-code) (Claude Code CLI) — run `claude login`
- [`codex`](https://developers.openai.com/codex) (Codex CLI) — run `codex login`
- [`gemini`](https://github.com/google-gemini/gemini-cli) (Gemini CLI) — sign in once by running `gemini`

Select the backend in config (`summarizer.backend`) or with `--backend`.

---

## Install / build

Pure Go, cross-compiles with no extra toolchain:

```sh
make build        # -> bin/vid-summary-cli
make install      # -> $GOBIN/vid-summary-cli
make dist         # darwin-arm64, linux-amd64, linux-arm64 artifacts
```

Or directly: `go install github.com/rursache/vid-summary-cli/cmd/vid-summary-cli@latest`

---

## Usage

```sh
vid-summary-cli <url|file> [flags]
```

By default the summary is printed to **stdout**; all progress goes to **stderr**,
so `vid-summary-cli video.mp4 > out.txt` captures only the summary.

```
Flags:
  --model string        Whisper model (default "large-v3-turbo-q8_0")
  --language string     Source language or "auto"
  --threads int         whisper-cli threads (0 = whisper default)
  --beam-size int       whisper beam size (0 = default 5; 1 = greedy/faster)
  --backend string      Summarizer backend: claude | codex | gemini
  --detail string       Summary detail level: short | medium | long (default "medium")
  --llm-model string    LLM model name for the summarizer CLI
  --prompt string       Path to a custom prompt template
  --output string       Write summary to file (default: stdout)
  --format string       summary | summary+transcript | json
  --keep-temp           Do not delete temp artifacts
  -v, --verbose         Verbose logging (streams ffmpeg/whisper/yt-dlp output)
  -h, --help            Help

Subcommands:
  vid-summary-cli models list
  vid-summary-cli models download <model>
  vid-summary-cli models rm <model>
  vid-summary-cli doctor        # check deps + config + model cache
  vid-summary-cli version
```

Examples:

```sh
vid-summary-cli https://www.youtube.com/watch?v=... -v
vid-summary-cli ./talk.mp4 --backend claude --format summary+transcript
vid-summary-cli ./talk.mp4 --output summary.txt
vid-summary-cli ./talk.mp4 --beam-size 1        # faster, greedy decoding
```

---

## Configuration

Config lives at `~/.config/vid-summary-cli/config.yaml` (**YAML**), created on
first run. Precedence: **flags > config > built-in defaults**.

```yaml
model: large-v3-turbo-q8_0
language: auto
threads: 0            # 0 = whisper default (4 on Apple Silicon Metal)
beam_size: 0          # 0 = whisper default (5); set 1 for greedy/faster

summarizer:
  backend: codex      # claude | codex | gemini (uses your local CLI login, no API key)
  model: ""           # optional model override; empty = the CLI's default
  detail: medium      # short | medium | long
  prompt: 'Read and summarize the following text by responding only with the summary itself. <DETAIL> The text is a full transcription of a video:: <TRANSCRIPTION>'

chunking:
  max_chars: 12000    # per map-reduce chunk
  overlap_chars: 500
```

### Prompt template

`summarizer.prompt` is shared by all backends. Two placeholders are filled at
summary time:
- `<TRANSCRIPTION>` — the transcript (or each chunk, during map-reduce)
- `<DETAIL>` — a length/depth instruction chosen by `--detail`
  (`short` | `medium` | `long`), so the model itself controls how thorough the
  summary is

If you omit a placeholder in a custom template, the corresponding content is
prepended/appended instead. Override per-run with `--prompt path/to/template.txt`
and `--detail`.

---

## How it works

1. **Acquire** (URL only): `yt-dlp -f bestaudio/best` downloads the audio.
   Local files skip this stage.
2. **Normalize**: `ffmpeg` resamples to 16 kHz mono signed-16 PCM WAV (mandatory
   for whisper.cpp), selecting the first audio stream and dropping video.
3. **Transcribe**: `whisper-cli` produces JSON; segments and timestamps are
   parsed from structured output, never scraped from stdout.
4. **Summarize**: the transcript is sent to the chosen AI CLI. Short transcripts
   are summarized in one pass; long ones use **map-reduce** (summarize each
   chunk, then summarize the summaries, recursing if needed) so the model
   context window is never exceeded.

The default model `large-v3-turbo-q8_0` (~833 MB) is auto-downloaded to
`~/.config/vid-summary-cli/` on first use, written atomically, and verified
against a known SHA-256.

---

## Performance & tuning

Transcription speed depends on the machine and the whisper.cpp build:

- **macOS (Apple Silicon):** whisper.cpp uses **Metal** automatically. The
  default thread count (4 = performance cores) and beam size are near-optimal;
  more threads do not help once the GPU is the bottleneck.
- **Linux / Windows:** whisper.cpp uses **CUDA** (NVIDIA), **Vulkan**, or CPU
  depending on the build. CPU-only builds benefit from raising `--threads` to
  your core count.
- `--beam-size 1` (greedy) is faster with a small accuracy cost — useful on
  slower machines or if a build exhibits the beam-search slowdown.

---

## Application directory

Everything persistent lives under `~/.config/vid-summary-cli/` (override with
`VID_SUMMARY_CLI_HOME`), matching the sibling `*-cli` tools. Config and the
downloaded models share the folder:

```
~/.config/vid-summary-cli/
├── config.yaml
├── ggml-large-v3-turbo-q8_0.bin
└── tmp/                       # per-run scratch, cleaned up unless --keep-temp
```

Run `vid-summary-cli doctor` to check dependency versions, resolved config, the
summarizer backend, and the model cache.

---

## Project layout

```
cmd/vid-summary-cli/    cobra commands (root/run, models, doctor, version)
internal/
├── config/             home dir, config load/create, defaults
├── deps/               dependency detection + OS-aware install hints
├── extract/            url detection, yt-dlp acquire, ffmpeg normalize
├── model/              model registry, download, verify, cache
├── proc/               shared external-process runner
├── summarize/          backend interface, claude/codex backends, map-reduce, chunking
├── transcribe/         whisper-cli wrapper + JSON parsing
└── pipeline/           stage orchestration, temp lifecycle, output rendering
```
