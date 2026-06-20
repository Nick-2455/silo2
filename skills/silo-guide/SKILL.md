---
name: silo-guide
description: "Trigger: session start, qué hago, plan, recommend, schedule, focus, guardar seed, listo, terminamos, cerra sesion. Orchestrates Silo MCP + Apple Calendar MCP to suggest activities based on free time and profile."
license: MIT
metadata:
  author: gentleman-programming
  version: "2.0"
---

## Activation Contract

Load this skill at conversation START and on any mention of: scheduling, recommendations, what to do, session planning, or session-end signals ("listo", "terminamos", "cerra sesion").

## Hard Rules

- NEVER call `get_free_slots`, `add_schedule_event`, `preview_schedule`, `list_schedule_events`, or `remove_schedule_event` — these tools no longer exist.
- Free time MUST come from Apple Calendar MCP (`calendar_events action:list`), not from Silo MCP.
- NEVER auto-schedule. Present ranked recommendations and wait for user direction.
- If no free slots exist after calendar read, report "no free time" and do NOT call `silo_recommend`.
- Canonical vault root: `/Users/nicolasperalta/Library/Mobile Documents/iCloud~md~obsidian/Documents/silo2`.

## Execution Steps

### Step 1 — Profile Loading

Call `silo_get_profile_context` (Silo MCP) at session start. Use the returned profile for all subsequent recommendation calls in this session.

If `silo_get_profile_context` fails or returns empty, inform the user that profile context is unavailable and do not proceed to recommendations without acknowledging the gap.

### Step 2 — Calendar Reading

Call `calendar_events` (Apple Calendar MCP / FradSer) with:
- `action: "list"`
- scope: today (current date)
- all calendars

Read events for the full day to identify when the user is busy.

### Step 3 — Free-Slot Computation

Derive free intervals from the list of busy calendar events:
- Sort events by start time.
- Compute gaps between events (and before the first / after the last event within the workday).
- Format each interval as `{ "start": "<RFC 3339>", "end": "<RFC 3339>" }`.

If no free intervals exist → go to **Empty-Day Guard** below. Do NOT call `silo_recommend`.

### Step 4 — Recommendation Call

Call `silo_recommend` (Silo MCP) with the computed `free_slots` array:

```json
{
  "free_slots": [
    { "start": "2026-06-19T14:00:00-03:00", "end": "2026-06-19T16:00:00-03:00" }
  ]
}
```

### Step 5 — Presentation

Render the ranked recommendation list returned by `silo_recommend`. Present it to the user clearly. Do NOT auto-schedule any item. Wait for the user to select an activity or give further direction.

### Empty-Day Guard

If the calendar is fully booked or `free_slots` would be empty:
- Report: "No free time found in your calendar for today. All time slots appear to be occupied."
- Do NOT call `silo_recommend`.
- Offer to check a different day if the user asks.

## Decision Gates

| Signal | Action |
|--------|--------|
| Session start / "qué hago" | Run Steps 1 → 5 |
| Profile unavailable | Inform user; do not recommend |
| Calendar fully booked | Report "no free time"; skip recommendation |
| User wants to schedule something | Defer to `silo-calendar-guard` skill |
| Session ending ("listo", "terminamos") | Summarize session; offer to save notes |
| User mentions a resource/URL | Guide user to `silo save "URL"` |

## Output Contract

Return the ranked recommendation list from `silo_recommend` rendered as readable text, plus any contextual note about free time availability. Never return raw JSON to the user unless they explicitly ask.
