import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import type { PricingModel } from '../types'
import {
  getDynamicPricingSummary,
  isDynamicPricingModel,
} from './dynamic-price'

function domesticCallModel(): PricingModel {
  return {
    id: 1,
    model_name: 'MiniMax-M2.7-call',
    quota_type: 1,
    model_ratio: 0,
    completion_ratio: 0,
    enable_groups: ['国模按次'],
    group_ratio: { 国模按次: 0.3 },
    billing_mode: 'per_call_expr',
    billing_expr: 'len <= 128000 ? tier("short", 0.05) : tier("long", 0.1)',
  }
}

describe('per-call expression pricing', () => {
  test('shows fixed request prices with the group ratio applied once', () => {
    const model = domesticCallModel()

    assert.equal(isDynamicPricingModel(model), true)
    const summary = getDynamicPricingSummary(model, {
      tokenUnit: 'M',
      groupRatioMultiplier: 0.3,
    })

    assert.equal(summary?.isPerCall, true)
    assert.deepEqual(
      summary?.fixedPriceEntries.map((entry) => entry.value),
      [0.015, 0.03]
    )
    assert.match(summary?.fixedPriceEntries[0]?.formatted || '', /0\.015/)
  })
})
