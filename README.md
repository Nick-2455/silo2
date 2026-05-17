# Silo

Silo is a small local CLI that projects knowledge from **Engram** into a human-editable **Obsidian** vault.

Core pipeline:

```text
Engram → Raw → Curated → Profile → Outputs
```

Engram remains the source of truth. Silo creates deterministic Markdown projections so humans can read, curate, and reuse that knowledge without introducing another database.

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

- No LLM generation yet. All output is deterministic template rendering.
- No SQLite or local database.
- No embeddings or semantic search.
- No TUI.
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
