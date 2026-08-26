# Group Settings Persistence and Availability Design

## Scope and priority

1. Restore saved user-specific group ratios and availability-monitoring switches whenever an administrator re-enters the group settings page.
2. Make the availability display tolerant of isolated failures and add a compact success/failure bar.

The work applies only to GPT text request monitoring. Image, video, and asynchronous task traffic remain excluded. No routing, retry, billing, channel status, database schema, or upstream-error projection behavior changes.

## Configuration persistence

The backend persists the two settings under their canonical option keys:

- `group_ratio_setting.user_group_ratio`
- `group_ratio_setting.availability_monitoring`

The default frontend settings model and the group form adapter must read those exact keys. The form continues to use the local field names `UserGroupRatio` and `AvailabilityMonitoring`, and saves through the existing canonical-key mapping. React Query invalidation remains the source of refreshed option data after a save.

A regression test will pass stored values under the canonical option keys through the group-settings adapter and assert that both values are restored into the form defaults.

## Availability window and status

Redis continues to retain only the newest 300 samples for each monitored group, with the existing two-hour TTL. The UI and API describe this explicitly as “recent X / 300 text requests.”

Status is derived from the same bounded window:

- 0 samples: `no_data`
- 1-19 samples: `observing`
- 20 or more samples and success rate at least 90%: `stable`
- 20 or more samples and success rate at least 60% but below 90%: `degraded`
- 20 or more samples and success rate below 60%: `unavailable`

There is no consecutive-failure override. A group cannot become unavailable before 20 valid samples have accumulated.

## Presentation

Each visible group card shows:

- group name and optional description;
- tolerant status label;
- recent sample count as X / 300;
- success rate;
- one horizontal stacked bar, green for successful requests and red for failed requests.

The bar is an aggregate of the current bounded window, not a latency chart or time series. It exposes no model, channel, account, upstream URL, key, request content, or error details.

## Validation

- Frontend regression test for canonical option-key hydration.
- Backend table tests for 0, 1-19, stable, degraded, unavailable, and 300-sample truncation behavior.
- Frontend presentation tests for observing status and success/failure bar proportions.
- Existing group availability permission/privacy tests remain green.
- Typecheck, focused lint, frontend build, and affected Go package tests must pass before a candidate is shown locally.
