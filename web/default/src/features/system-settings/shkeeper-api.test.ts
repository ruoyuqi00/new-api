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
import { afterEach, test } from 'node:test'

import {
  CanceledError,
  type AxiosAdapter,
  type InternalAxiosRequestConfig,
} from 'axios'

import { api } from '@/lib/api'

import { getSHKeeperSettings } from './api'
import type { SHKeeperSettingsResponse } from './types'

const originalAdapter = api.defaults.adapter

afterEach(() => {
  api.defaults.adapter = originalAdapter
})

test('retries canceled SHKeeper status immediately with a fresh GET', async () => {
  const requests: InternalAxiosRequestConfig[] = []
  let notifyStarted!: () => void
  const started = new Promise<void>((resolve) => {
    notifyStarted = resolve
  })
  let aborts = 0
  const response: SHKeeperSettingsResponse = {
    success: true,
    message: '',
    data: {
      enabled: false,
      base_url: 'https://fresh.example.com',
      api_key_configured: true,
      backend_key_configured: true,
      packages: [],
      enabled_networks: [],
      invoice_expiry_minutes: 30,
      reconcile_interval_seconds: 60,
      allow_private_url: false,
    },
  }
  const adapter: AxiosAdapter = (config) => {
    requests.push(config)
    if (requests.length === 1) {
      return new Promise((_resolve, reject) => {
        config.signal?.addEventListener?.(
          'abort',
          () => {
            aborts += 1
            const cancellation = new CanceledError('Status canceled')
            cancellation.config = config
            reject(cancellation)
          },
          { once: true }
        )
        notifyStarted()
      })
    }
    return Promise.resolve({
      config,
      data: response,
      headers: {},
      status: 200,
      statusText: 'OK',
    })
  }
  api.defaults.adapter = adapter
  const canceledController = new AbortController()
  const retryController = new AbortController()
  const first = getSHKeeperSettings(canceledController.signal)
  await started

  // Retry before the canceled request's rejection clears the global GET map.
  canceledController.abort()
  const retry = getSHKeeperSettings(retryController.signal)
  const [canceled, retried] = await Promise.allSettled([first, retry])

  assert.equal(aborts, 1)
  assert.equal(canceled.status, 'rejected')
  if (canceled.status === 'rejected') {
    assert.ok(canceled.reason instanceof CanceledError)
  }
  assert.equal(requests.length, 2)
  assert.equal(retried.status, 'fulfilled')
  if (retried.status === 'fulfilled') {
    assert.deepEqual(retried.value, response)
  }
  assert.equal(requests[0].signal, canceledController.signal)
  assert.equal(requests[1].signal, retryController.signal)
  assert.equal(retryController.signal.aborted, false)

  const refetchController = new AbortController()
  assert.deepEqual(
    await getSHKeeperSettings(refetchController.signal),
    response
  )
  assert.equal(requests.length, 3)
  assert.equal(requests[2].signal, refetchController.signal)
  for (const request of requests) {
    assert.equal(request.method, 'get')
    assert.equal(request.url, '/api/option/shkeeper/status')
  }
})
