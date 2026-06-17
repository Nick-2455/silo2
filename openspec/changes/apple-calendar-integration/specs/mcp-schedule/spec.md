# Delta for mcp-schedule

## REMOVED Requirements

### Requirement: Add Schedule Event

(Reason: Agent writes to Apple Calendar directly via `FradSer/mcp-server-apple-events` MCP. Silo no longer owns calendar I/O.)
(Migration: Agent calls `calendar_events action:create` on the Apple Calendar MCP, targeting the "Silo" calendar with explicit user confirmation via `silo-calendar-guard` skill.)

---

### Requirement: Remove Schedule Event

(Reason: Agent deletes from Apple Calendar directly via `FradSer/mcp-server-apple-events` MCP. Silo no longer owns calendar I/O.)
(Migration: Agent calls `calendar_events action:delete` on the Apple Calendar MCP with explicit user confirmation via `silo-calendar-guard` skill.)

---

### Requirement: List Schedule Events

(Reason: Agent reads events from Apple Calendar MCP. Silo's local JSON-backed list is obsolete once `schedule.json` is retired.)
(Migration: Agent calls `calendar_events action:list` on the Apple Calendar MCP for the target date.)

---

### Requirement: Get Free Slots

(Reason: Agent computes free slots from Apple Calendar MCP output, not from `schedule.json`. This computation is the agent's responsibility, not Silo's.)
(Migration: Agent queries Apple Calendar MCP for events and derives free intervals, which it then passes to `silo_recommend` via the `free_slots` parameter.)

---

### Requirement: Preview Schedule

(Reason: Schedule markdown rendering was backed by `schedule.json`. With `schedule.json` retired, there is no local source to render from. Agent-side rendering from Apple Calendar MCP output replaces this tool.)
(Migration: Agent renders the day view inline using calendar event data returned by `calendar_events action:list`. No Silo MCP tool needed.)

---

## Removal Scope

The following code surfaces MUST be removed as part of retiring this capability:

| Surface | Action |
|---------|--------|
| `internal/schedule/` directory | Delete entirely (store, resolver, model, tests) |
| `internal/mcp/handlers_schedule.go` | Delete |
| `internal/mcp/handlers_schedule_test.go` | Delete |
| `internal/mcp/server.go` — 5 tool registrations | Remove entries |
| `~/.config/silo2/schedule.json` | NOT deleted by code — user migrates manually |

## Migration Documentation Requirement

The change MUST include documentation of the manual `schedule.json` migration path in `AGENTS.md` or `README.md`. Documentation MUST cover:

- Where `schedule.json` is located (`~/.config/silo2/schedule.json` by default).
- How to read the existing events (title, type, start, duration_minutes, days).
- How to create equivalent events in the "Silo" Apple Calendar calendar manually or via the Apple Calendar MCP.
- Confirmation that Silo code does not delete `schedule.json` automatically.
