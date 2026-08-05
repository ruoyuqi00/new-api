import { describe, expect, test } from 'bun:test'

import {
  getOAuthSessionStorage,
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

  test('keeps a foreign-opener callback in login mode', () => {
    expect(
      resolveOAuthCallbackMode('oidc', state, {
        opener,
        storage: fakeStorage(),
      })
    ).toBe('login')
  })

  test('rejects stale, other-provider, missing-opener, and closed-opener marks', () => {
    const storage = fakeStorage()
    markOAuthBindPopup(storage, 'github', state)
    markOAuthBindPopup(storage, 'oidc', 'old-state')

    expect(resolveOAuthCallbackMode('oidc', state, { opener, storage })).toBe(
      'login'
    )
    expect(
      resolveOAuthCallbackMode('oidc', 'old-state', {
        opener: null,
        storage,
      })
    ).toBe('login')
    expect(
      resolveOAuthCallbackMode('oidc', 'old-state', {
        opener: { closed: true },
        storage,
      })
    ).toBe('login')
  })

  test('contains blocked session storage access', () => {
    const owner = {
      get sessionStorage(): OAuthModeStorage {
        throw new Error('blocked')
      },
    }
    expect(getOAuthSessionStorage(owner)).toBeNull()
    expect(markOAuthBindPopup(null, 'oidc', state)).toBe(false)
    expect(
      markOAuthBindPopup(
        {
          getItem: () => null,
          setItem: () => {
            throw new Error('blocked')
          },
        },
        'oidc',
        state
      )
    ).toBe(false)
  })
})
