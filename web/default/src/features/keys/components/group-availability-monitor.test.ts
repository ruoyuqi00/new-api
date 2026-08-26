import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  getAvailabilityBar,
  getAvailabilityCardStatus,
  type GroupAvailabilityItem,
} from './group-availability-monitor-utils'

describe('group availability monitor presentation', () => {
  test('maps API status to a stable card variant without latency fields', () => {
    const item: GroupAvailabilityItem = {
      group: 'china',
      description: 'Domestic models',
      request_count: 300,
      success_count: 299,
      success_rate: 99.67,
      status: 'stable',
      observed_at: 1_700_000_000,
    }

    assert.equal(getAvailabilityCardStatus(item), 'stable')
    assert.equal('latency_ms' in item, false)
    assert.equal('model' in item, false)
  })

  test('keeps groups in observing state until the sample floor is reached', () => {
    const item: GroupAvailabilityItem = {
      group: 'china',
      description: 'Domestic models',
      request_count: 19,
      success_count: 18,
      success_rate: 94.74,
      status: 'observing',
      observed_at: 1_700_000_000,
    }

    assert.equal(getAvailabilityCardStatus(item), 'observing')
  })

  test('calculates a clamped success and failure bar from recent samples', () => {
    const item: GroupAvailabilityItem = {
      group: 'china',
      description: 'Domestic models',
      request_count: 20,
      success_count: 18,
      success_rate: 90,
      status: 'stable',
      observed_at: 1_700_000_000,
    }

    assert.deepEqual(getAvailabilityBar(item), {
      successPercent: 90,
      failurePercent: 10,
    })
    assert.deepEqual(
      getAvailabilityBar({ ...item, request_count: 0, success_count: 0 }),
      { successPercent: 0, failurePercent: 0 }
    )
    assert.deepEqual(
      getAvailabilityBar({ ...item, request_count: 20, success_count: 25 }),
      { successPercent: 100, failurePercent: 0 }
    )
  })
})
