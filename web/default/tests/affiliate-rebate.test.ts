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
import { describe, expect, test } from 'bun:test'

import {
  affiliateBasisPointsToPercent,
  affiliatePercentToBasisPoints,
  formatAffiliateRebatePercent,
} from '../src/lib/affiliate-rebate'

describe('affiliate rebate percentage conversion', () => {
  test.each([
    [1, 0.01, '0.01%'],
    [525, 5.25, '5.25%'],
    [10000, 100, '100%'],
  ])('converts %i basis points exactly', (basisPoints, percent, label) => {
    expect(affiliateBasisPointsToPercent(basisPoints)).toBe(percent)
    expect(affiliatePercentToBasisPoints(percent)).toBe(basisPoints)
    expect(formatAffiliateRebatePercent(basisPoints)).toBe(label)
  })

  test('rejects percentage values with more than two decimal places', () => {
    expect(() => affiliatePercentToBasisPoints(5.255)).toThrow(RangeError)
  })
})
