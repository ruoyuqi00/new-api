import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { buildModelSnapshots } from './model-pricing-snapshots'

describe('per-call expression pricing snapshots', () => {
  test('preserves the billing mode and expression when the editor loads settings', () => {
    const [snapshot] = buildModelSnapshots({
      modelPrice: '{}',
      modelRatio: '{}',
      cacheRatio: '{}',
      createCacheRatio: '{}',
      completionRatio: '{}',
      imageRatio: '{}',
      audioRatio: '{}',
      audioCompletionRatio: '{}',
      billingMode: '{"MiniMax-M2.7-call":"per_call_expr"}',
      billingExpr:
        '{"MiniMax-M2.7-call":"len <= 128000 ? tier(\\"short\\", 0.05) : tier(\\"long\\", 0.1)"}',
    })

    assert.equal(snapshot?.name, 'MiniMax-M2.7-call')
    assert.equal(snapshot?.billingMode, 'per_call_expr')
    assert.equal(
      snapshot?.billingExpr,
      'len <= 128000 ? tier("short", 0.05) : tier("long", 0.1)'
    )
  })
})
