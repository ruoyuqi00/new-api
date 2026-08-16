/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { mkdir } from 'node:fs/promises'
import path from 'node:path'

import { expect, test } from '@playwright/test'

const docs = [
  {
    heading: 'YuAPI 视频 API：从零开始生成第一个视频',
    path: '/developer-docs/yucore-api.md',
  },
  {
    heading: 'YuAPI 影片 API：從零開始產生第一支影片',
    path: '/developer-docs/yucore-api.zh-TW.md',
  },
  {
    heading: 'YuAPI Video API: Generate Your First Video from Scratch',
    path: '/developer-docs/yucore-api.en.md',
  },
] as const

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    if (!sessionStorage.getItem('video-docs-e2e-initialized')) {
      localStorage.removeItem('yuapi:api-docs-locale:v1')
      sessionStorage.setItem('video-docs-e2e-initialized', 'true')
    }
    localStorage.setItem('i18nextLng', 'fr')
    localStorage.setItem('setup_status_checked', 'true')
  })
  await page.route('**/api/**', async (route) => {
    const path = new URL(route.request().url()).pathname
    if (path === '/api/user/auth/refresh') {
      await route.fulfill({
        status: 401,
        contentType: 'application/json',
        body: JSON.stringify({ success: false, message: 'anonymous test' }),
      })
      return
    }
    if (path === '/api/status') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          data: {
            system_name: 'YUAPI',
            announcements_enabled: false,
          },
        }),
      })
      return
    }
    if (path === '/api/notice') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ success: true, data: '' }),
      })
      return
    }
    if (path === '/api/setup') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          data: { status: true, root_init: true, database_type: 'sqlite' },
        }),
      })
      return
    }
    await route.fulfill({
      status: 500,
      contentType: 'application/json',
      body: JSON.stringify({ success: false, message: 'unexpected test API' }),
    })
  })
})

test('switches, remembers, and lays out every documentation edition', async ({
  page,
  browserName,
}, testInfo) => {
  const pageErrors: string[] = []
  const failedRequests: string[] = []
  const failedResponses: string[] = []

  page.on('pageerror', (error) => pageErrors.push(error.message))
  page.on('requestfailed', (request) => {
    if (new URL(request.url()).origin === new URL(page.url()).origin) {
      failedRequests.push(request.url())
    }
  })
  page.on('response', (response) => {
    if (response.status() < 400) return
    const url = new URL(response.url())
    if (url.origin !== new URL(page.url()).origin) return
    if (
      response.status() === 401 &&
      url.pathname === '/api/user/auth/refresh'
    ) {
      return
    }
    failedResponses.push(`${response.status()} ${url.pathname}`)
  })

  await page.goto('/docs')
  const initial =
    testInfo.project.name === 'chromium-mobile' ? docs[1] : docs[2]
  await expect(page.getByRole('heading', { level: 1 })).toHaveText(
    initial.heading
  )

  const localeButtons = page.locator('[data-slot="toggle-group-item"]')
  await expect(localeButtons).toHaveCount(3)
  for (const [index, document] of docs.entries()) {
    await localeButtons.nth(index).click()
    await expect(page.getByRole('heading', { level: 1 })).toHaveText(
      document.heading
    )
    await expect(
      page.locator('a').filter({ hasText: /^Markdown$/ })
    ).toHaveAttribute('href', document.path)
  }

  await page.reload()
  await expect(page.getByRole('heading', { level: 1 })).toHaveText(
    docs[2].heading
  )
  await expect(localeButtons.nth(2)).toHaveAttribute('aria-pressed', 'true')

  const hasHorizontalOverflow = await page.evaluate(
    () =>
      document.documentElement.scrollWidth >
      document.documentElement.clientWidth
  )
  expect(hasHorizontalOverflow, `${browserName} has horizontal overflow`).toBe(
    false
  )
  expect(pageErrors).toEqual([])
  expect(failedRequests).toEqual([])
  expect(failedResponses).toEqual([])
})

test('captures synthetic API key documentation screenshots', async ({
  page,
}, testInfo) => {
  test.skip(testInfo.project.name !== 'chromium-desktop')
  test.skip(process.env.VIDEO_DOCS_CAPTURE_SCREENSHOTS !== '1')

  const username = process.env.VIDEO_DOCS_E2E_USERNAME
  const password = process.env.VIDEO_DOCS_E2E_PASSWORD
  if (!username || !password) {
    throw new Error('Disposable screenshot credentials are required')
  }

  const captureDir = path.resolve(
    process.cwd(),
    '../../.local-tests/video-api-docs-screenshots'
  )
  await mkdir(captureDir, { recursive: true })

  await page.addInitScript(() => {
    localStorage.setItem('setup_status_checked', 'true')
    localStorage.setItem(
      'i18nextLng',
      sessionStorage.getItem('video-docs-capture-locale') ?? 'en'
    )
  })
  await page.route('**/api/**', async (route) => {
    const request = route.request()
    const requestPath = new URL(request.url()).pathname
    if (requestPath === '/api/status') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          data: {
            system_name: 'YUAPI',
            password_login_enabled: true,
            announcements_enabled: false,
            display_in_currency: true,
            quota_display_type: 'USD',
            quota_per_unit: 500000,
            usd_exchange_rate: 1,
          },
        }),
      })
      return
    }
    if (requestPath === '/api/notice') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ success: true, data: '' }),
      })
      return
    }
    if (
      requestPath === '/api/token/' &&
      request.method().toUpperCase() === 'GET'
    ) {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          data: {
            items: [
              {
                id: 1,
                name: 'video-quickstart',
                key: '************',
                status: 1,
                remain_quota: 12500000,
                used_quota: 0,
                unlimited_quota: false,
                expired_time: -1,
                created_time: 1786838400,
                accessed_time: 0,
                group: '多模态创作',
                cross_group_retry: false,
                model_limits_enabled: true,
                model_limits: 'seedance-2.0',
                allow_ips: '',
              },
            ],
            total: 1,
            page: 1,
            page_size: 20,
          },
        }),
      })
      return
    }
    if (requestPath === '/api/token/1/key') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ success: true, data: { key: '************' } }),
      })
      return
    }
    if (requestPath === '/api/user/self/groups') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          data: {
            多模态创作: {
              desc: '视频与图片创作',
              ratio: 1,
            },
          },
        }),
      })
      return
    }
    if (requestPath === '/api/user/models') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({ success: true, data: ['seedance-2.0'] }),
      })
      return
    }
    await route.continue()
  })

  await page.goto('/sign-in')
  await page.getByLabel(/Username or Email/i).fill(username)
  await page.getByLabel(/^Password$/i).fill(password)
  await page.getByRole('button', { name: /^Sign in$/i }).click()
  await expect(page).not.toHaveURL(/\/sign-in/)

  const editions = [
    {
      locale: 'zh',
      prefix: 'zh',
      create: '创建 API 密钥',
      name: '名称',
      month: '1 个月',
      unlimited: '无限配额',
      close: '关闭',
      fullKey: '完整 API 密钥',
    },
    {
      locale: 'en',
      prefix: 'en',
      create: 'Create API Key',
      name: 'Name',
      month: '1M',
      unlimited: 'Unlimited Quota',
      close: 'Close',
      fullKey: 'Full API Key',
    },
  ] as const

  for (const edition of editions) {
    await page.evaluate((locale) => {
      sessionStorage.setItem('video-docs-capture-locale', locale)
      localStorage.setItem('i18nextLng', locale)
    }, edition.locale)
    await page.goto('/keys')
    await page.reload()

    await expect(page.getByRole('heading', { level: 2 }).first()).toBeVisible()
    await expect(
      page.getByText('video-quickstart', { exact: true })
    ).toBeVisible()
    await page.screenshot({
      path: path.join(captureDir, `video-api-key-${edition.prefix}-01.png`),
      animations: 'disabled',
      caret: 'hide',
    })

    await page.getByRole('button', { name: edition.create }).click()
    const drawer = page.getByRole('dialog')
    await expect(
      drawer.getByText(edition.create, { exact: true })
    ).toBeVisible()
    await drawer
      .getByLabel(edition.name, { exact: true })
      .fill('video-quickstart')
    await drawer.getByRole('combobox').first().click()
    await page.getByRole('option', { name: /多模态创作/ }).click()
    await drawer.getByRole('button', { name: edition.month }).click()
    await drawer.getByRole('switch', { name: edition.unlimited }).click()
    await drawer.locator('input[type="number"]').nth(1).fill('25.00')
    await page.screenshot({
      path: path.join(captureDir, `video-api-key-${edition.prefix}-02.png`),
      animations: 'disabled',
      caret: 'hide',
    })

    await drawer.getByRole('button', { name: edition.close }).first().click()
    await expect(drawer).toHaveCount(0)
    const maskedKeyButton = page.getByRole('button', {
      name: 'sk-************',
    })
    await maskedKeyButton.click()
    await expect(page.getByText(edition.fullKey, { exact: true })).toBeVisible()
    await expect(
      page.locator('[data-slot="popover-content"] input[readonly]')
    ).toHaveValue('sk-************')
    await page.getByText(/1 model\(s\)/).hover()
    await expect(page.getByText('seedance-2.0', { exact: true })).toBeVisible()
    await page.screenshot({
      path: path.join(captureDir, `video-api-key-${edition.prefix}-03.png`),
      animations: 'disabled',
      caret: 'hide',
    })
  }
})
