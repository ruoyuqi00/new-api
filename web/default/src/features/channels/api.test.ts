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
import { afterEach, describe, test } from 'node:test'

import type { AxiosAdapter, InternalAxiosRequestConfig } from 'axios'

import { api } from '@/lib/api'

import { getChannelKey } from './api'

const originalAdapter = api.defaults.adapter

afterEach(() => {
  api.defaults.adapter = originalAdapter
})

describe('channel key security proof', () => {
  test('sends the proof as a header without placing it in the request body', async () => {
    let request: InternalAxiosRequestConfig | undefined
    const adapter: AxiosAdapter = async (config) => {
      request = config
      return {
        config,
        data: { success: true, data: { key: 'secret' } },
        headers: {},
        status: 200,
        statusText: 'OK',
      }
    }
    api.defaults.adapter = adapter

    const result = await getChannelKey(17, 'proof-token')

    assert.deepEqual(result, { success: true, data: { key: 'secret' } })
    assert.equal(request?.method, 'post')
    assert.equal(request?.url, '/api/channel/17/key')
    assert.equal(request?.headers.get('X-Security-Proof'), 'proof-token')
    assert.equal(request?.data, undefined)
  })
})
