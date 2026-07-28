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
  scheduleYucoreHomeDetails,
  scheduleYucoreHomeSecondaryDetails,
} from './yucore-home-details-scheduler'

type FakeSchedulerOptions = { idle: boolean }

function createFakeHomeScheduler(options: FakeSchedulerOptions) {
  let nextHandle = 1
  const listeners = new Map<string, Set<() => void>>()
  const idleCallbacks = new Map<number, () => void>()
  const frameCallbacks = new Map<number, FrameRequestCallback>()

  return {
    addEventListener(type: string, listener: () => void) {
      const registered = listeners.get(type) ?? new Set<() => void>()
      registered.add(listener)
      listeners.set(type, registered)
    },
    cancelAnimationFrame(handle: number) {
      frameCallbacks.delete(handle)
    },
    cancelIdleCallback(handle: number) {
      idleCallbacks.delete(handle)
    },
    emit(type: string) {
      for (const listener of listeners.get(type) ?? []) listener()
    },
    flushFrame() {
      const callbacks = [...frameCallbacks.values()]
      frameCallbacks.clear()
      for (const callback of callbacks) callback(16)
    },
    flushIdle() {
      const callbacks = [...idleCallbacks.values()]
      idleCallbacks.clear()
      for (const callback of callbacks) callback()
    },
    pendingFrameCount() {
      return frameCallbacks.size
    },
    pendingIdleCount() {
      return idleCallbacks.size
    },
    removeEventListener(type: string, listener: () => void) {
      listeners.get(type)?.delete(listener)
    },
    requestAnimationFrame(callback: FrameRequestCallback) {
      const handle = nextHandle++
      frameCallbacks.set(handle, callback)
      return handle
    },
    requestIdleCallback: options.idle
      ? (callback: () => void) => {
          const handle = nextHandle++
          idleCallbacks.set(handle, callback)
          return handle
        }
      : undefined,
  }
}

describe('YuCore home details scheduler', () => {
  test('user intent wins and reveals primary details once', () => {
    const host = createFakeHomeScheduler({ idle: true })
    const stages: string[] = []
    const dispose = scheduleYucoreHomeDetails(host, () =>
      stages.push('primary')
    )

    host.emit('scroll')
    host.flushIdle()

    assert.deepEqual(stages, ['primary'])
    assert.equal(host.pendingIdleCount(), 0)
    assert.equal(host.pendingFrameCount(), 0)
    const disposeSecondary = scheduleYucoreHomeSecondaryDetails(host, () =>
      stages.push('secondary')
    )
    assert.equal(host.pendingFrameCount(), 1)
    host.flushFrame()
    assert.deepEqual(stages, ['primary', 'secondary'])
    disposeSecondary()
    dispose()
  })

  test('falls back to separate animation frames without idle callbacks', () => {
    const host = createFakeHomeScheduler({ idle: false })
    const stages: string[] = []
    scheduleYucoreHomeDetails(host, () => stages.push('primary'))

    assert.equal(host.pendingFrameCount(), 1)
    host.flushFrame()
    assert.deepEqual(stages, ['primary'])
    assert.equal(host.pendingFrameCount(), 0)
    scheduleYucoreHomeSecondaryDetails(host, () => stages.push('secondary'))
    assert.equal(host.pendingFrameCount(), 1)
    host.flushFrame()
    assert.deepEqual(stages, ['primary', 'secondary'])
    assert.equal(host.pendingFrameCount(), 0)
  })

  test('cleanup prevents a late reveal', () => {
    const host = createFakeHomeScheduler({ idle: true })
    const stages: string[] = []
    const dispose = scheduleYucoreHomeDetails(host, () =>
      stages.push('primary')
    )

    dispose()
    host.emit('pointerdown')
    host.flushIdle()
    host.flushFrame()

    assert.deepEqual(stages, [])
  })

  test('cleanup prevents secondary details before their frame', () => {
    const host = createFakeHomeScheduler({ idle: true })
    const stages: string[] = []
    const dispose = scheduleYucoreHomeSecondaryDetails(host, () =>
      stages.push('secondary')
    )

    dispose()
    host.flushFrame()
    assert.deepEqual(stages, [])
  })
})
