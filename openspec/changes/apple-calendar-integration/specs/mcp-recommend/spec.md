# Delta for mcp-recommend

## MODIFIED Requirements

### Requirement: Free Slot Input

`silo_recommend` MUST accept `free_slots` as a required input: an array of objects each containing `start` and `end` as ISO 8601 timestamps.
(Previously: `silo_recommend` accepted only `date` as input and computed free time internally by reading schedule.json — which it bypassed entirely, calling a simplified renderer instead of the engine.)

#### Scenario: Valid free_slots provided

- GIVEN the agent has computed free slots from Apple Calendar MCP
- WHEN the agent calls `silo_recommend` with `free_slots: [{start, end}, ...]`
- THEN the tool routes the call through `internal/recommend.Engine.Recommend()`
- AND returns a ranked list of seeds with score, reason, and suggested duration

#### Scenario: Missing free_slots

- GIVEN the agent calls `silo_recommend` without the `free_slots` parameter
- WHEN the handler validates input
- THEN the tool returns an error indicating `free_slots` is required

#### Scenario: Empty free_slots array

- GIVEN the agent calls `silo_recommend` with `free_slots: []`
- WHEN the handler validates input
- THEN the tool returns an error indicating no free time is available

#### Scenario: Malformed free_slots entry

- GIVEN the agent provides a `free_slots` entry missing `start` or `end`, or with an invalid ISO 8601 value
- WHEN the handler parses the input
- THEN the tool returns a descriptive validation error identifying the malformed entry

---

### Requirement: Engine Wiring

`silo_recommend` MUST route through `internal/recommend.Engine.Recommend()` for all recommendation calls.
(Previously: the MCP handler called a simplified `renderRecommendMarkdown` path that bypassed the engine entirely, making output non-deterministic and non-scorable.)

#### Scenario: Engine called with correct inputs

- GIVEN valid `free_slots` and a loaded profile
- WHEN `silo_recommend` is invoked
- THEN the handler computes `free_minutes` from `free_slots` and passes it to `Engine.Recommend(profile, seeds, freeMinutes)`
- AND returns the engine's ranked output

---

### Requirement: Deterministic Output

Given the same `free_slots` array and the same vault state, `silo_recommend` MUST return identical output on repeated calls.

#### Scenario: Identical inputs produce identical output

- GIVEN `free_slots`, profile, and seed data are unchanged between two calls
- WHEN `silo_recommend` is called twice
- THEN both responses contain the same ranked list in the same order with the same scores

---

### Requirement: ProductiveHours Hint

`silo_recommend` SHOULD pass the `ProductiveHours` config field to the engine as a hint for scoring seeds that benefit from productive-window alignment.

#### Scenario: ProductiveHours configured

- GIVEN `ProductiveHours` is set in the Silo config
- WHEN `silo_recommend` processes a request
- THEN the engine receives `ProductiveHours` as a scoring hint
- AND seeds overlapping productive hours are ranked higher when all else is equal
