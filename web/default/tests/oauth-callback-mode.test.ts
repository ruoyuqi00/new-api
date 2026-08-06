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
import { describe, expect, test } from 'bun:test'

import {
  getOAuthSessionStorage,
  isTelegramOAuthBindCallback,
  markOAuthBindPopup,
  resolveOAuthCallbackMode,
  type OAuthModeStorage,
} from '../src/features/auth/lib/oauth-callback-mode'

function fakeStorage(initial: Record<string, string> = {}): OAuthModeStorage {
  const data = new Map(Object.entries(initial))
  return {
    getItem: (key) => data.get(key) ?? null,
    setItem: (key, value) => void data.set(key, value),
  }
}

describe('OAuth callback mode', () => {
  const opener = { closed: false }
  const state = 'bind-state'

  test('requires a matching provider and state marker for bind mode', () => {
    const storage = fakeStorage()
    expect(markOAuthBindPopup(storage, 'oidc', state)).toBe(true)
    expect(resolveOAuthCallbackMode('oidc', state, { opener, storage })).toBe(
      'bind'
    )
  })

  test('keeps a login callback with a foreign opener in login mode', () => {
    expect(
      resolveOAuthCallbackMode('oidc', state, {
        opener,
        storage: fakeStorage(),
      })
    ).toBe('login')
  })

  test('rejects incomplete or stale bind evidence', () => {
    const storage = fakeStorage()
    markOAuthBindPopup(storage, 'github', state)
    markOAuthBindPopup(storage, 'oidc', 'old-state')

    for (const context of [
      { provider: 'oidc', state, opener, storage },
      { provider: 'oidc', state: 'old-state', opener: null, storage },
      {
        provider: 'oidc',
        state: 'old-state',
        opener: { closed: true },
        storage,
      },
      { provider: 'oidc', state: '', opener, storage },
      { provider: 'oidc', state: 'old-state', opener, storage: null },
    ]) {
      expect(
        resolveOAuthCallbackMode(context.provider, context.state, {
          opener: context.opener,
          storage: context.storage,
        })
      ).toBe('login')
    }
  })

  test('contains blocked session storage access', () => {
    const owner = {
      get sessionStorage(): OAuthModeStorage {
        throw new Error('blocked')
      },
    }
    const blockedStorage: OAuthModeStorage = {
      getItem: () => {
        throw new Error('blocked')
      },
      setItem: () => {
        throw new Error('blocked')
      },
    }

    expect(getOAuthSessionStorage(owner)).toBeNull()
    expect(markOAuthBindPopup(null, 'oidc', state)).toBe(false)
    expect(markOAuthBindPopup(blockedStorage, 'oidc', state)).toBe(false)
    expect(
      resolveOAuthCallbackMode('oidc', state, {
        opener,
        storage: blockedStorage,
      })
    ).toBe('login')
  })

  test('recognizes only explicit Telegram binding callback results', () => {
    expect(isTelegramOAuthBindCallback('telegram', 'success')).toBe(true)
    expect(isTelegramOAuthBindCallback('telegram', 'error')).toBe(true)
    expect(isTelegramOAuthBindCallback('telegram', undefined)).toBe(false)
    expect(isTelegramOAuthBindCallback('oidc', 'success')).toBe(false)
  })
})
