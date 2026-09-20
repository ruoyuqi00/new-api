import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import type { VideoTierPricingMetadata } from '../types'
import {
  buildVideoTierRows,
  formatVideoTierPrice,
  getVideoBillingUnitLabelKey,
  getVideoTierCardPrice,
  getVideoTierStartingPrice,
} from './video-tier-price'

const seedanceMetadata: VideoTierPricingMetadata = {
  billing_unit: 'per_1m_video_tokens',
  inherited: false,
  tiers: ['480p', '720p', '1080p'],
  prices: {
    '480p': { standard: 18.4, with_reference_video: 11.2 },
    '720p': { standard: 36.8, with_reference_video: 22.4 },
    '1080p': { standard: 73.6, with_reference_video: 44.8 },
  },
}

describe('video tier marketplace prices', () => {
  test('builds group-adjusted rows for standard and reference profiles', () => {
    const rows = buildVideoTierRows(seedanceMetadata, {
      default: 1,
      vip: 0.8,
    })

    assert.deepEqual(
      rows.find((row) => row.tier === '720p'),
      {
        tier: '720p',
        standard: { default: 36.8, vip: 29.44 },
        withReferenceVideo: { default: 22.4, vip: 17.92 },
      }
    )
  })

  test('uses metadata tier order and omits reference prices when unavailable', () => {
    const metadata: VideoTierPricingMetadata = {
      billing_unit: 'per_second',
      inherited: true,
      tiers: ['1080p', '480p'],
      prices: {
        '480p': { standard: 0.1 },
        '1080p': { standard: 0.47 },
      },
    }

    assert.deepEqual(buildVideoTierRows(metadata, { default: 1 }), [
      { tier: '1080p', standard: { default: 0.47 } },
      { tier: '480p', standard: { default: 0.1 } },
    ])
  })

  test('uses the lowest standard tier for card summaries', () => {
    assert.equal(getVideoTierStartingPrice(seedanceMetadata), 18.4)
  })

  test('applies the lowest enabled group ratio to the card summary once', () => {
    assert.equal(
      getVideoTierCardPrice(seedanceMetadata, ['standard', 'vip', 'missing'], {
        standard: 1,
        vip: 0.8,
      }),
      14.72
    )
  })

  test('maps every backend billing unit to its localized unit label', () => {
    assert.deepEqual(
      [
        getVideoBillingUnitLabelKey('per_second'),
        getVideoBillingUnitLabelKey('per_successful_task'),
        getVideoBillingUnitLabelKey('per_1m_video_tokens'),
      ],
      ['per second', 'per successful task', 'per 1M video tokens']
    )
  })

  test('formats recharge prices with the same rate semantics as other prices', () => {
    assert.match(
      formatVideoTierPrice(36.8, {
        showRechargePrice: true,
        priceRate: 0.5,
        usdExchangeRate: 1,
      }),
      /18\.4/
    )
  })
})
