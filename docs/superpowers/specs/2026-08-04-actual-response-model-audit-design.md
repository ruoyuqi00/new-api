# Actual Response Model Audit Design

## Goal

Persist the model identifier reported by the upstream response so administrators can compare the client request model, the final forwarded model, and the upstream-reported model without storing request or response content.

## Data Model

- Add nullable `logs.actual_response_model` with a maximum length of 100 characters.
- Keep `logs.model_name` as the client-facing request model.
- Keep configured forwarding information in the existing `other.upstream_model_name` field.
- Add runtime-only `RelayInfo.ForwardedModelName` and `RelayInfo.ActualResponseModel` values. The forwarded value is snapshotted before a provider response handler can mutate its model state.
- Do not index the new column. Audits are expected to use bounded time windows, and adding a high-cardinality index would increase write cost on the production log table.

## Capture Flow

Raw upstream payloads are inspected before model normalization or protocol conversion:

- JSON: capture `response.model`, top-level `model`, `message.model`, or `session.model` from OpenAI-compatible, Responses, Claude, and xAI handlers.
- SSE: capture from the raw JSON event in the shared stream scanner before the event reaches provider-specific conversion code.
- WebSocket: capture from each raw upstream message before forwarding it to the client.

The first non-empty model wins. Once captured, later stream chunks are not parsed for this audit field. Values are trimmed and limited to 100 Unicode code points. Missing or invalid JSON leaves the field empty and does not interrupt the relay.

## Persistence And Visibility

Consumption logging copies `RelayInfo.ActualResponseModel` into the nullable log column for text, audio, and realtime requests. SQLite, MySQL, and PostgreSQL use GORM `AutoMigrate`; ClickHouse receives an idempotent `ADD COLUMN IF NOT EXISTS` migration.

The admin log API returns the field. User log formatting clears it before serialization. The admin details view displays:

1. Request Model
2. Forwarded Model, when model mapping occurred
3. Upstream Response Model, when the upstream reported one

The table model badge opens the same audit trail without changing the displayed public model.

## Failure Behavior

Model extraction is best-effort and cannot fail a user request. Invalid JSON, absent model fields, or unsupported response shapes leave the audit field null. Database migration failure retains the existing startup failure behavior so the service cannot silently run with an incompatible schema.

## Verification

- Unit tests cover all supported JSON paths, invalid or absent model fields, first-value retention, truncation, and response normalization remaining unchanged.
- Model tests cover nullable persistence and user-facing redaction.
- ClickHouse schema tests cover both new-table and existing-table migration SQL.
- Frontend tests cover request, forwarded, and upstream response model formatting.
- Production verification sends a non-sensitive authorized probe, queries only model identifiers, and confirms no response body or request content is stored by this feature.
