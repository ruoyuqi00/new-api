# Veo Auto Resolution Compatibility Design

## Goal

Allow downstream clients that send `resolution: "Auto"` for the Cangyuan
Veo 3.1 models to submit a valid video task, while preserving the real
upstream validation error when a task cannot be retried.

## Evidence

Production request `202607241340417809923538268d9d6T0QyRJIt` selected
channel `2360` for `veo-3-1-fast` in `下游多模态`. The upstream returned HTTP
422 because it accepts only `480p`, `720p`, or `1080p`, but received
`resolution: "Auto"`. The task retry then exhausted the only eligible channel
and replaced that parameter error with `get_channel_failed`.

## Design

The Sora task adaptor will normalize a top-level JSON `resolution` whose
trimmed value equals `auto` case-insensitively to `720p`, but only for the
`veo-3-1` model family. The gateway continues to overwrite the client model
with the configured upstream model. Other Sora-compatible models retain their
existing request body unchanged.

Task relay retry selection will stop for upstream HTTP 422 responses. This
keeps the original validation error visible instead of hiding it behind an
unrelated channel-selection error. Existing retry behavior for 401, 403, 429,
5xx, and configured retryable status codes remains unchanged.

The public developer guide will state that Veo accepts explicit `720p` or
`1080p` values and does not accept `Auto`.

## Verification

Regression tests will prove that an Auto value becomes `720p` only for
`veo-3-1-fast`, that non-Veo Sora bodies are unchanged, and that HTTP 422 task
errors are not retried. Production verification will use a no-charge request
without a prompt and confirm that routing reaches normal request validation
rather than `get_channel_failed`.
