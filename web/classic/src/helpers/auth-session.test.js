import { describe, expect, mock, test } from 'bun:test';

const showError = mock();

mock.module('./utils', () => ({
  formatMessageForAPI: (message) => message,
  getUserIdFromLocalStorage: () => '',
  isValidMessage: () => true,
  showError,
}));

const {
  API,
  acceptClassicAuthBundle,
  clearClassicAuthentication,
  logoutClassicAuthentication,
  refreshClassicAuthentication,
} = await import('./api');
const { default: axios } = await import('axios');
import {
  isClassicTransientRefreshStatus,
  parseClassicAuthBundle,
  resolveClassicOAuthIntent,
  shouldRefreshClassicOAuthSession,
} from './auth-session';

describe('classic authentication session helpers', () => {
  test('accepts a dashboard auth bundle without persisting its access token', () => {
    const bundle = parseClassicAuthBundle({
      access_token: 'in-memory-access-token',
      token_type: 'Bearer',
      access_expires_at: 1234567890,
      session: {
        sid: 'session-1',
        current: true,
        login_method: 'password',
        ip: '127.0.0.1',
        user_agent: 'classic-test',
        created_at: 1234567000,
        last_active_at: 1234567890,
        expires_at: 1234571490,
      },
      user: { id: 1, username: 'classic-user', role: 1 },
    });

    expect(bundle).toEqual({
      accessToken: 'in-memory-access-token',
      sessionID: 'session-1',
      user: { id: 1, username: 'classic-user', role: 1 },
    });
  });

  test('rejects an incomplete or non-current dashboard session bundle', () => {
    const baseBundle = {
      access_token: 'in-memory-access-token',
      token_type: 'Bearer',
      access_expires_at: 1234567890,
      session: {
        sid: 'session-1',
        current: true,
        login_method: 'password',
        ip: '127.0.0.1',
        user_agent: 'classic-test',
        created_at: 1234567000,
        last_active_at: 1234567890,
        expires_at: 1234571490,
      },
      user: { id: 1, username: 'classic-user', role: 1 },
    };

    expect(
      parseClassicAuthBundle({ ...baseBundle, token_type: 'Token' }),
    ).toBeNull();
    expect(
      parseClassicAuthBundle({
        ...baseBundle,
        session: { ...baseBundle.session, current: false },
      }),
    ).toBeNull();
    expect(
      parseClassicAuthBundle({ ...baseBundle, access_expires_at: 0 }),
    ).toBeNull();
    expect(
      parseClassicAuthBundle({ ...baseBundle, user: { id: 1 } }),
    ).toBeNull();
  });

  test('rejects incomplete auth responses and resolves OAuth intent from live auth', () => {
    expect(parseClassicAuthBundle({ user: { id: 1 } })).toBeNull();
    expect(resolveClassicOAuthIntent(false, false)).toBe('login');
    expect(resolveClassicOAuthIntent(true, false)).toBe('bind');
    expect(resolveClassicOAuthIntent(true, true)).toBe('login');
  });

  test('keeps a saved classic user during a transient refresh failure', () => {
    expect(isClassicTransientRefreshStatus(undefined)).toBe(true);
    expect(isClassicTransientRefreshStatus(429)).toBe(true);
    expect(isClassicTransientRefreshStatus(500)).toBe(true);
    expect(isClassicTransientRefreshStatus(401)).toBe(false);
    expect(isClassicTransientRefreshStatus(409)).toBe(false);
  });

  test('restores a saved browser session before starting an OAuth bind flow', () => {
    expect(shouldRefreshClassicOAuthSession(false, false, true)).toBe(true);
    expect(shouldRefreshClassicOAuthSession(true, false, true)).toBe(false);
    expect(shouldRefreshClassicOAuthSession(false, true, true)).toBe(false);
    expect(shouldRefreshClassicOAuthSession(false, false, false)).toBe(false);
  });

  test('logs out the refresh-cookie session before browser auth has bootstrapped', async () => {
    clearClassicAuthentication();
    const originalPost = API.post;
    const post = mock(() => Promise.resolve({ data: { success: true } }));
    API.post = post;

    try {
      await logoutClassicAuthentication();
      expect(post).toHaveBeenCalledWith('/api/user/auth/logout', undefined, {
        headers: undefined,
        skipAuthRefresh: true,
        skipErrorHandler: true,
      });
    } finally {
      API.post = originalPost;
    }
  });

  test('does not clear classic auth after a transient refresh failure', async () => {
    clearClassicAuthentication();
    showError.mockClear();
    const originalCreate = axios.create;
    axios.create = mock(() => ({
      post: () => Promise.reject({ response: { status: 503 } }),
    }));

    try {
      await expect(
        API.get('/classic-auth-refresh-test', {
          adapter: (config) =>
            Promise.reject({ response: { status: 401 }, config }),
        }),
      ).rejects.toMatchObject({ response: { status: 401 } });
      expect(showError).not.toHaveBeenCalled();
    } finally {
      axios.create = originalCreate;
    }
  });

  test('retries a stale classic session without the old SID', async () => {
    const staleBundle = {
      access_token: 'stale-access-token',
      token_type: 'Bearer',
      access_expires_at: 1234567890,
      session: {
        sid: 'stale-session',
        current: true,
        login_method: 'password',
        ip: '127.0.0.1',
        user_agent: 'classic-test',
        created_at: 1234567000,
        last_active_at: 1234567890,
        expires_at: 1234571490,
      },
      user: { id: 1, username: 'classic-user', role: 1 },
    };
    const currentBundle = {
      ...staleBundle,
      access_token: 'current-access-token',
      session: { ...staleBundle.session, sid: 'current-session' },
      user: { id: 2, username: 'current-user', role: 1 },
    };
    const originalCreate = axios.create;
    const post = mock()
      .mockImplementationOnce(() =>
        Promise.reject({
          response: {
            status: 409,
            data: { code: 'AUTH_SESSION_MISMATCH' },
          },
        }),
      )
      .mockImplementationOnce(() =>
        Promise.resolve({ data: { success: true, data: currentBundle } }),
      );

    acceptClassicAuthBundle(staleBundle);
    axios.create = mock(() => ({ post }));

    try {
      await expect(refreshClassicAuthentication()).resolves.toEqual(
        currentBundle.user,
      );
      expect(post).toHaveBeenNthCalledWith(
        1,
        '/api/user/auth/refresh',
        undefined,
        {
          headers: { 'X-Auth-Session': 'stale-session' },
        },
      );
      expect(post).toHaveBeenNthCalledWith(
        2,
        '/api/user/auth/refresh',
        undefined,
        {
          headers: undefined,
        },
      );
    } finally {
      axios.create = originalCreate;
      clearClassicAuthentication();
    }
  });
});
