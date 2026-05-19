import { describe, expect, it } from 'vitest'
import {
  WindsurfImportInputError,
  buildWindsurfImportPayload,
  createWindsurfImportIdempotencyKey
} from '../windsurfImport'

describe('windsurfImport', () => {
  it('builds token payload from lines and common separators', () => {
    expect(buildWindsurfImportPayload('token-a\ntoken-b; token-c, token-d', 'token')).toEqual({
      tokens: ['token-a', 'token-b', 'token-c', 'token-d']
    })
  })

  it('builds api_key accounts without treating keys as raw tokens', () => {
    expect(buildWindsurfImportPayload('key-a\nkey-b', 'api_key')).toEqual({
      accounts: [{ api_key: 'key-a' }, { api_key: 'key-b' }]
    })
  })

  it('builds email password accounts from the server-supported separator', () => {
    expect(buildWindsurfImportPayload('user@example.com----secret', 'email_password')).toEqual({
      accounts: [{ email: 'user@example.com', password: 'secret' }]
    })
  })

  it('accepts object and array JSON input', () => {
    expect(buildWindsurfImportPayload('{"raw":"token-a"}', 'json')).toEqual({ raw: 'token-a' })
    expect(buildWindsurfImportPayload('[{"api_key":"key-a"}]', 'json')).toEqual({
      accounts: [{ api_key: 'key-a' }]
    })
  })

  it('throws typed errors for empty and malformed input', () => {
    expect(() => buildWindsurfImportPayload('', 'token')).toThrow(WindsurfImportInputError)
    expect(() => buildWindsurfImportPayload('not-json', 'json')).toThrow(WindsurfImportInputError)
    expect(() => buildWindsurfImportPayload('user@example.com', 'email_password')).toThrow(WindsurfImportInputError)
  })

  it('creates a scoped idempotency key', () => {
    expect(createWindsurfImportIdempotencyKey()).toMatch(/^windsurf-import-\d+-[a-z0-9]+$/)
  })
})
