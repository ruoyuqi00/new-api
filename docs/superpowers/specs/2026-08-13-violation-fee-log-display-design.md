# Violation Fee Log Display Design

## Goal

Make failed violation-fee audit records unambiguous. A record with
`charge_succeeded=false` must say that the request was blocked but charging
failed, rather than presenting a misleading zero-value fee.

## Scope

- Change only the default frontend usage-log presentation.
- Keep successful violation-fee records showing the amount actually charged.
- For failed charges, show a localized failure status and the requested charge
  amount when `requested_quota` is available.
- Preserve the current display for legacy records that do not contain
  `charge_succeeded`.
- Apply the same semantics in the table detail text and the log details dialog.
- Add all new user-facing text to all six supported locales through the project
  i18n scripts.

## Explicit Non-Goals

- No billing or quota calculation changes.
- No database schema or stored-log changes.
- No API contract changes.
- No Caddy, container, deployment, or production traffic changes.
- No unrelated layout, theme, branding, or classic frontend changes.

## Display Rules

| Record | Summary | Amount label | Amount source |
| --- | --- | --- | --- |
| `charge_succeeded=false` | Violation blocked, charge failed | Attempted fee | `requested_quota`, falling back to `log.quota` |
| `charge_succeeded=true` | Violation fee | Fee | `charged_quota`, falling back to `fee_quota`, then `log.quota` |
| Field absent | Existing violation fee presentation | Fee | `fee_quota`, then `log.quota` |

The failure state remains visible as an audit record. It is not hidden and it
must not imply that money was deducted.

## Testing

- Add a deterministic unit test for the display-state formatter before changing
  production code.
- Cover successful, failed, and legacy records, including amount fallback.
- Run the focused frontend test, typecheck, lint on touched files, i18n sync,
  and the production frontend build.
- Start the local branded candidate only after all checks pass, for user review
  before any production action.
