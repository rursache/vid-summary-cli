# vid-summary-cli — Implementation Plan

A cross-platform CLI tool that summarizes videos through a
**video → audio → transcript → AI → summary** pipeline.

This document is the implementation spec. It captures all requirements and
the rationale behind each architectural decision so the implementer does not
have to re-derive them.

---

## 1. Overview

`vid-summary-cli` is a single Go binary that orchestrates three external tools
(`yt-dlp`, `ffmpeg`, `whisper-cli`) plus a pluggable LLM backend to turn a
video — either a URL or a local file — into a concise text summary.

The tool itself does **no** transcription or media decoding. It shells out to
best-in-class external binaries and is responsible only for orchestration,
model management, transcript parsing, and the summarization stage.

> **Naming convention.** Everything uses the `-cli` suffix consistently: the repository,
> Go module, release artifacts, and the installed/invoked command are all
> **`vid-summary-cli`**, and the app home/config directory is
> **`~/.vid-summary-cli/`**. Environment variables use the `VID_SUMMARY_CLI_` prefix.

**Primary target:** macOS on Apple Silicon.
**Secondary target:** Linux x86_64 / arm64 (Arch / CachyOS). Linux support is
expected to be near-free because of the architecture below; it must not require
a divergent code path.

---

## 2. Goals & Non-Goals

### Goals
- One static Go binary per platform, built via plain `GOOS`/`GOARCH` cross-compilation.
- Accept either a remote video URL (yt-dlp handles it) or a local media file.
- Produce an accurate transcript using a local Whisper model, then an AI summary.
- Auto-download and cache the Whisper model on first use.
- Clear, actionable errors when a required external dependency is missing.
- Pluggable summarization backend (local LLM endpoint or remote HTTP API).
- Handle long videos without exceeding the LLM context window.

### Non-Goals (v1)
- No bundling/auto-installing of `yt-dlp`, `ffmpeg`, or `whisper-cli`. The tool
  **detects and instructs**; the user installs them via `brew` / `yay`.
- No CGo binding of whisper.cpp. We shell out to `whisper-cli` to preserve
  trivial cross-compilation.
- No CoreML / Apple Neural Engine path (macOS-only) and no Parakeet/CUDA path
  (Linux+NVIDIA-only). Both would force per-platform branches for marginal gain.
- No streaming/concurrent pipeline stages in v1 (sequential only — see §13).
- No GUI.

---

## 3. Key Architectural Decisions (and why)

1. **Go, shelling out via `os/exec`.** The tool links nothing native. This is
   precisely why cross-compilation stays trivial — a CGo binding to whisper.cpp
   would reintroduce a per-platform toolchain. Go's strengths (single static
   binary, easy cross-compile, solid `os/exec`) line up exactly with an
   orchestrator's needs.

2. **whisper.cpp (`whisper-cli`) as the transcription engine.** It is the one
   engine where "macOS primary, Linux bonus" costs nothing: the *same* model
   weights run on Metal (macOS) and CUDA/CPU (Linux), so transcript accuracy is
   identical across platforms and only the runtime backend differs. mlx-whisper
   is ~30–40% faster on Apple Silicon but is macOS-only and drags in a Python
   stack — not worth fragmenting the codebase, especially since the real
   pipeline bottleneck is the LLM summary stage, not transcription.

3. **Default model: `large-v3-turbo`.** Near-`large-v3` accuracy (within ~1–2%
   WER) at a fraction of the runtime. Overridable via `--model`.

4. **16 kHz mono PCM is mandatory before transcription.** whisper.cpp expects
   16 kHz mono; skipping the resample silently degrades accuracy. ffmpeg owns
   this step regardless of input source.

5. **JSON transcript output, not stdout scraping.** Run `whisper-cli` with JSON
   output so segments/timestamps are parsed from structured data.

6. **Detect-and-instruct for dependencies, never auto-install.** Invoking a
   package manager on the user's behalf is invasive and breaks across
   environments. The model file is the only artifact the tool fetches itself.

---

## 4. Tech Stack

- **Language:** Go (latest stable; pin in `go.mod`).
- **CLI framework:** `spf13/cobra` (commands + flags) with `spf13/viper`
  optional for config/env binding.
- **External binaries (runtime deps):** `yt-dlp`, `ffmpeg`, `whisper-cli`.
- **HTTP:** stdlib `net/http` for model download and remote LLM calls.

### Build / install matrix for external deps (shown to user on missing dep)
| Dependency   | macOS (brew)              | Arch / CachyOS (yay/pacman)        |
|--------------|---------------------------|------------------------------------|
| yt-dlp       | `brew install yt-dlp`     | `yay -S yt-dlp`                    |
| ffmpeg       | `brew install ffmpeg`     | `sudo pacman -S ffmpeg`           |
| whisper-cli  | `brew install whisper-cpp`| `yay -S whisper.cpp` (AUR)        |

> Note: the `whisper-cli` binary name was renamed from `main` in recent
> whisper.cpp releases. Detect `whisper-cli`; optionally fall back to `main`
> with a deprecation warning.

---

## 5. Application Directory: `~/.vid-summary-cli/`

- All persistent state lives under `~/.vid-summary-cli/`.
- **Create the directory (and subdirs) if missing**, with `0700` perms.
- Resolve `$HOME` via `os.UserHomeDir()`. Allow override via
  `VID_SUMMARY_CLI_HOME` env var for testing.

Layout:
```
~/.vid-summary-cli/
├── config.yaml            # user config (see §10)
├── models/                # downloaded whisper.cpp GGML models
│   └── ggml-large-v3-turbo.bin
└── tmp/                   # scratch: downloaded audio, resampled wav (cleaned up)
```

---

## 6. Model Management

- Models are whisper.cpp GGML `.bin` files hosted on Hugging Face under
  `ggerganov/whisper.cpp`.
- Download URL pattern:
  `https://huggingface.co/ggerganov/whisper.cpp/resolve/main/ggml-<model>.bin`
  (e.g. `ggml-large-v3-turbo.bin`). Support quantized variants too
  (e.g. `ggml-large-v3-turbo-q5_0.bin`) via the `--model` value.
- Flow on each run:
  1. Resolve requested model (`--model`, else config default, else `large-v3-turbo`).
  2. Check `~/.vid-summary-cli/models/ggml-<model>.bin`.
  3. If present → verify (size, and SHA256 if a known-good hash is available).
  4. If missing → download with a progress bar, write atomically (download to
     `*.partial`, then rename), then verify.
- `vid-summary-cli models` subcommand: `list` (local + known-available), `download <model>`,
  `rm <model>`.
- Never block transcription to re-verify a model that already passed; cache the
  verification result if cheap to do so.

---

## 7. Pipeline

Input can be a **URL** or a **local file path**. The first stage differs; the
rest is identical.

```
URL        → yt-dlp (bestaudio) ─┐
                                 ├→ ffmpeg (16kHz mono PCM) → whisper-cli (JSON) → parse → summarize
local file ──────────────────────┘
```

### Stage 1a — Acquire (URL input)
- `yt-dlp -f bestaudio -o <tmp>/source.%(ext)s <url>`
- Capture the output path. Do not rely on yt-dlp's own ffmpeg post-processing
  for the resample; we do the canonical resample ourselves in Stage 2 to
  guarantee whisper-compatible audio regardless of source codec.

### Stage 1b — Local file input
- Skip yt-dlp entirely. Pass the local file straight to Stage 2.

### Stage 2 — Normalize audio (ffmpeg)
- `ffmpeg -i <input> -ar 16000 -ac 1 -c:a pcm_s16le -y <tmp>/audio.wav`
- 16 kHz, mono, signed 16-bit PCM WAV. Non-negotiable for whisper.cpp.

### Stage 3 — Transcribe (whisper-cli)
- `whisper-cli -m <model.bin> -f <tmp>/audio.wav -oj -of <tmp>/transcript [-l auto] [-t <threads>]`
- Use JSON output (`-oj`, or `-ojf` for full/verbose JSON) — parse segments and
  timestamps from the JSON file, never from stdout.
- Expose `--language` (default `auto`) and `--threads` flags.
- whisper.cpp chunks long audio internally, so transcription scales to long
  videos without our intervention.

### Stage 4 — Summarize (LLM)
- See §8. Consumes the parsed transcript, returns the final summary.

### Cleanup
- Remove `~/.vid-summary-cli/tmp/` artifacts on success. Provide `--keep-temp` to
  retain them for debugging.

---

## 8. Summarization (pluggable LLM backend)

This is the only stage with no external CLI dependency to lean on, and the only
part that is genuinely *designed* rather than orchestrated.

### Interface
Define a Go interface, e.g.:
```go
type Summarizer interface {
    Summarize(ctx context.Context, transcript string, opts SummarizeOpts) (string, error)
}
```
Implementations for v1:
- **Local endpoint** — Ollama / LM Studio OpenAI-compatible HTTP API
  (default: `http://localhost:11434` or `http://localhost:1234/v1`).
- **Remote HTTP API** — generic OpenAI-compatible / Anthropic API. API key from
  env (`VID_SUMMARY_CLI_API_KEY`), never from a flag (avoid shell history leakage).

### Config-driven
Backend selection, endpoint, model name, and the **prompt template** all come
from `config.yaml` (overridable by flags). Ship a sensible default summary
prompt; let users customize tone/length/format.

### Long videos → map-reduce summarization (build in from v1)
whisper.cpp transcribes long audio fine; the LLM context window is the limiter.
Implement:
1. **Chunk** the transcript into windows that fit the target model's context
   (respect segment boundaries; configurable token/char budget with overlap).
2. **Map** — summarize each chunk independently.
3. **Reduce** — summarize the concatenated chunk-summaries into the final output.
Recurse the reduce step if the combined summaries still exceed context.
For short transcripts, skip straight to a single-pass summary.

---

## 9. CLI Interface

```
vid-summary-cli <url|file> [flags]

Flags:
  --model string        Whisper model (default "large-v3-turbo")
  --language string     Source language or "auto" (default "auto")
  --threads int         whisper-cli threads (default: NumCPU)
  --backend string      Summarizer backend: "local" | "api" (default from config)
  --llm-model string    LLM model name for summarization
  --endpoint string     LLM endpoint URL
  --prompt string       Path to a custom prompt template
  --output string       Write summary to file (default: stdout)
  --format string       "summary" | "summary+transcript" | "json"
  --keep-temp           Do not delete temp artifacts
  -v, --verbose         Verbose logging

Subcommands:
  vid-summary-cli models list
  vid-summary-cli models download <model>
  vid-summary-cli models rm <model>
  vid-summary-cli doctor        # check all external deps + config, print status
  vid-summary-cli version
```

`doctor` runs the same dependency checks as a normal run and reports each
dep's presence/version plus the resolved config and model cache state — useful
for support.

---

## 10. Configuration (`~/.vid-summary-cli/config.yaml`)

```yaml
model: large-v3-turbo
language: auto
threads: 0            # 0 = NumCPU

summarizer:
  backend: local      # local | api
  endpoint: http://localhost:11434
  model: llama3.1:8b
  prompt: |
    Summarize the following video transcript into a concise overview.
    Capture the main topics, key points, and conclusions.
  # api_key sourced from env VID_SUMMARY_CLI_API_KEY, never stored here

chunking:
  max_chars: 12000    # per map chunk
  overlap_chars: 500
```

Precedence: flags > env > `config.yaml` > built-in defaults.
Generate a default config on first run if absent.

---

## 11. Dependency Detection

- On startup (and in `doctor`), `exec.LookPath` each of `yt-dlp`, `ffmpeg`,
  `whisper-cli`.
- Only require `yt-dlp` when the input is a URL; a local-file run with no
  network input should not fail for a missing `yt-dlp`.
- On a missing dep, print the exact install command for the detected OS
  (see §4 table) and exit non-zero. Do not attempt installation.
- Optionally capture `--version` of each found binary for `doctor` output.

---

## 12. Project Structure

```
vid-summary-cli/
├── go.mod
├── go.sum
├── PLAN.md
├── README.md
├── cmd/
│   └── vid-summary-cli/         # dir name = invoked binary name "vid-summary-cli"
│       └── main.go          # cobra root + command wiring
└── internal/
    ├── deps/                # LookPath checks, OS-aware install hints, doctor
    ├── config/              # load/merge config, defaults, ~/.vid-summary-cli bootstrap
    ├── model/              # resolve/download/verify/cache GGML models
    ├── extract/            # yt-dlp + ffmpeg wrappers (acquire + normalize)
    ├── transcribe/         # whisper-cli wrapper + JSON parsing into segments
    ├── summarize/          # Summarizer interface + local/api impls + map-reduce
    └── pipeline/           # orchestration: wires stages, temp lifecycle, errors
```

---

## 13. Execution Model (v1: sequential)

- v1 runs stages **sequentially**: acquire → normalize → transcribe →
  summarize. The wall-clock cost of sequential execution is small for a single
  video, and it is far simpler to get correct.
- Defer concurrency (e.g. transcribing chunk N while downloading chunk N+1, or
  batch processing multiple inputs) to a later version, added only if a batch
  mode is introduced. Design the pipeline package so this is an additive change,
  not a rewrite.

---

## 14. Error Handling & UX

- Wrap external-process failures with context: which binary, exit code, and the
  tail of stderr.
- Stream progress for the long stages (download %, transcription progress if
  parseable from whisper-cli).
- Use a temp dir per run under `~/.vid-summary-cli/tmp/`; always clean up unless
  `--keep-temp`. Guard against partial files via atomic rename.
- Validate the LLM endpoint is reachable before transcribing when backend is
  remote/local-server, so a long transcription isn't wasted on an
  unreachable summarizer (fail fast, but make this check cheap/optional).

---

## 15. Build & Distribution

- Pure Go ⇒ cross-compile with no extra toolchain (release artifacts carry the
  repo name; the binary inside is invoked as `vid-summary-cli`):
  ```
  GOOS=darwin  GOARCH=arm64 go build -o dist/vid-summary-cli-darwin-arm64 ./cmd/vid-summary-cli
  GOOS=linux   GOARCH=amd64 go build -o dist/vid-summary-cli-linux-amd64  ./cmd/vid-summary-cli
  GOOS=linux   GOARCH=arm64 go build -o dist/vid-summary-cli-linux-arm64  ./cmd/vid-summary-cli
  ```
- Provide a `Makefile` and (optionally) `goreleaser` config for tagged releases.
- README must document the external dependency installs up front (the §4 table).

---

## 16. Open Decisions for the Implementer

1. **Model verification strictness** — ship a small map of known model
   filenames → SHA256, or verify size-only? (Hashes are safer; require keeping
   them current.)
2. **Default local LLM** — pick the default `summarizer.model` to match a
   common Ollama install, or leave blank and force the user to set it.
3. **Subtitle reuse** — if a URL already has good closed captions, optionally
   let `yt-dlp` pull existing subtitles and skip transcription entirely
   (`--prefer-subs`). Potential v1.x feature; not required for v1.
4. **Output of timestamps** — whether `--format json` should include per-segment
   timestamps from the whisper JSON alongside the summary.

---

## 17. Acceptance Criteria (v1)

- `vid-summary-cli <youtube-url>` produces a coherent summary on macOS arm64 and
  Linux amd64 from the same source, using the same default model.
- `vid-summary-cli ./local.mp4` works with no network and without `yt-dlp` present.
- First run with no model present downloads it to `~/.vid-summary-cli/models/` and
  succeeds.
- A multi-hour video summarizes successfully via map-reduce without exceeding
  the LLM context window.
- Missing `ffmpeg`/`whisper-cli` yields a clear, OS-correct install instruction
  and a non-zero exit.
- `vid-summary-cli doctor` accurately reports dependency and config state.
