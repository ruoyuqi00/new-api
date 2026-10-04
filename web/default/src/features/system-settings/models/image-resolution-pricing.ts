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

export const IMAGE_RESOLUTION_TIERS = ['1k', '2k', '4k'] as const

export function normalizeImageResolutionModelName(model: string): string {
  return model.trim().toLowerCase().split('/').at(-1) ?? ''
}

export function buildImageResolutionPricingModels(
  models: PricingModel[],
  policies: ImageResolutionPricePolicies
): PricingModel[] {
  const byCanonicalName = new Map<string, PricingModel>()

  for (const model of models) {
    const metadata = model.image_resolution_pricing
    if (
      !metadata &&
      !model.supported_endpoint_types?.includes('image-generation') &&
      !model.output_modalities?.includes('image')
    ) {
      continue
    }
    const canonicalName = normalizeImageResolutionModelName(
      metadata?.pricing_model ?? model.model_name
    )
    byCanonicalName.set(canonicalName, {
      ...model,
      model_name: canonicalName,
    })
  }

  for (const [modelName, policy] of Object.entries(policies)) {
    const canonicalName = normalizeImageResolutionModelName(modelName)
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
  const normalized = normalizeImageResolutionModelName(model)
  const key = Object.keys(policies).find(
    (candidate) => normalizeImageResolutionModelName(candidate) === normalized
  )
  return key ? policies[key] : undefined
}

function isPositiveFinite(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value) && value > 0
}

export function validateImageResolutionDraft(
  draft: ImageResolutionPricePolicy,
  _metadata?: ImageResolutionPricingMetadata
): Record<string, string> {
  const errors: Record<string, string> = {}
  if (!IMAGE_RESOLUTION_TIERS.some((tier) => tier === draft.default_tier)) {
    errors.default_tier = 'Invalid default tier'
  }
  let previousPrice: number | undefined

  for (const tier of IMAGE_RESOLUTION_TIERS) {
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
  metadata?: ImageResolutionPricingMetadata
): ImageResolutionPricePolicies {
  const errors = validateImageResolutionDraft(draft, metadata)
  if (Object.keys(errors).length > 0) {
    throw new Error('Invalid image resolution prices')
  }

  const next = structuredClone(current)
  const normalized = normalizeImageResolutionModelName(
    metadata?.pricing_model ?? model
  )
  if (!normalized) throw new Error('Required')
  const existingKey = Object.keys(next).find(
    (candidate) => normalizeImageResolutionModelName(candidate) === normalized
  )
  if (existingKey && existingKey !== normalized) {
    delete next[existingKey]
  }
  next[normalized] = {
    default_tier: draft.default_tier,
    prices: Object.fromEntries(
      IMAGE_RESOLUTION_TIERS.map((tier) => [tier, draft.prices[tier]])
    ),
  }
  return next
}
