# YuAPI 影片 API：從零開始產生第一支影片

> 網站登入密碼不是 API Key。程式呼叫前，請先登入 YuAPI，然後在 [`/keys`](/keys) 建立獨立的 API Key。請勿把網站密碼填入程式碼、命令列或第三方用戶端。

第一次測試建議選擇 `多模态创作` 分組，只允許 `seedance-2.0`，並設定較短的有效期限與較小額度。確認流程正確後，再為正式應用程式建立獨立的正式 Key。

## 1. 開始前：帳號、API Key 與任務 ID

完成一次影片產生會用到三種不同資料：

| 名稱 | 用途 | 是否可以公開 |
| --- | --- | --- |
| 網站帳號與密碼 | 登入控制台、管理餘額與 Key | 不可以 |
| API Key | 放在 `Authorization: Bearer ...` 請求標頭 | 不可以 |
| 任務 ID | 查詢同一次影片任務與下載結果 | 只應提供給可信任的維運人員 |

YuAPI 提供兩個 Base URL：

| 用途 | Base URL |
| --- | --- |
| 查詢模型與一般 API | `https://api.yuaiapi.com/v1` |
| 圖片、影片與長任務 | `https://vip.yuaiapi.com/v1` |

兩個位址共用同一套 API Key、模型清單、帳戶餘額與價格。用戶端必須明確選擇 Base URL；系統不會自動切換、重新導向或取代這兩個位址。本文所有圖片與影片範例都使用第二個位址。

## 2. 建立你的第一個 API Key

1. 登入 YuAPI。
2. 開啟 [`/keys`](/keys)。
3. 點選「建立 API Key」。
4. 名稱填寫容易辨識的用途，例如 `video-quickstart`。
5. 分組選擇畫面上的 `多模态创作`。
6. 第一次測試建議設定 24 小時有效期限與 `25.00` 額度上限。
7. 開啟模型限制，只選擇 `seedance-2.0`。
8. 建立後立即保存 Key。關閉顯示視窗後，完整值可能不會再次出現。

![API Key 清單中的建立按鈕](/developer-docs/assets/video-api-key-zh-01.webp)

![測試 Key 的名稱、分組、有效期限與額度設定](/developer-docs/assets/video-api-key-zh-02.webp)

![只允許 seedance-2.0 的模型限制](/developer-docs/assets/video-api-key-zh-03.webp)

請勿把完整 Key 傳到聊天、工單或截圖中。本文統一使用環境變數 `YUAPI_API_KEY`，範例不會硬編碼 Key。

> 進階說明：部分帳戶可能還會看到 `下游多模態` 分組。它只是另一個可選分組名稱，不改變本文的請求路徑與任務協定；初次接入請使用畫面上的 `多模态创作`。

## 3. 驗證 API Key

先查詢模型清單。這個請求不會建立影片。

Windows PowerShell：

```powershell
$env:YUAPI_API_KEY = Read-Host "請輸入 API Key"
$env:YUAPI_BASE_URL = "https://api.yuaiapi.com/v1"

$headers = @{ Authorization = "Bearer $env:YUAPI_API_KEY" }
$models = Invoke-RestMethod `
  -Uri "$env:YUAPI_BASE_URL/models" `
  -Headers $headers `
  -Method Get

$models.data | Where-Object id -eq "seedance-2.0"
```

macOS/Linux：

```bash
read -rsp "YuAPI API Key: " YUAPI_API_KEY && echo
export YUAPI_API_KEY
export YUAPI_BASE_URL="https://api.yuaiapi.com/v1"
export YUAPI_MEDIA_BASE_URL="https://vip.yuaiapi.com/v1"

curl --fail-with-body "$YUAPI_BASE_URL/models" \
  -H "Authorization: Bearer $YUAPI_API_KEY"
```

結果中應包含 `seedance-2.0`。若回傳 `401`，請檢查 Key 是否完整；若回傳 `403` 或清單中沒有該模型，請檢查 Key 的分組與模型限制。

## 4. 使用 seedance-2.0 建立影片

建立介面是 `POST /v1/videos`。以下是第一個純文字產生請求，不需要準備參考素材：

```json
{
  "model": "seedance-2.0",
  "prompt": "清晨的海邊木棧道，鏡頭緩慢向前推進，柔和自然光，真實電影質感",
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
    "prompt": "清晨的海邊木棧道，鏡頭緩慢向前推進，柔和自然光，真實電影質感",
    "duration": 4,
    "aspect_ratio": "16:9",
    "generate_audio": true
  }'
```

成功回應會包含 `id` 或 `task_id`。請立即將它保存到資料庫或任務記錄。建立成功只代表任務已進入佇列，不代表影片已完成。

## 5. 查詢原任務並下載影片

查詢時必須使用建立回應中的原任務 ID：

```bash
export TASK_ID="在此填入建立回應中的任務 ID"

curl "$YUAPI_MEDIA_BASE_URL/videos/$TASK_ID" \
  -H "Authorization: Bearer $YUAPI_API_KEY"
```

狀態為 `completed`、`succeeded` 或 `success` 時下載內容：

```bash
curl -L "$YUAPI_MEDIA_BASE_URL/videos/$TASK_ID/content" \
  -H "Authorization: Bearer $YUAPI_API_KEY" \
  --output result.mp4
```

若用戶端逾時，或任務仍為 `queued`、`processing`，請勿再次送出建立請求。繼續查詢同一個任務 ID。重複 `POST /v1/videos` 會建立另一個任務，並可能再次計費。

## 6. 將測試 Key 調整為正式環境安全設定

測試完成後，請建立新的正式 Key：

- 每個應用程式與環境使用獨立 Key。
- 只允許實際使用的模型。
- 設定可接受的額度上限與有效期限，並定期輪替。
- 有固定出口 IP 時啟用 IP 限制。
- 只把 Key 放在伺服器環境變數或密鑰管理系統。
- 不要在瀏覽器 JavaScript、行動應用程式套件、公開儲存庫或日誌中保存 Key。
- Key 洩漏時立即刪除舊 Key、建立新 Key，並檢查使用記錄。

## 7. 影片模型與公開價格

以下為 `多模态创作` 分組的單次影片產生價格。查詢、讀取狀態與下載同一任務不會再次扣費。最終可用模型與金額仍以模型廣場及帳戶畫面為準。

<!-- video-model-catalog:start -->
| 模型 | `多模态创作` 單次價格 |
| --- | ---: |
| `grok-video` | 0.9936 |
| `grok-video-1.5` | 2.0016 |
| `happyhouse-1.0` | 6.48 |
| `happyhouse-1.1` | 4.176 |
| `minimax-h3-2k` | 5.04 |
| `omni-fast` | 0.95388 |
| `omni-fast-no-water` | 1.1664 |
| `omni-v2v` | 1.27536 |
| `omni-v2v-no-water` | 1.4904 |
| `sd7-seedance-2.0-1080p` | 7.056 |
| `sd7-seedance-2.0-720p` | 5.616 |
| `sd8-seedance-2.0` | 4.176 |
| `seedance-2.0` | 5.616 |
<!-- video-model-catalog:end -->

影片按次計費與文字 Token 計費、圖片按張計費彼此獨立。不要把文字介面的 `usage`、快取命中或串流中斷規則套用到影片任務。

## 8. 影片任務協定

```text
GET  https://api.yuaiapi.com/v1/models
POST https://vip.yuaiapi.com/v1/videos
GET  https://vip.yuaiapi.com/v1/videos/{task_id}
GET  https://vip.yuaiapi.com/v1/videos/{task_id}/content
```

建立回應範例：

```json
{
  "id": "task_example_01",
  "status": "queued"
}
```

| 狀態 | 操作 |
| --- | --- |
| `queued`、`processing` | 等待 5-10 秒後查詢同一 ID |
| `completed`、`succeeded`、`success` | 讀取結果或請求 `/content` |
| `failed`、`canceled`、`cancelled` | 停止查詢並記錄錯誤與 Request ID |

完成結果可能位於 `video_url`、`metadata.video_url`、`metadata.url` 或 `data[0].url`。失敗原因依序檢查 `error.message`、`reason` 與 `message`。

## 9. 模型參數與參考素材限制

| 模型 | 時長 | 解析度 | 參考素材與注意事項 |
| --- | --- | --- | --- |
| `grok-video` | 4、6、8、10、12、15 秒 | 480p、720p | 最多 1 張參考圖 |
| `grok-video-1.5` | 4、6、8、10、12、15 秒 | 480p、720p | 最多 7 張參考圖 |
| `happyhouse-1.0` | 3-15 秒 | 720p、1080p | 最多 9 張圖；或 1 段 3-10 秒影片加最多 5 張圖；支援 `generate_audio` |
| `happyhouse-1.1` | 3-15 秒 | 720p、1080p | 最多 9 張圖；支援 `generate_audio` |
| `minimax-h3-2k` | 5-15 秒 | 固定 2K | 最多 5 張圖與 3 段音訊，總數不超過 8 |
| `omni-fast*` | 固定約 10 秒 | 固定 720p | 不要傳時長、解析度或音訊開關 |
| `omni-v2v*` | 固定約 10 秒 | 固定 720p | 必須且只能提供 1 段來源影片 |
| `seedance-2.0` | 4-15 秒 | 固定 720p | 最多 5 張圖、3 段影片、3 段音訊，總數不超過 11 |
| `sd7-seedance-2.0-*` | 4-15 秒 | 由模型 ID 固定 | 最多 5 張圖、3 段影片、3 段音訊，總數不超過 11 |
| `sd8-seedance-2.0` | 5、10、15 秒 | 模型固定 | 最多 9 張圖、3 段影片、3 段音訊；不要傳 `resolution` 或 `generate_audio` |

參考素材 URL 必須是伺服器不需要 Cookie、登入狀態或 Referer 就能讀取的 HTTPS 位址。

```json
{
  "reference_image_urls": ["https://assets.example.com/reference/person.png"],
  "reference_videos": ["https://assets.example.com/reference/motion.mp4"],
  "reference_audios": ["https://assets.example.com/reference/ambient.mp3"]
}
```

## 10. Windows PowerShell 完整範例

```powershell
$mediaBase = "https://vip.yuaiapi.com/v1"
$headers = @{
  Authorization = "Bearer $env:YUAPI_API_KEY"
  "Content-Type" = "application/json"
}
$body = @{
  model = "seedance-2.0"
  prompt = "清晨的海邊木棧道，鏡頭緩慢向前推進，柔和自然光"
  duration = 4
  aspect_ratio = "16:9"
  generate_audio = $true
} | ConvertTo-Json

$created = Invoke-RestMethod -Uri "$mediaBase/videos" -Method Post -Headers $headers -Body $body
$taskId = if ($created.id) { $created.id } else { $created.task_id }
if (-not $taskId) { throw "建立回應缺少任務 ID" }
$deadline = (Get-Date).AddMinutes(15)

do {
  Start-Sleep -Seconds 5
  $task = Invoke-RestMethod -Uri "$mediaBase/videos/$taskId" -Headers @{
    Authorization = "Bearer $env:YUAPI_API_KEY"
  }
  $status = [string]$task.status
} while ($status -in @("queued", "processing") -and (Get-Date) -lt $deadline)

if ($status -notin @("completed", "succeeded", "success")) {
  throw "任務未完成，請保存任務 ID 並繼續查詢: $taskId"
}
Invoke-WebRequest -Uri "$mediaBase/videos/$taskId/content" -Headers @{
  Authorization = "Bearer $env:YUAPI_API_KEY"
} -OutFile .\result.mp4
```

## 11. macOS/Linux curl 完整範例

```bash
set -euo pipefail
: "${YUAPI_API_KEY:?請先設定 YUAPI_API_KEY}"
YUAPI_MEDIA_BASE_URL="https://vip.yuaiapi.com/v1"

created_json="$(curl --fail-with-body -sS -X POST "$YUAPI_MEDIA_BASE_URL/videos" \
  -H "Authorization: Bearer $YUAPI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "seedance-2.0",
    "prompt": "清晨的海邊木棧道，鏡頭緩慢向前推進，柔和自然光",
    "duration": 4,
    "aspect_ratio": "16:9",
    "generate_audio": true
  }')"

TASK_ID="$(printf '%s' "$created_json" | python3 -c \
  'import json,sys; d=json.load(sys.stdin); print(d.get("id") or d.get("task_id") or "")')"
test -n "$TASK_ID" || { echo "建立回應缺少任務 ID" >&2; exit 1; }

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

## 12. Python 完整範例

先執行 `python -m pip install requests`。

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
        "prompt": "清晨的海邊木棧道，鏡頭緩慢向前推進，柔和自然光",
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
    raise RuntimeError("建立回應缺少任務 ID")

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

## 13. Node.js 伺服器端範例

此範例只能在可信任的伺服器端執行，不能放進瀏覽器 JavaScript。

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
    prompt: '清晨的海邊木棧道，鏡頭緩慢向前推進，柔和自然光',
    duration: 4,
    aspect_ratio: '16:9',
    generate_audio: true,
  }),
})
if (!createdResponse.ok) throw new Error(await createdResponse.text())
const created = await createdResponse.json()
const taskId = created.id ?? created.task_id
if (!taskId) throw new Error('建立回應缺少任務 ID')

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
if (!['completed', 'succeeded', 'success'].includes(status)) throw new Error(`任務仍在處理中: ${taskId}`)

const content = await fetch(`${baseUrl}/videos/${taskId}/content`, { headers: authHeaders })
if (!content.ok) throw new Error(await content.text())
await writeFile('result.mp4', Buffer.from(await content.arrayBuffer()))
```

## 14. 狀態、錯誤與安全重試

| HTTP 狀態 | 常見原因 | 正確處理 |
| --- | --- | --- |
| `400` | 參數或素材不符合模型要求 | 修正請求，不要原樣重試 |
| `401` | Key 無效、過期或已刪除 | 檢查 Bearer 標頭並輪替 Key |
| `403` | 分組、模型權限、額度或帳戶狀態不允許 | 檢查 Key 設定與帳戶狀態 |
| `404` | 模型或任務不存在 | 重新查詢模型清單並核對原任務 ID |
| `429` | 併發或速率限制 | 降低併發並指數退避 |
| `500/502/503/504` | 服務暫時不可用或長任務逾時 | 先查詢已保存的任務 ID，再決定是否建立新任務 |

日誌可以保存時間、路徑、模型 ID、任務 ID、狀態碼與 Request ID，但不能保存完整 Key、網站密碼或私密素材。

## 15. 接入前檢查清單

- [ ] 已在 `/keys` 建立 API Key，而不是使用網站密碼。
- [ ] 測試 Key 只允許需要的模型。
- [ ] `GET /v1/models` 能看到準備呼叫的模型 ID。
- [ ] 圖片與影片請求使用 `https://vip.yuaiapi.com/v1`。
- [ ] 建立回應中的任務 ID 已持久化。
- [ ] `queued` 或 `processing` 時只查詢原任務，不重複建立。
- [ ] 正式 Key 有獨立額度、有效期限、模型與可選 IP 限制。
- [ ] Key 只存在於可信任的伺服器端。
- [ ] 參考素材可由伺服器直接讀取並符合模型限制。
- [ ] 應用程式會處理成功、失敗、取消與逾時狀態。

## 16. 圖片 API 參考

圖片產生是同步介面，和影片任務的輪詢流程不同：

- 文生圖：`POST /v1/images/generations`
- 圖片編輯：`POST /v1/images/edits`
- 每次只請求 `n=1`
- 成功結果讀取 `data[0].url` 或 `data[0].b64_json`

| 模型 | 固定檔位 | 單張價格 |
| --- | ---: | ---: |
| `gpt-image-2-1k` | 1K | 0.0325 |
| `gpt-image-2-2k` | 2K | 0.0650 |
| `gpt-image-2-4k` | 4K | 0.1040 |
| `nano-banana-pro-1k` | 1K | 0.1040 |
| `nano-banana-pro-2k` | 2K | 0.1300 |
| `nano-banana-pro-4k` | 4K | 0.1937 |
| `nano-banana2-1k` | 1K | 0.0767 |
| `nano-banana2-2k` | 2K | 0.1040 |
| `nano-banana2-4k` | 4K | 0.1560 |
| `grok-imagine-image` | 標準 | 0.072 |
| `grok-imagine-image-quality` | 高品質 | 0.072 |

`grok-4.5` 是文字模型；`grok-imagine-image` 與 `grok-imagine-image-quality` 是圖片模型；`grok-video` 與 `grok-video-1.5` 是非同步影片模型。請始終使用準確模型 ID 與對應介面。

```bash
curl --fail-with-body -X POST "$YUAPI_MEDIA_BASE_URL/images/generations" \
  -H "Authorization: Bearer $YUAPI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2-1k",
    "prompt": "雨後城市街道的電影感夜景，真實攝影，無文字",
    "n": 1,
    "response_format": "url"
  }'
```

```bash
curl --fail-with-body -X POST "$YUAPI_MEDIA_BASE_URL/images/edits" \
  -H "Authorization: Bearer $YUAPI_API_KEY" \
  -F "model=gpt-image-2-1k" \
  -F "prompt=保留主體結構，改為高級雜誌封面風格，無文字" \
  -F "image=@./input.png" \
  -F "n=1" \
  -F "response_format=url"
```

結果 URL 可能有有效期限。需要長期保存時，請及時下載到自己的受控儲存空間，並遵守素材版權與隱私要求。
