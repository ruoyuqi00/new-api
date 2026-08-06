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
  buildAffiliateRebateConfigPayload,
  formatAffiliateRebatePercent,
  hasAffiliateRebateConfigChanges,
  isAffiliateRebatePercentEditable,
  shouldDisplayAffiliateRebateRate,
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
    expect(() => affiliatePercentToBasisPoints(0.000000000001)).toThrow(
      RangeError
    )
  })

  test('rejects percentage values outside the supported range', () => {
    expect(() => affiliatePercentToBasisPoints(-0.01)).toThrow(RangeError)
    expect(() => affiliatePercentToBasisPoints(100.01)).toThrow(RangeError)
  })
})

describe('affiliate rebate settings behavior', () => {
  test('builds one atomic configuration payload from the submitted form', () => {
    expect(
      buildAffiliateRebateConfigPayload({
        AffiliateCreditRebateEnabled: true,
        AffiliateCreditRebatePercent: 5.25,
      })
    ).toEqual({ enabled: true, basis_points: 525 })
  })

  test('detects changes to either field in the atomic configuration', () => {
    expect(
      hasAffiliateRebateConfigChanges({
        AffiliateCreditRebateEnabled: true,
      })
    ).toBe(true)
    expect(
      hasAffiliateRebateConfigChanges({
        AffiliateCreditRebatePercent: 5.25,
      })
    ).toBe(true)
    expect(hasAffiliateRebateConfigChanges({ QuotaForNewUser: 1000 })).toBe(
      false
    )
  })

  test('enables percentage editing and wallet rate copy only in valid states', () => {
    expect(isAffiliateRebatePercentEditable(true, true)).toBe(true)
    expect(isAffiliateRebatePercentEditable(false, true)).toBe(false)
    expect(isAffiliateRebatePercentEditable(true, false)).toBe(false)
    expect(shouldDisplayAffiliateRebateRate(true, 525)).toBe(true)
    expect(shouldDisplayAffiliateRebateRate(true, 0)).toBe(false)
    expect(shouldDisplayAffiliateRebateRate(false, 525)).toBe(false)
  })
})
