# Proposal: Apple Calendar Integration

## Intent

Silo2 today owns time via `schedule.json`, duplicating the user's real Apple Calendar. The duplicate cannot sync, traps data locally, and forces Silo to behave as a scheduling app instead of a context/criterion engine. Additionally, `silo_recommend` (MCP) currently bypasses `internal/recommend.Engine` (stub path), so recommendations are not deterministic or reproducible — a blocker for a multi-user product. This change retires Silo's time ownership and delegates calendar I/O to an external Apple Calendar MCP, with the agent as the brain orchestrating both MCPs.

## Scope

### In Scope
- DELETE `internal/schedule/` package (store, model, resolver, tests, ~370 LOC).
- DELETE 5 MCP tools: `add_schedule_event`, `remove_schedule_event`, `list_schedule_events`, `get_free_slots`, `preview_schedule`.
- REFACTOR `silo_recommend`: accept `free_slots[]` (each `{start, end}` ISO 8601), wire to real `internal/recommend.Engine.Recommend()`.
- REMOVE `SchedulePath` + `DefaultSchedulePath()` from `internal/config`; KEEP `ProductiveHours` as engine hint.
- REFACTOR `cmd/silo/main.go:runRecommend()`: drop schedule.json read.
- REWRITE skill `~/.config/opencode/skills/silo-guide/SKILL.md` (orchestration: profile + free slots → recommend).
- CREATE skill `~/.config/opencode/skills/silo-calendar-guard/SKILL.md` (containment: write only to "Silo" calendar, confirmation required).
- DOCUMENT manual schedule.json → Apple Calendar migration in AGENTS.md / README.

### Out of Scope
- Implementing the external `FradSer/mcp-server-apple-events` MCP (we build + pin its version, we do not author it).
- Building a CLI migration tool (manual workflow documented only).
- Building a UI for calendar management.
- Defining whether Silo ships as a harness vs skill set (follow-up).
- Splitting into multiple calendars (e.g. "Silo" + "Silo-Sugerencias"); single "Silo" calendar only.

## Capabilities

### New Capabilities
- `agent-calendar-orchestration`: Defines the agent-as-brain model — agent orchestrates Silo MCP (context/recommend) and Apple Calendar MCP (read/write), with containment-by-convention writing only to the "Silo" calendar.

### Modified Capabilities
- `mcp-recommend`: `silo_recommend` tool signature gains required `free_slots[]` input; handler wires to real `internal/recommend.Engine.Recommend()` instead of stub renderer.
- `mcp-schedule`: REMOVED entirely (5 tools deleted); `internal/schedule/` package retired.
- `config`: REMOVED `SchedulePath` / `DefaultSchedulePath()`; `ProductiveHours` retained.

## Approach

**Agent-as-brain, two-sibling-MCP model.** The agent (opencode / Claude Desktop / Cursor) orchestrates two peer MCPs: (1) `silo` exposes context + deterministic `silo_recommend`, (2) `FradSer/mcp-server-apple-events` (built locally, pinned, no `npx -y`) exposes EventKit read/write. The agent reads free time from Calendar MCP, passes `free_slots[]` to `silo_recommend`, and presents results. Writes go to a single dedicated "Silo" calendar (user-created manually).

**Containment by convention.** EventKit grants OS-level read/write to ALL calendars; OS sandbox cannot scope it. We enforce "Silo-only writes" via the `silo-calendar-guard` skill: explicit user confirmation required for every create/delete/modify, refuse writes to non-Silo calendars, ask when in doubt. Read access to all calendars stays open for context.

**Two-skill separation.** `silo-guide` (orchestration: what to do) and `silo-calendar-guard` (containment: how to write safely) are decoupled so the guard's safety rules apply uniformly regardless of which workflow triggered a write.

**Deterministic recommend.** `silo_recommend` becomes pure: same `(profile, seeds, free_slots)` input → same output via `internal/recommend.Engine`. This makes Silo viable as a multi-client product.

## Affected Areas

| Area | Impact | Description |
|------|--------|-------------|
| `internal/schedule/` | Removed | Entire package + tests deleted (~370 LOC). |
| `internal/config/config.go` | Modified | Remove `SchedulePath`, `DefaultSchedulePath()`; keep `ProductiveHours`. |
| `internal/mcp/handlers_schedule.go` (+ test) | Removed | 5 tool handlers deleted. |
| `internal/mcp/server.go` | Modified | Remove 5 tool registrations. |
| `internal/mcp/handlers_recommend.go` | Modified | Add `free_slots[]` param; wire to `Engine.Recommend()`. |
| `internal/recommend/` | Unchanged | Engine, renderer, model, tests preserved. |
| `cmd/silo/main.go` | Modified | `runRecommend()` no longer reads schedule.json. |
| `~/.config/opencode/skills/silo-guide/SKILL.md` | Modified | Rewrite for two-MCP orchestration. |
| `~/.config/opencode/skills/silo-calendar-guard/SKILL.md` | New | Containment + confirmation skill. |
| `~/.config/silo2/schedule.json` | Removed | User migrates manually per documented workflow. |
| `AGENTS.md` / `README.md` | Modified | Document migration + agent-as-brain architecture. |

## Risks

| Risk | Likelihood | Mitigation |
|------|------------|------------|
| Supply chain (external MCP via npx) | Med | Build `FradSer/mcp-server-apple-events` locally; pin version; no `npx -y`. |
| Write blast radius (EventKit gives access to all calendars) | High | Dedicated "Silo" calendar + `silo-calendar-guard` skill rules + per-write user confirmation. Accepted tradeoff. |
| Recommend non-determinism | Med | Wire MCP handler to real `internal/recommend.Engine`; existing engine tests cover scoring. |
| schedule.json data loss for existing users | Low | Document manual export-to-calendar workflow; do not auto-delete `schedule.json`. |
| Skill ↔ code coupling (skills live outside repo) | Med | Ship both skill files in the same PR delivery; reference exact commit in skill metadata. |
| 600-line PR budget overrun | Med | Bulk of change is deletion (low review cost); refactor is localized. Monitor in `sdd-tasks`. |

## Rollback Plan

1. Revert the silo2 PR (single commit / squashed PR makes this clean).
2. Restore previous versions of `~/.config/opencode/skills/silo-guide/SKILL.md` from git history.
3. Remove `~/.config/opencode/skills/silo-calendar-guard/SKILL.md`.
4. Users who already migrated to Apple Calendar can keep events in "Silo" calendar (no data destruction occurs from rollback); reverting Silo just restores `schedule.json` as the active source.
5. Apple Calendar MCP can be uninstalled independently — Silo MCP works without it after revert.

## Dependencies

- External MCP: `FradSer/mcp-server-apple-events` (built locally, pinned version). Required at runtime, not at build time of silo2.
- User-created "Silo" calendar in Apple Calendar (manual prerequisite, documented).
- `internal/recommend.Engine.Recommend()` (already exists, unchanged).

## Success Criteria

- [ ] `internal/schedule/` package removed; `go build ./...` and `go test ./...` pass.
- [ ] `silo_recommend` MCP tool accepts `free_slots[]` and routes through `internal/recommend.Engine.Recommend()`; output is deterministic for identical input.
- [ ] All 5 schedule MCP tools removed from server registration; no references remain in `internal/mcp/server.go`.
- [ ] `silo-guide` skill rewritten and calls only `silo` MCP for context/recommend, only Apple Calendar MCP for time data.
- [ ] `silo-calendar-guard` skill present and enforces "Silo-only writes + explicit confirmation".
- [ ] Manual migration workflow documented in repo (AGENTS.md or README).
- [ ] PR delivered as single PR within 600-line budget (additions + deletions).

## Follow-ups (not in this change)

- Harness vs skills decision for Silo (currently the user's harness repo holds only skills).
- Re-evaluate splitting into two calendars ("Silo" + "Silo-Sugerencias") if user feedback demands separation between confirmed work and proposed suggestions.
