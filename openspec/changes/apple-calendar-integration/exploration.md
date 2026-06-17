# Exploration: Apple Calendar Bidirectional Integration

**Change name**: apple-calendar-integration
**Date**: 2026-06-16
**Status**: complete

---

## Current State

Silo's scheduling system is a fully self-contained local subsystem built around `schedule.json`. The data flow today:

```
schedule.json  →  Store.Load()  →  ResolveDay() / FreeSlots()  →  MCP handlers  →  agent
                                                                  ↓
                                                          handleSiloRecommend()  (v1: ignores schedule)
```

Critically, the **MCP `silo_recommend` tool does NOT currently read the schedule or use `freeMinutes`** at all. The full recommend engine (`internal/recommend/engine.go`) receives `freeMinutes int` and scores against it, but `handlers_recommend.go` calls a simplified version (`renderRecommendMarkdown`) that only lists seeds. The CLI `recommend` command (`cmd/silo/main.go:550`) is the only caller that actually reads schedule.json and passes real `freeMinutes` to the engine — and even then it does so with a naive inline inline parser, not the `internal/schedule` package.

The `silo-guide` skill calls these schedule MCP tools directly:
- `get_free_slots` — to discover available time
- `preview_schedule` — to render today's day
- `add_schedule_event` — to log focus blocks
- `list_schedule_events` — to show the day

---

## Affected Areas

### DELETE (entire subsystem becomes obsolete)

- `internal/schedule/store.go` — JSON persistence (read/write schedule.json). **Delete.**
- `internal/schedule/model.go` — `ScheduleEvent`, `Schedule`, `TimeSlot`, `ResolvedEvent` models. **Delete entirely** OR keep `TimeSlot` as a shared type if useful downstream.
- `internal/schedule/store_test.go` — tests for the store. **Delete.**
- `internal/schedule/resolver_test.go` — most tests become obsolete. **Delete after migration.**
- `internal/config/config.go` — `SchedulePath` field and `DefaultSchedulePath()` function. **Remove.**

### DELETE (MCP tools that no longer make sense)

- `internal/mcp/handlers_schedule.go` — all 5 handlers + tool defs:
  - `add_schedule_event` — writing to Apple Calendar is the agent's job (via FradSer MCP), not Silo's.
  - `remove_schedule_event` — same: agent calls Apple Calendar MCP directly.
  - `list_schedule_events` — agent gets this from Apple Calendar MCP, not Silo.
  - `get_free_slots` — agent gets free slots from Apple Calendar MCP, not Silo.
  - `preview_schedule` — CANDIDATE for keep/refactor (see below).
- `internal/mcp/handlers_schedule_test.go` — schedule handler tests. **Delete.**
- `internal/mcp/server.go` — remove the 5 schedule tool registrations.

### REFACTOR (core engine: minimal signature change)

- `internal/recommend/engine.go` — `Recommend(profile, seeds, freeMinutes int)` — the signature already accepts `freeMinutes` as an integer input. **No change needed to the engine itself.** The caller just changes from reading schedule.json to receiving the value from the MCP tool input.
- `internal/recommend/model.go` — `Engine` interface signature already correct. No change.
- `internal/recommend/renderer.go` — stateless, no schedule dependency. **Keep as-is.**
- `internal/recommend/engine_test.go` — tests use `freeMinutes` directly. **Keep all tests, they remain valid.**

### REFACTOR (MCP recommend handler: the key change)

- `internal/mcp/handlers_recommend.go` — `handleSiloRecommend` currently ignores free time entirely (v1 simplified path). Must be refactored to:
  1. Accept `free_slots []FreeSlot` (or just `free_minutes int`) as input parameter.
  2. Route through `internal/recommend.Engine.Recommend()` (currently bypassed).
  3. Expose a new or upgraded `silo_recommend` tool signature.

### KEEP AS-IS

- `internal/schedule/resolver.go` — `FreeSlots()` and `ResolveDay()` are schedule.json-specific algorithms. Once schedule.json is gone, these are orphaned. **Delete or archive.** However, `ValidateEvent()` and the `HH:MM` time utilities (`validateHHMM`, `mergeIntervals`, `newTimeSlot`) could be extracted to an internal `timeutil` package if needed downstream. Currently no other package imports them.
- `internal/recommend/engine.go`, `renderer.go`, `model.go` — pure logic, no schedule dependency. **Keep.**
- `internal/mcp/handlers_profile.go` — no schedule dependency. **Keep.**
- `internal/mcp/helpers.go` — no schedule dependency. **Keep.**

### MIGRATE (data)

- `~/.config/silo2/schedule.json` (or configured path) — events here (classes, exams, routines) need to move to Apple Calendar's "Silo" calendar.

### UPDATE (skill)

- `~/.config/opencode/skills/silo-guide/SKILL.md` — currently references `get_free_slots`, `preview_schedule`, `add_schedule_event`, `list_schedule_events`. All four references must be replaced with agent-level Apple Calendar MCP calls. The containment rules (write only to "Silo" calendar) must live here.

### UPDATE (CLI)

- `cmd/silo/main.go:runRecommend()` — reads schedule.json directly with an inline parser. Must be updated to remove the schedule.json read; `freeMinutes` would either come from a flag (`--free-minutes`) or be computed externally (not by Silo CLI).
- `cmd/silo/main.go` help text — remove schedule references.

### UPDATE (config)

- `internal/config/config.go` — remove `SchedulePath` and `DefaultSchedulePath()`. `ProductiveHours` can remain as it's relevant to defining the productive window when interpreting free slots from Calendar.

---

## What `recommend` Needs to Become

Today's `silo_recommend` tool:
```go
func siloRecommendTool() mcp.Tool {
    return mcp.NewTool("silo_recommend",
        mcp.WithDescription("..."),
        mcp.WithString("date", ...),
    )
}
```

Target `silo_recommend` tool signature (minimum shape):
```go
mcp.NewTool("silo_recommend",
    mcp.WithString("date", ...),
    mcp.WithNumber("free_minutes", mcp.Description("Total free minutes available. Agent-provided from Calendar MCP.")),
    // Optional: free_slots array for richer context (slot start/end times)
    mcp.WithArray("free_slots", mcp.Description("Optional list of {start, end, duration_minutes} free slot objects")),
)
```

The `freeMinutes` field in `Engine.Recommend()` already exists — the refactor is just wiring the MCP input to the engine call, replacing the v1 simplified path.

**Minimum viable shape**: `free_minutes int` is sufficient. The engine only uses total minutes for scoring (not individual slot boundaries). If slot-level scheduling (e.g. "this seed fits in the 14:00-15:30 slot") is desired in future, the `free_slots` array can be added as optional input now.

---

## Migration of Existing Schedule Data

**What's in schedule.json today?**

From `internal/schedule/model.go`: `ScheduleEvent` structs with:
- `title`, `type` (fixed/routine), `start` (HH:MM), `duration_minutes`, `days` (weekday keys or YYYY-MM-DD), `category`

Typical contents (based on test patterns and silo-guide usage): university classes (routine, by weekday), one-off exams (fixed, by YYYY-MM-DD), work blocks, gym sessions.

**Migration options:**

| Option | Description | Effort | Risk |
|--------|-------------|--------|------|
| A: Manual | User reads their schedule.json, creates Apple Calendar events manually in the "Silo" calendar via Calendar.app | Zero code | High friction, likely events are lost |
| B: One-shot export script | `silo export-schedule` prints human-readable list of events and Calendar.app AppleScript to create them | Low | User must run script; one-time |
| C: Migration tool via FradSer MCP | Agent reads schedule.json, then calls FradSer `calendar_events action:create` for each event in the "Silo" calendar | Medium | Requires FradSer MCP to be installed first |
| D: Document only | Include migration guide in AGENTS.md or README; user migrates at own pace | Zero | schedule.json orphaned until user acts |

**Recommendation**: Option C is best UX (agent-driven, one command), but it creates a hard dependency on FradSer MCP being set up before migration. Option B (export script) is the safest MVP: generate an AppleScript or list of `calendar_events` calls the user can run once, then delete schedule.json.

---

## MCP Tool Surface Change (Summary)

| Tool | Disposition | Reason |
|------|-------------|--------|
| `add_schedule_event` | **DELETE** | Agent writes to Apple Calendar via FradSer MCP, not Silo |
| `remove_schedule_event` | **DELETE** | Agent deletes from Apple Calendar via FradSer MCP, not Silo |
| `list_schedule_events` | **DELETE** | Agent reads Apple Calendar via FradSer MCP, not Silo |
| `get_free_slots` | **DELETE** | Agent computes free slots from Apple Calendar, not Silo |
| `preview_schedule` | **OPTIONAL DELETE** | Rendering markdown from Calendar data is now agent-level; Silo can drop this or transform it to accept agent-provided data. See open question #3. |
| `silo_recommend` | **REFACTOR** | Add `free_minutes` (required) and optionally `free_slots` (array) as input params; wire to real `Engine.Recommend()` |

**Net tool count change**: -4 to -5 tools from Silo MCP, 1 upgraded tool.

---

## Approaches

### Approach A: Full Delete + Clean Refactor
Remove `internal/schedule/` entirely, refactor `silo_recommend` to accept `free_minutes`, delete 4–5 MCP tools, remove `SchedulePath` from config.

- **Pros**: Clean break; Silo has no macOS coupling; smaller surface area; honest architecture.
- **Cons**: Breaking change; existing schedule.json users lose their data unless they migrate; `silo-guide` skill must be fully rewritten.
- **Effort**: Medium (3–4 tasks: delete schedule pkg, refactor recommend handler, update config, update CLI).

### Approach B: Deprecation Layer (keep tools, add new ones)
Keep existing schedule tools (they still work against local JSON), add new `silo_recommend_with_slots` tool that takes free slot input.

- **Pros**: Non-breaking; existing schedule.json users unaffected; gradual migration.
- **Cons**: Two codepaths to maintain; misleading: agents may call old tools thinking Calendar data is included; silo-guide skill ambiguity on which path to use; technical debt accumulates.
- **Effort**: Low upfront, but net higher ongoing.

**Recommendation: Approach A**. The v1 of `silo_recommend` never used the schedule anyway (it's a stub that bypasses the engine). There's no real user value to lose. The architecture story is cleaner. Migration effort is well-defined. Backward compat risk is low given the small user base (single user, local data).

---

## Risks

1. **silo-guide skill breakage**: The skill currently calls `get_free_slots` on every session start. If deleted without replacing the skill, the agent will error silently or fail. **Mitigation**: Update `silo-guide/SKILL.md` before or in the same PR as the code delete.

2. **schedule.json data loss**: If a user has classes/exams in schedule.json and deletes the file before migrating, data is lost. **Mitigation**: Build a one-shot migration tool (Option B above) and document the migration path clearly.

3. **Recommend engine wiring gap**: `handlers_recommend.go` v1 bypasses `internal/recommend.Engine` entirely. Refactoring to wire the real engine adds test surface — the MCP handler test must now exercise the engine path. **Mitigation**: Existing engine tests (`engine_test.go`) are solid; MCP handler test only needs one new case.

4. **`preview_schedule` edge case**: If dropped from Silo, the agent has no markdown table renderer for Calendar data. This is a regression for the silo-guide workflow. **Mitigation**: Either keep `preview_schedule` with agent-provided `events` + `free_slots` as inputs (rendering only, no disk read), or the agent renders it inline using the Calendar MCP output directly.

5. **Containment rules are convention, not code**: Agreed tradeoff. The agent-level rule "only write to Silo calendar" lives in AGENTS.md/skill — not enforceable in code. If the agent is misconfigured, it can write to any calendar. **Accepted risk.** Must be documented clearly in the skill.

6. **CLI `recommend` command**: `cmd/silo/main.go:runRecommend()` reads schedule.json with an inline parser that duplicates the `internal/schedule` logic. If `internal/schedule` is deleted before the CLI is updated, build breaks. **Mitigation**: Update CLI in the same task that removes `internal/schedule`.

---

## Open Questions

1. **Does `preview_schedule` survive?** Keep it as a pure renderer (agent passes events+slots as input, Silo renders markdown) or delete it and let the agent render? If kept, the handler must change from reading local JSON to accepting `events` and `free_slots` arrays.

2. **Free slots input shape for `silo_recommend`**: Is `free_minutes int` enough, or should the tool also accept `free_slots: [{start, end, duration_minutes}]` for slot-aware scheduling in the future? The engine today only uses total minutes, but adding `free_slots` now is cheap and future-proofs.

3. **Migration tool**: Should the export/migration be a CLI subcommand (`silo migrate-to-calendar`), a one-time agent-driven flow, or just a markdown doc? This affects whether any new code is needed.

4. **`ProductiveHours` in config**: Still useful? With Apple Calendar, the agent determines the window by querying Calendar events directly. `ProductiveHours` in config becomes the fallback window hint for the agent. Keep or remove?

5. **State for `apple-calendar-integration` change**: Should the `state.yaml` track this as a multi-PR delivery given the scope (delete + refactor + skill update + migration)?

---

## Recommendation

Delete `internal/schedule/` entirely and all 5 schedule MCP tools. Refactor `silo_recommend` to accept `free_minutes` (required) and `free_slots` (optional array) as MCP input — wiring to the real `Engine.Recommend()` which already exists and is already tested. Update `silo-guide/SKILL.md` to call Apple Calendar MCP for free slot discovery and write to the "Silo" calendar for event creation. Build a one-shot `silo migrate-to-calendar` CLI command that reads the existing schedule.json and outputs the equivalent Apple Calendar creation commands. Remove `SchedulePath` from config; keep `ProductiveHours` as an agent hint.

The total change scope is small: ~400 lines deleted (schedule pkg + handlers), ~50 lines changed (recommend handler refactor + MCP registration), ~50 lines changed (config + CLI). Well within the 400-line review budget for a single PR.

---

## Ready for Proposal

Yes. The architecture is clear, the affected surfaces are mapped, and the open questions above are refinement decisions (not blockers). The proposal should define which open questions are in-scope for this change vs. deferred.

---

**Artifacts produced**: `openspec/changes/apple-calendar-integration/exploration.md`
**Next recommended phase**: `sdd-propose`
