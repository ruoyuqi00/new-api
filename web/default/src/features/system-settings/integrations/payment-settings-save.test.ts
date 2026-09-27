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
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { runPaymentSettingsSaves } from './payment-settings-save'

describe('payment settings page save', () => {
  test('includes each legacy and SHKeeper save exactly once', async () => {
    const calls: string[] = []

    await runPaymentSettingsSaves(
      async () => {
        calls.push('legacy')
      },
      async () => {
        calls.push('shkeeper')
      }
    )

    assert.deepEqual(calls, ['legacy', 'shkeeper'])
  })

  test('rejects the page save when SHKeeper persistence fails', async () => {
    const calls: string[] = []

    await assert.rejects(
      runPaymentSettingsSaves(
        async () => {
          calls.push('legacy')
        },
        async () => {
          calls.push('shkeeper')
          throw new Error('SHKeeper save failed')
        }
      ),
      /SHKeeper save failed/
    )
    assert.deepEqual(calls, ['legacy', 'shkeeper'])
  })

  test('saves TokenPay after the existing payment settings', async () => {
    const calls: string[] = []
    await runPaymentSettingsSaves(
      async () => {
        calls.push('legacy')
      },
      async () => {
        calls.push('shkeeper')
      },
      async () => {
        calls.push('tokenpay')
      }
    )
    assert.deepEqual(calls, ['legacy', 'shkeeper', 'tokenpay'])
  })
})
