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

import { initializeFrontendCache } from './frontend-cache'

class MemoryStorage implements Storage {
  private readonly entries = new Map<string, string>()

  get length(): number {
    return this.entries.size
  }

  clear(): void {
    this.entries.clear()
  }

  getItem(key: string): string | null {
    return this.entries.get(key) ?? null
  }

  key(index: number): string | null {
    return [...this.entries.keys()][index] ?? null
  }

  removeItem(key: string): void {
    this.entries.delete(key)
  }

  setItem(key: string, value: string): void {
    this.entries.set(key, value)
  }
}

afterEach(() => {
  Reflect.deleteProperty(globalThis, 'window')
})

describe('frontend cache migration', () => {
  test('removes legacy authentication storage while preserving affiliate data', () => {
    const localStorage = new MemoryStorage()
    localStorage.setItem('newapi:default:cache-version', 'default-v1')
    localStorage.setItem('aff', 'affiliate-code')
    localStorage.setItem('user', '{"id":1}')
    localStorage.setItem('uid', '1')
    localStorage.setItem('oauth:binding:result', '{"success":true}')
    Object.defineProperty(globalThis, 'window', {
      configurable: true,
      value: { localStorage },
    })

    initializeFrontendCache()

    assert.equal(localStorage.getItem('aff'), 'affiliate-code')
    assert.equal(localStorage.getItem('user'), null)
    assert.equal(localStorage.getItem('uid'), null)
    assert.equal(localStorage.getItem('oauth:binding:result'), null)
    assert.equal(
      localStorage.getItem('newapi:default:cache-version'),
      'default-v2'
    )
  })
})
