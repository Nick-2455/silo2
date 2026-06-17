# Design: Apple Calendar Integration

Silo stops owning time. The agent becomes the brain that orchestrates two sibling MCPs — Silo MCP (context + deterministic recommendations) and FradSer Apple Calendar MCP (EventKit read/write) — with containment-by-convention enforcing writes to a single "Silo" calendar.

## 1. Architecture

```
                    ┌────────────────────────┐
                    │   Agent (orchestrator) │
                    │   opencode / Claude    │
                    │                        │
                    │  skills:               │
                    │   - silo-guide        │
                    │   - silo-calendar-    │
                    │       guard           │
                    └───┬────────────────┬───┘
                        │                │
        ┌───────────────┘                └────────────────┐
        ▼                                                  ▼
┌──────────────────┐                            ┌────────────────────┐
│   Silo MCP       │                            │ FradSer Apple      │
│                  │                            │ Calendar MCP       │
│ - get_profile_   │                            │ - calendar_events  │
│     context      │                            │     action:list    │
│ - silo_recommend │                            │     action:create  │
│   (free_slots[]) │                            │     action:delete  │
└────────┬─────────┘                            └─────────┬──────────┘
         │ reads                                          │ EventKit
         ▼                                                ▼
   ┌──────────┐                                    ┌────────────┐
   │ Obsidian │                                    │ User       │
   │ vault    │                                    │ calendars  │
   │ (Silo/   │                                    │ (read all, │
   │  Inbox/) │                                    │  write     │
   └──────────┘                                    │  "Silo"    │
                                                   │  only)     │
                                                   └────────────┘
```

### Journey A — "what should I do now"
1. Agent calls `silo_get_profile_context` (Silo MCP).
2. Agent calls `calendar_events action:list` (FradSer) for today across all calendars.
3. Agent computes `free_slots[]` from busy events.
4. Agent calls `silo_recommend(free_slots)` (Silo MCP).
5. Agent presents ranked recommendations; waits for user direction.

### Journey B — "schedule X"
1. Agent confirms intent and proposes `{title, start, end, calendar:"Silo"}`.
2. `silo-calendar-guard` validates: target calendar must be exactly `Silo`.
3. Agent requests explicit user confirmation.
4. Agent calls `calendar_events action:create` (FradSer) targeting `Silo` calendar.

## 2. MCP tool contracts

### `silo_recommend` (modified)

**Input**:

```json
{
  "free_slots": [
    { "start": "2026-06-17T14:00:00Z", "end": "2026-06-17T16:00:00Z" }
  ]
}
```

`free_slots` is **required**. Each entry MUST have `start` and `end` as RFC 3339 timestamps. Non-UTC offsets are accepted and normalized to UTC server-side via `time.Parse(time.RFC3339, ...)`.

**Output**:

```json
{
  "recommendations": [
    {
      "title": "string",
      "source": "Inbox/open/seed-xxx.md",
      "type": "article",
      "duration_estimate": 45,
      "category": "personal",
      "score": 65,
      "label": "watch-now",
      "reason": "matches your current focus and fits your free time"
    }
  ],
  "free_minutes": 120,
  "seeds_considered": 31
}
```

The handler returns top 5 from `Engine.Recommend()`. Markdown rendering is dropped from the JSON response — agents render presentation, Silo returns data.

**Behavior**: deterministic. Same `(profile, seeds, free_slots, ProductiveHours)` → same output. Wires through `internal/recommend.Engine.Recommend(profile, seeds, freeMinutes)`.

**Error table**:

| Case | MCP result |
|------|-----------|
| `free_slots` missing | `NewToolResultError("free_slots is required")` |
| `free_slots` empty | `NewToolResultError("no free time available")` |
| Malformed entry (missing field or bad RFC 3339) | `NewToolResultError("free_slots[i]: invalid <start\|end>: <parse error>")` |
| `end` ≤ `start` | `NewToolResultError("free_slots[i]: end must be after start")` |

Non-UTC timestamps are accepted (no error) and normalized.

### Removed tools

`add_schedule_event`, `remove_schedule_event`, `list_schedule_events`, `get_free_slots`, `preview_schedule` are unregistered. **No MCP-level migration path** — old clients receive `tool not found` from `mcp-go`. This is intentional; the agent migration is the skill rewrite.

### `silo_get_profile_context` (unchanged)

No changes to tool name, schema, or handler.

## 3. Go code design

### `internal/recommend` (unchanged)

Engine signature stays:

```go
func (e *SimpleEngine) Recommend(profile Profile, seeds []SeedInput, freeMinutes int) ([]Recommendation, error)
```

`ProductiveHours` is **not** a parameter today. The wiring requirement says "engine SHOULD receive ProductiveHours". To avoid changing the engine signature in this PR (out of design budget), we extend with a minimal additive method:

```go
// Recommendation engine: new method, additive only.
func (e *SimpleEngine) RecommendWithHints(profile Profile, seeds []SeedInput, freeMinutes int, hints Hints) ([]Recommendation, error)

type Hints struct {
    ProductiveHours [][2]string // HH:MM windows, empty = no bias
}
```

If `Hints.ProductiveHours` is empty, behavior is identical to `Recommend()`. If populated, scoring adds a small bonus when a seed's category matches productive-window expectations. **Scoring impl is a one-line hint pass-through in this PR** — actual scoring tuning is follow-up; the API is what we need now.

### `internal/mcp/handlers_recommend.go` (refactor)

- Delete `renderRecommendMarkdown`, `seedSummary`, `scanOpenSeeds`, `parseSeedTitle` (or move scan/parse helpers to a new `internal/seeds` package if reused; this PR keeps them inlined but private).
- Add `parseFreeSlots(req) ([]FreeSlot, int, error)` returning slots + total minutes.
- Load `Profile` and `SeedInput[]` from vault (existing logic, kept).
- Call `recommend.NewEngine().RecommendWithHints(profile, seeds, freeMinutes, recommend.Hints{ProductiveHours: deps.Config.ProductiveHours})`.
- Return JSON envelope above.

**Config injection**: `deps.Config` is **already injected** at MCP server construction in `cmd/silo/main.go:runServer()` via `siloMCP.SetDeps(...)`. No new injection needed. The handler reads `deps.Config.ProductiveHours` directly. Per-call config loading is **rejected** (recomputes filesystem read on each call).

### `internal/schedule/` and friends — deletion order

To keep `go build` green at every step:

1. Delete `internal/mcp/handlers_schedule.go` + `handlers_schedule_test.go`.
2. Remove 5 tool registrations from `internal/mcp/server.go`.
3. Refactor `cmd/silo/main.go:runRecommend()` (stops reading schedule.json).
4. Delete `internal/schedule/` package.
5. Remove `SchedulePath` field and `DefaultSchedulePath()` from `internal/config/config.go`.

### `internal/config` (modify)

- **Remove**: `SchedulePath string`, `DefaultSchedulePath()`.
- **Keep**: `ProductiveHours [][2]string` (existing type).
- **Default behavior**: empty `ProductiveHours` → engine ignores hint (no error, no fallback windows applied at handler level).

### `cmd/silo/main.go runRecommend()`

CLI promise: `silo recommend` keeps working as a quick local sanity check.

**Decision**: thin wrapper. Add `--free-minutes int` flag (default 480). No more schedule.json read, no `--date` semantics for free time. Profile + seeds still read from vault.

```
silo recommend --free-minutes 120
```

Justification: simplest option that doesn't break the CLI contract; one flag, one defaulted value. `stdin` free_slots parsing is over-engineered for CLI use; agents use MCP not CLI.

## 4. Skill design

Both skills live at `~/.config/opencode/skills/`. Shipped in the same PR as a `skills/` directory inside the silo2 repo (mirror), with install instructions in README.

### `silo-guide` (rewrite)

**Frontmatter**:

```yaml
---
name: silo-guide
description: "Trigger: session start, qué hago, plan, recommend, schedule, focus. Orchestrates Silo MCP + Apple Calendar MCP to suggest activities based on free time and profile."
triggers:
  - session start
  - qué hago
  - plan
  - recommend
  - schedule
license: MIT
---
```

**Required sections**:

1. **Profile loading** — call `silo_get_profile_context` (Silo MCP) at session start.
2. **Calendar reading** — call `calendar_events` with `action: "list"`, scope: today, all calendars (FradSer MCP).
3. **Free-slot computation** — derive intervals between busy events, format as `[{start, end}]` RFC 3339.
4. **Recommendation call** — call `silo_recommend` (Silo MCP) with `free_slots`.
5. **Presentation** — render ranked results to the user; do not auto-schedule.

**Required MCP tool calls**:
- FradSer: `calendar_events` (action: `list`)
- Silo: `silo_get_profile_context`, `silo_recommend`

### `silo-calendar-guard` (new)

**Frontmatter**:

```yaml
---
name: silo-calendar-guard
description: "Trigger: any operation that creates, modifies, or deletes calendar events. Enforces Silo-only writes and explicit confirmation."
triggers:
  - add to calendar
  - schedule this
  - block time
  - delete event
  - move event
license: MIT
---
```

**Containment rules**:

| Rule | Behavior |
|------|----------|
| Write-only-to-Silo | Reject any create/modify/delete on calendars whose name is not exactly `Silo` (case-sensitive). |
| Confirmation required | Present `{title, start, end, calendar}` and wait for explicit user `yes` before calling FradSer write. |
| Ask when in doubt | If slot, title, or duration is ambiguous, ask before scheduling. |
| Read stays open | Read access to all calendars is allowed for context only. |

**Decision tree**:

```
user asks to schedule
  └─ guard validates intent
     ├─ target calendar specified and ≠ "Silo"? → REFUSE
     ├─ "Silo" calendar exists in EventKit?
     │    ├─ no → halt + instruct user to create it
     │    └─ yes → propose event details
     └─ user confirms?
          ├─ yes → call calendar_events action:create with calendar="Silo"
          └─ no → cancel, ask if alternative needed
```

**Exact error messages**:

- "Silo" calendar missing:
  > Can't schedule — there is no calendar named exactly `Silo` in your Apple Calendar. Open Calendar.app, create a calendar named `Silo`, then try again.
- Write to non-Silo calendar attempted:
  > I won't write to `<calendar_name>`. Silo only writes to the `Silo` calendar. If you want this event elsewhere, you'll have to add it manually.
- Unconfirmed write:
  > Holding off — I need a clear `yes` before I create `<title>` at `<start>`.

## 5. Verification strategy

| Capability | Verification | Location |
|------------|-------------|----------|
| `mcp-recommend` | Go unit tests | `internal/mcp/handlers_recommend_test.go` (new) covering: valid slots, missing param, empty array, malformed RFC 3339, end ≤ start, ProductiveHours hint pass-through. |
| `mcp-schedule` | Build proof (deletion verified by `go build ./...` + `go test ./...` passing with zero references to `internal/schedule`). | CI / `sdd-verify` runs. |
| `config` | Go unit tests | `internal/config/config_test.go` (extend) — load with/without `ProductiveHours`, confirm `SchedulePath` no longer in struct. |
| `agent-calendar-orchestration` | **Manual integration checklist** (skills run outside Go). | `openspec/changes/apple-calendar-integration/verification-checklist.md`. |

### Manual integration checklist (lives at `verification-checklist.md`)

Acceptance for `sdd-verify`: a human runs each journey end-to-end and ticks the box. Format:

- [ ] **Session start**: agent calls `silo_get_profile_context` first and surfaces current focus.
- [ ] **Recommend journey**: agent computes `free_slots` from calendar, calls `silo_recommend`, presents ranked list without auto-scheduling.
- [ ] **Empty calendar day**: agent reports "no free time" and does not call `silo_recommend`.
- [ ] **Schedule to Silo**: agent proposes event, awaits confirmation, creates in `Silo` calendar.
- [ ] **Schedule attempt to non-Silo**: agent refuses with exact guard message.
- [ ] **`Silo` calendar missing**: agent halts with creation instructions.
- [ ] **Ambiguous slot**: agent asks which slot to use rather than picking.

Verify phase passes when all boxes are ticked by the reviewing human.

## 6. Migration docs delivery

**File**: append a `## Migration: schedule.json → Apple Calendar` section to `README.md`. No separate MIGRATION.md (one less file to maintain for a single-user product).

**Content outline**:

```markdown
## Migration: schedule.json → Apple Calendar

Silo no longer owns time. Your `~/.config/silo2/schedule.json` is obsolete.

### What to do
1. Open `~/.config/silo2/schedule.json` and read your events.
2. In Apple Calendar.app, create a calendar named exactly `Silo` (case-sensitive).
3. For each event in `schedule.json`, create the equivalent event in the `Silo` calendar:
   - `type: routine` with `days: [mon, tue, ...]` → recurring weekly event.
   - `type: fixed` with `days: [YYYY-MM-DD]` → one-off event.
4. Once migrated, you may delete `schedule.json` manually. Silo will not touch it.

### Why
- Silo MCP now reads free time from Apple Calendar via the FradSer MCP, not from a local file.
- Recommendations stay deterministic; calendar I/O is the agent's job.
```

**`schedule.json` disposition**: left in place. Silo never deletes it. The README explicitly tells the user they can remove it manually after migrating.

## 7. ProductiveHours wiring

| Step | Where | What happens |
|------|-------|-------------|
| 1. Load | `cmd/silo/main.go:runServer()` | `config.Load()` reads `ProductiveHours` from `silo.config.json`. |
| 2. Inject | `cmd/silo/main.go:runServer()` | `siloMCP.SetDeps(Deps{Config: cfg, ...})` stores it on package-level `deps`. |
| 3. Read | `internal/mcp/handlers_recommend.go:handleSiloRecommend` | Handler reads `deps.Config.ProductiveHours` per call (in-memory access, no IO). |
| 4. Forward | same | Pass into `recommend.Hints{ProductiveHours: deps.Config.ProductiveHours}`. |
| 5. Engine consumes | `internal/recommend/engine.go:RecommendWithHints` | If non-empty, applies window bonus; if empty, behaves identically to `Recommend()`. |

**Empty/missing behavior**: zero-value `[][2]string{}` flows end-to-end. No error. No fallback windows. Engine simply skips the hint.

## 8. Delivery and review budget

| Area | Net lines | Review cost |
|------|----------|-------------|
| Delete `internal/schedule/` | -753 | low |
| Delete `handlers_schedule*.go` | -643 | low |
| Modify `internal/mcp/server.go` | -5 | trivial |
| Modify `internal/mcp/handlers_recommend.go` | +90 / -50 | **high** |
| New `handlers_recommend_test.go` | +160 | medium |
| Modify `internal/recommend/engine.go` (add `RecommendWithHints` + `Hints`) | +25 | medium |
| Modify `internal/config/config.go` | -10 | trivial |
| Modify `cmd/silo/main.go` (runRecommend trim) | -60 / +20 | low |
| README migration section | +30 | low |
| Skills: `silo-guide` rewrite + `silo-calendar-guard` new | +220 | medium |

**Net code**: ~-1200 deletions, ~+545 additions. Estimated changed-line count for review (`additions + deletions`): ~1750 — overshoots the 600-line budget on raw count, but **70% is mechanical deletion**. We surface this to the orchestrator as a known overrun candidate.

**If we must cut to stay near 600**:
1. First cut: move skill files to a follow-up PR (skills can ship out-of-band; they live outside the Go module). Saves ~220 lines.
2. Second cut: drop `RecommendWithHints`, do `ProductiveHours` wiring as a follow-up. Saves ~50 lines + test surface.
3. Third cut: keep `renderRecommendMarkdown` as legacy code-path until next PR (not recommended; muddies the engine wiring).

Recommended: ship as **one PR** but flag the deletion-heavy ratio in PR body so reviewers focus on the ~545 additions.

## 9. Open questions

None blocking. Two follow-ups noted but resolved within this design:

- `RecommendWithHints` vs changing `Recommend` signature: chose additive method to avoid breaking the existing `Engine` interface and tests. Follow-up could collapse to a single method in a future PR.
- Skills repo location: kept under `~/.config/opencode/skills/` for opencode discovery; mirrored in `skills/` directory in silo2 repo for shipping. Install step documented in README.
