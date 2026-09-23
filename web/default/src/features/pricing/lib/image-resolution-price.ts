import { formatCurrencyFromUSD } from '@/lib/currency'

import type { ImageResolutionPricingMetadata } from '../types'

export type ImageResolutionDisplayRow = {
  tier: string
  prices: Record<string, number>
}

export type ImageResolutionFormatOptions = {
  showRechargePrice?: boolean
  priceRate?: number
  usdExchangeRate?: number
}

export function buildImageResolutionRows(
  metadata: ImageResolutionPricingMetadata,
  groupRatios: Record<string, number>
): ImageResolutionDisplayRow[] {
  return Object.entries(metadata.prices).map(([tier, basePrice]) => ({
    tier,
    prices: Object.fromEntries(
      Object.entries(groupRatios).map(([group, ratio]) => [
        group,
        multiplyPrice(basePrice, ratio),
      ])
    ),
  }))
}

export function getImageResolutionStartingPrice(
  metadata: ImageResolutionPricingMetadata
): number {
  return Math.min(...Object.values(metadata.prices))
}

export function getImageResolutionCardPrice(
  metadata: ImageResolutionPricingMetadata,
  enabledGroups: string[],
  groupRatios: Record<string, number>
): number {
  const configuredRatios = enabledGroups
    .map((group) => groupRatios[group])
    .filter((ratio): ratio is number => Number.isFinite(ratio) && ratio > 0)
  const minimumRatio =
    configuredRatios.length > 0 ? Math.min(...configuredRatios) : 1
  return multiplyPrice(getImageResolutionStartingPrice(metadata), minimumRatio)
}

export function formatImageResolutionPrice(
  priceInUSD: number,
  options: ImageResolutionFormatOptions = {}
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

function multiplyPrice(price: number, ratio: number): number {
  return Math.round((price * ratio + Number.EPSILON) * 1e12) / 1e12
}
