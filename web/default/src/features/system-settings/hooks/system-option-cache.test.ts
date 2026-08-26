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

import type { SystemOptionsResponse } from '../types'
import { applySystemOptionUpdates } from './system-option-cache'

describe('system option cache hydration', () => {
  test('keeps saved group settings visible while the server snapshot refetches', () => {
    const current: SystemOptionsResponse = {
      success: true,
      message: '',
      data: [
        { key: 'group_ratio_setting.user_group_ratio', value: '{}' },
        { key: 'group_ratio_setting.availability_monitoring', value: '{}' },
      ],
    }

    const next = applySystemOptionUpdates(current, [
      {
        key: 'group_ratio_setting.user_group_ratio',
        value: '{"79":{"gptpro":0.3}}',
      },
      {
        key: 'group_ratio_setting.availability_monitoring',
        value: '{"gptpro":true}',
      },
    ])

    assert.ok(next)
    assert.deepEqual(next.data, [
      {
        key: 'group_ratio_setting.user_group_ratio',
        value: '{"79":{"gptpro":0.3}}',
      },
      {
        key: 'group_ratio_setting.availability_monitoring',
        value: '{"gptpro":true}',
      },
    ])
  })

  test('adds a newly-created option without mutating the cached response', () => {
    const current: SystemOptionsResponse = {
      success: true,
      message: '',
      data: [{ key: 'GroupRatio', value: '{}' }],
    }

    const next = applySystemOptionUpdates(current, [
      { key: 'group_ratio_setting.availability_monitoring', value: '{}' },
    ])

    assert.deepEqual(current.data, [{ key: 'GroupRatio', value: '{}' }])
    assert.ok(next)
    assert.deepEqual(next.data, [
      { key: 'GroupRatio', value: '{}' },
      { key: 'group_ratio_setting.availability_monitoring', value: '{}' },
    ])
  })
})
