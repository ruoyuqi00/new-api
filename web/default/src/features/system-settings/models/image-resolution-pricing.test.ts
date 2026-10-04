import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import type {
  ImageResolutionPricingMetadata,
  PricingModel,
} from '@/features/pricing/types'

import {
  buildImageResolutionPricingModels,
  getImageResolutionModelPolicy,
  parseImageResolutionPolicies,
  saveImageResolutionPolicy,
  validateImageResolutionDraft,
  type ImageResolutionPricePolicy,
} from './image-resolution-pricing'

const gptImageMetadata: ImageResolutionPricingMetadata = {
  pricing_model: 'gpt-image-2',
  default_tier: '1k',
  prices: {
    '1k': 0.01,
    '2k': 0.04,
    '4k': 0.045,
  },
}

const gptImageDraft: ImageResolutionPricePolicy = {
  default_tier: '1k',
  prices: {
    '1k': 0.02,
    '2k': 0.05,
    '4k': 0.08,
  },
}

describe('image resolution pricing state', () => {
  test('offers unconfigured image models without including text or vision-only models', () => {
    const baseModel: PricingModel = {
      id: 1,
      model_name: 'gpt-image-2.5-sunburst',
      quota_type: 1,
      model_ratio: 0,
      completion_ratio: 0,
      enable_groups: ['image'],
      supported_endpoint_types: ['image-generation'],
    }
    const result = buildImageResolutionPricingModels(
      [
        baseModel,
        {
          ...baseModel,
          id: 2,
          model_name: 'vision-chat-model',
          supported_endpoint_types: ['openai'],
          input_modalities: ['image', 'text'],
          output_modalities: ['text'],
        },
      ],
      {}
    )

    assert.deepEqual(
      result.map((model) => model.model_name),
      ['gpt-image-2.5-sunburst']
    )
    assert.equal(result[0].image_resolution_pricing, undefined)
  })

  test('creates a custom model policy without backend pricing metadata', () => {
    const result = saveImageResolutionPolicy(
      { 'gpt-image-2': gptImageDraft },
      ' Provider/CUSTOM-image ',
      {
        default_tier: '2k',
        prices: { '4k': 0.03, '1k': 0.01, '2k': 0.02 },
      }
    )

    assert.deepEqual(result, {
      'gpt-image-2': gptImageDraft,
      'custom-image': {
        default_tier: '2k',
        prices: { '1k': 0.01, '2k': 0.02, '4k': 0.03 },
      },
    })
  })

  test('rejects an invalid default tier before saving', () => {
    assert.deepEqual(
      validateImageResolutionDraft(
        { ...gptImageDraft, default_tier: '8k' },
        gptImageMetadata
      ),
      { default_tier: 'Invalid default tier' }
    )
  })

  test('keeps configured image pricing models editable while channels are disabled', () => {
    const result = buildImageResolutionPricingModels([], {
      'gpt-image-2': gptImageDraft,
    })

    assert.equal(result.length, 1)
    assert.equal(result[0].model_name, 'gpt-image-2')
    assert.deepEqual(result[0].image_resolution_pricing, {
      pricing_model: 'gpt-image-2',
      default_tier: '1k',
      prices: gptImageDraft.prices,
    })
  })

  test('parses settings JSON and finds canonical model names case-insensitively', () => {
    const policies = parseImageResolutionPolicies(
      JSON.stringify({ 'gpt-image-2': gptImageDraft })
    )

    assert.deepEqual(
      getImageResolutionModelPolicy(policies, 'GPT-IMAGE-2'),
      gptImageDraft
    )
  })

  test('saving one model preserves every unrelated image pricing policy', () => {
    const nanoBananaPolicy: ImageResolutionPricePolicy = {
      default_tier: '1k',
      prices: { '1k': 0.08, '2k': 0.1, '4k': 0.16 },
    }

    const result = saveImageResolutionPolicy(
      { 'nano-banana-pro': nanoBananaPolicy },
      'gpt-image-2',
      gptImageDraft,
      gptImageMetadata
    )

    assert.deepEqual(result['nano-banana-pro'], nanoBananaPolicy)
    assert.deepEqual(result['gpt-image-2'], gptImageDraft)
  })

  test('requires every positive finite tier and non-decreasing prices', () => {
    const missing = structuredClone(gptImageDraft)
    delete missing.prices['4k']
    assert.deepEqual(validateImageResolutionDraft(missing, gptImageMetadata), {
      'prices.4k': 'Required',
    })

    for (const invalid of [0, -1, Number.NaN, Number.POSITIVE_INFINITY]) {
      const draft = structuredClone(gptImageDraft)
      draft.prices['2k'] = invalid
      assert.deepEqual(validateImageResolutionDraft(draft, gptImageMetadata), {
        'prices.2k': 'Must be greater than zero',
      })
    }

    const decreasing = structuredClone(gptImageDraft)
    decreasing.prices['4k'] = 0.03
    assert.deepEqual(
      validateImageResolutionDraft(decreasing, gptImageMetadata),
      { 'prices.4k': 'Higher tiers cannot cost less than lower tiers' }
    )
  })
})
