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

import { scheduleYucoreHomeRendererPrewarm } from './yucore-home-renderer-prewarm'

function createFakeTimerHost() {
  let now = 0
  let nextHandle = 1
  const timers = new Map<number, { callback: () => void; dueAt: number }>()

  return {
    advanceTo(target: number) {
      while (true) {
        const next = [...timers.entries()]
          .filter(([, timer]) => timer.dueAt <= target)
          .sort((left, right) => left[1].dueAt - right[1].dueAt)[0]
        if (!next) break
        const [handle, timer] = next
        timers.delete(handle)
        now = timer.dueAt
        timer.callback()
      }
      now = target
    },
    clearTimeout(handle: number) {
      timers.delete(handle)
    },
    setTimeout(callback: () => void, delay: number) {
      const handle = nextHandle++
      timers.set(handle, { callback, dueAt: now + delay })
      return handle
    },
  }
}

describe('YuCore home renderer prewarm', () => {
  test('prepares signal and Earth in separate ordered stages', () => {
    const host = createFakeTimerHost()
    const stages: string[] = []

    scheduleYucoreHomeRendererPrewarm(
      host,
      1000,
      () => stages.push('signal'),
      () => stages.push('all')
    )

    host.advanceTo(799)
    assert.deepEqual(stages, [])
    host.advanceTo(800)
    assert.deepEqual(stages, ['signal'])
    host.advanceTo(900)
    assert.deepEqual(stages, ['signal', 'all'])
  })

  test('cleanup cancels every pending preparation stage', () => {
    const host = createFakeTimerHost()
    const stages: string[] = []
    const dispose = scheduleYucoreHomeRendererPrewarm(
      host,
      1000,
      () => stages.push('signal'),
      () => stages.push('all')
    )

    dispose()
    host.advanceTo(1000)
    assert.deepEqual(stages, [])
  })
})
