# Silo

Silo is a small local CLI that projects knowledge from **Engram** into a human-editable **Obsidian** vault.

Core pipeline:

```text
Engram → Raw → Curated → Profile → Outputs
```

Plus a parallel **capture → seed → triage** loop:

```text
silo save → Engram (Memory) → AI seed proposal → Inbox/open/ → human triage
```

Engram remains the source of truth. Silo creates deterministic Markdown projections so humans can read, curate, and reuse that knowledge without introducing another database.

Philosophy: **Memory is sacred. Synthesis is cheap. Identity is earned.**
The seed inbox is the synthesis layer's editorial gate: AI proposes, the human disposes. Nothing flows up to Curated without human approval.

## What Silo is

- A local Markdown projection layer over Engram.
- A bridge from machine memory to human-readable Obsidian notes.
- A deterministic generator for:
  - raw observation notes
  - curated seed notes
  - identity/profile notes
  - professional output seeds: CV, LinkedIn, Professional Bio

## What Silo is not

- Not a replacement for Engram.
- Not a second memory store.
- Not an LLM writer.
- Not a SQLite app.
- Not an embeddings/search engine.
- Not a TUI product yet.
- Not a cloud or multi-user system.

## Architecture

```text
Engram
  ↓ silo sync
Raw/Observations/          100% automatic source projection
  ↓ silo curate
Curated/                   human-editable synthesis layer
  ↓ silo profile
Identity.md / Skills.md / Projects.md / Outputs.md
  ↓ silo outputs
Outputs/CV.md / LinkedIn.md / ProfessionalBio.md
```

### Layer responsibilities

- **Engram**: persistent memory and source of truth.
- **Raw**: deterministic Markdown files generated from Engram observations. Safe to regenerate.
- **Curated**: human-editable notes seeded from Raw/Engram. Silo never overwrites existing Curated notes.
- **Profile**: identity notes derived from Curated when useful, otherwise Raw/Engram fallback.
- **Outputs**: professional artifacts derived from the same identity model. Silo never overwrites existing output files.

See [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md) for more detail.

## Install / build

```bash
go build -o silo ./cmd/silo
```

Or run directly during development:

```bash
go run ./cmd/silo <command>
```

## Commands

### `silo init`

Creates `./silo.config.json` if it does not exist.

```bash
go run ./cmd/silo init
```

Idempotent: if the config already exists, it is left unchanged.

### `silo sync`

Reads observations from Engram and writes automatic Raw notes:

```text
vault/Raw/Observations/*.md
```

```bash
go run ./cmd/silo sync
```

Supports project override:

```bash
go run ./cmd/silo sync --project silo2
```

### `silo curate`

Reads observations and creates seed notes under:

```text
vault/Curated/
```

```bash
go run ./cmd/silo curate
```

Existing Curated notes are **never overwritten**.

### `silo profile`

Builds identity/profile notes:

```text
vault/Identity.md
vault/Skills.md
vault/Projects.md
vault/Outputs.md
```

```bash
go run ./cmd/silo profile
```

Source selection:

1. Use Curated notes if they contain useful human-written content.
2. Otherwise fall back to Raw/Engram observations.

The CLI prints which source was used:

```text
source: curated (2 notes)
```

or:

```text
source: raw/engram fallback (8 observations)
```

### `silo outputs`

Creates professional output seed files:

```text
vault/Outputs/CV.md
vault/Outputs/LinkedIn.md
vault/Outputs/ProfessionalBio.md
```

```bash
go run ./cmd/silo outputs
```

Existing files under `vault/Outputs/` are **never overwritten**.

### `silo save`

Captures a text observation and proposes a Seed under `vault/Inbox/open/`.

```bash
go run ./cmd/silo save "MVVM-C navigation insight"
go run ./cmd/silo save "MVVM-C navigation insight" --why "I keep forgetting this pattern"
```

What happens:

1. **Synchronous**: the observation is persisted to Memory (the configured Engram backend) and the CLI prints `Saved. Seed pending.` immediately.
2. **Best-effort**: Silo synthesizes only `Proposed Summary`, `Suggested Themes`, and `Why It Might Matter`, then writes `vault/Inbox/open/seed-<id>.md`.
3. **Fallback-safe**: if AI is disabled, misconfigured, times out, rate-limits, returns bad JSON, or otherwise fails, Silo falls back to a deterministic proposal and still finishes the save.

Flag order is flexible: `--why` and `--project` may appear before or after the text.

**Capture metadata vs content**: `--why` is stored as `Observation.Why` (capture metadata), never merged into the observation content. The seed renders it under a dedicated `## Capture Why` section attributed to the human, separate from any AI-authored sections.

**Human-owned boundary**: AI may propose text only for `Proposed Summary`, `Suggested Themes`, and `Why It Might Matter`. Seed ID, filename, title, source observation IDs, legacy path, frontmatter status, and `Human Notes` remain deterministic and human-owned.

### `silo import-wiki`

Imports a legacy Markdown wiki into reviewable Inbox seeds without mutating the source vault.

```bash
go run ./cmd/silo import-wiki ./path/to/wiki
go run ./cmd/silo import-wiki ./path/to/wiki --limit 10 --include-readme
```

What happens:

1. Each legacy Markdown file is read from the source wiki only.
2. Its content is persisted to Memory first.
3. Silo synthesizes proposal-only AI fields best-effort, then writes `vault/Inbox/open/seed-<id>.md` with `WriteNoteIfAbsent`.
4. Seed identity stays deterministic from `relPath + content`, so re-imports skip existing files instead of overwriting them.

If AI is disabled or fails, import continues with deterministic fallback text. `LegacyPath` remains derived from the source relative path only.

### `silo inbox`

Lists seed counts by status and the filenames currently in `vault/Inbox/open/`.

```bash
go run ./cmd/silo inbox
```

Example output:

```text
Inbox: ./vault/Inbox

open       3
deferred   1
discarded  0
approved   2

Open seeds:
  seed-1f2a4b8c.md
  seed-233e177f.md
  seed-a91d33e0.md
```

Status lives in each seed's frontmatter (`status: open | deferred | discarded | approved`). To triage a seed, open it in Obsidian and edit the field, or move the file to `vault/Inbox/archive/` once you are done thinking about it. Promotion to `Curated/` is also a human act — the system never moves seeds for you.

## Configuration

Config file: `./silo.config.json`

```json
{
  "vault_path": "./vault",
  "project": "silo2",
  "engram_endpoint": "http://127.0.0.1:7437",
  "engram_api_key": "",
  "identity_name": "Nicolas Peralta"
}
```

Fields:

- `vault_path`: Obsidian vault path. Defaults to `./vault`.
- `project`: Engram project to read from. If missing or empty, Silo temporarily falls back to `"silo2"` for local development.
- `engram_endpoint`: Engram HTTP endpoint. If empty, Silo uses the built-in mock client.
- `engram_api_key`: optional bearer token for Engram HTTP requests.
- `identity_name`: optional override for the generated identity name.
- `llm_provider`: optional AI provider. Empty disables AI and uses deterministic fallback. Current supported values: `openai`, `opencode`.
- `llm_model`: optional provider model override. Empty uses the provider default.
- `llm_api_key`: optional provider API key. If missing for a configured provider, Silo falls back deterministically.
- `llm_timeout_seconds`: optional synthesis timeout in seconds. Defaults to 5. Use higher values (e.g. 30) for slow providers that involve subprocesses.

Example AI-enabled config:

```json
{
  "vault_path": "./vault",
  "project": "silo2",
  "llm_provider": "openai",
  "llm_model": "gpt-4.1-mini",
  "llm_api_key": "<your-key>"
}
```

Fallback rules:

- Empty provider: AI disabled, deterministic fallback only.
- Invalid provider: deterministic fallback.
- Missing key: deterministic fallback.
- Timeout, rate limit, provider outage, or bad JSON: deterministic fallback.
- No AI failure is allowed to break `silo save` or `silo import-wiki` after Memory persistence succeeds.

### Optional: OpenCode synthesis provider (experimental)

Silo can delegate proposal synthesis to a locally installed OpenCode CLI, which manages provider auth itself. Set `"llm_provider": "opencode"` and `"llm_model": "anthropic/claude-haiku-4-5"` in `silo.config.json`.

Requirements: `opencode` binary on PATH and at least one provider authenticated via `opencode providers login`. On any failure (missing binary, no auth, bad response), Silo falls back to deterministic proposal text.

Project precedence:

```text
--project flag > config.project > "silo2" dev fallback
```

## Usage with mock data

If `engram_endpoint` is empty or missing, Silo uses a deterministic in-memory mock client.

Example config:

```json
{
  "vault_path": "./vault"
}
```

Run:

```bash
go run ./cmd/silo init
go run ./cmd/silo sync
go run ./cmd/silo curate
go run ./cmd/silo profile
go run ./cmd/silo outputs
```

This is useful for local smoke tests and development without Engram running.

## Usage with real Engram

Start Engram HTTP server:

```bash
engram serve
```

By default it listens on:

```text
http://127.0.0.1:7437
```

Configure Silo:

```json
{
  "vault_path": "./vault",
  "project": "silo2",
  "engram_endpoint": "http://127.0.0.1:7437"
}
```

Run:

```bash
go run ./cmd/silo sync --project silo2
go run ./cmd/silo curate --project silo2
go run ./cmd/silo profile --project silo2
go run ./cmd/silo outputs --project silo2
```

## Vault structure

After running the full flow:

```text
vault/
├── Raw/
│   └── Observations/
│       ├── <observation>.md
│       └── README.md              # only when no observations exist
├── Curated/
│   ├── Architecture/
│   ├── Projects/
│   ├── Identity/
│   └── Career/
├── Inbox/
│   ├── open/                      # fresh seeds awaiting human triage
│   │   └── seed-<id>.md
│   └── archive/                   # seeds the human is done with
├── Outputs/
│   ├── CV.md
│   ├── LinkedIn.md
│   └── ProfessionalBio.md
├── Identity.md
├── Skills.md
├── Projects.md
└── Outputs.md
```

## No-overwrite policy

Silo treats generated layers differently:

- `Raw/Observations/`: automatic projection. Safe to regenerate.
- `Identity.md`, `Skills.md`, `Projects.md`, `Outputs.md`: generated profile files. Rewritten by `silo profile`.
- `Curated/`: human-editable. `silo curate` only creates missing files and skips existing ones.
- `Outputs/`: human-editable professional artifacts. `silo outputs` only creates missing files and skips existing ones.

If you want to regenerate a Curated or Outputs seed, delete that specific file and run the command again.

## Idempotence

Silo avoids dynamic timestamps and random ordering in generated Markdown.

Expected behavior:

- Running `silo sync` twice with unchanged Engram data produces byte-stable Raw notes.
- Running `silo curate` twice creates files on the first run and skips them on later runs.
- Running `silo outputs` twice creates files on the first run and skips them on later runs.
- Human edits in `Curated/` and `Outputs/` survive later Silo runs.

## Current limitations

- LLM generation is optional and proposal-only; deterministic fallback remains the default safety path.
- `silo save` works against both the mock backend and the real Engram HTTP backend. With Engram, each capture upserts a long-lived `silo-save-{project}` session and POSTs the observation. Engram v1.15.13 has no `why` column in its schema and silently drops the field; Silo forwards it in the payload anyway for forward-compatibility, and the durable record of `--why` lives in the seed file's `## Capture Why` section.
- In mock mode (no `engram_endpoint`), the Memory layer is in-memory per-process: each `silo save` invocation starts a fresh mock store. Seeds persist on disk. Useful for offline iteration on the triage loop.
- No SQLite or local database.
- No embeddings or semantic search.
- No TUI for the inbox yet; triage happens by editing seed frontmatter or moving files in Obsidian.
- No cloud sync.
- No multi-user support.
- No orphan sweep/delete yet; Silo does not remove files when Engram observations disappear.
- Curated notes are not automatically merged when new related observations arrive. Delete a curated seed if you want to regenerate it.
- Professional outputs are seed files, not final polished documents.

## Development validation

```bash
go test ./...
go vet ./...
go run ./cmd/silo init
go run ./cmd/silo sync
go run ./cmd/silo curate
go run ./cmd/silo profile
go run ./cmd/silo outputs
```

## More docs

- [`QUICKSTART.md`](QUICKSTART.md)
- [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md)
- [`docs/ROADMAP.md`](docs/ROADMAP.md)
