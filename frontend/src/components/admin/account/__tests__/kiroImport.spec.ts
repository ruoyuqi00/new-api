import { describe, expect, it } from 'vitest'
import {
  KiroImportInputError,
  buildKiroImportPayload,
  createKiroImportIdempotencyKey
} from '../kiroImport'

describe('kiroImport', () => {
  it('builds refresh token accounts from lines and common separators', () => {
    expect(buildKiroImportPayload('refresh-a\nrefresh-b; refresh-c, refresh-d', 'refresh_token')).toEqual({
      accounts: [
        { refresh_token: 'refresh-a' },
        { refresh_token: 'refresh-b' },
        { refresh_token: 'refresh-c' },
        { refresh_token: 'refresh-d' }
      ]
    })
  })

  it('keeps optional email labels when using email----credential lines', () => {
    expect(buildKiroImportPayload('user@example.com----refresh-a', 'refresh_token')).toEqual({
      accounts: [{ email: 'user@example.com', refresh_token: 'refresh-a' }]
    })
    expect(buildKiroImportPayload('key@example.com----ksk_test', 'api_key')).toEqual({
      accounts: [{ email: 'key@example.com', kiro_api_key: 'ksk_test' }]
    })
  })

  it('builds api key accounts without treating keys as refresh tokens', () => {
    expect(buildKiroImportPayload('ksk_a\nksk_b', 'api_key')).toEqual({
      accounts: [{ kiro_api_key: 'ksk_a' }, { kiro_api_key: 'ksk_b' }]
    })
  })

  it('adds sorted unique group ids when provided', () => {
    expect(buildKiroImportPayload('refresh-a', 'refresh_token', [3, 1, 3, 0])).toEqual({
      accounts: [{ refresh_token: 'refresh-a' }],
      group_ids: [1, 3]
    })
  })

  it('accepts object and array JSON input', () => {
    expect(buildKiroImportPayload('{"refresh_token":"refresh-a"}', 'json')).toEqual({ refresh_token: 'refresh-a' })
    expect(buildKiroImportPayload('[{"kiroApiKey":"ksk_a"}]', 'json')).toEqual({
      accounts: [{ kiroApiKey: 'ksk_a' }]
    })
    expect(buildKiroImportPayload('{"refresh_token":"refresh-a","group_ids":[9]}', 'json', [2])).toEqual({
      refresh_token: 'refresh-a',
      group_ids: [2]
    })
  })

  it('wraps full exported credential JSON as an account to preserve metadata', () => {
    expect(
      buildKiroImportPayload(
        '{"email":"user@example.com","refresh_token":"refresh-a","access_token":"access-a","profileArn":"profile-a","expires_at":1778755870}',
        'json'
      )
    ).toEqual({
      accounts: [
        {
          email: 'user@example.com',
          refresh_token: 'refresh-a',
          access_token: 'access-a',
          profileArn: 'profile-a',
          expires_at: 1778755870
        }
      ]
    })
  })

  it('throws typed errors for empty and malformed input', () => {
    expect(() => buildKiroImportPayload('', 'refresh_token')).toThrow(KiroImportInputError)
    expect(() => buildKiroImportPayload('not-json', 'json')).toThrow(KiroImportInputError)
    expect(() => buildKiroImportPayload('----refresh-a', 'refresh_token')).toThrow(KiroImportInputError)
  })

  it('creates a scoped idempotency key', () => {
    expect(createKiroImportIdempotencyKey()).toMatch(/^kiro-import-\d+-[a-z0-9]+$/)
  })
})
