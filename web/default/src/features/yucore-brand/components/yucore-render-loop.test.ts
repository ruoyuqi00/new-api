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

import { createYucoreRenderLoop } from './yucore-render-loop'

function createFrameScheduler() {
  let nextId = 1
  const queued = new Map<number, FrameRequestCallback>()

  return {
    cancelAnimationFrame(id: number) {
      queued.delete(id)
    },
    flush(timestamp: number) {
      const next = [...queued.entries()]
      queued.clear()
      for (const [, callback] of next) callback(timestamp)
    },
    queuedCount() {
      return queued.size
    },
    requestAnimationFrame(callback: FrameRequestCallback) {
      const id = nextId++
      queued.set(id, callback)
      return id
    },
  }
}

describe('YuCore render loop', () => {
  test('does not schedule while inactive, hidden, or offscreen and resumes one frame when active', () => {
    const scheduler = createFrameScheduler()
    let renders = 0
    const loop = createYucoreRenderLoop({
      isActive: true,
      render: () => {
        renders += 1
      },
      scheduler,
    })

    loop.setViewportVisible(false)
    loop.start()
    assert.equal(scheduler.queuedCount(), 0)

    loop.setViewportVisible(true)
    assert.equal(scheduler.queuedCount(), 1)
    loop.setViewportVisible(true)
    assert.equal(scheduler.queuedCount(), 1)

    loop.setDocumentVisible(false)
    assert.equal(scheduler.queuedCount(), 0)
    loop.setDocumentVisible(true)
    assert.equal(scheduler.queuedCount(), 1)

    scheduler.flush(100)
    assert.equal(renders, 1)
    assert.equal(scheduler.queuedCount(), 1)

    loop.setActive(false)
    assert.equal(scheduler.queuedCount(), 0)
    loop.dispose()
  })

  test('activates an initialized inactive renderer without duplicate frames', () => {
    const scheduler = createFrameScheduler()
    const loop = createYucoreRenderLoop({
      isActive: false,
      render: () => undefined,
      scheduler,
    })

    loop.start()
    assert.equal(scheduler.queuedCount(), 0)
    loop.setActive(true)
    loop.setActive(true)
    assert.equal(scheduler.queuedCount(), 1)
    loop.setActive(false)
    assert.equal(scheduler.queuedCount(), 0)
  })
})
