# Tasks: Apple Calendar Integration

## Review Workload Forecast

| Field | Value |
|-------|-------|
| Estimated changed lines (total) | ~1750 (additions + deletions) |
| Per-PR split | PR 1: ~1400 del / 30 add; PR 2: ~300 del / 275 add; PR 3: ~0 del / 220 add |
| 400-line budget risk (per PR) | PR 1: **High** (deletion-heavy, low cognitive cost); PR 2: **Medium**; PR 3: **Low** |
| 600-line budget risk (per PR) | PR 1: **Exceeds** (~1430 lines, 97% deletion); PR 2: **Within** (~575); PR 3: **Within** (~220) |
| Chained PRs recommended | Yes |
| Suggested split | PR 1 → PR 2 → PR 3 (each targets `main`) |
| Delivery strategy | chained PRs (user-decided, no re-litigation) |
| Chain strategy | stacked-to-main |

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High

### Suggested Work Units

| Unit | Goal | Likely PR | Notes |
|------|------|-----------|-------|
| 1 | Delete all schedule code; build green | PR 1 → `main` | ~97% deletion; reviewer focuses on what was removed |
| 2 | Wire `silo_recommend` to real engine + `ProductiveHours` | PR 2 → `main` | Depends on PR 1 merged; new code only |
| 3 | Rewrite `silo-guide` skill + new `silo-calendar-guard` skill + checklist | PR 3 → `main` | Depends on PR 2 merged; skills live outside Go module |

---

## PR 1 — Remove Schedule Capability

**Goal**: silo2 no longer ships any schedule-related code or MCP tools. After this PR, `go build ./...` and `go test ./...` pass; MCP server exposes only `silo_recommend` + `get_profile_context` + `init_profile` (unchanged behavior).

**Deletion order (MUST follow to keep `go build` green at every step)**:

### Phase 1.1 — Delete MCP handler + test

- [x] **1.1** Delete `internal/mcp/handlers_schedule.go` (237 LOC) and `internal/mcp/handlers_schedule_test.go` (~406 LOC).
  - _~643 lines deleted_
  - Verify: `go build ./internal/mcp/...` fails — expected, because `server.go` still references the deleted symbols. Continue to 1.2 immediately.
  - Rollback: `git checkout HEAD -- internal/mcp/handlers_schedule.go internal/mcp/handlers_schedule_test.go`

### Phase 1.2 — Remove tool registrations from server.go

- [x] **1.2** In `internal/mcp/server.go`, remove the 5 `s.AddTool(...)` calls and the `// Schedule tools.` comment block (lines 21–26). Keep the `// Profile tools.` and `// Recommend tool.` blocks intact.
  - _~7 lines deleted_
  - _Depends on: 1.1_
  - Verify: `go build ./internal/mcp/...` passes (no more references to deleted handler functions).

### Phase 1.3 — Remove schedule.json read from runRecommend

- [x] **1.3** In `cmd/silo/main.go:runRecommend()`: remove the `-date` flag, the `schedulePath` variable, the JSON unmarshal block (lines 580–607), and `eventMatchesDate()`. Replace `freeMin` calculation with a hardcoded `freeMin := 480` constant (temporary stand-in; PR 2 ships the `--free-minutes` flag). Remove the `encoding/json`, `time`, `strings` imports if they are no longer needed by other functions in main.go (check before removing — other functions may use them).
  - _~50 lines deleted, ~3 lines added_
  - _Depends on: 1.2_
  - Verify: `go build ./cmd/silo/...` passes.

### Phase 1.4 — Delete internal/schedule/ package

- [x] **1.4** Delete the entire `internal/schedule/` directory: `model.go`, `store.go`, `store_test.go`, `resolver.go`, `resolver_test.go`.
  - _~370–400 lines deleted_
  - _Depends on: 1.3 (no remaining references to `internal/schedule`)_
  - Verify: `go build ./...` passes. `grep -r "internal/schedule" .` returns no results.

### Phase 1.5 — Remove SchedulePath from config

- [x] **1.5** In `internal/config/config.go`: remove `SchedulePath string` field (line 32–33) and the `DefaultSchedulePath()` function (lines 39–46). Remove `filepath` import if no longer used.
  - _~10 lines deleted_
  - _Depends on: 1.4 (the only caller of `DefaultSchedulePath()` was `handlers_schedule.go` and `cmd/silo/main.go`, both already cleaned)_
  - Verify: `go build ./...` passes.

### Phase 1.6 — Update config_test.go

- [x] **1.6** In `internal/config/config_test.go`: remove any test assertions that reference `SchedulePath` or `DefaultSchedulePath()`. Add assertion confirming `SchedulePath` field does NOT exist on `Config` struct (compile-time proof via absent field in test setup).
  - _~5 lines removed, ~2 lines added_
  - _Depends on: 1.5_
  - Verify: `go test ./internal/config/...` passes.

### Phase 1.7 — Migration docs in README.md

- [x] **1.7** Append the `## Migration: schedule.json → Apple Calendar` section to `README.md` (per design section 6 content outline). Include: what happened, 4-step migration guide, `schedule.json` disposition (Silo never deletes it).
  - _~30 lines added_
  - No code dependencies.
  - Verify: section renders correctly in Markdown preview.

### Phase 1.8 — Full build + test verification

- [x] **1.8** Run `go build ./...` — must pass with zero errors.
- [x] **1.8b** Run `go test ./...` — must pass with zero failures.
- [x] **1.8c** Run `grep -r "schedule" internal/mcp/ internal/config/ cmd/silo/main.go` — must return zero schedule-related references (excluding unrelated strings).

**PR 1 done when**:
- [x] `go build ./...` green
- [x] `go test ./...` green
- [x] Zero references to `internal/schedule` in `go list` output
- [x] `internal/mcp/server.go` registers exactly 3 tools: `silo_recommend`, `get_profile_context`, `init_profile`
- [x] `README.md` contains `## Migration: schedule.json → Apple Calendar`

---

## PR 2 — Wire silo_recommend to Engine + ProductiveHours

**Goal**: `silo_recommend` MCP tool accepts `free_slots[]` (RFC 3339), routes through `internal/recommend.Engine.RecommendWithHints()`, respects `ProductiveHours` from injected config, and returns a deterministic JSON envelope. Fixes the bypass bug (Engram obs #1617). `silo recommend` CLI becomes a thin `--free-minutes` wrapper.

**Depends on: PR 1 merged to main.**

### Phase 2.1 — Add Hints type + RecommendWithHints to engine

- [ ] **2.1** In `internal/recommend/engine.go`: add `Hints` struct (`ProductiveHours [][2]string`) and `RecommendWithHints(profile Profile, seeds []SeedInput, freeMinutes int, hints Hints) ([]Recommendation, error)` method. If `hints.ProductiveHours` is empty, delegate directly to `Recommend()`. Stub scoring bonus is acceptable for this PR (scoring tuning is follow-up).
  - _~25 lines added_
  - Verify: `go build ./internal/recommend/...` passes.

### Phase 2.2 — Refactor handlers_recommend.go

- [ ] **2.2** Rewrite `internal/mcp/handlers_recommend.go`:
  - Replace `siloRecommendTool()` definition: remove `date` param, add required `free_slots` array param (array of objects with `start`/`end` strings).
  - Delete `renderRecommendMarkdown()`, `seedSummary`, `scanOpenSeeds()`, `parseSeedTitle()`.
  - Add `FreeSlot` struct (`Start`, `End string`) and `parseFreeSlots(req) ([]FreeSlot, int, error)` function.
  - `parseFreeSlots` validates: missing `free_slots` → `"free_slots is required"`, empty array → `"no free time available"`, bad RFC 3339 → `"free_slots[i]: invalid <start|end>: <parse error>"`, `end ≤ start` → `"free_slots[i]: end must be after start"`.
  - Load `SeedInput[]` from vault `Inbox/open/` using `internal/seed` or inline scanning (keep existing vault read logic, adapted for `recommend.SeedInput`).
  - Call `recommend.NewEngine().RecommendWithHints(profile, seeds, freeMinutes, recommend.Hints{ProductiveHours: deps.Config.ProductiveHours})`.
  - Return JSON: `{recommendations: [...top5], free_minutes: int, seeds_considered: int}`.
  - _~116 lines deleted, ~90 lines added_
  - _Depends on: 2.1_
  - Verify: `go build ./internal/mcp/...` passes.

### Phase 2.3 — Update runRecommend CLI in main.go

- [ ] **2.3** In `cmd/silo/main.go:runRecommend()`: replace `--date` flag with `--free-minutes int` (default 480). Remove `parseProfileFromFile`, `scanOpenSeedsForCLI`, `renderCLIRecommend`, `eventMatchesDate` if they are only used by `runRecommend` (verify scope before deleting). Load profile + seeds from vault, call `recommend.NewEngine().RecommendWithHints(...)` with the flag value, print JSON output.
  - _~60 lines deleted, ~20 lines added_
  - _Depends on: 2.2_
  - Verify: `go build ./cmd/silo/...` passes. `go run ./cmd/silo recommend --free-minutes 120` prints JSON without error.

### Phase 2.4 — Unit tests for handler (happy path + all error cases)

- [ ] **2.4** Create `internal/mcp/handlers_recommend_test.go`:
  - Test: valid `free_slots` → returns `recommendations` array, `free_minutes`, `seeds_considered`.
  - Test: missing `free_slots` → tool result error `"free_slots is required"`.
  - Test: empty `free_slots: []` → tool result error `"no free time available"`.
  - Test: malformed RFC 3339 `start` → tool result error matching `"free_slots[0]: invalid start: ..."`.
  - Test: `end ≤ start` → tool result error `"free_slots[0]: end must be after start"`.
  - Test: `ProductiveHours` hint pass-through → `RecommendWithHints` called with non-empty `Hints` when config has `ProductiveHours`.
  - _~160 lines added_
  - _Depends on: 2.2_
  - Verify: `go test ./internal/mcp/... -run TestHandleSiloRecommend` passes all subtests.

### Phase 2.5 — Unit test for RecommendWithHints

- [ ] **2.5** In `internal/recommend/engine_test.go`: add tests for `RecommendWithHints`:
  - Test: empty `Hints` → output identical to `Recommend()` for same inputs.
  - Test: same inputs called twice → identical output (determinism).
  - Test: non-empty `ProductiveHours` → compiles + runs without panic (scoring tuning is follow-up, not tested in depth here).
  - _~40 lines added_
  - _Depends on: 2.1_
  - Verify: `go test ./internal/recommend/...` passes.

### Phase 2.6 — Full build + test verification

- [ ] **2.6** Run `go build ./...` — must pass.
- [ ] **2.6b** Run `go test ./...` — must pass, including new handler tests.
- [ ] **2.6c** Verify determinism: run `go test ./internal/mcp/... -run TestSiloRecommendDeterminism` (if named so) passes.

**PR 2 done when**:
- [ ] `go build ./...` green
- [ ] `go test ./...` green (including `handlers_recommend_test.go` and `engine_test.go` additions)
- [ ] `silo_recommend` in `server.go` uses `free_slots` array param (verify tool definition)
- [ ] No references to `renderRecommendMarkdown`, `scanOpenSeeds`, `parseSeedTitle` remain in codebase
- [ ] `silo recommend --free-minutes 120` runs without error

---

## PR 3 — Skills + Verification Checklist

**Goal**: ship the two agent skills that implement the orchestration model and the manual verification checklist that gates the `agent-calendar-orchestration` capability.

**Depends on: PR 2 merged to main.**

### Phase 3.1 — Rewrite silo-guide skill

- [ ] **3.1** Rewrite `~/.config/opencode/skills/silo-guide/SKILL.md` (also mirror to `skills/silo-guide/SKILL.md` in repo):
  - Frontmatter: name `silo-guide`, updated description + triggers (session start, qué hago, plan, recommend, schedule, focus).
  - Remove all references to `get_free_slots`, `add_schedule_event`, `preview_schedule` (deleted in PR 1).
  - Section 1 — Profile loading: call `silo_get_profile_context` (Silo MCP) first.
  - Section 2 — Calendar reading: call FradSer `calendar_events` with `action: "list"`, scope today, all calendars.
  - Section 3 — Free-slot computation: derive `[{start, end}]` intervals RFC 3339 from busy events.
  - Section 4 — Recommendation: call `silo_recommend(free_slots)` (Silo MCP).
  - Section 5 — Presentation: render ranked list; do NOT auto-schedule.
  - Section 6 — Empty day guard: if no free slots, report "no free time" and do NOT call `silo_recommend`.
  - _~90 lines rewritten_
  - Verify: skill loads in opencode without parse errors; triggers fire on "qué hago".

### Phase 3.2 — Create silo-calendar-guard skill

- [ ] **3.2** Create `~/.config/opencode/skills/silo-calendar-guard/SKILL.md` (also create `skills/silo-calendar-guard/SKILL.md` in repo):
  - Frontmatter: name `silo-calendar-guard`, triggers (add to calendar, schedule this, block time, delete event, move event).
  - Containment rules table (write-only-to-Silo, confirmation required, ask when in doubt, read stays open).
  - Decision tree: validate target calendar → check "Silo" existence → propose details → await `yes`.
  - Exact error messages from design (3 messages: missing calendar, wrong calendar, unconfirmed write).
  - _~130 lines added_
  - Verify: skill file parses valid YAML frontmatter; skill loads without error.

### Phase 3.3 — Create verification-checklist.md

- [ ] **3.3** Create `openspec/changes/apple-calendar-integration/verification-checklist.md` with 7 manual journey checkboxes (per design section 5):
  - Session start, recommend journey, empty calendar day, schedule to Silo, schedule attempt to non-Silo, `Silo` calendar missing, ambiguous slot.
  - Each item: checkbox + scenario title + expected agent behavior + pass/fail space.
  - _~40 lines added_
  - No code dependencies.

### Phase 3.4 — Run verification checklist

- [ ] **3.4** Manually execute all 7 journeys from `verification-checklist.md` with FradSer MCP active:
  - Tick each box in the checklist as you verify.
  - Record any deviation as a blocker comment on the PR.
  - Verify: all 7 boxes ticked, no blocker deviations.

### Phase 3.5 — Add skill install instructions to README.md

- [ ] **3.5** Append a `## Skills Install` section (or extend existing agent setup section) in `README.md`: document that `skills/silo-guide/SKILL.md` and `skills/silo-calendar-guard/SKILL.md` must be copied to `~/.config/opencode/skills/` (or equivalent agent config dir). Include the FradSer MCP setup prerequisite (build locally, pin version, no `npx -y`).
  - _~20 lines added_

**PR 3 done when**:
- [ ] `~/.config/opencode/skills/silo-guide/SKILL.md` contains no references to deleted schedule tools
- [ ] `~/.config/opencode/skills/silo-calendar-guard/SKILL.md` exists with correct containment rules
- [ ] `openspec/changes/apple-calendar-integration/verification-checklist.md` exists with all 7 journeys
- [ ] All 7 checklist journeys manually verified and ticked
- [ ] `README.md` contains skill install instructions

---

## Review Workload Forecast (Detailed)

| PR | Deletions | Additions | Total lines | 400-line risk | 600-line risk |
|----|-----------|-----------|-------------|---------------|---------------|
| PR 1 | ~1400 | ~33 | ~1433 | High (97% deletion, low cog cost) | Exceeds (acceptable: mechanical) |
| PR 2 | ~176 | ~335 | ~511 | Medium | Within budget |
| PR 3 | ~48 (rewrite) | ~240 | ~288 | Low | Within budget |
| **Total** | **~1624** | **~608** | **~2232** | — | — |

> **Note on PR 1 overrun**: the 600-line budget is exceeded due to mechanical deletion of `internal/schedule/` (~750 LOC) and `handlers_schedule*.go` (~643 LOC). Reviewer cognitive cost is minimal (verify what was removed, not complex logic). PR body MUST flag this ratio and recommend reviewers focus on the ~33 additions.

Decision needed before apply: No
Chained PRs recommended: Yes
Chain strategy: stacked-to-main
400-line budget risk: High
