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

import type { AxiosAdapter } from 'axios'

import { api } from '@/lib/api'

import { handleTestChannel } from './channel-actions'

const originalAdapter = api.defaults.adapter

afterEach(() => {
  api.defaults.adapter = originalAdapter
})

describe('channel test response content', () => {
  test('passes the safe response content to the batch test result', async () => {
    const adapter: AxiosAdapter = async (config) => ({
      config,
      data: {
        success: true,
        time: 0.125,
        response_content: 'hello from upstream',
        response_truncated: true,
      },
      headers: {},
      status: 200,
      statusText: 'OK',
    })
    api.defaults.adapter = adapter

    let completion:
      | {
          success: boolean
          responseTime?: number
          responseContent?: string
          responseTruncated?: boolean
        }
      | undefined

    await handleTestChannel(
      17,
      { testModel: 'gpt-test', silent: true },
      (
        success,
        responseTime,
        _error,
        _errorCode,
        responseContent,
        responseTruncated
      ) => {
        completion = {
          success,
          responseTime,
          responseContent,
          responseTruncated,
        }
      }
    )

    assert.deepEqual(completion, {
      success: true,
      responseTime: 125,
      responseContent: 'hello from upstream',
      responseTruncated: true,
    })
  })
})
