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
export function affiliateBasisPointsToPercent(basisPoints: number): number {
  return basisPoints / 100
}

export function hasSupportedAffiliateRebatePrecision(percent: number): boolean {
  const basisPoints = percent * 100
  const tolerance = Number.EPSILON * Math.max(1, Math.abs(basisPoints)) * 4
  return Math.abs(basisPoints - Math.round(basisPoints)) <= tolerance
}

export function affiliatePercentToBasisPoints(percent: number): number {
  const basisPoints = percent * 100
  const roundedBasisPoints = Math.round(basisPoints)
  if (
    !Number.isFinite(percent) ||
    percent < 0 ||
    percent > 100 ||
    !hasSupportedAffiliateRebatePrecision(percent)
  ) {
    throw new RangeError('unsupported affiliate rebate percentage')
  }
  return roundedBasisPoints
}

export function buildAffiliateRebateOptionUpdates(
  changedFields: Record<string, unknown>
): ReadonlyArray<readonly [string, unknown]> {
  const entries = Object.entries(changedFields).map(([key, value]) =>
    key === 'AffiliateCreditRebatePercent'
      ? ([
          'AffiliateCreditRebateBasisPoints',
          affiliatePercentToBasisPoints(value as number),
        ] as const)
      : ([key, value] as const)
  )
  const enabledEntry = entries.find(
    ([key]) => key === 'AffiliateCreditRebateEnabled'
  )
  const otherEntries = entries.filter(
    ([key]) => key !== 'AffiliateCreditRebateEnabled'
  )
  if (enabledEntry?.[1] === false) return [enabledEntry, ...otherEntries]
  if (enabledEntry) return [...otherEntries, enabledEntry]
  return otherEntries
}

export function isAffiliateRebatePercentEditable(
  enabled: boolean,
  complianceConfirmed: boolean
): boolean {
  return enabled && complianceConfirmed
}

export function shouldDisplayAffiliateRebateRate(
  enabled: boolean,
  basisPoints: number
): boolean {
  return enabled && basisPoints > 0
}

export function formatAffiliateRebatePercent(basisPoints: number): string {
  return `${affiliateBasisPointsToPercent(basisPoints)}%`
}
