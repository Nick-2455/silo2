# AI seed synthesis review order

Review this chain in order.

1. `feature/synthesis-base` — synthesis contract, fallback, config surface.
2. `feature/synthesis-openai` — OpenAI provider adapter only.
3. `feature/synthesis-wiring` — command wiring, invariant protection tests, docs.

## What this PR adds

- Wires configured synthesis into `silo save` and `silo import-wiki`.
- Persists Memory first, then synthesizes best-effort with deterministic fallback.
- Protects human-owned and deterministic fields with tests.
- Documents optional AI config and the fallback boundary.

## Out of scope

- Auto-curation or approval flows.
- Embeddings, vector DBs, or background jobs.
- Any write path into Curated, Identity, Outputs, or Human Notes.
