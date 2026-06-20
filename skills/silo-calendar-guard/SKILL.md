---
name: silo-calendar-guard
description: "Trigger: add to calendar, schedule this, block time, delete event, move event. Enforces Silo-only writes and explicit confirmation before any Apple Calendar write."
license: MIT
metadata:
  author: gentleman-programming
  version: "1.0"
---

## Activation Contract

Load this skill whenever the user asks to create, modify, or delete a calendar event. Also load when any other skill is about to issue a write to Apple Calendar MCP.

## Containment Rules

| Rule | Behavior |
|------|----------|
| Write-only-to-Silo | Reject any create, modify, or delete operation on calendars whose name is not exactly `Silo` (case-sensitive). |
| Confirmation required | Present `{ title, start, end, calendar: "Silo" }` to the user and wait for an explicit `yes` before calling any FradSer write tool. |
| Ask when in doubt | If the slot, title, or duration is ambiguous, ask the user to clarify before scheduling. Never assume. |
| Read stays open | Reading events from any calendar is allowed for context. No restrictions on read access. |

## Decision Tree

```
user asks to schedule / modify / delete
  └─ guard validates intent
     ├─ target calendar specified AND not "Silo"?
     │    └─ REFUSE — emit "Write to non-Silo calendar" error message
     ├─ "Silo" calendar exists in EventKit?
     │    ├─ no → HALT — emit "Silo calendar missing" error message
     │    └─ yes → propose event details to user
     └─ user confirms with explicit "yes"?
          ├─ yes → call calendar_events action:create (or :delete) with calendar="Silo"
          └─ no  → cancel; ask if the user needs an alternative
```

## Exact Error Messages

**"Silo" calendar missing:**
> Can't schedule — there is no calendar named exactly `Silo` in your Apple Calendar. Open Calendar.app, create a calendar named `Silo`, then try again.

**Write to non-Silo calendar attempted:**
> I won't write to `<calendar_name>`. Silo only writes to the `Silo` calendar. If you want this event elsewhere, you'll have to add it manually.

**Unconfirmed write:**
> Holding off — I need a clear `yes` before I create `<title>` at `<start>`.

## Execution Steps

1. Identify the target calendar from user intent. If not specified, assume `Silo`.
2. If the target is not `Silo`, refuse immediately with the "Write to non-Silo" message.
3. Check whether a calendar named exactly `Silo` exists by reading available calendars from Apple Calendar MCP. If missing, halt with the "Silo calendar missing" message.
4. Propose event details to the user: title, start (RFC 3339), end (RFC 3339), calendar = "Silo".
5. Wait for explicit user confirmation (`yes`).
6. If confirmed: call `calendar_events action:create` (or `:delete`) targeting calendar = "Silo".
7. If not confirmed or ambiguous: cancel and offer to revisit.

## Output Contract

- On refusal: return the exact error message text — do not paraphrase.
- On success: confirm the event was created/deleted in the `Silo` calendar with its details.
- On ambiguity: ask a single clarifying question and stop.
