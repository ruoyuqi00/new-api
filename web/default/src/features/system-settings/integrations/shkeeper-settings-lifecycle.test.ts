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

import { SHKeeperSettingsLifecycle } from './shkeeper-settings-lifecycle'

function createDeferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (reason?: unknown) => void
  const promise = new Promise<T>((resolvePromise, rejectPromise) => {
    resolve = resolvePromise
    reject = rejectPromise
  })
  return { promise, resolve, reject }
}

describe('SHKeeper settings request lifecycle', () => {
  test('rejects pre-save status ownership and accepts a fresh post-save status', async () => {
    const lifecycle = new SHKeeperSettingsLifecycle()
    const pendingStatusGeneration = lifecycle.beginStatusRequest()
    const saveCompletion = createDeferred<void>()
    const current = {
      base_url: 'https://edited.example.com',
      api_key_configured: true,
    }
    const stale = {
      base_url: 'https://old.example.com',
      api_key_configured: false,
    }

    const save = lifecycle.runSave(
      async () => undefined,
      async () => saveCompletion.promise
    )

    const afterStaleStatus = lifecycle.canApplyStatus(pendingStatusGeneration)
      ? stale
      : current
    assert.deepEqual(afterStaleStatus, current)

    saveCompletion.resolve()
    await save

    assert.equal(lifecycle.canApplyStatus(pendingStatusGeneration), false)
    const freshStatusGeneration = lifecycle.beginStatusRequest()
    assert.equal(lifecycle.canApplyStatus(freshStatusGeneration), true)
    const fresh = {
      base_url: 'https://fresh.example.com',
      api_key_configured: true,
    }
    const afterFreshStatus = lifecycle.canApplyStatus(freshStatusGeneration)
      ? fresh
      : current
    assert.deepEqual(afterFreshStatus, fresh)
  })

  test('coalesces overlapping direct and Save all callers into one writer', async () => {
    const lifecycle = new SHKeeperSettingsLifecycle()
    const cancellation = createDeferred<void>()
    const writerStarted = createDeferred<void>()
    const completion = createDeferred<string>()
    let writes = 0
    let activeWriters = 0
    let maxActiveWriters = 0

    const directSave = lifecycle.runSave(
      async () => cancellation.promise,
      async () => {
        writes += 1
        activeWriters += 1
        maxActiveWriters = Math.max(maxActiveWriters, activeWriters)
        writerStarted.resolve()
        const result = await completion.promise
        activeWriters -= 1
        return result
      }
    )
    const saveAll = lifecycle.runSave(
      async () => undefined,
      async () => {
        writes += 1
        return 'unexpected second write'
      }
    )

    assert.equal(directSave, saveAll)
    assert.equal(writes, 0)
    cancellation.resolve()
    await writerStarted.promise
    assert.equal(writes, 1)
    assert.equal(maxActiveWriters, 1)

    completion.resolve('saved')
    assert.equal(await directSave, 'saved')
    assert.equal(await saveAll, 'saved')
    assert.equal(writes, 1)
  })

  test('cancels pending status work before starting the save writer', async () => {
    const lifecycle = new SHKeeperSettingsLifecycle()
    const events: string[] = []

    await lifecycle.runSave(
      async () => {
        events.push('cancel status')
      },
      async () => {
        events.push('write settings')
      }
    )

    assert.deepEqual(events, ['cancel status', 'write settings'])
  })
})
