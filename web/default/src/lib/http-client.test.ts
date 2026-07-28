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

import {
  default as axios,
  AxiosError,
  AxiosHeaders,
  type AxiosAdapter,
  type AxiosResponse,
  type InternalAxiosRequestConfig,
} from 'axios'

import {
  applyAuthBundle,
  applyAuthRotation,
  captureAuthRequestSnapshot,
  clearAuthentication,
  isAuthRequestSessionCurrent,
} from '@/lib/auth-session'
import { api } from '@/lib/http-client'
import { useAuthStore, type AuthBundle } from '@/stores/auth-store'

const originalAdapter = api.defaults.adapter

function createBundle(
  sid: string,
  userID: number,
  accessToken: string
): AuthBundle {
  return {
    access_token: accessToken,
    token_type: 'Bearer',
    access_expires_at: Math.floor(Date.now() / 1000) + 600,
    user: { id: userID, username: `user-${userID}`, role: 1 },
    session: {
      sid,
      current: true,
      login_method: 'password',
      ip: '127.0.0.1',
      user_agent: 'test',
      created_at: 100,
      last_active_at: 100,
      expires_at: 1_900_000_000,
    },
  }
}

function createUnauthorizedError(
  config: InternalAxiosRequestConfig
): AxiosError {
  const response: AxiosResponse = {
    config,
    data: { success: false },
    headers: new AxiosHeaders(),
    status: 401,
    statusText: 'Unauthorized',
  }
  return new AxiosError(
    'Unauthorized',
    AxiosError.ERR_BAD_REQUEST,
    config,
    undefined,
    response
  )
}

afterEach(() => {
  api.defaults.adapter = originalAdapter
  clearAuthentication(false, 'idle')
})

describe('HTTP authentication request identity', () => {
  test('captures an immutable dispatch identity outside serialized request data', async () => {
    const bundleA = createBundle('session-a', 1, 'token-a')
    applyAuthBundle(bundleA, false)
    let requestConfig: InternalAxiosRequestConfig | undefined
    const adapter: AxiosAdapter = async (config) => {
      requestConfig = config
      return {
        config,
        data: { success: true },
        headers: new AxiosHeaders(),
        status: 200,
        statusText: 'OK',
      }
    }
    api.defaults.adapter = adapter

    await api.post('/api/account/mutation', { displayName: 'Account A' })

    assert.ok(requestConfig)
    assert.equal(
      Number.isInteger(requestConfig.authRequestSnapshot?.epoch),
      true
    )
    assert.equal(requestConfig.authRequestSnapshot?.sessionSID, 'session-a')
    assert.equal(requestConfig.authRequestSnapshot?.userID, 1)
    assert.equal(Object.isFrozen(requestConfig.authRequestSnapshot), true)
    assert.equal(requestConfig.headers.get('authRequestSnapshot'), undefined)
    assert.equal(requestConfig.params, undefined)
    assert.deepEqual(JSON.parse(String(requestConfig.data)), {
      displayName: 'Account A',
    })
  })

  test('does not refresh or replay a stale request after switching accounts', async () => {
    const bundleA = createBundle('session-a', 1, 'token-a')
    const bundleB = createBundle('session-b', 2, 'token-b')
    applyAuthBundle(bundleA, false)
    let attempts = 0
    let unauthorizedError: AxiosError | undefined
    const adapter: AxiosAdapter = async (config) => {
      attempts += 1
      applyAuthBundle(bundleB, false)
      unauthorizedError = createUnauthorizedError(config)
      throw unauthorizedError
    }
    api.defaults.adapter = adapter

    await assert.rejects(
      api.post(
        '/api/account/mutation',
        { displayName: 'Account A' },
        { skipErrorHandler: true }
      ),
      (error) => error === unauthorizedError
    )

    const auth = useAuthStore.getState().auth
    assert.equal(attempts, 1)
    assert.equal(auth.session?.sid, 'session-b')
    assert.equal(auth.accessToken, 'token-b')
    assert.equal(auth.bootstrapState, 'complete')
  })

  test('does not clear a new account when a retried request returns a stale 401', async () => {
    const bundleA = createBundle('session-a', 1, 'token-a')
    const bundleB = createBundle('session-b', 2, 'token-b')
    applyAuthBundle(bundleA, false)
    const snapshot = captureAuthRequestSnapshot()
    let unauthorizedError: AxiosError | undefined
    const adapter: AxiosAdapter = async (config) => {
      applyAuthBundle(bundleB, false)
      unauthorizedError = createUnauthorizedError(config)
      throw unauthorizedError
    }
    api.defaults.adapter = adapter

    await assert.rejects(
      api.post(
        '/api/account/mutation',
        { displayName: 'Account A' },
        {
          authRetry: true,
          authRequestSnapshot: snapshot,
          skipErrorHandler: true,
        }
      ),
      (error) => error === unauthorizedError
    )

    const auth = useAuthStore.getState().auth
    assert.equal(auth.session?.sid, 'session-b')
    assert.equal(auth.accessToken, 'token-b')
    assert.equal(auth.bootstrapState, 'complete')
  })

  test('retries once with a rotated token when the session SID is unchanged', async () => {
    const bundleA = createBundle('session-a', 1, 'token-a')
    applyAuthBundle(bundleA, false)
    const snapshot = captureAuthRequestSnapshot()
    applyAuthRotation({
      access_token: 'token-a-rotated',
      token_type: 'Bearer',
      access_expires_at: bundleA.access_expires_at + 60,
      session: { ...bundleA.session, last_active_at: 200 },
    })
    assert.equal(isAuthRequestSessionCurrent(snapshot), true)

    let attempts = 0
    let retryConfig: InternalAxiosRequestConfig | undefined
    const adapter: AxiosAdapter = async (config) => {
      attempts += 1
      retryConfig = config
      return {
        config,
        data: { success: true },
        headers: new AxiosHeaders(),
        status: 200,
        statusText: 'OK',
      }
    }
    api.defaults.adapter = adapter

    await api.post(
      '/api/account/mutation',
      { displayName: 'Account A' },
      { authRetry: true, authRequestSnapshot: snapshot }
    )

    assert.equal(attempts, 1)
    assert.equal(
      retryConfig?.headers.get('Authorization'),
      'Bearer token-a-rotated'
    )
    assert.deepEqual(retryConfig?.authRequestSnapshot, snapshot)
    assert.equal(Object.isFrozen(retryConfig?.authRequestSnapshot), true)
  })

  test('caps a same-session retry at one attempt before clearing authentication', async () => {
    const bundleA = createBundle('session-a', 1, 'token-a')
    applyAuthBundle(bundleA, false)
    const snapshot = captureAuthRequestSnapshot()
    applyAuthRotation({
      access_token: 'token-a-rotated',
      token_type: 'Bearer',
      access_expires_at: bundleA.access_expires_at + 60,
      session: { ...bundleA.session, last_active_at: 200 },
    })
    let attempts = 0
    const adapter: AxiosAdapter = async (config) => {
      attempts += 1
      throw createUnauthorizedError(config)
    }
    api.defaults.adapter = adapter

    await assert.rejects(
      api.post(
        '/api/account/mutation',
        { displayName: 'Account A' },
        {
          authRetry: true,
          authRequestSnapshot: snapshot,
          skipErrorHandler: true,
        }
      ),
      AxiosError
    )

    assert.equal(attempts, 1)
    assert.equal(useAuthStore.getState().auth.session, null)
    assert.equal(useAuthStore.getState().auth.accessToken, null)
  })

  test('revalidates the session before dispatching a retry with the current token', async () => {
    const bundleA = createBundle('session-a', 1, 'token-a')
    const bundleB = createBundle('session-b', 2, 'token-b')
    applyAuthBundle(bundleA, false)
    const snapshot = captureAuthRequestSnapshot()
    let adapterCalled = false
    const adapter: AxiosAdapter = async (config) => {
      adapterCalled = true
      return {
        config,
        data: { success: true },
        headers: new AxiosHeaders(),
        status: 200,
        statusText: 'OK',
      }
    }
    api.defaults.adapter = adapter
    const interceptorID = api.interceptors.request.use((config) => {
      if (config.authRetry) applyAuthBundle(bundleB, false)
      return config
    })
    let rejection: unknown

    try {
      await api.post(
        '/api/account/mutation',
        { displayName: 'Account A' },
        { authRetry: true, authRequestSnapshot: snapshot }
      )
    } catch (error: unknown) {
      rejection = error
    } finally {
      api.interceptors.request.eject(interceptorID)
    }

    const auth = useAuthStore.getState().auth
    assert.equal(adapterCalled, false)
    assert.equal(axios.isCancel(rejection), true)
    assert.equal((rejection as AxiosError).config?.skipErrorHandler, true)
    assert.equal(auth.session?.sid, 'session-b')
    assert.equal(auth.accessToken, 'token-b')
    assert.equal(auth.bootstrapState, 'complete')
  })

  test('does not replay an anonymous request under a newly authenticated account', async () => {
    const bundleB = createBundle('session-b', 2, 'token-b')
    clearAuthentication(false, 'complete')
    let attempts = 0
    let unauthorizedError: AxiosError | undefined
    const adapter: AxiosAdapter = async (config) => {
      attempts += 1
      applyAuthBundle(bundleB, false)
      unauthorizedError = createUnauthorizedError(config)
      throw unauthorizedError
    }
    api.defaults.adapter = adapter

    await assert.rejects(
      api.post(
        '/api/public/bootstrap',
        { source: 'anonymous' },
        { skipErrorHandler: true }
      ),
      (error) => error === unauthorizedError
    )

    const auth = useAuthStore.getState().auth
    assert.equal(attempts, 1)
    assert.equal(auth.session?.sid, 'session-b')
    assert.equal(auth.accessToken, 'token-b')
    assert.equal(auth.bootstrapState, 'complete')
  })
})
