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

import {
  getYucoreMotionBudget,
  resolveYucoreMotionProfile,
} from './yucore-motion-performance'

describe('YuCore motion performance profile', () => {
  test('always selects reduced rendering for reduced-motion users', () => {
    assert.equal(
      resolveYucoreMotionProfile({
        deviceMemory: 16,
        devicePixelRatio: 1,
        hardwareConcurrency: 16,
        reducedMotion: true,
        viewportWidth: 1920,
      }),
      'reduced'
    )
  })

  test('selects reduced rendering for constrained CPUs and dense small screens', () => {
    assert.equal(
      resolveYucoreMotionProfile({
        deviceMemory: 8,
        devicePixelRatio: 1,
        hardwareConcurrency: 4,
        reducedMotion: false,
        viewportWidth: 1366,
      }),
      'reduced'
    )
    assert.equal(
      resolveYucoreMotionProfile({
        deviceMemory: 8,
        devicePixelRatio: 3,
        hardwareConcurrency: 8,
        reducedMotion: false,
        viewportWidth: 430,
      }),
      'reduced'
    )
  })

  test('keeps unknown hardware balanced and reserves full rendering for capable desktops', () => {
    assert.equal(
      resolveYucoreMotionProfile({
        devicePixelRatio: 1,
        reducedMotion: false,
        viewportWidth: 1440,
      }),
      'balanced'
    )
    assert.equal(
      resolveYucoreMotionProfile({
        deviceMemory: 8,
        devicePixelRatio: 1.5,
        hardwareConcurrency: 12,
        reducedMotion: false,
        viewportWidth: 1440,
      }),
      'full'
    )
  })

  test('keeps every reduced and balanced render budget below the next tier', () => {
    const reduced = getYucoreMotionBudget('reduced')
    const balanced = getYucoreMotionBudget('balanced')
    const full = getYucoreMotionBudget('full')

    for (const key of [
      'bootParticleCount',
      'bootShardCount',
      'bootSpherePointCount',
      'bootTargetFps',
      'signalParticleCount',
      'signalRouteSegments',
      'signalTargetFps',
    ] as const) {
      assert.ok(reduced[key] < balanced[key], key)
      assert.ok(balanced[key] < full[key], key)
    }

    assert.ok(reduced.maxPixelRatio <= balanced.maxPixelRatio)
    assert.ok(balanced.maxPixelRatio <= full.maxPixelRatio)
  })
})
