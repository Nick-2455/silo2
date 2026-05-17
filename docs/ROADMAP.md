# Silo Roadmap

This roadmap separates what is usable now from what should wait. The MVP is intentionally small.

## Now

Already in the MVP:

- Local Go CLI.
- JSON config at `./silo.config.json`.
- Mock Engram client for offline development.
- Real Engram HTTP adapter for `engram serve`.
- Project selection via `--project`, `config.project`, or temporary `"silo2"` fallback.
- `silo sync` writes deterministic Raw observation notes.
- `silo curate` seeds human-editable Curated notes and never overwrites them.
- `silo profile` builds Identity/Skills/Projects/Outputs from Curated when useful, with Raw/Engram fallback.
- `silo outputs` creates CV, LinkedIn, and Professional Bio seed files and never overwrites them.
- Byte-stable generation for unchanged inputs.

## Next

Likely near-term improvements:

- Better identity heuristics without LLMs.
- More configurable output templates.
- Optional command to print source diagnostics: why Curated was or was not considered useful.
- More tests around mixed Curated + Raw scenarios.
- Documentation examples using a larger real Engram project.
- Safer regeneration UX for Curated/Outputs, for example `--force <file>` or explicit delete instructions.
- Improve handling of stale Curated links when new related observations arrive.

## Later

Possible future work, after the MVP stays stable:

- LLM-assisted drafting from Curated notes, behind explicit commands.
- Template customization per user/project.
- Better project inference from working directory.
- Optional orphan reporting: list files whose source observations no longer exist.
- Import/export helpers for sharing vault templates.
- Small TUI or interactive review mode, only if the CLI workflow proves insufficient.

## Not Yet

Explicitly out of scope for the current MVP:

- LLM generation as the default path.
- SQLite or another local app database.
- Embeddings.
- Semantic search.
- Cloud sync.
- Multi-user collaboration.
- Large TUI product.
- Graph database.
- Automatic deletion/sweep of orphan files.
- Background daemon.

The product should stay boring until the projection workflow is obviously valuable.
