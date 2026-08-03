# Group-level Sensitive Input Policy

## Goal

Allow administrators to decide, for each pricing group, whether the local
sensitive-word input check is enforced. Groups with the check disabled must
continue through the normal relay and billing flow without local sensitive-word
scanning.

## Scope

This policy applies only to the local prompt sensitive-word check controlled by
the existing sensitive-word settings. It does not change input moderation,
channel retry behavior, model pricing, group ratios, or any other safety or
billing feature.

## Behavior

- Every pricing group exposes a `Sensitive input check` toggle in the visual
  group editor.
- New groups default to enabled.
- Existing groups without an explicit stored value are treated as enabled.
- When enabled, the current behavior is preserved: matching input is blocked
  and the configured local violation fee is charged.
- When disabled, the application does not run the local sensitive-word scan for
  that request. The request continues normally and is billed with the existing
  model price and effective group ratio.
- The global sensitive-word switches remain the outer gate. A group toggle
  cannot enable scanning when the global feature or prompt checking is disabled.

## Effective Group

The policy must use the group that will actually serve the request. When
automatic cross-group selection has populated `auto_group`, that group takes
precedence over the initially requested group. This decision must happen before
building combined prompt text and before calling the local sensitive-word
matcher so exempt groups genuinely avoid the scan cost.

## Configuration

Store a boolean map keyed by pricing-group name alongside the existing group
settings. The backend owns the backward-compatible default: a missing key means
enabled. The frontend serializes explicit values for visible pricing groups and
keeps the map synchronized when groups are added, renamed, duplicated, or
removed.

The group ratio remains a number-only map. The new policy is stored separately
so existing group-ratio parsing, public-group authorization, and database
options remain compatible.

## Admin Interface

Add one compact toggle column to the pricing-group table. The label and helper
text must make the distinction explicit:

- Enabled: local sensitive input is blocked and charged as a violation.
- Disabled: local sensitive-word checking is skipped; normal usage billing
  still applies.

All user-facing text must use the existing i18n system for every supported
locale.

## Request Flow

1. Parse the request and generate relay metadata as today.
2. Resolve the effective group from `auto_group` when present, otherwise use
   `relayInfo.UsingGroup`.
3. Combine the global sensitive-word setting with the effective group's policy.
4. Only when both are enabled, build the prompt text needed for the local scan
   and run the matcher.
5. Preserve the existing token estimation, pricing, violation charge, relay,
   and normal settlement paths.

## Validation

Backend regression tests must prove:

- An enabled group still blocks matching input and enters the existing
  violation-charge path.
- A disabled group skips local matching and continues through normal request
  processing.
- A missing group-policy entry defaults to enabled.
- `auto_group` controls the policy when it differs from the initial group.
- Global sensitive-word switches still disable all local scanning.

Frontend tests must prove that group rows parse and serialize policy values,
default missing entries to enabled, and keep the policy map correct through
group lifecycle operations.

## Deployment Safety

The configuration default is fail-closed for compatibility: deployment alone
does not exempt any production group. Administrators must explicitly disable
the check for selected groups after the new version is live.
