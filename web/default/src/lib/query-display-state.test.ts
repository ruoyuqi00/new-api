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
  getQueryDisplayState,
  getRetainedQueryData,
} from './query-display-state'

describe('query display state', () => {
  test('distinguishes load, refresh, stale error, and terminal error', () => {
    assert.equal(
      getQueryDisplayState({
        hasData: false,
        isPending: true,
        isFetching: true,
        isError: false,
      }),
      'initial-loading'
    )
    assert.equal(
      getQueryDisplayState({
        hasData: true,
        isPending: false,
        isFetching: true,
        isError: false,
      }),
      'refreshing'
    )
    assert.equal(
      getQueryDisplayState({
        hasData: true,
        isPending: false,
        isFetching: false,
        isError: true,
      }),
      'stale-error'
    )
    assert.equal(
      getQueryDisplayState({
        hasData: false,
        isPending: false,
        isFetching: false,
        isError: true,
      }),
      'terminal-error'
    )
    assert.equal(
      getQueryDisplayState({
        hasData: true,
        isPending: false,
        isFetching: false,
        isError: false,
      }),
      'ready'
    )
  })

  test('retains the last successful data only within the same display scope', () => {
    const retainedData = {
      scope: 'common:user',
      data: { items: [{ id: 1 }] },
    }

    assert.deepEqual(
      getRetainedQueryData({
        data: undefined,
        scope: 'common:user',
        retainedData,
      }),
      retainedData.data
    )
    assert.equal(
      getRetainedQueryData({
        data: undefined,
        scope: 'task:user',
        retainedData,
      }),
      undefined
    )

    const currentData = { items: [{ id: 2 }] }
    assert.equal(
      getRetainedQueryData({
        data: currentData,
        scope: 'common:user',
        retainedData,
      }),
      currentData
    )
  })
})
