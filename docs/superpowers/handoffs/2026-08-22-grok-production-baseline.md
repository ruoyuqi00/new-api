# Grok 生产基线与价格交接

更新时间：2026-08-22（Asia/Shanghai）

## 一、基线范围

本文以 2026-08-22 完成热切换后正在承载流量的生产容器为唯一运行基线。文中只记录 Grok 模型、价格、媒体能力和回滚信息；品牌 UI、文字模型、缓存亲和、流恢复、Codex 前置、计费保护、数据库和其他渠道均保持当前生产行为。

不得使用旧 Grok 分支、镜像标签或提交号单独推断生产内容，也不得用旧分支整体覆盖当前生产文件。生产数据库和 `/data` 是权威状态，不恢复旧快照，不重置交易或素材。

## 二、当前生产运行产物

| 项目 | 当前值 |
| --- | --- |
| 活动镜像 | `yuapi:production-20260822-grok-40fcc8ca1` |
| 活动容器 | `newapi-grok-hotfix-rc5` |
| 容器状态 | `running`、`healthy`、重启次数 `0` |
| Caddy 当前目标 | `newapi-grok-hotfix-rc5:3000` |
| 生产源码提交 | `40fcc8ca1` |
| 生产分支 | `codex/grok-production-baseline-20260822` |
| 公开主页 SHA-256 | `97102537d7dba340791fa533746b7a66f6275c763e5af647f9c28c1a8b8c5b7f` |
| 回滚容器 | `newapi-grok-production-20260822-rc3` |
| 回滚镜像 | `yuapi:production-20260822-grok-d3921c36f` |

旧生产容器和镜像仍保留，Caddy 切换前运行配置也已单独备份。出现 UI 指纹变化、初始化向导、品牌缺失、Grok 任务 502、容器重启或非 Grok 回归时，应先恢复 Caddy 目标，不删除当前或旧容器。

## 三、价格状态

本次热切换没有新增价格变更；以下是当前生产代码和公开开发者文档中的已验证价格。基础价格以 USD 保存，之后只应用一次用户所属分组倍率，不能重复乘倍率。没有执行数据库价格覆盖更新。

### Grok Imagine 图片

| 模型 | 计费单位 | 基础价格 |
| --- | --- | ---: |
| `grok-imagine-image` | 每张 | `0.02619` |
| `grok-imagine-image-quality` | 每张 | `0.02619` |

### Grok Imagine 视频

| 分辨率 | 计费单位 | 基础价格 |
| --- | --- | ---: |
| `480p` | 每输出秒 | `0.0414` |
| `720p` | 每输出秒 | `0.0594` |
| `1080p` | 每输出秒 | `0.0774` |

三个模型 `grok-imagine-video`、`grok-imagine-video-1.5`、`grok-imagine-video-1.5-preview` 共用上述按秒价格；默认时长为 5 秒，允许整数 1-15 秒，分辨率只接受 `480p`、`720p`、`1080p` 或能明确解析出对应高度的尺寸别名。

### Grok 按次视频

这两个模型不能套用 Grok Imagine 的按秒价格：

| 模型 | 上游成本参考 | YuAPI 基础价格 | `下游多模态`（倍率 1.0） | `多模态创作`（倍率 1.2） |
| --- | ---: | ---: | ---: | ---: |
| `grok-video` | `0.69` | `0.828` | `0.828` | `0.9936` |
| `grok-video-1.5` | `1.39` | `1.668` | `1.668` | `2.0016` |

按次视频创建成功后只扣一次；状态查询、轮询、内容读取、缩略图回退和下载不重复扣费。分组倍率保持现有配置，不在模型价格中再次预乘。

## 四、模型能力边界

- Grok Imagine 图片使用同步图片接口，不支持参考图、音频和 seed。
- Grok Imagine 视频使用异步任务接口，支持一张参考图，不支持音频和 seed。
- `grok-video` 支持 4/6/8/10/12/15 秒、480p/720p，以及 `1:1`、`16:9`、`9:16`、`4:3`、`3:4`、`3:2`、`2:3`；`grok-video-1.5` 支持最多 7 张参考图片。
- `grok-imagine-edit` 不在已验证能力目录和默认价格中。
- 任务创建、轮询和内容代理不得把上游地址、账号池、Key、Token 或原始错误返回给下游。
- xAI 视频内容只在任务绑定的同源、同端口和 base path 范围内使用保存的选中 Key；跨域和重定向默认不携带凭据。

## 五、公开文档同步状态

以下三份当前基线文档已包含 Grok 图片、Grok Imagine 按秒视频、Grok 按次视频的模型 ID、参数边界和价格：

- `web/default/public/developer-docs/yucore-api.md`
- `web/default/public/developer-docs/yucore-api.zh-TW.md`
- `web/default/public/developer-docs/yucore-api.en.md`

文档中的价格表是公开说明；运行时模型目录和计费原语仍是实际行为的权威来源。更新任一价格前，必须同时更新三份公开文档、相关测试和本交接文档，并在私有候选中验证预估、预扣、结算和倍率只应用一次。

## 六、验证与回滚

当前代码验证通过：

```text
go test ./... -count=1
go test ./controller ./model ./service ./router ./relay/channel/task/xai -count=1
```

热切换后已连续检查 `api`、`global`、`vip` 的 `/api/status`，均返回 HTTP 200；候选容器健康且重启次数为 0。回滚时只需将 Caddy 运行目标恢复到 `newapi-grok-production-20260822-rc3:3000`，不要恢复数据库快照或修改用户余额。
