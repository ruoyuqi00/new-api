import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import type { ImageResolutionPricingMetadata } from '../types'
import {
  buildImageResolutionRows,
  formatImageResolutionPrice,
  getImageResolutionCardPrice,
  getImageResolutionStartingPrice,
} from './image-resolution-price'

const metadata: ImageResolutionPricingMetadata = {
  pricing_model: 'gpt-image-2',
  default_tier: '1k',
  prices: {
    '1k': 0.01,
    '2k': 0.04,
    '4k': 0.045,
  },
}

describe('image resolution marketplace prices', () => {
  test('builds group-adjusted rows in 1K, 2K, and 4K order', () => {
    assert.deepEqual(
      buildImageResolutionRows(metadata, { standard: 1, vip: 0.8 }),
      [
        { tier: '1k', prices: { standard: 0.01, vip: 0.008 } },
        { tier: '2k', prices: { standard: 0.04, vip: 0.032 } },
        { tier: '4k', prices: { standard: 0.045, vip: 0.036 } },
      ]
    )
  })

  test('uses the lowest enabled group price for card summaries', () => {
    assert.equal(getImageResolutionStartingPrice(metadata), 0.01)
    assert.equal(
      getImageResolutionCardPrice(metadata, ['standard', 'vip'], {
        standard: 1,
        vip: 0.8,
      }),
      0.008
    )
  })

  test('formats recharge prices with existing currency semantics', () => {
    assert.match(
      formatImageResolutionPrice(0.04, {
        showRechargePrice: true,
        priceRate: 0.5,
        usdExchangeRate: 1,
      }),
      /0\.02/
    )
  })
})
