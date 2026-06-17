# Delta for config

## REMOVED Requirements

### Requirement: SchedulePath Field

(Reason: `schedule.json` is retired. No code path reads or writes it after `internal/schedule/` is deleted.)
(Migration: Remove `SchedulePath` field from `internal/config/config.go` and delete `DefaultSchedulePath()` helper. Any references in `cmd/silo/main.go` or tests must be removed in the same task.)

---

## MODIFIED Requirements

### Requirement: ProductiveHours Field

`ProductiveHours` MUST be retained in `internal/config` and passed to `internal/recommend.Engine` as a scoring hint.
(Previously: `ProductiveHours` was a config field available but used inconsistently — the MCP recommend handler did not read it. After this change, `silo_recommend` MUST read `ProductiveHours` from config and pass it to the engine.)

#### Scenario: ProductiveHours present in config

- GIVEN `ProductiveHours` is set in the user's Silo config file
- WHEN `silo_recommend` processes a request
- THEN the value is read from config and forwarded to `Engine.Recommend()` as a hint
- AND seeds scheduled within the productive window receive a scoring advantage

#### Scenario: ProductiveHours absent from config

- GIVEN `ProductiveHours` is not set (zero value or omitted)
- WHEN `silo_recommend` processes a request
- THEN the engine uses its default scoring without a productive-hours bias
- AND no error is returned
