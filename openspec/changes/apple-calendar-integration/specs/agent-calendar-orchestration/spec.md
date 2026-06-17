# Agent Calendar Orchestration Specification

## Purpose

Defines the contract for the agent-as-brain model: agent orchestrates two peer MCPs — `silo` (context + deterministic recommendations) and `FradSer/mcp-server-apple-events` (EventKit read/write) — with containment-by-convention rules enforcing writes exclusively to the user-created "Silo" calendar.

## Requirements

### Requirement: Profile Load at Session Start

The agent MUST call `silo_get_profile_context` at the beginning of every session before making any recommendation or calendar query.

#### Scenario: Profile loaded successfully

- GIVEN the user starts a session
- WHEN the agent initializes
- THEN the agent calls `silo_get_profile_context` via Silo MCP
- AND uses the returned profile for all subsequent recommendation calls in that session

#### Scenario: Profile unavailable

- GIVEN `silo_get_profile_context` returns an error or empty result
- WHEN the agent initializes
- THEN the agent informs the user that profile context could not be loaded
- AND does not proceed to recommendations without acknowledging the gap

---

### Requirement: Free Slot Discovery via Apple Calendar MCP

The agent MUST read free time exclusively from the Apple Calendar MCP (`calendar_events action:list` or equivalent), never from Silo MCP.

#### Scenario: Free slots read for today

- GIVEN the user requests recommendations for today
- WHEN the agent queries the calendar
- THEN the agent calls Apple Calendar MCP to list events for the target date
- AND computes free intervals from the returned event list
- AND passes those intervals to `silo_recommend` as `free_slots`

#### Scenario: No free slots found

- GIVEN the calendar is fully booked for the target date
- WHEN the agent queries the calendar
- THEN the agent informs the user that no free time was found
- AND does not call `silo_recommend`

---

### Requirement: Recommendation Presentation

The agent MUST pass computed free slots to `silo_recommend` and present ranked results to the user before writing any event.

#### Scenario: Recommendations returned

- GIVEN free slots are available and profile is loaded
- WHEN the agent calls `silo_recommend` with `free_slots`
- THEN the agent presents the ranked recommendation list to the user
- AND waits for user selection or direction before scheduling anything

---

### Requirement: Write Containment — "Silo" Calendar Only

The agent MUST NOT create, modify, or delete events in any calendar other than the calendar named exactly "Silo" (case-sensitive).

#### Scenario: Write to "Silo" calendar

- GIVEN the user confirms they want an event created
- WHEN the agent calls Apple Calendar MCP to create an event
- THEN the agent specifies the target calendar as "Silo" (exact name)
- AND the event is created in the "Silo" calendar only

#### Scenario: Write attempted to non-Silo calendar

- GIVEN any operation targets a calendar that is not named exactly "Silo"
- WHEN the agent evaluates the target
- THEN the agent MUST refuse the operation
- AND inform the user that writes outside "Silo" are not permitted

---

### Requirement: Explicit User Confirmation Before Write

The agent MUST obtain explicit user confirmation before executing any create, modify, or delete operation on the Apple Calendar MCP.

#### Scenario: Create event with confirmation

- GIVEN the user selects a recommendation
- WHEN the agent is about to call `calendar_events action:create`
- THEN the agent presents the event details (title, start, end, calendar="Silo") to the user
- AND waits for explicit confirmation before issuing the call

#### Scenario: Delete event with confirmation

- GIVEN the user requests removal of a "Silo" calendar event
- WHEN the agent is about to call `calendar_events action:delete`
- THEN the agent presents the event details and asks for confirmation
- AND cancels if the user does not confirm

---

### Requirement: Read-Only Access to Non-Silo Calendars

The agent MAY read events from any calendar for context (titles, time blocks) but MUST NOT write to, modify, or delete from any calendar other than "Silo".

#### Scenario: Reading context from work calendar

- GIVEN the agent needs to compute free slots
- WHEN querying calendar events across all calendars
- THEN the agent reads event data (title, start, end) for context only
- AND never issues a write operation against those calendars

---

### Requirement: "Silo" Calendar Existence Check

If the "Silo" calendar does not exist in EventKit, the agent MUST stop and instruct the user to create it manually before proceeding.

#### Scenario: "Silo" calendar missing

- GIVEN the Apple Calendar MCP lists available calendars
- WHEN "Silo" (case-sensitive) is not present
- THEN the agent halts all scheduling operations
- AND tells the user to create a calendar named exactly "Silo" in Apple Calendar.app

---

### Requirement: Ask When in Doubt

When the agent is uncertain about what to schedule, where to schedule it, or which free slot to use, the agent MUST ask the user rather than assume.

#### Scenario: Ambiguous slot selection

- GIVEN two overlapping or adjacent free slots are available
- WHEN the agent cannot determine which slot is most appropriate
- THEN the agent presents the options to the user
- AND waits for user direction before scheduling
