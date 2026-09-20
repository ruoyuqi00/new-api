import { formatCurrencyFromUSD } from '@/lib/currency'

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
import type { VideoBillingUnit, VideoTierPricingMetadata } from '../types'

export type VideoTierDisplayRow = {
  tier: string
  standard: Record<string, number>
  withReferenceVideo?: Record<string, number>
}

export type VideoTierFormatOptions = {
  showRechargePrice?: boolean
  priceRate?: number
  usdExchangeRate?: number
}

function multiplyPrice(price: number, ratio: number): number {
  return Math.round((price * ratio + Number.EPSILON) * 1e12) / 1e12
}

export function getVideoTierStartingPrice(
  metadata: VideoTierPricingMetadata
): number {
  return Math.min(
    ...metadata.tiers.map((tier) => metadata.prices[tier]?.standard ?? Infinity)
  )
}

export function getVideoTierCardPrice(
  metadata: VideoTierPricingMetadata,
  enabledGroups: string[],
  groupRatios: Record<string, number>
): number {
  const configuredRatios = enabledGroups
    .map((group) => groupRatios[group])
    .filter((ratio): ratio is number => Number.isFinite(ratio) && ratio > 0)
  const minimumRatio =
    configuredRatios.length > 0 ? Math.min(...configuredRatios) : 1

  return multiplyPrice(getVideoTierStartingPrice(metadata), minimumRatio)
}

export function buildVideoTierRows(
  metadata: VideoTierPricingMetadata,
  groupRatios: Record<string, number>
): VideoTierDisplayRow[] {
  return metadata.tiers.map((tier) => {
    const price = metadata.prices[tier]
    const standard = Object.fromEntries(
      Object.entries(groupRatios).map(([group, ratio]) => [
        group,
        multiplyPrice(price.standard, ratio),
      ])
    )
    const withReferenceVideo =
      price.with_reference_video === undefined
        ? undefined
        : Object.fromEntries(
            Object.entries(groupRatios).map(([group, ratio]) => [
              group,
              multiplyPrice(price.with_reference_video as number, ratio),
            ])
          )

    return {
      tier,
      standard,
      ...(withReferenceVideo ? { withReferenceVideo } : {}),
    }
  })
}

export function getVideoBillingUnitLabelKey(unit: VideoBillingUnit): string {
  switch (unit) {
    case 'per_second':
      return 'per second'
    case 'per_successful_task':
      return 'per successful task'
    case 'per_1m_video_tokens':
      return 'per 1M video tokens'
  }
}

export function formatVideoTierPrice(
  priceInUSD: number,
  options: VideoTierFormatOptions = {}
): string {
  const showRechargePrice = options.showRechargePrice ?? false
  const priceRate = options.priceRate ?? 1
  const usdExchangeRate = options.usdExchangeRate ?? 1
  const displayPrice = showRechargePrice
    ? (priceInUSD * priceRate) / usdExchangeRate
    : priceInUSD

  return formatCurrencyFromUSD(displayPrice, {
    digitsLarge: 4,
    digitsSmall: 6,
    abbreviate: false,
  })
}
