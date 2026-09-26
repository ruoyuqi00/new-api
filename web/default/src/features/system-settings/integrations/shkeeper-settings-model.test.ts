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

import type { SHKeeperSettingsStatus } from '../types'
import {
  buildSHKeeperFormDefaults,
  buildSHKeeperSettingsRequest,
  createSHKeeperSettingsSchema,
  getSHKeeperArrayErrorMessage,
  type SHKeeperSettingsFormValues,
} from './shkeeper-settings-model'

const translate = (message: string) => message

const status: SHKeeperSettingsStatus = {
  enabled: false,
  base_url: '',
  api_key_configured: false,
  backend_key_configured: false,
  packages: null,
  enabled_networks: null,
  invoice_expiry_minutes: 30,
  reconcile_interval_seconds: 60,
  allow_private_url: false,
}

function fixedSettings(
  packages: SHKeeperSettingsFormValues['packages'] = [
    { usdt: 10, balance: '66', label: '' },
  ]
): SHKeeperSettingsFormValues {
  return {
    ...buildSHKeeperFormDefaults(status),
    enabled: true,
    base_url: 'https://payments.example.com',
    api_key: 'api-key',
    backend_key: 'backend-key',
    enabled_networks: ['USDT'],
    packages,
  }
}

describe('SHKeeper settings form model', () => {
  test('normalizes first-install omitted and null arrays', () => {
    const defaults = buildSHKeeperFormDefaults(status)

    assert.deepEqual(defaults.packages, [])
    assert.deepEqual(defaults.enabled_networks, [])
  })

  test('deep-copies status arrays and normalizes omitted labels', () => {
    const source: SHKeeperSettingsStatus = {
      ...status,
      packages: [{ usdt: 10, balance: '66' }],
      enabled_networks: ['USDT'],
    }

    const defaults = buildSHKeeperFormDefaults(source)
    defaults.packages[0].balance = '99'
    defaults.enabled_networks.push('BNB-USDT')

    assert.deepEqual(source.packages, [{ usdt: 10, balance: '66' }])
    assert.deepEqual(source.enabled_networks, ['USDT'])
    assert.equal(defaults.packages[0].label, '')
    assert.equal(defaults.api_key, '')
    assert.equal(defaults.backend_key, '')
  })

  test('requires a network and package only when SHKeeper is enabled', () => {
    const schema = createSHKeeperSettingsSchema(translate)
    const disabled = schema.safeParse(buildSHKeeperFormDefaults(status))
    const enabled = schema.safeParse({
      ...buildSHKeeperFormDefaults(status),
      enabled: true,
      base_url: 'https://payments.example.com',
      api_key: 'api-key',
    })

    assert.equal(disabled.success, true)
    assert.equal(enabled.success, false)
    if (enabled.success) return
    assert.equal(
      enabled.error.issues.some(
        (issue) => issue.path.join('.') === 'enabled_networks'
      ),
      true
    )
    assert.equal(
      enabled.error.issues.some((issue) => issue.path.join('.') === 'packages'),
      true
    )
  })

  test('accepts configured write-only secrets when enabled', () => {
    const schema = createSHKeeperSettingsSchema(translate)
    const result = schema.safeParse({
      ...fixedSettings(),
      api_key: '',
      backend_key: '',
      api_key_configured: true,
      backend_key_configured: true,
    })

    assert.equal(result.success, true)
  })

  test('rejects duplicate fixed package USDT amounts', () => {
    const result = createSHKeeperSettingsSchema(translate).safeParse(
      fixedSettings([
        { usdt: 10, balance: '66', label: '' },
        { usdt: 10, balance: '70', label: '' },
      ])
    )

    assert.equal(result.success, false)
  })

  test('requires positive whole USDT amounts and positive balances', () => {
    const schema = createSHKeeperSettingsSchema(translate)
    const cases = [
      { usdt: 0, balance: '66', label: '' },
      { usdt: 1.5, balance: '66', label: '' },
      { usdt: 10, balance: '0', label: '' },
      { usdt: 10, balance: '-1', label: '' },
    ]

    for (const item of cases) {
      assert.equal(schema.safeParse(fixedSettings([item])).success, false)
    }
  })

  test('trims strings and normalizes an omitted package label', () => {
    const result = createSHKeeperSettingsSchema(translate).safeParse({
      ...fixedSettings(),
      base_url: ' https://payments.example.com/ ',
      packages: [{ usdt: 10, balance: ' 66.5 ' }],
    })

    assert.equal(result.success, true)
    if (!result.success) return
    assert.equal(result.data.base_url, 'https://payments.example.com')
    assert.deepEqual(result.data.packages, [
      { usdt: 10, balance: '66.5', label: '' },
    ])
  })

  test('selects direct and root array errors for visible field feedback', () => {
    assert.equal(
      getSHKeeperArrayErrorMessage({ message: 'Add a package' }),
      'Add a package'
    )
    assert.equal(
      getSHKeeperArrayErrorMessage({ root: { message: 'Duplicate amount' } }),
      'Duplicate amount'
    )
    assert.equal(getSHKeeperArrayErrorMessage(undefined), undefined)
  })

  test('builds a dedicated API payload without status-only secret flags', () => {
    const request = buildSHKeeperSettingsRequest({
      ...fixedSettings(),
      api_key: '',
      backend_key: '',
      api_key_configured: true,
      backend_key_configured: true,
    })

    assert.equal('api_key_configured' in request, false)
    assert.equal('backend_key_configured' in request, false)
    assert.equal(request.api_key, '')
    assert.equal(request.backend_key, '')
  })
})
