# Verification Checklist: Apple Calendar Integration

Manual integration test for the `agent-calendar-orchestration` capability.
Run each journey end-to-end with FradSer MCP active and Silo MCP running.
Tick each box when the journey passes. All 7 must be ticked before this PR is considered verified.

---

## Journey 1 — Session Start

- [ ] **Session start: agent calls `silo_get_profile_context` first and surfaces current focus.**

**Scenario**: User starts a new session without any specific request.

**Expected agent behavior**:
1. Agent calls `silo_get_profile_context` via Silo MCP before any other action.
2. Agent uses the returned profile context to personalize the session.
3. Agent surfaces the user's current focus or identity summary from the profile.
4. Agent does NOT proceed to recommendations without first loading the profile.

**Pass / Fail**: ___________

---

## Journey 2 — Recommend Journey

- [ ] **Recommend journey: agent computes `free_slots` from calendar, calls `silo_recommend`, presents ranked list without auto-scheduling.**

**Scenario**: User asks "qué hago" or "recommend something" with a partially-filled calendar.

**Expected agent behavior**:
1. Agent calls `calendar_events action:list` (FradSer MCP) for today across all calendars.
2. Agent derives `free_slots` RFC 3339 intervals from the gaps between busy events.
3. Agent calls `silo_recommend` (Silo MCP) with the `free_slots` array.
4. Agent presents the ranked recommendation list to the user.
5. Agent does NOT auto-schedule any item — waits for user direction.

**Pass / Fail**: ___________

---

## Journey 3 — Empty Calendar Day

- [ ] **Empty calendar day: agent reports "no free time" and does not call `silo_recommend`.**

**Scenario**: User's calendar is fully booked for the day (or calendar has no events but computed free slots are empty after filtering).

**Expected agent behavior**:
1. Agent calls `calendar_events action:list` and finds no available free intervals.
2. Agent reports that no free time was found for today.
3. Agent does NOT call `silo_recommend`.
4. Agent optionally offers to check a different day.

**Pass / Fail**: ___________

---

## Journey 4 — Schedule to Silo

- [ ] **Schedule to Silo: agent proposes event, awaits confirmation, creates in `Silo` calendar.**

**Scenario**: User selects a recommendation and asks to schedule it.

**Expected agent behavior**:
1. Agent proposes event details: title, start (RFC 3339), end (RFC 3339), calendar = "Silo".
2. Agent explicitly asks for user confirmation before writing.
3. User confirms with "yes".
4. Agent calls `calendar_events action:create` (FradSer MCP) with `calendar: "Silo"`.
5. Agent confirms the event was created and shows the details.

**Pass / Fail**: ___________

---

## Journey 5 — Schedule Attempt to Non-Silo Calendar

- [ ] **Schedule attempt to non-Silo calendar: agent refuses with exact guard message.**

**Scenario**: User explicitly requests an event be added to a non-Silo calendar (e.g., "Work" or "Personal").

**Expected agent behavior**:
1. Agent detects the target calendar is not exactly `Silo`.
2. Agent refuses the operation.
3. Agent returns the exact message: "I won't write to `<calendar_name>`. Silo only writes to the `Silo` calendar. If you want this event elsewhere, you'll have to add it manually."
4. Agent does NOT call `calendar_events action:create`.

**Pass / Fail**: ___________

---

## Journey 6 — `Silo` Calendar Missing

- [ ] **`Silo` calendar missing: agent halts with creation instructions.**

**Scenario**: The user's Apple Calendar does not contain a calendar named exactly `Silo`.

**Expected agent behavior**:
1. Agent checks available calendars from Apple Calendar MCP.
2. Agent finds no calendar named exactly `Silo` (case-sensitive).
3. Agent halts all scheduling operations.
4. Agent returns the exact message: "Can't schedule — there is no calendar named exactly `Silo` in your Apple Calendar. Open Calendar.app, create a calendar named `Silo`, then try again."
5. Agent does NOT attempt to create the calendar itself.

**Pass / Fail**: ___________

---

## Journey 7 — Ambiguous Slot

- [ ] **Ambiguous slot: agent asks which slot to use rather than picking.**

**Scenario**: Two or more non-overlapping free slots are available and the user has not specified a preference.

**Expected agent behavior**:
1. Agent identifies multiple candidate free slots.
2. Agent cannot determine which slot is most appropriate without user input.
3. Agent presents the available options to the user and asks which slot to use.
4. Agent does NOT pick a slot unilaterally.
5. Agent waits for explicit user direction before scheduling.

**Pass / Fail**: ___________

---

## Summary

| # | Journey | Result |
|---|---------|--------|
| 1 | Session start | |
| 2 | Recommend journey | |
| 3 | Empty calendar day | |
| 4 | Schedule to Silo | |
| 5 | Schedule attempt to non-Silo | |
| 6 | `Silo` calendar missing | |
| 7 | Ambiguous slot | |

**Verified by**: ___________  
**Date**: ___________  
**FradSer MCP version**: ___________  
**Silo MCP build**: ___________
