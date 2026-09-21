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
import { expect, test } from '@playwright/test'

const apiKeys = Array.from({ length: 12 }, (_, index) => ({
  id: index + 1,
  name: `mobile-scroll-key-${String(index + 1).padStart(2, '0')}`,
  key: '************',
  status: 1,
  remain_quota: 5_000_000,
  used_quota: 0,
  unlimited_quota: false,
  expired_time: -1,
  created_time: 1_789_848_000,
  accessed_time: 0,
  group: 'default',
  cross_group_retry: false,
  model_limits_enabled: false,
  model_limits: '',
  allow_ips: '',
}))

test.beforeEach(async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 667 })
  await page.addInitScript(() => {
    localStorage.setItem('i18nextLng', 'en')
    localStorage.setItem('setup_status_checked', 'true')
  })

  await page.route('**/api/**', async (route) => {
    const requestPath = new URL(route.request().url()).pathname

    if (requestPath === '/api/user/auth/refresh') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          data: {
            access_token: 'mobile-scroll-access-token',
            token_type: 'Bearer',
            access_expires_at: 4_102_444_800,
            user: {
              id: 1,
              username: 'mobile-scroll-user',
              role: 1,
              group: 'default',
            },
            session: {
              sid: 'mobile-scroll-session',
              current: true,
              login_method: 'password',
              ip: '127.0.0.1',
              user_agent: 'Playwright',
              created_at: 1_789_848_000,
              last_active_at: 1_789_848_000,
              expires_at: 4_102_444_800,
            },
          },
        }),
      })
      return
    }

    if (requestPath === '/api/status') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          data: {
            system_name: 'YUAPI',
            announcements_enabled: false,
            display_in_currency: true,
            quota_display_type: 'USD',
            quota_per_unit: 500_000,
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

    if (requestPath === '/api/user/self/group-availability') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          data: [
            {
              group: 'OpenAI',
              description: 'OpenAI compatible models',
              request_count: 300,
              success_count: 297,
              success_rate: 99,
              status: 'stable',
              observed_at: 1_789_848_000,
            },
            {
              group: 'Claude',
              description: 'Claude Messages models',
              request_count: 300,
              success_count: 294,
              success_rate: 98,
              status: 'stable',
              observed_at: 1_789_848_000,
            },
          ],
        }),
      })
      return
    }

    if (requestPath === '/api/token/') {
      await route.fulfill({
        status: 200,
        contentType: 'application/json',
        body: JSON.stringify({
          success: true,
          data: {
            items: apiKeys,
            total: apiKeys.length,
            page: 1,
            page_size: 20,
          },
        }),
      })
      return
    }

    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ success: true, data: {} }),
    })
  })
})

test('scrolling from the availability monitor reaches the last API key on mobile', async ({
  page,
}, testInfo) => {
  test.skip(testInfo.project.name !== 'chromium-mobile')

  await page.goto('/keys')

  const firstKey = page.getByText('mobile-scroll-key-01', { exact: true })
  const lastKey = page.getByText('mobile-scroll-key-12', { exact: true })
  await expect(firstKey).toBeVisible()
  await expect(lastKey).not.toBeInViewport()

  await page.getByRole('heading', { name: 'Group availability' }).hover()
  await page.mouse.wheel(0, 4_000)

  await expect(lastKey).toBeInViewport()
  await expect(firstKey).not.toBeInViewport()
})

test('desktop keeps the availability monitor visible while the key table scrolls', async ({
  page,
}, testInfo) => {
  test.skip(testInfo.project.name !== 'chromium-desktop')
  await page.setViewportSize({ width: 1440, height: 960 })

  await page.goto('/keys')

  const monitorHeading = page.getByRole('heading', {
    name: 'Group availability',
  })
  const firstKey = page.getByText('mobile-scroll-key-01', { exact: true })
  const lastKey = page.getByText('mobile-scroll-key-12', { exact: true })
  await expect(monitorHeading).toBeInViewport()
  await expect(firstKey).toBeInViewport()
  await expect(lastKey).not.toBeInViewport()

  await firstKey.hover()
  await page.mouse.wheel(0, 4_000)

  await expect(lastKey).toBeInViewport()
  await expect(monitorHeading).toBeInViewport()
})
