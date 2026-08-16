# Beginner Video API Documentation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Publish beginner-first Simplified Chinese, Traditional Chinese, and English video API documentation with a docs-only remembered language selector, synthetic screenshots, automated contract/privacy checks, and no regression to the existing public image API reference.

**Architecture:** Keep the existing Simplified Chinese Markdown URL stable and add two sibling Markdown files. Isolate locale selection and persistence in a pure TypeScript module, fetch each language under a language-scoped React Query key, and verify all three documents against one checker containing only YuAPI public facts. Capture API-key screenshots from a disposable local instance with synthetic responses so no real account or credential can enter the repository.

**Tech Stack:** React 19, TypeScript, TanStack Query, Base UI/shadcn ToggleGroup, i18next, Bun tests, Node ESM, Markdown, Playwright, Rsbuild.

---

## File Map

- Create `web/default/src/features/docs/document-locale.ts`: docs-only locale mapping, path lookup, versioned storage read/write.
- Create `web/default/src/features/docs/document-locale.test.ts`: locale precedence, invalid storage, and path-contract tests.
- Modify `web/default/src/features/docs/index.tsx`: segmented locale control, locale-scoped query, retry state, and current-language Markdown link.
- Modify through script `web/default/src/i18n/locales/{en,zh,fr,ja,ru,vi}.json`: labels used by the docs selector and retry state.
- Temporarily modify `web/default/scripts/add-missing-keys.mjs`: apply all six locale values, then restore the script before committing.
- Replace `web/default/public/developer-docs/yucore-api.md`: beginner-first Simplified Chinese guide and detailed reference.
- Create `web/default/public/developer-docs/yucore-api.zh-TW.md`: complete Traditional Chinese guide.
- Create `web/default/public/developer-docs/yucore-api.en.md`: complete English guide.
- Create `web/default/scripts/check-video-api-docs.mjs`: public model/path/price parity, JSON parsing, and privacy denylist checks.
- Create `web/default/scripts/check-video-api-docs.test.mjs`: regression tests for drift and forbidden-content detection.
- Modify `web/default/package.json`: add `docs:check` and Playwright development dependency.
- Modify `web/bun.lock`: lock Playwright dependency.
- Create `web/default/playwright.config.ts`: private local browser-test configuration.
- Create `web/default/e2e/video-api-docs.spec.ts`: locale switching, persistence, desktop/mobile layout, and screenshot capture.
- Create `web/default/public/developer-docs/assets/video-api-key-{zh,en}-0{1,2,3}.webp`: six synthetic-data screenshots.

### Task 1: Add a deterministic docs-locale contract

**Files:**
- Create: `web/default/src/features/docs/document-locale.test.ts`
- Create: `web/default/src/features/docs/document-locale.ts`

- [ ] **Step 1: Write the failing locale tests**

Create tests that lock the approved precedence and stable paths:

```ts
import { describe, expect, test } from 'bun:test'

import {
  API_DOCS_LOCALES,
  resolveApiDocsLocale,
  readApiDocsLocale,
  writeApiDocsLocale,
} from './document-locale'

describe('API documentation locale', () => {
  test('prefers a valid remembered choice', () => {
    expect(resolveApiDocsLocale('zh-TW', 'en', ['zh-CN'])).toBe('zh-TW')
  })

  test('maps the supported site language before browser languages', () => {
    expect(resolveApiDocsLocale(null, 'en-US', ['zh-TW'])).toBe('en')
  })

  test('uses browser language when the site locale has no docs edition', () => {
    expect(resolveApiDocsLocale(null, 'fr', ['zh-HK', 'en'])).toBe('zh-TW')
  })

  test('falls back to Simplified Chinese', () => {
    expect(resolveApiDocsLocale('invalid', 'ja', ['ru'])).toBe('zh-CN')
  })

  test('keeps the legacy Simplified Chinese URL', () => {
    expect(API_DOCS_LOCALES['zh-CN'].path).toBe(
      '/developer-docs/yucore-api.md'
    )
    expect(API_DOCS_LOCALES['zh-TW'].path).toBe(
      '/developer-docs/yucore-api.zh-TW.md'
    )
    expect(API_DOCS_LOCALES.en.path).toBe(
      '/developer-docs/yucore-api.en.md'
    )
  })

  test('contains storage failures and persists only a locale code', () => {
    const values = new Map<string, string>()
    const storage = {
      getItem: (key: string) => values.get(key) ?? null,
      setItem: (key: string, value: string) => values.set(key, value),
    }
    writeApiDocsLocale('en', storage)
    expect(readApiDocsLocale(storage)).toBe('en')
    expect([...values.values()]).toEqual(['en'])

    const broken = {
      getItem: () => {
        throw new Error('disabled')
      },
      setItem: () => {
        throw new Error('disabled')
      },
    }
    expect(readApiDocsLocale(broken)).toBeNull()
    expect(() => writeApiDocsLocale('zh-TW', broken)).not.toThrow()
  })
})
```

- [ ] **Step 2: Run the test and verify the missing module failure**

Run from `web/default`:

```powershell
bun test src/features/docs/document-locale.test.ts
```

Expected: FAIL because `./document-locale` does not exist.

- [ ] **Step 3: Implement the pure locale module**

Create the complete public contract:

```ts
export type ApiDocsLocale = 'zh-CN' | 'zh-TW' | 'en'

export interface ApiDocsLocaleConfig {
  labelKey: 'Simplified Chinese' | 'Traditional Chinese' | 'English'
  path: string
}

interface StorageLike {
  getItem(key: string): string | null
  setItem(key: string, value: string): void
}

export const API_DOCS_LOCALE_STORAGE_KEY = 'yuapi:api-docs-locale:v1'

export const API_DOCS_LOCALES: Record<
  ApiDocsLocale,
  ApiDocsLocaleConfig
> = {
  'zh-CN': {
    labelKey: 'Simplified Chinese',
    path: '/developer-docs/yucore-api.md',
  },
  'zh-TW': {
    labelKey: 'Traditional Chinese',
    path: '/developer-docs/yucore-api.zh-TW.md',
  },
  en: {
    labelKey: 'English',
    path: '/developer-docs/yucore-api.en.md',
  },
}

function mapApiDocsLocale(language?: string | null): ApiDocsLocale | null {
  const normalized = language?.trim().replace('_', '-').toLowerCase()
  if (!normalized) return null
  if (
    normalized === 'zh-tw' ||
    normalized === 'zh-hk' ||
    normalized.startsWith('zh-hant')
  ) {
    return 'zh-TW'
  }
  if (normalized === 'zh' || normalized.startsWith('zh-')) return 'zh-CN'
  if (normalized === 'en' || normalized.startsWith('en-')) return 'en'
  return null
}

export function resolveApiDocsLocale(
  remembered: string | null,
  siteLanguage: string | null,
  browserLanguages: readonly string[]
): ApiDocsLocale {
  if (remembered && remembered in API_DOCS_LOCALES) {
    return remembered as ApiDocsLocale
  }
  const siteLocale = mapApiDocsLocale(siteLanguage)
  if (siteLocale) return siteLocale
  for (const language of browserLanguages) {
    const browserLocale = mapApiDocsLocale(language)
    if (browserLocale) return browserLocale
  }
  return 'zh-CN'
}

export function readApiDocsLocale(
  storage?: Pick<StorageLike, 'getItem'>
): ApiDocsLocale | null {
  try {
    const target =
      storage ?? (typeof window === 'undefined' ? null : window.localStorage)
    if (!target) return null
    const value = target.getItem(API_DOCS_LOCALE_STORAGE_KEY)
    return value && value in API_DOCS_LOCALES ? (value as ApiDocsLocale) : null
  } catch {
    return null
  }
}

export function writeApiDocsLocale(
  locale: ApiDocsLocale,
  storage?: Pick<StorageLike, 'setItem'>
): void {
  try {
    const target =
      storage ?? (typeof window === 'undefined' ? null : window.localStorage)
    target?.setItem(API_DOCS_LOCALE_STORAGE_KEY, locale)
  } catch {
    // The selector still works for this tab when storage is unavailable.
  }
}
```

- [ ] **Step 4: Run the focused test**

```powershell
bun test src/features/docs/document-locale.test.ts
```

Expected: 6 tests PASS.

- [ ] **Step 5: Commit the locale contract**

```powershell
git add web/default/src/features/docs/document-locale.ts web/default/src/features/docs/document-locale.test.ts
git commit -m "feat: add video docs locale contract"
```

### Task 2: Add the docs-only segmented language selector

**Files:**
- Modify: `web/default/src/features/docs/index.tsx`
- Temporarily modify: `web/default/scripts/add-missing-keys.mjs`
- Modify through script: `web/default/src/i18n/locales/{en,zh,fr,ja,ru,vi}.json`

- [ ] **Step 1: Run the current i18n sync baseline**

```powershell
Set-Location web/default
bun run i18n:sync
```

Expected: command exits 0. Record existing report counts; do not broaden this task to unrelated existing translations.

- [ ] **Step 2: Refactor the docs page around the locale contract**

Use lazy state initialization so storage is read once, keep the query key locale-scoped, and use Base UI ToggleGroup's array value contract:

```tsx
const [docsLocale, setDocsLocale] = useState<ApiDocsLocale>(() =>
  resolveApiDocsLocale(
    readApiDocsLocale(),
    i18n.resolvedLanguage ?? i18n.language,
    typeof navigator === 'undefined' ? [] : navigator.languages
  )
)
const docsConfig = API_DOCS_LOCALES[docsLocale]
const docsQuery = useQuery({
  queryKey: ['public-api-docs', docsLocale],
  queryFn: () => fetchApiDocs(docsConfig.path),
  staleTime: Number.POSITIVE_INFINITY,
})

function handleDocsLocaleChange(values: unknown[]): void {
  const nextLocale = values[0]
  if (typeof nextLocale !== 'string' || !(nextLocale in API_DOCS_LOCALES)) {
    return
  }
  const locale = nextLocale as ApiDocsLocale
  setDocsLocale(locale)
  writeApiDocsLocale(locale)
}
```

Render the control in the sidebar above the Markdown/Pricing actions:

```tsx
<ToggleGroup
  value={[docsLocale]}
  onValueChange={handleDocsLocaleChange}
  variant='outline'
  size='sm'
  spacing={0}
  aria-label={t('Documentation language')}
  className='mt-5 grid w-full grid-cols-3'
>
  {(Object.entries(API_DOCS_LOCALES) as Array<
    [ApiDocsLocale, ApiDocsLocaleConfig]
  >).map(([locale, config]) => (
    <ToggleGroupItem key={locale} value={locale} className='min-w-0 px-2'>
      <span className='truncate'>{t(config.labelKey)}</span>
    </ToggleGroupItem>
  ))}
</ToggleGroup>
```

Change the Markdown link to `docsConfig.path`, pass the path into `fetchApiDocs`, and render a retry button using the existing `RefreshCw` icon when the current locale request fails. Do not retain the previous language body while the new language is loading.

- [ ] **Step 3: Add the three new UI keys through the sanctioned script**

Replace `newKeys` in `add-missing-keys.mjs` for this run with exactly:

```js
const newKeys = {
  en: {
    'Documentation language': 'Documentation language',
    'Simplified Chinese': 'Simplified Chinese',
    'Traditional Chinese': 'Traditional Chinese',
  },
  zh: {
    'Documentation language': '文档语言',
    'Simplified Chinese': '简体中文',
    'Traditional Chinese': '繁體中文',
  },
  fr: {
    'Documentation language': 'Langue de la documentation',
    'Simplified Chinese': 'Chinois simplifié',
    'Traditional Chinese': 'Chinois traditionnel',
  },
  ja: {
    'Documentation language': 'ドキュメントの言語',
    'Simplified Chinese': '簡体字中国語',
    'Traditional Chinese': '繁体字中国語',
  },
  ru: {
    'Documentation language': 'Язык документации',
    'Simplified Chinese': 'Упрощённый китайский',
    'Traditional Chinese': 'Традиционный китайский',
  },
  vi: {
    'Documentation language': 'Ngôn ngữ tài liệu',
    'Simplified Chinese': 'Tiếng Trung giản thể',
    'Traditional Chinese': 'Tiếng Trung phồn thể',
  },
}
```

Then run:

```powershell
node scripts/add-missing-keys.mjs
bun run i18n:sync
```

Expected: each locale reports 3 applied translations, sync exits 0, and all six locale files contain the keys. Restore `add-missing-keys.mjs` to its pre-task contents with `apply_patch`; it must not be staged.

- [ ] **Step 4: Verify React and locale changes**

```powershell
bun test src/features/docs/document-locale.test.ts
bun run typecheck
bunx oxlint src/features/docs/index.tsx src/features/docs/document-locale.ts src/features/docs/document-locale.test.ts
bun run i18n:sync
```

Expected: all commands exit 0.

- [ ] **Step 5: Commit the selector and locale values**

```powershell
Set-Location ../..
git add web/default/src/features/docs/index.tsx web/default/src/i18n/locales/en.json web/default/src/i18n/locales/zh.json web/default/src/i18n/locales/fr.json web/default/src/i18n/locales/ja.json web/default/src/i18n/locales/ru.json web/default/src/i18n/locales/vi.json
git commit -m "feat: add language selector to API docs"
```

### Task 3: Rewrite the Simplified Chinese guide for first-time API users

**Files:**
- Modify: `web/default/public/developer-docs/yucore-api.md`

- [ ] **Step 1: Replace the document with the approved beginner-first structure**

Use these exact top-level sections, in order. Keep the existing public image-generation and image-editing material as a sanitized appendix after the video guide, so the video rewrite does not remove an already documented API:

```markdown
# YuAPI 视频 API：从零开始生成第一个视频
## 1. 开始前：账号、API Key 和任务 ID
## 2. 创建你的第一个 API Key
## 3. 验证 API Key
## 4. 使用 seedance-2.0 创建视频
## 5. 查询原任务并下载视频
## 6. 把测试 Key 调整为生产安全配置
## 7. 视频模型与公开价格
## 8. 视频任务协议
## 9. 模型参数与参考素材限制
## 10. Windows PowerShell 完整示例
## 11. macOS/Linux curl 完整示例
## 12. Python 完整示例
## 13. Node.js 服务端示例
## 14. 状态、错误和安全重试
## 15. 接入前检查清单
## 16. 图片 API 参考
```

The first screen after the title must state that the website password is not an API Key, show `/keys`, recommend `多模态创作`, and explain the test-key versus production-key settings. Mention `下游多模态` only in an advanced note without internal routing details.

- [ ] **Step 2: Add the exact first-task contract**

Use only YuAPI public paths and a pure prompt request:

```json
{
  "model": "seedance-2.0",
  "prompt": "清晨的海边木栈道，镜头缓慢向前推进，柔和自然光，真实电影质感",
  "duration": 4,
  "aspect_ratio": "16:9",
  "generate_audio": true
}
```

Document these paths exactly. Use `https://api.yuaiapi.com/v1` as the beginner default, and also identify `https://vip.yuaiapi.com/v1` as the existing VIP entry point without making any claim about internal routing:

```text
GET  https://api.yuaiapi.com/v1/models
POST https://api.yuaiapi.com/v1/videos
GET  https://api.yuaiapi.com/v1/videos/{task_id}
GET  https://api.yuaiapi.com/v1/videos/{task_id}/content
```

The image appendix must retain the public `/v1/images/generations` and `/v1/images/edits` contracts, model IDs, public prices, and safe examples that already exist in the document. Rewrite any provider, source-cost, markup, margin, or routing language into YuAPI-only public wording.

State next to the polling example: never repeat the POST because a client timed out or the task remains queued/processing; keep polling the persisted task ID.

- [ ] **Step 3: Preserve the exact approved public price table**

Place this table between markers used by the checker:

```markdown
<!-- video-model-catalog:start -->
| 模型 | `多模态创作` 单次价格 |
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
```

Keep per-generation wording. Do not expose base cost, source price, markup, margin, provider grouping, or capacity observations.

- [ ] **Step 4: Add complete platform examples**

Every example must read `YUAPI_API_KEY` from the environment, use `https://api.yuaiapi.com/v1`, save one returned task ID, poll only that ID, and download content. The Python loop must use a deadline and terminal sets:

```python
SUCCESS = {"completed", "succeeded", "success"}
FAILURE = {"failed", "canceled", "cancelled"}
deadline = time.monotonic() + 15 * 60
while time.monotonic() < deadline:
    task = requests.get(f"{BASE_URL}/videos/{task_id}", headers=headers, timeout=60).json()
    status = str(task.get("status", "")).lower()
    if status in SUCCESS:
        break
    if status in FAILURE:
        raise RuntimeError(f"video task failed: {task}")
    time.sleep(5)
else:
    raise TimeoutError(f"video task still processing: {task_id}")
```

The Node example must state that it runs on a trusted server, never in browser JavaScript.

- [ ] **Step 5: Perform the first public-content scan and commit**

```powershell
rg -n '上游|供应源|成本|加价|利润|渠道 ID|adapter|provider account|capacity observation|cangyuansuanli|@|sk-[A-Za-z0-9_-]{12,}' web/default/public/developer-docs/yucore-api.md
```

Expected: no matches. Then commit:

```powershell
git add web/default/public/developer-docs/yucore-api.md
git commit -m "docs: add beginner video API guide"
```

### Task 4: Add complete Traditional Chinese and English editions

**Files:**
- Create: `web/default/public/developer-docs/yucore-api.zh-TW.md`
- Create: `web/default/public/developer-docs/yucore-api.en.md`

- [ ] **Step 1: Translate all narrative content without changing technical literals**

Both documents must contain all 16 sections from Task 3, including the image API appendix. Keep these tokens byte-for-byte identical across languages:

```text
api.yuaiapi.com
vip.yuaiapi.com
/v1/models
/v1/videos
/v1/videos/{task_id}
/v1/videos/{task_id}/content
/v1/images/generations
/v1/images/edits
YUAPI_API_KEY
seedance-2.0
queued processing completed failed canceled
```

Keep every JSON key, model ID, duration, price, and code block behavior unchanged. Translate headings, instructions, warnings, errors, comments, and the retained image appendix naturally. Traditional Chinese uses `多模態創作` in prose while preserving any literal UI label inside backticks as it appears in the current site.

- [ ] **Step 2: Copy the marked price table with translated headers only**

Use the same `video-model-catalog:start/end` comments and the exact 13 model/price rows from Task 3. Do not sort models differently.

- [ ] **Step 3: Verify all JSON blocks parse**

Run a temporary read-only extraction command from `web/default`:

```powershell
Set-Location web/default
node -e "const fs=require('fs');for(const f of ['public/developer-docs/yucore-api.md','public/developer-docs/yucore-api.zh-TW.md','public/developer-docs/yucore-api.en.md']){const s=fs.readFileSync(f,'utf8');for(const m of s.matchAll(/```json\s*([\s\S]*?)```/g))JSON.parse(m[1]);console.log('JSON OK',f)}"
Set-Location ../..
```

Expected: `JSON OK` for all three files.

- [ ] **Step 4: Scan all editions and commit**

```powershell
rg -n '上游|上游|供應源|成本|加價|利润|利潤|adapter|provider account|source cost|markup|capacity observation|cangyuansuanli|sk-[A-Za-z0-9_-]{12,}' web/default/public/developer-docs/yucore-api*.md
```

Expected: no matches. Then commit:

```powershell
git add web/default/public/developer-docs/yucore-api.zh-TW.md web/default/public/developer-docs/yucore-api.en.md
git commit -m "docs: add traditional Chinese and English video guides"
```

### Task 5: Enforce parity and privacy with a docs checker

**Files:**
- Create: `web/default/scripts/check-video-api-docs.test.mjs`
- Create: `web/default/scripts/check-video-api-docs.mjs`
- Modify: `web/default/package.json`

- [ ] **Step 1: Write failing checker tests**

Test exported `checkDocument` and `checkParity` functions with one valid in-memory document and mutations that remove a model, alter a price, break JSON, or add a forbidden dependency phrase. Assert exact diagnostic codes: `MODEL_SET_MISMATCH`, `PRICE_MISMATCH`, `INVALID_JSON`, and `PRIVATE_CONTENT`.

```js
import { describe, expect, test } from 'bun:test'
import { checkDocument, checkParity } from './check-video-api-docs.mjs'

test('rejects a changed public price', () => {
  const changed = validDocument.replace('| `seedance-2.0` | 5.616 |', '| `seedance-2.0` | 5.5 |')
  expect(() => checkDocument('changed.md', changed)).toThrow('PRICE_MISMATCH')
})

test('rejects internal dependency wording', () => {
  expect(() => checkDocument('private.md', `${validDocument}\n上游账号`)).toThrow(
    'PRIVATE_CONTENT'
  )
})

test('requires the three normalized contracts to match', () => {
  expect(() => checkParity([validContract, validContract, changedContract])).toThrow(
    'DOCUMENT_PARITY_MISMATCH'
  )
})
```

- [ ] **Step 2: Run the test and verify the missing module failure**

```powershell
bun test scripts/check-video-api-docs.test.mjs
```

Expected: FAIL because the checker module does not exist.

- [ ] **Step 3: Implement the checker with only public facts**

The module must export `checkDocument`, `checkParity`, and `main`. Define the expected catalog exactly as:

```js
export const EXPECTED_VIDEO_PRICES = new Map([
  ['grok-video', '0.9936'],
  ['grok-video-1.5', '2.0016'],
  ['happyhouse-1.0', '6.48'],
  ['happyhouse-1.1', '4.176'],
  ['minimax-h3-2k', '5.04'],
  ['omni-fast', '0.95388'],
  ['omni-fast-no-water', '1.1664'],
  ['omni-v2v', '1.27536'],
  ['omni-v2v-no-water', '1.4904'],
  ['sd7-seedance-2.0-1080p', '7.056'],
  ['sd7-seedance-2.0-720p', '5.616'],
  ['sd8-seedance-2.0', '4.176'],
  ['seedance-2.0', '5.616'],
])
```

Parse only rows between the two catalog markers, require the four public paths, parse every `json` fence, and reject generic internal-dependency/cost terms. Optionally read newline-delimited regexes from `YUAPI_PRIVATE_PATTERN_FILE`; never print matching secret text, only the file name and rule number.

Run `main()` only when `path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)`. `main()` reads the three fixed docs paths, checks each, then compares normalized model/price/path/status contracts. This keeps the command compatible with Node while allowing Bun tests to import the module without executing the CLI.

- [ ] **Step 4: Add and run the package command**

Add:

```json
"docs:check": "node scripts/check-video-api-docs.mjs"
```

Run:

```powershell
Set-Location web/default
bun test scripts/check-video-api-docs.test.mjs
bun run docs:check
```

Expected: tests PASS and the command prints one redacted success line per document plus `DOCUMENT_PARITY_OK`.

- [ ] **Step 5: Commit the checker**

```powershell
Set-Location ../..
git add web/default/scripts/check-video-api-docs.mjs web/default/scripts/check-video-api-docs.test.mjs web/default/package.json
git commit -m "test: enforce video API docs parity"
```

### Task 6: Capture synthetic screenshots and browser-test the docs page

**Files:**
- Modify: `web/default/package.json`
- Modify: `web/bun.lock`
- Create: `web/default/playwright.config.ts`
- Create: `web/default/e2e/video-api-docs.spec.ts`
- Create: `web/default/public/developer-docs/assets/video-api-key-{zh,en}-0{1,2,3}.webp`
- Modify: all three Markdown documents to embed the approved screenshots and localized alt text.

- [ ] **Step 1: Add the reproducible browser-test dependency**

```powershell
Set-Location web/default
bun add -d @playwright/test
bunx playwright install chromium
```

Expected: `package.json` and `web/bun.lock` change and Chromium installs successfully.

- [ ] **Step 2: Configure a private local test target**

Create a config that uses `PLAYWRIGHT_BASE_URL` or defaults to `http://127.0.0.1:31845`, runs Chromium desktop and mobile projects, disables retries locally, and writes transient output under `test-results/`.

- [ ] **Step 3: Write the locale and layout browser test**

The test must:

1. Navigate to `/docs` with no docs storage key and verify the expected browser-language mapping.
2. Select each of the three ToggleGroup values and assert the matching H1.
3. Reload after selecting English and assert English remains selected.
4. Assert the Markdown link targets the current language file.
5. Assert `document.documentElement.scrollWidth <= document.documentElement.clientWidth` at desktop and mobile widths.
6. Record `pageerror` and failed same-origin requests, allowing only the known anonymous auth-refresh 401.

- [ ] **Step 4: Capture API-key screenshots from synthetic responses**

Start a disposable local SQLite instance, initialize a synthetic root account, and log in only to that disposable instance. In Playwright, intercept token list, token-key reveal, user groups, and user model responses so the visible data is exactly:

```text
username: docs-demo
key name: video-quickstart
visible key value: fully masked; no complete secret-like string is rendered
group: 多模态创作
model: seedance-2.0
quota: 25.00
```

Capture these states in Chinese and English UI languages at 1440x960:

1. API Keys page with the Create API Key button.
2. Create drawer showing name, group, expiry, and quota controls.
3. Advanced model limit plus a synthetic created-row copy/reveal state.

Write WebP files directly to the six approved asset paths. No production API, user, balance, Key, cookie, or task ID may be used.

- [ ] **Step 5: Embed and inspect the screenshots**

Add the three Chinese images to both Chinese documents with language-specific captions, and the three English images to the English document. Every image gets useful localized alt text.

Open all six files with the local image viewer. Check visible text, cropping, cursor/focus artifacts, and that no content overlaps. Inspect metadata with:

```powershell
exiftool public/developer-docs/assets/video-api-key-*.webp
```

Expected: no author, comment, source URL, account, or location metadata. If `exiftool` is unavailable, use `magick identify -verbose` and record that fallback.

- [ ] **Step 6: Run browser tests and commit**

```powershell
bunx playwright test e2e/video-api-docs.spec.ts --project=chromium-desktop
bunx playwright test e2e/video-api-docs.spec.ts --project=chromium-mobile
bun run docs:check
bun run typecheck
bun run build
```

Expected: all commands exit 0. Commit:

```powershell
Set-Location ../..
git add web/default/package.json web/bun.lock web/default/playwright.config.ts web/default/e2e/video-api-docs.spec.ts web/default/public/developer-docs
git commit -m "test: verify multilingual video API docs"
```

### Task 7: Complete local verification and hand off to the asset plan

**Files:**
- Modify: `docs/superpowers/handoffs/2026-08-15-cangyuan-video-validation.md`

- [ ] **Step 1: Run all focused checks from a clean shell**

```powershell
Set-Location web/default
bun test src/features/docs/document-locale.test.ts scripts/check-video-api-docs.test.mjs
bun run docs:check
bun run i18n:sync
bun run typecheck
bunx oxlint src/features/docs/index.tsx src/features/docs/document-locale.ts src/features/docs/document-locale.test.ts
bun run build
Set-Location ../..
git diff --check
```

Expected: all commands exit 0.

- [ ] **Step 2: Re-run the privacy scan on sources and build output**

Run the checker with the operator-local private pattern file, then scan the generated frontend output without printing matched secret values. Any match blocks release.

- [ ] **Step 3: Record redacted evidence**

Append only commit IDs, command outcomes, the six screenshot file names, the three document URLs, and desktop/mobile pass counts to the validation handoff. Do not record local absolute paths, credentials, cookies, real identifiers, or provider facts.

- [ ] **Step 4: Commit verification evidence**

```powershell
git add docs/superpowers/handoffs/2026-08-15-cangyuan-video-validation.md
git commit -m "docs: record multilingual API guide verification"
```

Stop here and continue with `docs/superpowers/plans/2026-08-16-admin-video-sample-assets.md`. Do not deploy the docs separately; one candidate will contain both approved sub-projects.
