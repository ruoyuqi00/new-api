# Sub2API CCS and Group Verification - 2026-06-01

## Scope

This pass checked the public key configuration flow, CC Switch import content,
live server deployment, group model exposure, and upstream merge status.

The local folder `D:\wflogin\注册机相关项目` was intentionally not inspected.

## Code Changes

- Added group-aware OpenAI/Codex model selection for CC Switch imports.
- CC Switch OpenAI imports now prefer the selected key group's model list and
  fall back to `gpt-5.5`.
- Claude/CCS imports include `sonnetModel`, `opusModel`, and `haikuModel`
  fields derived from the group's model list.
- The "Use Key" modal receives the selected key group's `models_list_config`,
  so displayed local client config matches the chosen group.
- API key DTOs now include group model list metadata needed by the frontend.

## Deployment

- Deployed image:
  `sub2api-provider-adapters:ccs-codex-model-20260601`
- Public URL:
  `https://api.vyywcw.cn`
- Server path:
  `/opt/sub2api`
- Container status after deployment:
  `sub2api` healthy

Temporary Kiro smoke keys created during testing were disabled and soft-deleted
after the test run. No API keys or provider secrets are recorded in this note.

## Verification

Local checks:

- `npm run test:run -- src/utils/__tests__/ccswitchImport.spec.ts`
- `npm run test:run -- src/components/keys/__tests__/UseKeyModal.spec.ts`
- `npm run build`

Live checks:

- `/v1/models` returned HTTP 200 for:
  - `provider-mixed`
  - `windsurf-opus4.6`
  - `windsurf-opus4.7`
  - `windsurf-gpt5.5`
  - `windsurf-gpt5.4`
  - `windsurf-grok`
  - `GPT5.5`
  - `kiro-opus4.6`
  - `kiro-opus4.7`
- `GPT5.5` with model `gpt-5.5` returned HTTP 200 and usable text.

## Current Group Status

Ready to use:

- `GPT5.5`: callable through the OpenAI-compatible endpoint with `gpt-5.5`.
- CC Switch/Codex import for this group now selects `gpt-5.5` instead of the
  old hardcoded `gpt-5.4`.

Configured but not currently callable end-to-end:

- `kiro-opus4.6`
- `kiro-opus4.7`

The Kiro adapter itself returned upstream text saying `Invalid model ID` for
`claude-sonnet-4.6`, `claude-opus-4.6`, and `claude-opus-4.7`. Direct adapter
smoke did return usable text for `auto`, `claude-sonnet-4`,
`qwen3-coder-next`, `glm-5`, `deepseek-3.2`, and `minimax-m2.1`. The deployed
Kiro account currently reports `KIRO FREE`, so this appears to be an upstream
Kiro account/model entitlement issue rather than a Sub2API model-list export
bug.

Configured but blocked by upstream account state:

- `windsurf-opus4.6`
- `windsurf-opus4.7`
- `windsurf-gpt5.5`
- `windsurf-gpt5.4`
- `windsurf-grok`

These groups expose the expected model lists through `/v1/models`, but the
bound Windsurf upstream account was not schedulable during this verification.

## Upstream Sub2API Status

Official `Wei-Shaw/sub2api` was fetched but not merged.

- Latest observed upstream head: `aa69e394`
- This private branch is behind upstream by 5 commits.
- New upstream work mainly covers:
  - Codex Responses to Chat Completions bridge redesign
  - Antigravity Gemini scheduling/rate-limit fixes

No merge was performed because the upstream diff intersects private
Kiro/Windsurf adapter areas. Merge should be handled separately with a clean
test pass.

## Operational Notes

- For today, the safest user-facing configuration is the `GPT5.5` group.
- Kiro Opus/Sonnet 4.6+ groups should remain treated as configured placeholders
  until a Kiro account or adapter path with those model entitlements is
  available.
- Windsurf groups should be retested after the Windsurf upstream account becomes
  schedulable again.
