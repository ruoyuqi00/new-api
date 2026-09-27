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
import { test } from 'node:test'

import {
  buildTokenPaySettingsRequest,
  createTokenPaySettingsSchema,
} from './tokenpay-settings-model'

test('TokenPay settings reject duplicate package amounts and unsupported networks', () => {
  const schema = createTokenPaySettingsSchema(true)
  const base = {
    enabled: true,
    base_url: 'https://pay.example.com',
    api_token: '',
    allow_private_url: false,
    packages: [{ usdt: 10, balance: '66', label: '' }],
    enabled_networks: ['USDT_TRC20'],
  }
  assert.equal(schema.safeParse(base).success, true)
  assert.equal(
    schema.safeParse({
      ...base,
      packages: [...base.packages, ...base.packages],
    }).success,
    false
  )
  assert.equal(
    schema.safeParse({ ...base, enabled_networks: ['EVM_ETH_USDT_ERC20'] })
      .success,
    false
  )
})

test('TokenPay settings require a secret when enabled and preserve blank write-only input', () => {
  const values = {
    enabled: true,
    base_url: 'https://pay.example.com',
    api_token: '',
    allow_private_url: false,
    packages: [{ usdt: 10, balance: '66', label: '' }],
    enabled_networks: ['USDT_TRC20' as const],
  }
  assert.equal(
    createTokenPaySettingsSchema(false).safeParse(values).success,
    false
  )
  assert.equal(
    createTokenPaySettingsSchema(true).safeParse(values).success,
    true
  )
  assert.equal(buildTokenPaySettingsRequest(values).api_token, '')
})
