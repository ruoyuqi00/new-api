/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { expect, test } from '@playwright/test'

test.beforeEach(async ({ page }) => {
  await page.addInitScript(() => {
    localStorage.setItem('i18nextLng', 'zh')
    localStorage.setItem('setup_status_checked', 'true')
  })

  await page.route('**/api/**', async (route) => {
    const path = new URL(route.request().url()).pathname
    let data: unknown = {}

    if (path === '/api/user/auth/refresh') {
      data = {
        access_token: 'wallet-preview-access-token',
        token_type: 'Bearer',
        access_expires_at: 4_102_444_800,
        user: { id: 1, username: 'wallet-preview', role: 1, group: 'default' },
        session: {
          sid: 'wallet-preview-session',
          current: true,
          login_method: 'password',
          ip: '127.0.0.1',
          user_agent: 'Playwright',
          created_at: 1_789_848_000,
          last_active_at: 1_789_848_000,
          expires_at: 4_102_444_800,
        },
      }
    } else if (path === '/api/status') {
      data = {
        system_name: 'YUAPI',
        announcements_enabled: false,
        display_in_currency: true,
        quota_display_type: 'USD',
        quota_per_unit: 500_000,
        usd_exchange_rate: 1,
      }
    } else if (path === '/api/notice') {
      data = ''
    } else if (path === '/api/user/self') {
      data = {
        id: 1,
        username: 'wallet-preview',
        quota: 5_000_000,
        used_quota: 0,
        request_count: 0,
        group: 'default',
      }
    } else if (path === '/api/user/aff') {
      data = 'wallet-preview'
    } else if (path === '/api/user/topup/info') {
      data = {
        enable_online_topup: false,
        enable_stripe_topup: false,
        enable_tokenpay_topup: true,
        tokenpay_packages: [{ usdt: 10, balance: '66' }],
        tokenpay_networks: ['USDT_TRC20'],
        pay_methods: [],
        min_topup: 1,
        stripe_min_topup: 1,
        amount_options: [],
        discount: {},
        enable_redemption: true,
      }
    } else if (path === '/api/subscription/plans') {
      data = []
    } else if (path === '/api/subscription/self') {
      data = {
        billing_preference: 'subscription_first',
        subscriptions: [],
        all_subscriptions: [],
      }
    }

    await route.fulfill({
      status: 200,
      contentType: 'application/json',
      body: JSON.stringify({ success: true, data }),
    })
  })
})

test('USDT recharge fills its row and opens the package dialog', async ({
  page,
}, testInfo) => {
  await page.addInitScript(
    (theme) => {
      document.cookie = `vite-ui-theme=${theme}; path=/`
    },
    testInfo.project.name === 'chromium-desktop' ? 'dark' : 'light'
  )
  await page.goto('/wallet')

  const entry = page.getByRole('button', { name: 'USDT 充值' })
  await expect(entry).toBeVisible()
  await expect(page.getByRole('button', { name: /TokenPay USDT/ })).toHaveCount(
    0
  )

  const button = await entry.boundingBox()
  const contentWidth = await page
    .locator('[data-slot="card-content"]')
    .filter({ has: entry })
    .evaluate((element) => {
      const style = getComputedStyle(element)
      return (
        element.clientWidth -
        Number.parseFloat(style.paddingLeft) -
        Number.parseFloat(style.paddingRight)
      )
    })
  if (!button) throw new Error('USDT recharge button has no layout box')
  expect(Math.abs(button.width - contentWidth)).toBeLessThanOrEqual(2)
  expect(button.height).toBeGreaterThanOrEqual(64)

  await page.screenshot({
    path: testInfo.outputPath('wallet-usdt-entry.png'),
    fullPage: true,
  })

  await entry.click()
  await expect(page.getByRole('dialog')).toBeVisible()
})
