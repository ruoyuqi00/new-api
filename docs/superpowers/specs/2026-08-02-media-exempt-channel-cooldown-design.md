# Media-Exempt Channel Cooldown Design

## Goal

Apply a 10-second transient cooldown after retryable upstream failures so text requests stop repeatedly selecting an unhealthy channel, while image and video request paths remain exempt.

## Behavior

- Keep channel affinity enabled and preserve one retry.
- Cool a channel pool for 10 seconds after the existing retryable statuses: 409, 425, 429, 529, and retryable 5xx responses.
- Continue scoping cooldown state by channel, group, and model.
- Exclude OpenAI image generation and editing, all Midjourney media modes, and video submit or fetch modes from channel cooldown.
- Long request duration alone never triggers cooldown. Only an eligible upstream error can create a cooldown entry.
- After the Redis TTL expires, the channel automatically returns to selection.

## Implementation

Add a relay-mode eligibility check to `service/channel_pool.go` before the existing cooldown mutation. Keep the existing error classification and Redis TTL implementation unchanged. Configure existing production channels with `channel_pool_cooldown_seconds=10`; the relay-mode policy makes that value inert for image and video requests, including mixed-capability channels.

## Verification

- A text request receiving a retryable 5xx creates a cooldown entry.
- Image, Midjourney, and video relay modes do not create cooldown entries for the same error.
- Existing service and relay tests pass.
- Production retains healthy containers, affinity enabled, and `RetryTimes=1` after deployment.
