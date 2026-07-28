# YuAPI 接入与模型调用文档

> 最后验证：2026-07-23<br>
> 模型、价格和可用分组以本站模型广场与 `GET /v1/models` 的实时结果为准。

## 1. 客户端配置

在 OpenAI 兼容客户端、SDK 或服务端程序中，根据调用类型填写对应的 Base URL：

| 调用类型 | Base URL | 网络入口 |
| --- | --- | --- |
| 普通文本与日常模型流量 | `https://api.yuaiapi.com/v1` | Cloudflare 公网入口 |
| 图片、视频、长任务及下游直连流量 | `https://vip.yuaiapi.com/v1` | DNS-only 源站直连 |

两个地址共用同一套 API Key、模型列表、账户余额和价格。用户需要在客户端中明确选择要使用的 Base URL；系统不会自动切换、重定向或替换这两个地址。不要重复拼接 `/v1`，也不要把 `/api` 当成模型接口前缀。

`https://yuaiapi.com` 与 `https://global.yuaiapi.com` 是网站访问入口，不是上述模型 API Base URL。

所有请求使用 Bearer 认证：

```http
Authorization: Bearer <YUAPI_API_KEY>
```

API Key 仅应保存在服务端环境变量或密钥管理系统中，不要写入网页代码、安装包或公开仓库。

## 2. 快速验证

```bash
export YUAPI_BASE_URL="https://api.yuaiapi.com/v1"
export YUAPI_MEDIA_BASE_URL="https://vip.yuaiapi.com/v1"
export YUAPI_API_KEY="sk-你的密钥"

curl "$YUAPI_BASE_URL/models" \
  -H "Authorization: Bearer $YUAPI_API_KEY"
```

只调用返回列表中存在的模型 ID。客户端显示名不一定等于 API 模型名，例如 Banana 系列必须使用 `nano-banana-*`。

Python SDK：

```python
import os
from openai import OpenAI

client = OpenAI(
    base_url="https://api.yuaiapi.com/v1",
    api_key=os.environ["YUAPI_API_KEY"],
)

models = client.models.list()
print([model.id for model in models.data])
```

## 3. 按次生图模型与价格

以下模型仅在“生图按次”和“多模态创作”分组提供，按成功生成的一张图片固定计费；价格已在上游当前单价基础上增加 30%，单位与账户余额展示一致。它们不是 IMAGE / `gpt-image-2` 分组的模型或价格。

| API 模型 ID | 固定档位 | 最终价格/张 |
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

分辨率由模型 ID 固定。提示词中写“4K”不会把 1K 模型升级为 4K；需要更换对应模型 ID。每次仅支持 `n=1`。

截图中的 `banana-pro-*` 对应 API ID `nano-banana-pro-*`，`banana2-*` 对应 `nano-banana2-*`。无后缀的 `gpt-image-2` 仍由站内独立的 IMAGE / `gpt-image-2` 低倍率图片池提供；其调度和定价与本表完全独立。沧源当前 Key 不授权无后缀的 `gpt-image-2`，本上游接入和 30% 加价只适用于明确列出的 `gpt-image-2-1k/2k/4k` 固定按张档位。

## 4. 文生图

`POST /v1/images/generations`

以下图片示例由调用方显式选择直连地址；网关不会在两个地址之间自动切换。

GPT Image 2 同步生成：

```bash
curl -X POST "$YUAPI_MEDIA_BASE_URL/images/generations" \
  -H "Authorization: Bearer $YUAPI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "gpt-image-2-1k",
    "prompt": "雨后城市街道的电影感夜景，真实摄影，无文字",
    "n": 1,
    "response_format": "url"
  }'
```

Nano Banana 同步生成：

```bash
curl -X POST "$YUAPI_MEDIA_BASE_URL/images/generations" \
  -H "Authorization: Bearer $YUAPI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "nano-banana2-1k",
    "prompt": "简洁的产品摄影，白色背景，柔和棚拍光线，无文字",
    "n": 1
  }'
```

当前下游代理只公布同步图片调用，不要传 `async=true`。成功响应读取 `data[0].url` 或 `data[0].b64_json`；URL 可能有有效期，需要长期保存时请下载到自己的对象存储。

## 5. 图生图

`POST /v1/images/edits`

```bash
curl -X POST "$YUAPI_MEDIA_BASE_URL/images/edits" \
  -H "Authorization: Bearer $YUAPI_API_KEY" \
  -F "model=gpt-image-2-1k" \
  -F "prompt=保持主体结构，改为高级杂志封面风格，无文字" \
  -F "image=@./input.png" \
  -F "n=1" \
  -F "response_format=url"
```

GPT Image 2 支持蒙版；GPT Image 2 与 Nano Banana 最多可接收 9 张参考图。参考图可使用 multipart 文件、可公网访问的 HTTPS URL 或 data URI，具体形式以客户端能力为准。

## 6. 视频模型与价格

以下模型仅在“多模态创作”分组提供，按一次成功提交的视频生成任务固定计费；不会再按 `duration`、分辨率或参考素材数量重复乘价。价格与图片按次模型、IMAGE / `gpt-image-2` 图片池分别独立。

| API 模型 ID | 用途 | 最终价格/条 |
| --- | --- | ---: |
| `omni-fast` | Omni 图生视频 | 0.86112 |
| `omni-fast-no-water` | Omni 图生视频，无水印 | 1.0530 |
| `omni-v2v` | Omni 视频生视频 | 1.15128 |
| `omni-v2v-no-water` | Omni 视频生视频，无水印 | 1.3455 |
| `sora-2` | Sora 视频 | 0.9100 |
| `sora-2-pro` | Sora Pro 视频 | 1.1700 |
| `veo-3-1` | Veo 3.1 | 1.1700 |
| `veo-3-1-fast` | Veo 3.1 Fast | 0.9100 |
| `veo-3-1-ref` | Veo 3.1 多参考图 | 1.1700 |
| `sd5-seedance-2.0` | Seedance 2.0 多模态 | 5.0050 |
| `sd5-seedance-2.0-fast` | Seedance 2.0 多模态 Fast | 3.3800 |
| `seedance-2.0` | Seedance 2.0 | 6.3050 |
| `seedance-2.0-fast` | Seedance 2.0 Fast | 4.6800 |
| `seedance-2.0-mini` | Seedance 2.0 Mini | 3.7700 |
| `seedance-2.0-mini-8s` | Seedance 2.0 Mini 8s | 2.5870 |

## 7. 视频任务协议

视频统一使用异步任务：

- 创建：`POST /v1/videos`
- 查询：`GET /v1/videos/{task_id}`
- 下载：`GET /v1/videos/{task_id}/content`

创建成功后保存响应中的 `id` 或 `task_id`，每 5-10 秒查询同一个任务。创建接口返回成功只表示任务已经进入队列，不表示视频已经生成。客户端超时也不代表生成失败，不要重新提交同一任务，以免重复扣费。

视频字段不是所有 OpenAI 兼容客户端都能完整配置。模型列表和图片接口可以直接使用 OpenAI SDK；视频任务建议使用原生 HTTP 请求，确保 `duration`、`aspect_ratio`、参考素材等自定义字段不会被客户端删除。

Sora 最小参数示例：

```bash
curl -X POST "$YUAPI_MEDIA_BASE_URL/videos" \
  -H "Authorization: Bearer $YUAPI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "sora-2",
    "prompt": "清晨海边的固定机位，轻微海浪，自然光，无文字",
    "duration": 4,
    "aspect_ratio": "16:9",
    "generate_audio": false
  }'
```

Sora 参考图放在 `images` 数组中，标准版和 Pro 版都最多接收 1 张。`duration` 可选 4、8、12，`generate_audio` 控制是否生成音频；`sora-2-pro` 使用相同字段，只需更换模型 ID。

### 7.1 Omni 图生视频

`omni-fast` 与 `omni-fast-no-water` 固定约 10 秒、720p，不要传 `duration` 或 `resolution`。单张参考图使用 `image_url`：

```bash
curl -X POST "$YUAPI_MEDIA_BASE_URL/videos" \
  -H "Authorization: Bearer $YUAPI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "omni-fast",
    "prompt": "保持人物和服装一致，镜头缓慢向前推进",
    "aspect_ratio": "16:9",
    "image_url": "https://cdn.example.com/input.jpg"
  }'
```

首尾帧分别使用 `first_image_url` 与 `last_image_url`。多张普通参考图必须改用 multipart，并重复提交 `input_reference` 文件字段，最多 5 张，每张不超过 5 MB：

```bash
curl -X POST "$YUAPI_MEDIA_BASE_URL/videos" \
  -H "Authorization: Bearer $YUAPI_API_KEY" \
  -F "model=omni-fast" \
  -F "prompt=综合参考图中的人物、服装和场景生成自然运镜" \
  -F "aspect_ratio=9:16" \
  -F "input_reference=@./person.jpg" \
  -F "input_reference=@./scene.jpg"
```

无水印版本的请求参数相同，只需将模型改为 `omni-fast-no-water`。

### 7.2 Omni 视频生视频

`omni-v2v` 与 `omni-v2v-no-water` 需要源视频。公网视频使用 `video_url`：

```bash
curl -X POST "$YUAPI_MEDIA_BASE_URL/videos" \
  -H "Authorization: Bearer $YUAPI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "omni-v2v",
    "prompt": "保留原始动作和构图，将画面转换为写实电影风格",
    "aspect_ratio": "16:9",
    "video_url": "https://cdn.example.com/source.mp4"
  }'
```

本地文件使用 multipart 字段 `input_video`。源文件不超过 5 MB，分辨率不超过 1920x1080。无水印版本只需将模型改为 `omni-v2v-no-water`。

### 7.3 Veo Standard、Fast 与 Reference

`veo-3-1` 与 `veo-3-1-fast` 的参数一致。`images` 中第 1 张是首帧，第 2 张是尾帧，最多 2 张：

`resolution` 仅接受 `720p` 或 `1080p`，不要传 `Auto`。

```bash
curl -X POST "$YUAPI_MEDIA_BASE_URL/videos" \
  -H "Authorization: Bearer $YUAPI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "veo-3-1-fast",
    "prompt": "从首帧自然过渡到尾帧，人物一致，镜头稳定",
    "duration": 4,
    "aspect_ratio": "16:9",
    "resolution": "720p",
    "generate_audio": false,
    "reference_mode": "frame",
    "images": [
      "https://cdn.example.com/first.jpg",
      "https://cdn.example.com/last.jpg"
    ]
  }'
```

`veo-3-1-ref` 的图片是主体或素材约束，不代表首尾帧。固定使用 `reference_mode: "image"`，最多 3 张：

```json
{
  "model": "veo-3-1-ref",
  "prompt": "让参考图片中的产品出现在现代展厅中，保持外观一致",
  "duration": 8,
  "aspect_ratio": "16:9",
  "resolution": "1080p",
  "generate_audio": true,
  "reference_mode": "image",
  "images": [
    "https://cdn.example.com/product-front.jpg",
    "https://cdn.example.com/product-side.jpg"
  ]
}
```

Veo 三个模型都只接受 4、6、8 秒和 720p、1080p，不支持 `negative_prompt`、`seed`、`n`、音频参考或 `response_format`。

### 7.4 Seedance 2.0 全能参考

`sd5-seedance-2.0` 与 `sd5-seedance-2.0-fast` 支持首尾帧和全能参考两种互斥模式。

首尾帧模式必须成对传入 `first_image_url`、`last_image_url`：

```json
{
  "model": "sd5-seedance-2.0-fast",
  "prompt": "从室内自然过渡到窗外城市夜景",
  "duration": 4,
  "aspect_ratio": "16:9",
  "resolution": "720p",
  "generate_audio": true,
  "reference_mode": "frame",
  "first_image_url": "https://cdn.example.com/first.png",
  "last_image_url": "https://cdn.example.com/last.png"
}
```

全能参考模式固定使用 `reference_mode: "media"`。最多 9 张图片、3 条视频、3 条音频，三类素材总数不能超过 12：

```json
{
  "model": "sd5-seedance-2.0",
  "prompt": "参考图片中的人物，在参考视频的动作节奏下穿过夜市，使用参考音频的氛围",
  "negative_prompt": "画面抖动、主体变形、文字水印",
  "duration": 8,
  "aspect_ratio": "16:9",
  "resolution": "720p",
  "generate_audio": true,
  "reference_mode": "media",
  "images": [
    "https://cdn.example.com/person.png",
    "https://cdn.example.com/scene.png"
  ],
  "reference_videos": [
    "https://cdn.example.com/motion.mp4"
  ],
  "reference_audios": [
    "https://cdn.example.com/ambient.wav"
  ],
  "seed": 12345
}
```

### 7.5 Seedance 2.0、Fast 与 Mini

`seedance-2.0`、`seedance-2.0-fast`、`seedance-2.0-mini` 与 `seedance-2.0-mini-8s` 使用同一套字段。注意这个模型族使用 `audio`，不是 `generate_audio`：

```bash
curl -X POST "$YUAPI_MEDIA_BASE_URL/videos" \
  -H "Authorization: Bearer $YUAPI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "seedance-2.0-mini-8s",
    "prompt": "白色纸飞机从桌面缓慢滑过，镜头稳定，无文字",
    "duration": 4,
    "resolution": "480p",
    "aspect_ratio": "16:9",
    "audio": false
  }'
```

单图可传 HTTPS URL 或 data URI；多模态图片总数最多 4 张，第一张使用 `image_url`，其余放入 `reference_image_urls`。参考视频最多 3 条且总时长不超过 15 秒，参考音频最多 1 条且不超过 15 秒：

```json
{
  "model": "seedance-2.0-mini",
  "prompt": "参考 @image1 与 @image2 的人物和场景，沿用 @video1 的动作节奏与 @audio1 的音乐氛围",
  "duration": 6,
  "resolution": "480p",
  "aspect_ratio": "1:1",
  "audio": true,
  "image_url": "https://cdn.example.com/person.jpg",
  "reference_image_urls": [
    "https://cdn.example.com/scene.jpg"
  ],
  "reference_videos": [
    "https://cdn.example.com/motion.mp4"
  ],
  "reference_audios": [
    "https://cdn.example.com/music.mp3"
  ]
}
```

首尾帧使用成对的 `first_image_url`、`last_image_url`，不能同时再传多模态参考素材。`seedance-2.0-mini-8s` 必须使用完整模型 ID，时长不能超过 8 秒。

### 7.6 查询与下载

查询任务：

```bash
curl "$YUAPI_MEDIA_BASE_URL/videos/$TASK_ID" \
  -H "Authorization: Bearer $YUAPI_API_KEY"
```

任务完成后优先读取 `video_url`、`metadata.url`、`metadata.video_url` 或 `data[0].url`。也可以通过本站内容代理下载：

```bash
curl -L "$YUAPI_MEDIA_BASE_URL/videos/$TASK_ID/content" \
  -H "Authorization: Bearer $YUAPI_API_KEY" \
  --output result.mp4
```

## 8. 视频参数范围

| 模型族 | 时长 | 分辨率 | 参考素材 |
| --- | --- | --- | --- |
| `sora-2*` | 4、8、12 秒 | 模型默认 | 最多 1 张图，支持 `negative_prompt` |
| `veo-3-1` / `fast` | 4、6、8 秒 | 720p、1080p | 首尾帧最多 2 张 |
| `veo-3-1-ref` | 4、6、8 秒 | 720p、1080p | 素材图最多 3 张 |
| `sd5-seedance-*` | 4-15 秒 | 480p、720p | 首尾帧，或最多 9 图/3 视频/3 音频，总数不超过 12 |
| `seedance-2.0*` | 4-15 秒 | 480p、720p | 最多 4 图/3 视频/1 音频；多模态与首尾帧不能混用 |
| `seedance-2.0-mini-8s` | 4-8 秒 | 480p、720p | 与 Seedance Mini 相同 |
| `omni-fast*` | 固定约 10 秒 | 固定 720p | 单图 `image_url`，多图最多 5 张，或首尾帧 |
| `omni-v2v*` | 固定约 10 秒 | 固定 720p | `video_url` 或 multipart `input_video`，文件不超过 5 MB |

模型族限制：

- Omni 图生视频不要传 `duration` 或 `resolution`；Omni V2V 必须提供视频输入。
- Sora 与 Veo 使用 `generate_audio`；`seedance-2.0*` 使用 `audio`；`sd5-seedance-*` 使用 `generate_audio`。
- `duration` 与兼容别名 `seconds` 只传一个。分辨率、时长和模型 ID 不匹配时，请求会在创建任务前返回 `400`。
- `image_url`、`images`、`reference_image_urls`、`input_reference` 的用途不同，不要在模型族之间照搬字段。
- data URI 适合小图片；视频和音频参考应使用可由服务端访问的 HTTPS URL。不要使用需要 Cookie、Referer 或临时登录态的地址。
- 参考视频使用 MP4/MOV、H.264/H.265 和 24-60 FPS。普通 Seedance 参考视频每边建议 720-2160 px。

## 9. 状态与错误处理

创建响应示例：

```json
{
  "id": "task_01HZX8A2...",
  "status": "queued",
  "progress": 0
}
```

查询完成响应可能通过不同兼容字段返回视频：

```json
{
  "id": "task_01HZX8A2...",
  "status": "completed",
  "progress": 100,
  "metadata": {
    "video_url": "/v1/videos/task_01HZX8A2.../content"
  }
}
```

状态处理规则：

| 状态 | 含义 | 客户端动作 |
| --- | --- | --- |
| `queued`、`pending` | 已排队 | 5-10 秒后继续查询 |
| `processing`、`in_progress` | 正在生成或处理 | 继续查询，不要重复创建 |
| `completed` | 已完成 | 读取结果 URL，或请求 `/content` |
| `failed`、`cancelled` | 已终止 | 停止查询并读取错误信息 |

`progress` 可能是 0-100 的数字，也可能是带 `%` 的字符串。失败原因依次检查 `error.message`、`reason`、`message`；完成结果依次检查 `video_url`、`metadata.video_url`、`metadata.url`、`data[0].url`。相对 URL 需要以当前选择的直连 Base URL 对应源站补全，或者直接调用本站 `/content` 接口。

Python 轮询示例：

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
        "model": "veo-3-1-fast",
        "prompt": "雨夜城市街道，镜头缓慢推进",
        "duration": 4,
        "aspect_ratio": "16:9",
        "resolution": "720p",
    },
    timeout=180,
)
created.raise_for_status()
task_id = created.json().get("id") or created.json().get("task_id")
if not task_id:
    raise RuntimeError("创建响应缺少任务 ID")

while True:
    response = requests.get(
        f"{base_url}/videos/{task_id}", headers=headers, timeout=60
    )
    response.raise_for_status()
    task = response.json()
    status = task.get("status", "").lower()

    if status == "completed":
        metadata = task.get("metadata") or {}
        data = task.get("data") or []
        video_url = (
            task.get("video_url")
            or metadata.get("video_url")
            or metadata.get("url")
            or (data[0].get("url") if data else None)
        )
        if video_url:
            print(urljoin(api_origin, video_url))
        else:
            print(f"{base_url}/videos/{task_id}/content")
        break

    if status in {"failed", "cancelled"}:
        error = task.get("error") or {}
        raise RuntimeError(
            error.get("message") or task.get("reason") or "视频任务失败"
        )

    time.sleep(5)
```

| HTTP 状态 | 常见原因 | 处理方式 |
| --- | --- | --- |
| `400` | 参数、时长、分辨率或素材格式错误 | 修正请求，不要原样重试 |
| `401` | API Key 无效、过期或已删除 | 检查 Bearer 认证 |
| `403` | 余额、分组或模型权限不足 | 检查 Key 与账户状态 |
| `404` | 模型或任务不存在 | 重新查询 `/v1/models`，核对任务 ID |
| `429` | 并发或上游限流 | 降低并发，指数退避 |
| `500/502/503/504` | 上游故障、排队或超时 | 保存 Request ID；先查询旧任务，再有限重试 |

排查时请保存请求时间、路径、模型 ID、任务 ID、脱敏后的请求体和 Request ID，不要记录完整 API Key。

## 10. 上线检查

- `GET /v1/models` 能看到准备调用的模型。
- Key 所在分组允许该模型，账户余额充足。
- 图片请求使用同步模式，`n=1`。
- 客户端会原样保留目标模型所需的自定义 JSON 字段。
- 参考素材 URL 无需登录即可从公网下载，类型、大小、时长和分辨率符合模型限制。
- 视频客户端支持保存任务 ID 并轮询，不会在超时后自动重复创建。
- HTTP 超时建议不少于 180 秒；长任务以查询接口状态为准。
- 生产日志保留 Request ID 和任务 ID，但不保存完整凭证或私密素材。
