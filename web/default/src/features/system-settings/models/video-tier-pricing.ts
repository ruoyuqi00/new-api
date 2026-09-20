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
import type {
  VideoTierPricePoint,
  VideoTierPricingMetadata,
} from '@/features/pricing/types'

import { safeJsonParse } from '../utils/json-parser'

export type VideoTierModelPrices = Record<string, VideoTierPricePoint>
export type VideoTierOverrides = Record<string, VideoTierModelPrices>

function findModelKey(
  overrides: VideoTierOverrides,
  model: string
): string | undefined {
  const normalized = model.trim().toLowerCase()
  return Object.keys(overrides).find(
    (candidate) => candidate.trim().toLowerCase() === normalized
  )
}

function isVideoTierOverrides(value: unknown): value is VideoTierOverrides {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

export function parseVideoTierOverrides(value: string): VideoTierOverrides {
  const parsed = safeJsonParse<unknown>(value, {
    fallback: {},
    silent: true,
  })
  return isVideoTierOverrides(parsed) ? (parsed as VideoTierOverrides) : {}
}

export function getVideoTierModelOverride(
  overrides: VideoTierOverrides,
  model: string
): VideoTierModelPrices | undefined {
  const key = findModelKey(overrides, model)
  return key ? overrides[key] : undefined
}

function isPositiveFinite(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value) && value > 0
}

export function validateVideoTierDraft(
  draft: VideoTierModelPrices,
  metadata: VideoTierPricingMetadata
): Record<string, string> {
  const errors: Record<string, string> = {}

  for (const tier of metadata.tiers) {
    const point = draft[tier]
    if (!point) {
      errors[`${tier}.standard`] = 'Required'
      continue
    }
    if (!isPositiveFinite(point.standard)) {
      errors[`${tier}.standard`] = 'Must be greater than zero'
    }
    if (metadata.prices[tier]?.with_reference_video !== undefined) {
      if (point.with_reference_video === undefined) {
        errors[`${tier}.with_reference_video`] = 'Required'
      } else if (!isPositiveFinite(point.with_reference_video)) {
        errors[`${tier}.with_reference_video`] = 'Must be greater than zero'
      }
    }
  }

  return errors
}

export function saveVideoTierOverride(
  current: VideoTierOverrides,
  model: string,
  draft: VideoTierModelPrices,
  metadata: VideoTierPricingMetadata
): VideoTierOverrides {
  const errors = validateVideoTierDraft(draft, metadata)
  if (Object.keys(errors).length > 0) {
    throw new Error('Invalid video tier prices')
  }

  const next = removeVideoTierOverride(current, model)
  next[model] = Object.fromEntries(
    metadata.tiers.map((tier) => {
      const source = draft[tier]
      const point: VideoTierPricePoint = { standard: source.standard }
      if (metadata.prices[tier]?.with_reference_video !== undefined) {
        point.with_reference_video = source.with_reference_video
      }
      return [tier, point]
    })
  )
  return next
}

export function removeVideoTierOverride(
  current: VideoTierOverrides,
  model: string
): VideoTierOverrides {
  const next = structuredClone(current)
  const key = findModelKey(next, model)
  if (key) delete next[key]
  return next
}
