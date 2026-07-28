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
import { describe, expect, it } from 'bun:test'

import { parseLogOther } from '../src/features/usage-logs/lib/format'
import {
  KEEP_CURRENT_PAGE_ON_QUERY_ERROR,
  NAVIGATE_TO_SERVER_ERROR_ON_QUERY_ERROR,
  shouldNavigateToServerError,
} from '../src/lib/query-error-policy'

describe('usage log error resilience', () => {
  it('keeps the usage log page mounted when its query returns HTTP 500', () => {
    expect(
      shouldNavigateToServerError(500, KEEP_CURRENT_PAGE_ON_QUERY_ERROR)
    ).toBeFalse()
    expect(shouldNavigateToServerError(500, undefined)).toBeFalse()
    expect(
      shouldNavigateToServerError(500, NAVIGATE_TO_SERVER_ERROR_ON_QUERY_ERROR)
    ).toBeTrue()
  })

  it('accepts object-shaped log metadata', () => {
    expect(parseLogOther('{"group":"vip","model_ratio":1}')).toEqual({
      group: 'vip',
      model_ratio: 1,
    })
  })

  it('rejects malformed and non-object historical metadata', () => {
    expect(parseLogOther('not-json')).toBeNull()
    expect(parseLogOther('null')).toBeNull()
    expect(parseLogOther('["legacy"]')).toBeNull()
    expect(parseLogOther('"legacy"')).toBeNull()
  })
})
