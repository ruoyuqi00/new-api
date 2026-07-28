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

import type { AuthUser } from '@/stores/auth-store'

import {
  createAuthRedirectSearch,
  getSavedLanguage,
  rememberOAuthLoginRedirect,
  sanitizeAuthRedirect,
  takeOAuthLoginRedirect,
} from './auth-redirect'

const origin = 'https://dashboard.example.com'

describe('authentication redirect validation', () => {
  test('preserves safe internal paths, search parameters, and fragments', () => {
    assert.equal(
      sanitizeAuthRedirect('/console?tab=usage#recent', origin),
      '/console?tab=usage#recent'
    )
    assert.equal(
      sanitizeAuthRedirect(
        'https://dashboard.example.com/dashboard?tab=quota#daily',
        origin
      ),
      '/dashboard?tab=quota#daily'
    )
  })

  test('rejects external and ambiguously parsed redirect targets', () => {
    const unsafeTargets: unknown[] = [
      undefined,
      '',
      'dashboard',
      '//attacker.example/path',
      'https://attacker.example/path',
      'javascript:alert(1)',
      '/\\attacker.example/path',
      'https:\\attacker.example/path',
    ]

    for (const target of unsafeTargets) {
      assert.equal(sanitizeAuthRedirect(target, origin), null)
    }
  })

  test('rejects invalid or non-HTTP application origins', () => {
    assert.equal(sanitizeAuthRedirect('/dashboard', 'not-an-origin'), null)
    assert.equal(sanitizeAuthRedirect('/dashboard', 'file:///tmp/app'), null)
  })

  test('binds an OAuth login destination to one state and consumes it once', () => {
    const storage = new Map<string, string>()
    const sessionStorage = {
      getItem: (key: string) => storage.get(key) ?? null,
      setItem: (key: string, value: string) => storage.set(key, value),
      removeItem: (key: string) => storage.delete(key),
    }

    const redirect = createAuthRedirectSearch(
      '/playground?tab=media#render',
      origin
    )
    assert.deepEqual(redirect, { redirect: '/playground?tab=media#render' })
    assert.equal(
      rememberOAuthLoginRedirect(
        'valid_oauth_flow_token_1234567890',
        redirect.redirect,
        origin,
        sessionStorage
      ),
      true
    )
    assert.equal(
      takeOAuthLoginRedirect(
        'valid_oauth_flow_token_1234567890',
        origin,
        sessionStorage
      ),
      '/playground?tab=media#render'
    )
    assert.equal(
      takeOAuthLoginRedirect(
        'valid_oauth_flow_token_1234567890',
        origin,
        sessionStorage
      ),
      null
    )
  })

  test('does not persist unsafe OAuth login destinations', () => {
    const storage = new Map<string, string>()
    const sessionStorage = {
      getItem: (key: string) => storage.get(key) ?? null,
      setItem: (key: string, value: string) => storage.set(key, value),
      removeItem: (key: string) => storage.delete(key),
    }

    assert.equal(
      rememberOAuthLoginRedirect(
        'valid_oauth_flow_token_1234567890',
        'https://attacker.example/path',
        origin,
        sessionStorage
      ),
      false
    )
    assert.equal(storage.size, 0)
  })
})

describe('saved authentication language', () => {
  const user: AuthUser = { id: 1, username: 'user', role: 1 }

  test('prefers the explicit user language', () => {
    assert.equal(
      getSavedLanguage({
        ...user,
        language: 'ja',
        setting: { language: 'fr' },
      }),
      'ja'
    )
  })

  test('reads object and JSON string settings', () => {
    assert.equal(
      getSavedLanguage({ ...user, setting: { language: 'fr' } }),
      'fr'
    )
    assert.equal(
      getSavedLanguage({ ...user, setting: '{"language":"ru"}' }),
      'ru'
    )
  })

  test('ignores malformed and non-string setting languages', () => {
    assert.equal(getSavedLanguage({ ...user, setting: '{' }), undefined)
    assert.equal(
      getSavedLanguage({ ...user, setting: { language: 123 } }),
      undefined
    )
  })
})
