# Silo Quickstart

This guide gets you from zero to a usable local Obsidian vault.

## Option A — run without Engram (mock mode)

Use this when you just want to test the MVP.

```bash
go run ./cmd/silo init
```

Keep `silo.config.json` minimal:

```json
{
  "vault_path": "./vault"
}
```

Run the full flow:

```bash
go run ./cmd/silo sync
go run ./cmd/silo curate
go run ./cmd/silo profile
go run ./cmd/silo outputs
```

Open `./vault` in Obsidian.

You should see:

```text
Raw/Observations/
Curated/
Identity.md
Skills.md
Projects.md
Outputs.md
Outputs/CV.md
Outputs/LinkedIn.md
Outputs/ProfessionalBio.md
```

## Option B — run with real Engram

Start Engram:

```bash
engram serve
```

Default endpoint:

```text
http://127.0.0.1:7437
```

Create or update `silo.config.json`:

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

If your Engram project has no observations yet, Raw will contain a README placeholder and the rest of the flow still completes.

## Open the vault in Obsidian

1. Open Obsidian.
2. Choose **Open folder as vault**.
3. Select the configured `vault_path`, for example `./vault`.
4. Start from `Identity.md`, then browse:
   - `Raw/Observations/`
   - `Curated/`
   - `Outputs/`

## Edit a Curated note

Curated notes are the human synthesis layer. Silo seeds them, then leaves them alone.

Example:

```text
vault/Curated/Identity/profile.md
```

Write real content in `Summary` or `Notes`:

```markdown
# Profile

## Summary

Nicolas is a software architect focused on developer tooling,
knowledge management, Go, SwiftUI, Engram, and Obsidian.

## Notes

- Ships local-first developer tools.
- Uses Silo to project Engram memory into Markdown.
```

Then regenerate the profile and outputs:

```bash
go run ./cmd/silo profile
go run ./cmd/silo outputs
```

`profile` should now print something like:

```text
source: curated (1 note)
```

If Curated only contains TODO placeholders, profile falls back to Raw/Engram:

```text
source: raw/engram fallback (N observations)
```

## Regenerate profile and outputs

Profile files are generated files and may be overwritten:

```bash
go run ./cmd/silo profile
```

This rewrites:

```text
Identity.md
Skills.md
Projects.md
Outputs.md
```

Professional output files are human-editable seeds and are not overwritten:

```bash
go run ./cmd/silo outputs
```

If `Outputs/CV.md` already exists, Silo prints:

```text
skipped  ./vault/Outputs/CV.md (already exists)
```

To regenerate one output seed, delete that file and run `silo outputs` again.

## Common smoke test

```bash
go test ./...
go vet ./...
rm -rf vault
go run ./cmd/silo init
go run ./cmd/silo sync
go run ./cmd/silo curate
go run ./cmd/silo profile
go run ./cmd/silo outputs
find vault -type f | sort
```
