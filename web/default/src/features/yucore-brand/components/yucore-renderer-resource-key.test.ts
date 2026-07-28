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

import {
  getEarthResourceKey,
  getSignalFieldResourceKey,
} from './yucore-renderer-resource-key'

describe('YuCore renderer resource keys', () => {
  test('activation does not change signal field resource identity', () => {
    const base = {
      active: false,
      colorMode: 'dark' as const,
      coreMode: 'ambient' as const,
      corePlacement: 'hero' as const,
      intensity: 'hero' as const,
      renderProfile: 'default' as const,
    }

    assert.equal(
      getSignalFieldResourceKey(base),
      getSignalFieldResourceKey({ ...base, active: true })
    )
    assert.notEqual(
      getSignalFieldResourceKey(base),
      getSignalFieldResourceKey({ ...base, colorMode: 'light' })
    )
  })

  test('activation does not change Earth resource identity', () => {
    const base = {
      active: false,
      colorMode: 'dark' as const,
      density: 'loader' as const,
      timeOffsetSeconds: 0,
    }

    assert.equal(
      getEarthResourceKey(base),
      getEarthResourceKey({ ...base, active: true })
    )
    assert.notEqual(
      getEarthResourceKey(base),
      getEarthResourceKey({ ...base, density: 'persistent' })
    )
  })
})
