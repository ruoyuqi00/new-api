import type {
  ImageResolutionPricingMetadata,
  PricingModel,
} from '@/features/pricing/types'

import { safeJsonParse } from '../utils/json-parser'

export type ImageResolutionPricePolicy = {
  default_tier: string
  prices: Record<string, number>
}

export type ImageResolutionPricePolicies = Record<
  string,
  ImageResolutionPricePolicy
>

export type ImageResolutionPricingModel = PricingModel & {
  image_resolution_pricing: ImageResolutionPricingMetadata
}

export function buildImageResolutionPricingModels(
  models: PricingModel[],
  policies: ImageResolutionPricePolicies
): ImageResolutionPricingModel[] {
  const byCanonicalName = new Map<string, ImageResolutionPricingModel>()

  for (const model of models) {
    const metadata = model.image_resolution_pricing
    if (!metadata) continue
    const canonicalName = metadata.pricing_model.trim().toLowerCase()
    byCanonicalName.set(canonicalName, {
      ...model,
      model_name: metadata.pricing_model,
      image_resolution_pricing: metadata,
    })
  }

  for (const [modelName, policy] of Object.entries(policies)) {
    const canonicalName = modelName.trim().toLowerCase()
    const existing = byCanonicalName.get(canonicalName)
    byCanonicalName.set(canonicalName, {
      id: existing?.id ?? 0,
      model_name: existing?.model_name ?? modelName,
      quota_type: existing?.quota_type ?? 1,
      model_ratio: existing?.model_ratio ?? 0,
      completion_ratio: existing?.completion_ratio ?? 0,
      enable_groups: existing?.enable_groups ?? [],
      ...existing,
      image_resolution_pricing: {
        pricing_model: modelName,
        default_tier:
          policy.default_tier as ImageResolutionPricingMetadata['default_tier'],
        prices: policy.prices as ImageResolutionPricingMetadata['prices'],
      },
    })
  }

  return [...byCanonicalName.values()].sort((left, right) =>
    left.model_name.localeCompare(right.model_name)
  )
}

export function parseImageResolutionPolicies(
  value: string
): ImageResolutionPricePolicies {
  const parsed = safeJsonParse<unknown>(value, {
    fallback: {},
    silent: true,
  })
  return typeof parsed === 'object' && parsed !== null && !Array.isArray(parsed)
    ? (parsed as ImageResolutionPricePolicies)
    : {}
}

export function getImageResolutionModelPolicy(
  policies: ImageResolutionPricePolicies,
  model: string
): ImageResolutionPricePolicy | undefined {
  const normalized = model.trim().toLowerCase()
  const key = Object.keys(policies).find(
    (candidate) => candidate.trim().toLowerCase() === normalized
  )
  return key ? policies[key] : undefined
}

function isPositiveFinite(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value) && value > 0
}

export function validateImageResolutionDraft(
  draft: ImageResolutionPricePolicy,
  metadata: ImageResolutionPricingMetadata
): Record<string, string> {
  const errors: Record<string, string> = {}
  const tiers = Object.keys(metadata.prices)
  let previousPrice: number | undefined

  for (const tier of tiers) {
    const price = draft.prices[tier]
    if (price === undefined) {
      errors[`prices.${tier}`] = 'Required'
      continue
    }
    if (!isPositiveFinite(price)) {
      errors[`prices.${tier}`] = 'Must be greater than zero'
      continue
    }
    if (previousPrice !== undefined && price < previousPrice) {
      errors[`prices.${tier}`] =
        'Higher tiers cannot cost less than lower tiers'
    }
    previousPrice = price
  }

  return errors
}

export function saveImageResolutionPolicy(
  current: ImageResolutionPricePolicies,
  model: string,
  draft: ImageResolutionPricePolicy,
  metadata: ImageResolutionPricingMetadata
): ImageResolutionPricePolicies {
  const errors = validateImageResolutionDraft(draft, metadata)
  if (Object.keys(errors).length > 0) {
    throw new Error('Invalid image resolution prices')
  }

  const next = structuredClone(current)
  const normalized = model.trim().toLowerCase()
  const existingKey = Object.keys(next).find(
    (candidate) => candidate.trim().toLowerCase() === normalized
  )
  if (existingKey && existingKey !== metadata.pricing_model) {
    delete next[existingKey]
  }
  next[metadata.pricing_model] = {
    default_tier: metadata.default_tier,
    prices: Object.fromEntries(
      Object.keys(metadata.prices).map((tier) => [tier, draft.prices[tier]])
    ),
  }
  return next
}
