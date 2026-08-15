# YuAPI 接入与模型调用文档

> 最后验证：2026-08-15<br>
> 模型、价格和可用分组以本站模型广场与 `GET /v1/models` 的实时结果为准。

## 1. 客户端配置

在 OpenAI 兼容客户端、SDK 或服务端程序中，根据调用类型填写对应的 Base URL：

| 调用类型                         | Base URL                     | 网络入口            |
| -------------------------------- | ---------------------------- | ------------------- |
| 普通文本与日常模型流量           | `https://api.yuaiapi.com/v1` | Cloudflare 公网入口 |
| 图片、视频、长任务及下游直连流量 | `https://vip.yuaiapi.com/v1` | DNS-only 源站直连   |

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

### 2.1 Grok 分组区别

- `grok-4.5` 是文本模型，按实际输入与输出 Token 计费，仅由 `grok`、`下游grok` 等文本分组提供。
- `grok-imagine-image` 与 `grok-imagine-image-quality` 是按张计费的图片模型，由 `生图按次` 和 `多模态创作` 分组提供。
- `grok-video` 与 `grok-video-1.5` 是按次计费的异步视频模型，仅由 `多模态创作` 等已授权视频分组提供。

文本、图片和视频使用不同的渠道协议。切换分组不会把 `grok-4.5` 自动转换为图片或视频模型，调用时必须填写下表中的准确 API 模型 ID。

## 3. 按次生图模型与价格

以下模型仅在“生图按次”和“多模态创作”分组提供，按成功生成的一张图片固定计费。表内价格是最后验证时的用户最终价格，单位与账户余额展示一致；实时价格以模型广场为准。它们不是 IMAGE / `gpt-image-2` 分组的模型或价格。

| API 模型 ID                  |   固定档位 | 最终价格/张 |
| ---------------------------- | ---------: | ----------: |
| `gpt-image-2-1k`             |         1K |      0.0325 |
| `gpt-image-2-2k`             |         2K |      0.0650 |
| `gpt-image-2-4k`             |         4K |      0.1040 |
| `nano-banana-pro-1k`         |         1K |      0.1040 |
| `nano-banana-pro-2k`         |         2K |      0.1300 |
| `nano-banana-pro-4k`         |         4K |      0.1937 |
| `nano-banana2-1k`            |         1K |      0.0767 |
| `nano-banana2-2k`            |         2K |      0.1040 |
| `nano-banana2-4k`            |         4K |      0.1560 |
| `grok-imagine-image`         |   标准生图 |       0.072 |
| `grok-imagine-image-quality` | 高质量生图 |       0.072 |

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

Grok Imagine 同步生成：

```bash
curl -X POST "$YUAPI_MEDIA_BASE_URL/images/generations" \
  -H "Authorization: Bearer $YUAPI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "grok-imagine-image-quality",
    "prompt": "未来城市上空的高速列车，电影级光影，清晰细节，无文字",
    "n": 1,
    "response_format": "url"
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

以下模型仅在“多模态创作”等已授权视频分组提供，全部按一次视频生成计费。`duration`、分辨率、生成音频和参考素材不会重复乘价，查询、内容读取和下载也不会再次扣费。视频计费与 GPT 文字 Token、图片按次模型及 IMAGE / `gpt-image-2` 图片池严格隔离。

表内价格是“多模态创作”分组倍率计算后的最终价格。`下游多模态` 分组倍率为 `1.0`，其价格等于定价 API 为该分组返回的基础价格；所有分组的实时金额仍以模型广场和定价 API 为准。

| API 模型 ID              | 用途                         | 最终价格/条 |
| ------------------------ | ---------------------------- | ----------: |
| `grok-video`             | Grok 视频生成                |      0.9936 |
| `grok-video-1.5`         | Grok 1.5 视频生成            |      2.0016 |
| `happyhouse-1.0`         | Happyhouse 图像/视频参考生成 |        6.48 |
| `happyhouse-1.1`         | Happyhouse 多图参考生成      |       4.176 |
| `minimax-h3-2k`          | Minimax H3 2K 视频生成       |        5.04 |
| `omni-fast`              | Omni 图生视频                |     0.95388 |
| `omni-fast-no-water`     | Omni 图生视频，无水印        |      1.1664 |
| `omni-v2v`               | Omni 视频生视频              |     1.27536 |
| `omni-v2v-no-water`      | Omni 视频生视频，无水印      |      1.4904 |
| `sd4-seedance-2.0`       | Seedance SD4 多模态生成      |       5.616 |
| `sd4-seedance-2.0-fast`  | Seedance SD4 多模态快速生成  |       4.176 |
| `sd7-seedance-2.0-1080p` | Seedance SD7 固定 1080p      |       7.056 |
| `sd7-seedance-2.0-720p`  | Seedance SD7 固定 720p       |       5.616 |
| `sd8-seedance-2.0`       | Seedance SD8 多模态生成      |       4.176 |

## 7. 视频任务协议

视频统一使用异步任务：

- 创建：`POST /v1/videos`
- 查询：`GET /v1/videos/{task_id}`
- 下载：`GET /v1/videos/{task_id}/content`

创建成功后保存响应中的 `id` 或 `task_id`，每 5-10 秒查询同一个任务。创建接口返回成功只表示任务已经进入队列，不表示视频已经生成。客户端超时也不代表生成失败，不要重新提交同一任务，以免重复扣费。

视频字段不是所有 OpenAI 兼容客户端都能完整配置。模型列表和图片接口可以直接使用 OpenAI SDK；视频任务建议使用原生 HTTP 请求，确保 `duration`、`resolution`、`aspect_ratio`、`generate_audio` 和 `reference_*` 字段不会被客户端删除。

所有示例都使用公开模型 ID。参考素材 URL 必须可由服务端直接访问，不得依赖 Cookie、Referer 或登录态。创建接口只调用一次；收到任务 ID 后只能查询同一个 ID，不能用重复 `POST` 代替轮询。

### 7.1 Grok 视频

`grok-video` 与 `grok-video-1.5` 使用 `duration`，不要使用 `seconds`。两者支持 4、6、8、10、12、15 秒，480p 或 720p；`grok-video` 最多 1 张参考图，`grok-video-1.5` 最多 7 张。

```bash
curl -X POST "$YUAPI_MEDIA_BASE_URL/videos" \
  -H "Authorization: Bearer $YUAPI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "grok-video",
    "prompt": "保持主体一致，镜头缓慢向前推进，光影自然，无文字",
    "duration": 4,
    "resolution": "480p",
    "aspect_ratio": "16:9",
    "reference_image_urls": ["https://assets.example.com/video/subject.jpg"]
  }'
```

### 7.2 Happyhouse

`happyhouse-1.0` 与 `happyhouse-1.1` 支持 3-15 秒、720p 或 1080p，并可通过 `generate_audio` 显式控制模型原生音频。1.0 最多接收 9 张图片，或一条 3-10 秒参考视频加最多 5 张图片，总素材数不超过 9；1.1 最多接收 9 张图片，不接收视频或音频参考。

```json
{
  "model": "happyhouse-1.0",
  "prompt": "沿用人物外观和参考视频动作，生成稳定的电影感运镜",
  "duration": 3,
  "resolution": "720p",
  "aspect_ratio": "16:9",
  "generate_audio": false,
  "reference_image_urls": ["https://assets.example.com/video/person.png"],
  "reference_videos": ["https://assets.example.com/video/motion.mp4"]
}
```

### 7.3 Minimax H3 2K

`minimax-h3-2k` 支持 5-15 秒，分辨率固定为 `2k`。最多接收 5 张图片和 3 条音频，总数不超过 8；每条及全部参考音频总时长均不得超过 15 秒。使用首尾帧时传 `first_image_url` 与 `last_image_url`，且不能同时开启模型生成音频。

```json
{
  "model": "minimax-h3-2k",
  "prompt": "参考人物和音乐节奏，生成夜间街景中的自然行走镜头",
  "duration": 5,
  "resolution": "2k",
  "aspect_ratio": "16:9",
  "generate_audio": false,
  "reference_image_urls": ["https://assets.example.com/video/person.png"],
  "reference_audios": ["https://assets.example.com/video/ambient.mp3"]
}
```

### 7.4 Omni 图生视频

`omni-fast` 与 `omni-fast-no-water` 固定约 10 秒、720p，只支持 `16:9` 或 `9:16`。不要传 `duration`、`resolution` 或 `generate_audio`。普通参考图统一使用 `reference_image_urls`，最多 5 张：

```bash
curl -X POST "$YUAPI_MEDIA_BASE_URL/videos" \
  -H "Authorization: Bearer $YUAPI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "omni-fast",
    "prompt": "保持人物和服装一致，镜头缓慢向前推进",
    "aspect_ratio": "16:9",
    "reference_image_urls": ["https://assets.example.com/video/input.jpg"]
  }'
```

首尾帧分别使用 `first_image_url` 与 `last_image_url`，使用首尾帧时不要再传普通参考图。无水印版本参数相同，只需将模型改为 `omni-fast-no-water`。

### 7.5 Omni 视频生视频

`omni-v2v` 与 `omni-v2v-no-water` 固定约 10 秒、720p，只支持 `16:9` 或 `9:16`，并且必须提供且只能提供一条源视频。不要传 `duration`、`resolution` 或 `generate_audio`。

```bash
curl -X POST "$YUAPI_MEDIA_BASE_URL/videos" \
  -H "Authorization: Bearer $YUAPI_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{
    "model": "omni-v2v",
    "prompt": "保留原始动作和构图，将画面转换为写实电影风格",
    "aspect_ratio": "16:9",
    "reference_videos": ["https://assets.example.com/video/source.mp4"]
  }'
```

无水印版本只需将模型改为 `omni-v2v-no-water`。

### 7.6 SD4 Seedance

`sd4-seedance-2.0` 与 `sd4-seedance-2.0-fast` 支持 4-15 秒、480p 或 720p。普通多模态模式最多接收 4 张图片、3 条视频和 1 条音频，总数不超过 8；参考视频单条为 4-10 秒、总时长不超过 15 秒，参考音频不超过 15 秒。

首尾帧模式使用 `first_image_url` 与 `last_image_url`，不能再传普通多模态参考，也不能把 `generate_audio` 设为 `true`：

```json
{
  "model": "sd4-seedance-2.0-fast",
  "prompt": "从室内自然过渡到窗外城市夜景，镜头稳定",
  "duration": 4,
  "resolution": "480p",
  "aspect_ratio": "16:9",
  "generate_audio": false,
  "first_image_url": "https://assets.example.com/video/first.png",
  "last_image_url": "https://assets.example.com/video/last.png"
}
```

普通多模态参考使用三个 `reference_*` 数组：

```json
{
  "model": "sd4-seedance-2.0",
  "prompt": "参考人物、动作节奏和环境音，生成夜市中的自然行走镜头",
  "duration": 6,
  "resolution": "720p",
  "aspect_ratio": "16:9",
  "generate_audio": false,
  "reference_image_urls": ["https://assets.example.com/video/person.png"],
  "reference_videos": ["https://assets.example.com/video/motion.mp4"],
  "reference_audios": ["https://assets.example.com/video/ambient.mp3"]
}
```

### 7.7 SD7 Seedance

SD7 的分辨率固定在模型 ID 中：720p 使用 `sd7-seedance-2.0-720p`，1080p 使用 `sd7-seedance-2.0-1080p`。请求中不要再传 `resolution`。两者支持 4-15 秒、最多 5 张图片、3 条视频和 3 条音频，总数不超过 11，并支持 `generate_audio`。

```json
{
  "model": "sd7-seedance-2.0-720p",
  "prompt": "保持参考角色一致，在城市天台完成缓慢环绕运镜",
  "duration": 4,
  "aspect_ratio": "16:9",
  "generate_audio": true,
  "reference_image_urls": ["https://assets.example.com/video/character.png"]
}
```

### 7.8 SD8 Seedance

`sd8-seedance-2.0` 仅支持 5、10、15 秒，不接收 `resolution` 或 `generate_audio`。最多接收 9 张图片、3 条视频和 3 条音频，总数不超过 15。含人物的参考图片必须先按上游规则遮挡眼部。

```json
{
  "model": "sd8-seedance-2.0",
  "prompt": "参考环境与动作节奏，生成稳定的写实电影镜头",
  "duration": 5,
  "aspect_ratio": "16:9",
  "reference_image_urls": [
    "https://assets.example.com/video/masked-reference.png"
  ],
  "reference_videos": ["https://assets.example.com/video/motion.mp4"]
}
```

### 7.9 查询与下载

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

| 模型族               | 时长              | 分辨率         | 原生音频 | 参考素材限制                               |
| -------------------- | ----------------- | -------------- | -------- | ------------------------------------------ |
| `grok-video*`        | 4/6/8/10/12/15 秒 | 480p、720p     | 不支持   | 标准版 1 图，1.5 版 7 图                   |
| `happyhouse-1.0`     | 3-15 秒           | 720p、1080p    | 支持     | 9 图；含 1 视频时最多 5 图，总数最多 9     |
| `happyhouse-1.1`     | 3-15 秒           | 720p、1080p    | 支持     | 最多 9 图                                  |
| `minimax-h3-2k`      | 5-15 秒           | 固定 2K        | 支持     | 5 图、3 音频，总数最多 8；也支持首尾帧     |
| `omni-fast*`         | 固定约 10 秒      | 固定 720p      | 不支持   | 最多 5 图，或首尾帧                        |
| `omni-v2v*`          | 固定约 10 秒      | 固定 720p      | 不支持   | 必须且只能提供 1 条视频                    |
| `sd4-seedance-2.0*`  | 4-15 秒           | 480p、720p     | 支持     | 4 图、3 视频、1 音频，总数最多 8；或首尾帧 |
| `sd7-seedance-2.0-*` | 4-15 秒           | 由模型 ID 固定 | 支持     | 5 图、3 视频、3 音频，总数最多 11          |
| `sd8-seedance-2.0`   | 5/10/15 秒        | 模型固定       | 不支持   | 9 图、3 视频、3 音频，总数最多 15          |

支持的宽高比：

- Grok：`1:1`、`16:9`、`9:16`、`4:3`、`3:4`、`3:2`、`2:3`。
- Happyhouse：`16:9`、`9:16`、`1:1`、`3:4`、`4:3`。
- Minimax H3 与 SD4：`16:9`、`9:16`、`1:1`、`21:9`、`3:4`、`4:3`。
- Omni：`16:9`、`9:16`。
- SD7：`16:9`、`9:16`、`1:1`、`4:3`、`3:4`、`21:9`。
- SD8：`16:9`、`9:16`、`1:1`、`4:3`、`3:4`。

模型族限制：

- 当前启用模型统一使用 `duration`；不要传旧的 `seconds` 或 `audio` 布尔字段。
- 只有 Happyhouse、Minimax、SD4 和 SD7 接收 `generate_audio`；显式 `false` 会原样保留。
- Omni 固定时长模型不要传 `duration` 或 `resolution`；Omni V2V 必须使用 `reference_videos` 提供一条视频。
- SD7 的分辨率由模型 ID 决定；SD8 不接收分辨率字段。字段、时长或模型 ID 不匹配时，请求会在创建任务前返回 `400`。
- 普通图片、视频和音频参考分别使用 `reference_image_urls`、`reference_videos`、`reference_audios`；首尾帧只使用 `first_image_url`、`last_image_url`，不要混用。
- data URI 适合小图片；视频和音频参考应使用可由服务端访问的 HTTPS URL。不要使用需要 Cookie、Referer 或临时登录态的地址。
- SD4 首尾帧模式和 Minimax 首尾帧模式不能同时开启模型生成音频；SD8 含人物的参考图片必须遮挡眼部。

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

| 状态                        | 含义           | 客户端动作                      |
| --------------------------- | -------------- | ------------------------------- |
| `queued`、`pending`         | 已排队         | 5-10 秒后继续查询               |
| `processing`、`in_progress` | 正在生成或处理 | 继续查询，不要重复创建          |
| `completed`                 | 已完成         | 读取结果 URL，或请求 `/content` |
| `failed`、`cancelled`       | 已终止         | 停止查询并读取错误信息          |

`progress` 可能是 0-100 的数字，也可能是带 `%` 的字符串。失败原因依次检查 `error.message`、`reason`、`message`；完成结果依次检查 `video_url`、`metadata.video_url`、`metadata.url`、`data[0].url`。相对 URL 需要以当前选择的直连 Base URL 对应源站补全，或者直接调用本站 `/content` 接口。

Python 轮询示例：

```python
import os
import time
from urllib.parse import urljoin

import requests

base_url = os.environ.get(
    "YUAPI_MEDIA_BASE_URL", "https://vip.yuaiapi.com/v1"
).rstrip("/")
api_origin = base_url.removesuffix("/v1")
headers = {"Authorization": f"Bearer {os.environ['YUAPI_API_KEY']}"}

created = requests.post(
    f"{base_url}/videos",
    headers=headers,
    json={
        "model": "grok-video",
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

| HTTP 状态         | 常见原因                         | 处理方式                                  |
| ----------------- | -------------------------------- | ----------------------------------------- |
| `400`             | 参数、时长、分辨率或素材格式错误 | 修正请求，不要原样重试                    |
| `401`             | API Key 无效、过期或已删除       | 检查 Bearer 认证                          |
| `403`             | 余额、分组或模型权限不足         | 检查 Key 与账户状态                       |
| `404`             | 模型或任务不存在                 | 重新查询 `/v1/models`，核对任务 ID        |
| `429`             | 并发或上游限流                   | 降低并发，指数退避                        |
| `500/502/503/504` | 上游故障、排队或超时             | 保存 Request ID；先查询旧任务，再有限重试 |

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
