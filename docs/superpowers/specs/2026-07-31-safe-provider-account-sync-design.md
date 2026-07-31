# Safe Provider Account Sync Design

## Purpose

Make YuAPI the source of truth for provider-account scheduling while allowing repeated Sub2API exports or refreshed OAuth credentials to be imported safely. Replenishing an existing account must not undo operator decisions made in YuAPI.

## Current Problem

The importer already matches Codex and Grok accounts by stable upstream identity and merges refresh tokens. After a match, however, it writes every imported field back to the existing row. A partial export can therefore re-enable a disabled account, reset priority or concurrency, clear cooldowns, or replace YuAPI-specific endpoint and model-routing settings.

## Import Contract

For a newly discovered identity, create the provider account using the normalized import values and existing defaults.

For an identity already present in the target pool:

- Refresh the credential using the existing provider-specific merge rules.
- Update the account name, credential type, expiry, metadata, and update timestamp from the normalized import.
- Preserve the YuAPI-owned operational fields: status, priority, weight, concurrency limit, cooldown seconds, base URL, and model mapping.
- Preserve the existing refresh token when the incoming Codex or Grok credential omits it, as today.
- Keep duplicate identities within one batch counted as skipped.

This is the only import mode in this change. An explicit destructive replace mode is deliberately deferred until there is a concrete operational need.

## API And UI

The import endpoint already returns `count`, `created`, `updated`, and `skipped`. The frontend API type will expose those fields and the success toast will report all three result categories.

The import dialog will state that matching accounts refresh credentials while YuAPI routing and status are retained. This makes repeated Sub2API imports understandable before the operator submits them.

## Error Handling

Import remains transactional. Any invalid account or database error rolls back the complete batch. Existing adapter matching, identity validation, expiry validation, and credential validation remain unchanged.

## Compatibility

No schema migration is required. The implementation uses the existing GORM update path and remains compatible with SQLite, MySQL, and PostgreSQL. Existing API clients remain compatible because response fields are additive and already emitted by the backend.

## Verification

- A regression test proves a matched Codex import refreshes source-owned data while preserving every YuAPI operational field.
- A regression test proves a newly created account still uses imported routing values.
- Existing Codex and Grok identity/refresh-token tests remain green.
- Frontend typecheck, lint, i18n sync, focused tests, and production build pass.
- Production deployment uses the existing blue/green procedure and does not change channel bindings or statuses.
