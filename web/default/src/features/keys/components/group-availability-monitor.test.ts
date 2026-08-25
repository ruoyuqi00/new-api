import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
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
})
