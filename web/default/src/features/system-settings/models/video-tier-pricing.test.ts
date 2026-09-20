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
import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import type { VideoTierPricingMetadata } from '@/features/pricing/types'

import {
  getVideoTierModelOverride,
  parseVideoTierOverrides,
  removeVideoTierOverride,
  saveVideoTierOverride,
  validateVideoTierDraft,
  type VideoTierModelPrices,
} from './video-tier-pricing'

const h3Metadata: VideoTierPricingMetadata = {
  billing_unit: 'per_second',
  inherited: true,
  tiers: ['480p', '768p', '1080p', '2k', '4k'],
  prices: {
    '480p': { standard: 0.1 },
    '768p': { standard: 0.16 },
    '1080p': { standard: 0.18 },
    '2k': { standard: 0.26 },
    '4k': { standard: 0.36 },
  },
}

const completeH3Draft: VideoTierModelPrices = {
  '480p': { standard: 0.11 },
  '768p': { standard: 0.17 },
  '1080p': { standard: 0.21 },
  '2k': { standard: 0.31 },
  '4k': { standard: 0.47 },
}

const completeWanOverride: VideoTierModelPrices = {
  '480p': { standard: 0.28 },
  '720p': { standard: 0.5 },
  '1080p': { standard: 0.9 },
}

describe('video tier pricing state', () => {
  test('parses settings JSON and preserves canonical model names', () => {
    const result = parseVideoTierOverrides(
      JSON.stringify({ 'seedance2.0-fast-PT': completeWanOverride })
    )
    assert.deepEqual(
      getVideoTierModelOverride(result, 'seedance2.0-fast-PT'),
      completeWanOverride
    )
    assert.deepEqual(
      getVideoTierModelOverride(result, 'SEEDANCE2.0-FAST-pt'),
      completeWanOverride
    )
  })

  test('saving one model writes every tier without changing other models', () => {
    const result = saveVideoTierOverride(
      { 'wan3.0-video': completeWanOverride },
      'minimax-h3',
      completeH3Draft,
      h3Metadata
    )

    assert.deepEqual(result['wan3.0-video'], completeWanOverride)
    assert.deepEqual(Object.keys(result['minimax-h3']), [
      '480p',
      '768p',
      '1080p',
      '2k',
      '4k',
    ])
  })

  test('reset removes only the selected model override', () => {
    const overrides = {
      'minimax-h3': completeH3Draft,
      'wan3.0-video': completeWanOverride,
    }
    assert.deepEqual(removeVideoTierOverride(overrides, 'MINIMAX-H3'), {
      'wan3.0-video': completeWanOverride,
    })
  })

  test('requires every supported positive finite standard price', () => {
    const missing = structuredClone(completeH3Draft)
    delete missing['4k']
    assert.deepEqual(validateVideoTierDraft(missing, h3Metadata), {
      '4k.standard': 'Required',
    })

    for (const invalid of [0, -1, Number.NaN, Number.POSITIVE_INFINITY]) {
      const draft = structuredClone(completeH3Draft)
      draft['1080p'].standard = invalid
      assert.deepEqual(validateVideoTierDraft(draft, h3Metadata), {
        '1080p.standard': 'Must be greater than zero',
      })
    }
  })

  test('requires reference prices only when metadata exposes that profile', () => {
    const metadata: VideoTierPricingMetadata = {
      billing_unit: 'per_1m_video_tokens',
      inherited: true,
      tiers: ['480p', '720p'],
      prices: {
        '480p': { standard: 11.5, with_reference_video: 7 },
        '720p': { standard: 11.5, with_reference_video: 7 },
      },
    }
    const draft: VideoTierModelPrices = {
      '480p': { standard: 12.5, with_reference_video: 7.5 },
      '720p': { standard: 13.5 },
    }

    assert.deepEqual(validateVideoTierDraft(draft, metadata), {
      '720p.with_reference_video': 'Required',
    })
  })
})
