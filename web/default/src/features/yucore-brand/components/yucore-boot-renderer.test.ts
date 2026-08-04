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

import { createBootScene } from './yucore-boot-renderer'
import { getYucoreMotionBudget } from './yucore-motion-performance'

describe('YuCore boot renderer budget', () => {
  test('allocates each deterministic scene layer from the selected budget', () => {
    const budget = getYucoreMotionBudget('reduced')
    const first = createBootScene(budget)
    const second = createBootScene(budget)

    assert.equal(first.particles.length, budget.bootParticleCount)
    assert.equal(first.shards.length, budget.bootShardCount)
    assert.equal(first.spherePoints.length, budget.bootSpherePointCount)
    assert.deepEqual(first.particles.at(0), second.particles.at(0))
    assert.deepEqual(first.particles.at(-1), second.particles.at(-1))
    assert.deepEqual(first.spherePoints.at(-1), second.spherePoints.at(-1))
  })
})
