# YuAPI Video API: Generate Your First Video from Scratch

> Your website password is not an API Key. Before calling the API, sign in to YuAPI and create a separate API Key at [`/keys`](/keys). Never place your website password in source code, a terminal command, or a third-party client.

For your first test, select the `多模态创作` group, allow only `seedance-2.0`, and set a short expiration and a small quota. After the workflow succeeds, create a separate production Key for your application.

## 1. Before You Start: Account, API Key, and Task ID

One video generation uses three different values:

| Value | Purpose | Safe to publish? |
| --- | --- | --- |
| Website account and password | Sign in, manage balance, and manage Keys | No |
| API Key | Sent in the `Authorization: Bearer ...` header | No |
| Task ID | Poll the original task and download its result | Share only with trusted support staff when needed |

YuAPI exposes two Base URLs:

| Purpose | Base URL |
| --- | --- |
| Model discovery and ordinary APIs | `https://api.yuaiapi.com/v1` |
| Images, videos, and long-running tasks | `https://vip.yuaiapi.com/v1` |

Both addresses use the same API Keys, model list, account balance, and prices. Your client must select the intended Base URL explicitly. The system does not automatically switch, redirect, or replace either address. All image and video examples in this guide use the second address.

## 2. Create Your First API Key

1. Sign in to YuAPI.
2. Open [`/keys`](/keys).
3. Select **Create API Key**.
4. Enter a descriptive name such as `video-quickstart`.
5. Select the `多模态创作` group shown in the interface.
6. For a first test, use a 24-hour expiration and a `25.00` quota limit.
7. Enable the model restriction and select only `seedance-2.0`.
8. Store the Key immediately after creation. The complete value may not be shown again after you close the dialog.

![Create API Key action on the API Keys page](/developer-docs/assets/video-api-key-en-01.webp)

![Test Key name, group, expiration, and quota controls](/developer-docs/assets/video-api-key-en-02.webp)

![Model restriction allowing only seedance-2.0](/developer-docs/assets/video-api-key-en-03.webp)

Do not include a complete Key in a chat, support ticket, or screenshot. Every example below reads `YUAPI_API_KEY` from the environment instead of embedding it in source code.

To call the additional video models listed later on this page, select each complete model ID in the same model restriction, such as `minimax-h3` or `seedance2.0-fast-PT`. Model IDs include significant punctuation and version numbers; do not replace them with display names.

> Advanced note: some accounts may also show a group named `下游多模态`. It is another selectable group name and does not change the request paths or task protocol in this guide. First-time users should select `多模态创作`.

## 3. Verify the API Key

Start by listing models. This request does not create a video.

Windows PowerShell:

```powershell
$env:YUAPI_API_KEY = Read-Host "Enter your API Key"
$env:YUAPI_BASE_URL = "https://api.yuaiapi.com/v1"

$headers = @{ Authorization = "Bearer $env:YUAPI_API_KEY" }
$models = Invoke-RestMethod `
  -Uri "$env:YUAPI_BASE_URL/models" `
  -Headers $headers `
  -Method Get

$models.data | Where-Object id -eq "seedance-2.0"
```

macOS/Linux:

```bash
read -rsp "YuAPI API Key: " YUAPI_API_KEY && echo
export YUAPI_API_KEY
export YUAPI_BASE_URL="https://api.yuaiapi.com/v1"
export YUAPI_MEDIA_BASE_URL="https://vip.yuaiapi.com/v1"

curl --fail-with-body "$YUAPI_BASE_URL/models" \
  -H "Authorization: Bearer $YUAPI_API_KEY"
```

The response should include `seedance-2.0`. For `401`, verify that the Key was copied in full. For `403`, or when the model is absent, check the Key's group and model restriction.

## 4. Create a Video with seedance-2.0

Create tasks with `POST /v1/videos`. This first request is prompt-only and needs no reference media:

```json
{
  "model": "seedance-2.0",
  "prompt": "A wooden boardwalk beside the sea at dawn, slow forward camera movement, soft natural light, realistic cinematic look",
  "duration": 4,
  "aspect_ratio": "16:9",
  "generate_audio": true
}
```

```bash
curl --fail-with-body -X POST "$YUAPI_MEDIA_BASE_URL/videos" \
  -H "Authorization: Bearer $YUAPI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "seedance-2.0",
    "prompt": "A wooden boardwalk beside the sea at dawn, slow forward camera movement, soft natural light, realistic cinematic look",
    "duration": 4,
    "aspect_ratio": "16:9",
    "generate_audio": true
  }'
```

A successful response contains `id` or `task_id`. Persist it in your database or job record immediately. A successful create response means that the task entered the queue; it does not mean that the video is complete.

## 5. Poll the Original Task and Download the Video

Always poll with the task ID returned by the create request:

```bash
export TASK_ID="paste the task ID from the create response"

curl "$YUAPI_MEDIA_BASE_URL/videos/$TASK_ID" \
  -H "Authorization: Bearer $YUAPI_API_KEY"
```

When the status is `completed`, `succeeded`, or `success`, download the content:

```bash
curl -L "$YUAPI_MEDIA_BASE_URL/videos/$TASK_ID/content" \
  -H "Authorization: Bearer $YUAPI_API_KEY" \
  --output result.mp4
```

If your client times out, or the task remains `queued` or `processing`, do not submit the create request again. Keep polling the same task ID. Repeating `POST /v1/videos` creates another task and may charge again.

## 6. Turn a Test Key into a Production-Safe Setup

Create a new production Key after testing instead of keeping the test Key indefinitely:

- Use a separate Key for every application and environment.
- Allow only the models the application actually calls.
- Set an acceptable quota limit and expiration, then rotate before expiration.
- Enable an IP restriction when your service has stable outbound addresses.
- Store the Key only in server-side environment variables or a secrets manager.
- Never place it in browser JavaScript, a mobile package, a public repository, or client logs.
- If a Key leaks, delete it, issue a replacement, and review usage records.

## 7. Video Models and Billing

The following table lists video models, billing units, and resolution capabilities for the `多模态创作` group. Polling, reading status, and downloading the same task do not charge again. Amounts may change with configuration; use the [model marketplace](/pricing) as the live source.

<!-- video-model-catalog:start -->
| Model | Billing | Resolution |
| --- | --- | --- |
| `grok-video` | `per_successful_task` | `model_default` |
| `grok-video-1.5` | `per_successful_task` | `model_default` |
| `happyhouse-1.0` | `per_successful_task` | `model_default` |
| `happyhouse-1.1` | `per_successful_task` | `model_default` |
| `minimax-h3-2k` | `per_successful_task` | `2K` |
| `omni-fast` | `per_successful_task` | `model_default` |
| `omni-fast-no-water` | `per_successful_task` | `model_default` |
| `omni-v2v` | `per_successful_task` | `model_default` |
| `omni-v2v-no-water` | `per_successful_task` | `model_default` |
| `sd7-seedance-2.0-1080p` | `per_successful_task` | `1080p` |
| `sd7-seedance-2.0-720p` | `per_successful_task` | `720p` |
| `sd8-seedance-2.0` | `per_successful_task` | `model_default` |
| `seedance-2.0` | `per_successful_task` | `model_default` |
<!-- video-model-catalog:end -->

Per-video billing is separate from text Token billing and per-image billing. Do not apply text `usage`, cache-hit, or stream-interruption rules to video tasks.

### Additional Video Models

The following 11 models use the same `POST /v1/videos`, task polling, and content download endpoints. `per_1m_video_tokens` means per one million video Tokens, `per_second` means per second, and `per_successful_task` means per successful task.

<!-- expanded-video-model-catalog:start -->
| Model ID | Billing unit | Resolution tier |
| --- | --- | --- |
| `seedance-2-0-mini-official` | `per_1m_video_tokens` | `480p/720p` |
| `seedance-2-0-fast-official` | `per_1m_video_tokens` | `480p/720p` |
| `seedance-2-0-official` | `per_1m_video_tokens` | `480p/720p/1080p/4K` |
| `seedance-2-5-official` | `per_1m_video_tokens` | `720p/1080p` |
| `minimax-h3` | `per_second` | `480p/768p/1080p/2K/4K` |
| `wan3.0-video` | `per_second` | `480p/720p/1080p` |
| `wan3.0-video-prime` | `per_second` | `480p/720p/1080p` |
| `seedance2.0-9-3-3-PT` | `per_second` | `480p/720p` |
| `seedance2.5-30-10-10-PT` | `per_second` | `480p/720p` |
| `seedance2.0-fast-PT` | `per_second` | `480p/720p` |
| `grok-v1.5-video` | `per_successful_task` | `720p/1080p` |
<!-- expanded-video-model-catalog:end -->

Billing and parameter rules:

- Official Seedance Token models are billed from final video Tokens. Requests with a reference video use the separate reference-video tier.
- `seedance-2-0-mini-official`, `seedance-2-0-fast-official`, and `seedance-2-0-official` support 4-15 seconds. `seedance-2-5-official` supports 4-30 seconds. These models also accept `-1` to let the service choose the duration automatically.
- `minimax-h3` is billed by output seconds and resolution, with no time-of-day discount.
- Wan 3.0 bills output duration plus reference-video duration. The three PT models bill output duration only.
- `grok-v1.5-video` is billed per successful task; duration does not multiply the billed quantity.
- Wan 3.0 supports 2-30 seconds. `seedance2.0-9-3-3-PT` and `seedance2.0-fast-PT` support 5-15 seconds. `seedance2.5-30-10-10-PT` supports 5-30 seconds. `grok-v1.5-video` supports 4-15 seconds.

Every create request should send a stable, unique `Idempotency-Key`. Reuse the same value when retrying the same business request, and use a new value for a new video task. After a successful submission, store the task ID and poll only that task instead of creating duplicates while it is still running.

#### Official Seedance Token Model

```bash
curl --fail-with-body -X POST "$YUAPI_MEDIA_BASE_URL/videos" \
  -H "Authorization: Bearer $YUAPI_API_KEY" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: video-order-20260920-001" \
  -d '{
    "model": "seedance-2-0-mini-official",
    "prompt": "A seaside boardwalk at sunrise, slow forward camera movement",
    "duration": 5,
    "resolution": "720p",
    "ratio": "16:9"
  }'
```

#### Wan 3.0

```bash
curl --fail-with-body -X POST "$YUAPI_MEDIA_BASE_URL/videos" \
  -H "Authorization: Bearer $YUAPI_API_KEY" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: video-order-20260920-002" \
  -d '{
    "model": "wan3.0-video",
    "prompt": "A rain-soaked city street, slow cinematic lateral movement",
    "duration": 5,
    "resolution": "720p",
    "aspect_ratio": "16:9"
  }'
```

#### MiniMax H3

H3 requires `workflow_id`, `seconds`, and an exact `size`. For example, use `1920x1088` for landscape 1080p instead of supplying only an aspect ratio. Durations from 4-15 seconds are supported. Common exact sizes are:

- 480p: `864x480`, `480x864`, `640x640`, `544x800`, `800x544`, `576x736`, `736x576`, `992x416`
- 768p: `1376x768`, `768x1376`, `1024x1024`, `832x1248`, `1248x832`, `896x1184`, `1184x896`, `1568x672`
- 1080p: `1920x1088`, `1088x1920`, `1440x1440`, `1184x1760`, `1760x1184`, `1248x1664`, `1664x1248`, `2208x960`
- Higher tiers: `2K`, `4K`

```bash
curl --fail-with-body -X POST "$YUAPI_MEDIA_BASE_URL/videos" \
  -H "Authorization: Bearer $YUAPI_API_KEY" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: video-order-20260920-003" \
  -d '{
    "model": "minimax-h3",
    "prompt": "A sailboat crossing a golden sea, stable aerial camera",
    "workflow_id": "text-to-video",
    "seconds": 5,
    "size": "1920x1088"
  }'
```

#### Grok Video 1.5

```bash
curl --fail-with-body -X POST "$YUAPI_MEDIA_BASE_URL/videos" \
  -H "Authorization: Bearer $YUAPI_API_KEY" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: video-order-20260920-004" \
  -d '{
    "model": "grok-v1.5-video",
    "prompt": "A futuristic city under neon lights, forward-moving camera",
    "seconds": 6,
    "size": "1080p",
    "aspect_ratio": "16:9"
  }'
```

#### Seedance PT

```bash
curl --fail-with-body -X POST "$YUAPI_MEDIA_BASE_URL/videos" \
  -H "Authorization: Bearer $YUAPI_API_KEY" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: video-order-20260920-005" \
  -d '{
    "model": "seedance2.0-fast-PT",
    "prompt": "A forest path with sunlight through the leaves, steady forward camera",
    "duration": 9,
    "resolution": "720p",
    "ratio": "16:9",
    "generate_audio": true
  }'
```

### Grok Imagine asynchronous video

The following three models are independent asynchronous video models billed by generated second with your group multiplier applied. The default duration is 5 seconds when omitted, and integer durations from 1 through 15 seconds are supported. `size` may be `480p`, `720p`, or `1080p`, or dimensions containing the corresponding height, such as `1280x720`. See the [model marketplace](/pricing) for live amounts.

| Model | Billing unit | Supported resolutions |
| --- | --- | --- |
| `grok-imagine-video` | `per_second` | `480p/720p/1080p` |
| `grok-imagine-video-1.5` | `per_second` | `480p/720p/1080p` |
| `grok-imagine-video-1.5-preview` | `per_second` | `480p/720p/1080p` |

Save the task ID returned by creation and use the status endpoint to poll it. During `queued` or `processing`, poll the original task instead of creating another one. A duration or resolution outside the supported range returns `400` before a task is submitted.

```json
{
  "model": "grok-imagine-video",
  "prompt": "A vintage sports car on a coastal road, smooth tracking shot",
  "seconds": 5,
  "size": "1280x720"
}
```

## 8. Video Task Protocol

The four common paths are:

```text
GET  https://api.yuaiapi.com/v1/models
POST https://vip.yuaiapi.com/v1/videos
GET  https://vip.yuaiapi.com/v1/videos/{task_id}
GET  https://vip.yuaiapi.com/v1/videos/{task_id}/content
```

Example create response:

```json
{
  "id": "task_example_01",
  "status": "queued"
}
```

| Status | Action |
| --- | --- |
| `queued`, `processing` | Wait 5-10 seconds, then poll the same ID |
| `completed`, `succeeded`, `success` | Read the result or request `/content` |
| `failed`, `canceled`, `cancelled` | Stop polling and record the error and Request ID |

The result can appear in `video_url`, `metadata.video_url`, `metadata.url`, or `data[0].url`. For failure details, check `error.message`, `reason`, and `message` in that order.

## 9. Model Parameters and Reference Media Limits

| Model | Duration | Resolution | Reference media and notes |
| --- | --- | --- | --- |
| `grok-video` | 4, 6, 8, 10, 12, or 15 seconds | 480p or 720p | Up to 1 reference image |
| `grok-video-1.5` | 4, 6, 8, 10, 12, or 15 seconds | 480p or 720p | Up to 7 reference images |
| `happyhouse-1.0` | 3-15 seconds | 720p or 1080p | Up to 9 images; or one 3-10 second video with up to 5 images; supports `generate_audio` |
| `happyhouse-1.1` | 3-15 seconds | 720p or 1080p | Up to 9 images; supports `generate_audio` |
| `minimax-h3-2k` | 5-15 seconds | Fixed 2K | Up to 5 images and 3 audio files, no more than 8 total |
| `omni-fast*` | Fixed at about 10 seconds | Fixed 720p | Do not send duration, resolution, or audio generation fields |
| `omni-v2v*` | Fixed at about 10 seconds | Fixed 720p | Exactly one source video is required |
| `seedance-2.0` | 4-15 seconds | Fixed 720p | Up to 5 images, 3 videos, and 3 audio files, no more than 11 total |
| `sd7-seedance-2.0-*` | 4-15 seconds | Fixed by model ID | Up to 5 images, 3 videos, and 3 audio files, no more than 11 total |
| `sd8-seedance-2.0` | 5, 10, or 15 seconds | Model-defined | Up to 9 images, 3 videos, and 3 audio files; do not send `resolution` or `generate_audio` |

Reference URLs must be HTTPS resources the server can read without cookies, login state, or a Referer header. Do not record private media URLs in production logs.

```json
{
  "reference_image_urls": ["https://assets.example.com/reference/person.png"],
  "reference_videos": ["https://assets.example.com/reference/motion.mp4"],
  "reference_audios": ["https://assets.example.com/reference/ambient.mp3"]
}
```

## 10. Complete Windows PowerShell Example

```powershell
$mediaBase = "https://vip.yuaiapi.com/v1"
$headers = @{
  Authorization = "Bearer $env:YUAPI_API_KEY"
  "Content-Type" = "application/json"
}
$body = @{
  model = "seedance-2.0"
  prompt = "A boardwalk beside the sea at dawn with slow forward camera motion"
  duration = 4
  aspect_ratio = "16:9"
  generate_audio = $true
} | ConvertTo-Json

$created = Invoke-RestMethod -Uri "$mediaBase/videos" -Method Post -Headers $headers -Body $body
$taskId = if ($created.id) { $created.id } else { $created.task_id }
if (-not $taskId) { throw "Create response did not include a task ID" }
$taskId | Set-Content -Encoding utf8 .\video-task-id.txt
$deadline = (Get-Date).AddMinutes(15)

do {
  Start-Sleep -Seconds 5
  $task = Invoke-RestMethod -Uri "$mediaBase/videos/$taskId" -Headers @{
    Authorization = "Bearer $env:YUAPI_API_KEY"
  }
  $status = [string]$task.status
} while ($status -in @("queued", "processing") -and (Get-Date) -lt $deadline)

if ($status -notin @("completed", "succeeded", "success")) {
  throw "Task is not complete; keep polling the saved task ID: $taskId"
}
Invoke-WebRequest -Uri "$mediaBase/videos/$taskId/content" -Headers @{
  Authorization = "Bearer $env:YUAPI_API_KEY"
} -OutFile .\result.mp4
```

## 11. Complete macOS/Linux curl Example

```bash
set -euo pipefail
: "${YUAPI_API_KEY:?set YUAPI_API_KEY first}"
YUAPI_MEDIA_BASE_URL="https://vip.yuaiapi.com/v1"

created_json="$(curl --fail-with-body -sS -X POST "$YUAPI_MEDIA_BASE_URL/videos" \
  -H "Authorization: Bearer $YUAPI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "seedance-2.0",
    "prompt": "A boardwalk beside the sea at dawn with slow forward camera motion",
    "duration": 4,
    "aspect_ratio": "16:9",
    "generate_audio": true
  }')"

TASK_ID="$(printf '%s' "$created_json" | python3 -c \
  'import json,sys; d=json.load(sys.stdin); print(d.get("id") or d.get("task_id") or "")')"
test -n "$TASK_ID" || { echo "Create response did not include a task ID" >&2; exit 1; }
printf '%s\n' "$TASK_ID" > video-task-id.txt

deadline=$((SECONDS + 900))
while (( SECONDS < deadline )); do
  task_json="$(curl --fail-with-body -sS "$YUAPI_MEDIA_BASE_URL/videos/$TASK_ID" \
    -H "Authorization: Bearer $YUAPI_API_KEY")"
  status="$(printf '%s' "$task_json" | python3 -c \
    'import json,sys; print(str(json.load(sys.stdin).get("status", "")).lower())')"
  case "$status" in
    completed|succeeded|success) break ;;
    failed|canceled|cancelled) echo "$task_json" >&2; exit 1 ;;
  esac
  sleep 5
done

case "$status" in completed|succeeded|success) ;; *) exit 1 ;; esac
curl --fail-with-body -L "$YUAPI_MEDIA_BASE_URL/videos/$TASK_ID/content" \
  -H "Authorization: Bearer $YUAPI_API_KEY" --output result.mp4
```

## 12. Complete Python Example

Install the dependency with `python -m pip install requests`.

```python
import os
import time
from urllib.parse import urljoin

import requests

base_url = "https://vip.yuaiapi.com/v1"
api_origin = base_url.removesuffix("/v1")
headers = {"Authorization": f"Bearer {os.environ['YUAPI_API_KEY']}"}

created = requests.post(
    f"{base_url}/videos",
    headers=headers,
    json={
        "model": "seedance-2.0",
        "prompt": "A boardwalk beside the sea at dawn with slow forward camera motion",
        "duration": 4,
        "aspect_ratio": "16:9",
        "generate_audio": True,
    },
    timeout=180,
)
created.raise_for_status()
body = created.json()
task_id = body.get("id") or body.get("task_id")
if not task_id:
    raise RuntimeError("Create response did not include a task ID")

SUCCESS = {"completed", "succeeded", "success"}
FAILURE = {"failed", "canceled", "cancelled"}
deadline = time.monotonic() + 15 * 60
task = {}
while time.monotonic() < deadline:
    response = requests.get(f"{base_url}/videos/{task_id}", headers=headers, timeout=60)
    response.raise_for_status()
    task = response.json()
    status = str(task.get("status", "")).lower()
    if status in SUCCESS:
        break
    if status in FAILURE:
        raise RuntimeError(f"video task failed: {task}")
    time.sleep(5)
else:
    raise TimeoutError(f"video task still processing: {task_id}")

metadata = task.get("metadata") or {}
data = task.get("data") or []
video_url = task.get("video_url") or metadata.get("video_url") or metadata.get("url") or (data[0].get("url") if data else None)
if video_url:
    print(urljoin(api_origin, video_url))

content = requests.get(f"{base_url}/videos/{task_id}/content", headers=headers, timeout=180)
content.raise_for_status()
with open("result.mp4", "wb") as output:
    output.write(content.content)
```

## 13. Node.js Server Example

Run this example only on a trusted server. Never put it in browser JavaScript. Node.js 20 and newer include `fetch`.

```javascript
import { writeFile } from 'node:fs/promises'

const baseUrl = 'https://vip.yuaiapi.com/v1'
const apiKey = process.env.YUAPI_API_KEY
if (!apiKey) throw new Error('YUAPI_API_KEY is required')
const authHeaders = { Authorization: `Bearer ${apiKey}` }

const createdResponse = await fetch(`${baseUrl}/videos`, {
  method: 'POST',
  headers: { ...authHeaders, 'Content-Type': 'application/json' },
  body: JSON.stringify({
    model: 'seedance-2.0',
    prompt: 'A boardwalk beside the sea at dawn with slow forward camera motion',
    duration: 4,
    aspect_ratio: '16:9',
    generate_audio: true,
  }),
})
if (!createdResponse.ok) throw new Error(await createdResponse.text())
const created = await createdResponse.json()
const taskId = created.id ?? created.task_id
if (!taskId) throw new Error('Create response did not include a task ID')

const deadline = Date.now() + 15 * 60 * 1000
let status = ''
while (Date.now() < deadline) {
  const response = await fetch(`${baseUrl}/videos/${taskId}`, { headers: authHeaders })
  if (!response.ok) throw new Error(await response.text())
  const task = await response.json()
  status = String(task.status ?? '').toLowerCase()
  if (['completed', 'succeeded', 'success'].includes(status)) break
  if (['failed', 'canceled', 'cancelled'].includes(status)) throw new Error(JSON.stringify(task))
  await new Promise((resolve) => setTimeout(resolve, 5000))
}
if (!['completed', 'succeeded', 'success'].includes(status)) throw new Error(`Task is still processing: ${taskId}`)

const content = await fetch(`${baseUrl}/videos/${taskId}/content`, { headers: authHeaders })
if (!content.ok) throw new Error(await content.text())
await writeFile('result.mp4', Buffer.from(await content.arrayBuffer()))
```

## 14. Status, Errors, and Safe Retries

| HTTP status | Common cause | Correct action |
| --- | --- | --- |
| `400` | A parameter or reference does not meet model requirements | Correct the request; do not replay it unchanged |
| `401` | The Key is invalid, expired, or deleted | Check the Bearer header and rotate the Key |
| `403` | Group, model permission, quota, or account state blocks the request | Review the Key and account settings |
| `404` | The model or task does not exist | List models again and verify the original task ID |
| `429` | Concurrency or rate limit | Reduce concurrency and apply exponential backoff |
| `500/502/503/504` | Temporary service failure or long-task timeout | Poll a saved task ID first, then decide whether a new task is needed |

Poll every 5-10 seconds and set an application deadline. Logs may contain timestamps, paths, model IDs, task IDs, status codes, and Request IDs. They must not contain complete Keys, website passwords, or private media.

## 15. Pre-Integration Checklist

- [ ] Created an API Key at `/keys` instead of using the website password.
- [ ] Restricted the test Key to the required models.
- [ ] Confirmed the model ID with `GET /v1/models`.
- [ ] Used `https://vip.yuaiapi.com/v1` for image and video requests.
- [ ] Persisted the task ID from the create response.
- [ ] Poll the original task during `queued` or `processing`; never create a duplicate task as a polling strategy.
- [ ] Configured a separate production quota, expiration, model list, and optional IP restriction.
- [ ] Kept the Key only on a trusted server.
- [ ] Confirmed reference media is directly readable and within model limits.
- [ ] Handles success, failure, cancellation, and timeout states.

## 16. Image API Reference

Image generation is synchronous and does not use the video polling workflow:

- Generation: `POST /v1/images/generations`
- Editing: `POST /v1/images/edits`
- Request only `n=1`
- Read `data[0].url` or `data[0].b64_json` from a successful response

| Model | Fixed tier |
| --- | --- |
| `gpt-image-2-1k` | `1K` |
| `gpt-image-2-2k` | `2K` |
| `gpt-image-2-4k` | `4K` |
| `nano-banana-pro-1k` | `1K` |
| `nano-banana-pro-2k` | `2K` |
| `nano-banana-pro-4k` | `4K` |
| `nano-banana2-1k` | `1K` |
| `nano-banana2-2k` | `2K` |
| `nano-banana2-4k` | `4K` |
| `grok-imagine-image` | `standard` |
| `grok-imagine-image-quality` | `high_quality` |

Images are billed by the actual successful image count with your group multiplier applied. See the [model marketplace](/pricing) for live amounts.

`grok-4.5` is a text model. `grok-imagine-image` and `grok-imagine-image-quality` are image models. `grok-video` and `grok-video-1.5` are asynchronous video models. Always use the exact model ID and its corresponding endpoint.

```bash
curl --fail-with-body -X POST "$YUAPI_MEDIA_BASE_URL/images/generations" \
  -H "Authorization: Bearer $YUAPI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2-1k",
    "prompt": "A cinematic city street after rain, realistic photography, no text",
    "n": 1,
    "response_format": "url"
  }'
```

```bash
curl --fail-with-body -X POST "$YUAPI_MEDIA_BASE_URL/images/edits" \
  -H "Authorization: Bearer $YUAPI_API_KEY" \
  -F "model=gpt-image-2-1k" \
  -F "prompt=Keep the subject structure and apply an editorial cover style without text" \
  -F "image=@./input.png" \
  -F "n=1" \
  -F "response_format=url"
```

Result URLs may expire. Download long-lived results into storage you control, and comply with copyright and privacy requirements for all media.
