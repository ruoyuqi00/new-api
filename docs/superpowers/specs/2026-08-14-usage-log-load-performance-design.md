# Usage Log Load Performance Design

## Context

The production MySQL `logs` table currently contains about 7.65 million rows,
including more than 1.1 million rows for the current day. The current desktop
usage-log page requests 100 rows and defaults to the current day. Read-only
production measurements showed that this results in approximately 24.8 seconds
for the page query and 19.6 seconds for the usage aggregate. The equivalent
queries over the most recent hour completed in approximately 0.2 seconds.

The database is not using an efficient ordered index for the current-day list
query. Adding or rebuilding an index on a roughly 10 GB table is operationally
risky and is deliberately excluded from this release.

## Goals

- Make the initial log page and normal refresh complete quickly under current
  production volume.
- Preserve access to full-day and historical logs through the existing date
  range controls.
- Reduce browser rendering work without changing the page layout or branding.
- Keep the change reversible through the existing container rollback process.

## Non-Goals

- Do not delete, archive, or rewrite existing log rows.
- Do not add, remove, or rebuild production database indexes in this release.
- Do not change billing, authentication, relay, cache-affinity, or log privacy
  behavior.
- Do not change the classic or default theme branding.

## Design

### Default query window

When the route contains no explicit start or end time, the frontend will query
from one hour before the current time through one hour after the current time.
The future allowance preserves the existing tolerance for small clock skew.
The date-range control displays the same values sent to the API. Applying a
manual range continues to use the selected timestamps without modification.

### Page size

The desktop log table will default to 30 rows instead of 100. Mobile remains at
20 rows. Users may still choose another supported page size through the existing
pagination control.

### Query and rendering behavior

The list and aggregate requests continue to use the same filter builder, so a
refresh cannot accidentally request a different statistics range. No automatic
polling is added. Existing retained-data behavior remains in place while a
manual refresh is in flight.

## Error Handling

Existing stale-data and retry behavior is unchanged. A failed refresh retains
the last successful page and shows the current error state. The optimization
does not alter API error bodies or expose database or upstream details.

## Verification

- A deterministic frontend test must prove the default range is rolling and
  explicit date ranges are preserved.
- Frontend type checking and production build must pass.
- A local candidate must show the production brand and request 30 rows over the
  rolling-hour range.
- Before cutover, the candidate health endpoint and log endpoints must pass.
- After hot cutover, verify container health/restart count, public UI, login,
  and aggregate log latency. Roll back immediately if these checks regress.

## Deferred database work

A later maintenance change may add new, correctly ordered indexes for log list
and aggregate queries after rehearsal against a production-size copy. That work
requires its own design, disk-space check, lock-impact assessment, and rollback
plan.
