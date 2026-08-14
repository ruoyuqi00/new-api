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

import { buildApiParams, getDefaultTimeRange } from './utils'

describe('usage log time range', () => {
  test('defaults to a rolling one-hour window with clock-skew tolerance', () => {
    const now = new Date('2026-08-14T08:30:00.000Z')

    const range = getDefaultTimeRange(now)

    assert.equal(range.start.toISOString(), '2026-08-14T07:30:00.000Z')
    assert.equal(range.end.toISOString(), '2026-08-14T09:30:00.000Z')
  })

  test('preserves an explicitly selected time range', () => {
    const startTime = Date.parse('2026-08-13T00:00:00.000Z')
    const endTime = Date.parse('2026-08-14T00:00:00.000Z')

    const params = buildApiParams({
      page: 2,
      pageSize: 30,
      searchParams: { startTime, endTime },
      isAdmin: true,
    })

    assert.equal(params.start_timestamp, startTime / 1000)
    assert.equal(params.end_timestamp, endTime / 1000)
  })
})
