import assert from 'node:assert/strict'
import { describe, it } from 'node:test'

import { parseAccountImport } from './import-parser.ts'

const defaults = { type: 'api_key', namePrefix: 'openai', startIndex: 0 }

function codexAccessToken(accountId: string) {
  const payload = Buffer.from(
    JSON.stringify({
      'https://api.openai.com/auth': { chatgpt_account_id: accountId },
    })
  ).toString('base64url')
  return `header.${payload}.signature`
}

function codexSessionToken(
  accountId: string,
  userId: string,
  email: string,
  expiresAt = 2_000_000_000
) {
  const payload = Buffer.from(
    JSON.stringify({
      email,
      exp: expiresAt,
      'https://api.openai.com/auth': {
        chatgpt_account_id: accountId,
        chatgpt_user_id: userId,
        chatgpt_plan_type: 'plus',
      },
    })
  ).toString('base64url')
  return `header.${payload}.signature`
}

describe('parseAccountImport', () => {
  it('imports one credential per line from text files', () => {
    const rows = parseAccountImport('key-a\n\nkey-b', 'accounts.txt', defaults)

    assert.deepEqual(
      rows.map((row) => row.account?.credential),
      ['key-a', 'key-b']
    )
  })

  it('parses CSV account routing fields and reports invalid rows', () => {
    const rows = parseAccountImport(
      [
        'name,api_key,base_url,model_mapping,concurrency',
        'primary,key-a,https://api.example.com,"{""gpt-4o"":""upstream""}",3',
        'broken,key-b,ftp://invalid.example,,1',
      ].join('\n'),
      'accounts.csv',
      defaults
    )

    assert.equal(rows.length, 2)
    assert.deepEqual(rows[0].account, {
      name: 'primary',
      type: 'api_key',
      credential: 'key-a',
      base_url: 'https://api.example.com',
      model_mapping: '{"gpt-4o":"upstream"}',
      concurrency_limit: 3,
      priority: 0,
      weight: 100,
      cooldown_seconds: 20,
    })
    assert.equal(rows[1].error, 'Base URL must use HTTP or HTTPS')
  })

  it('accepts JSON objects using Sub2API-compatible field names', () => {
    const rows = parseAccountImport(
      JSON.stringify([
        {
          name: 'account-a',
          api_key: 'key-a',
          base_url: 'https://api.example.com/',
          model_mapping: { model: 'upstream-model' },
          cooldown: 30,
        },
      ]),
      'accounts.json',
      defaults
    )

    assert.deepEqual(rows[0].account, {
      name: 'account-a',
      type: 'api_key',
      credential: 'key-a',
      base_url: 'https://api.example.com',
      model_mapping: '{"model":"upstream-model"}',
      cooldown_seconds: 30,
      priority: 0,
      weight: 100,
      concurrency_limit: 0,
    })
  })

  it('rejects malformed OAuth JSON before adding it to the form', () => {
    const rows = parseAccountImport('not-json', 'oauth.txt', {
      ...defaults,
      type: 'oauth_json',
    })

    assert.equal(rows[0].account, undefined)
    assert.equal(rows[0].error, 'OAuth credential must be valid JSON')
  })

  it('imports one account from a Sub2API export envelope', () => {
    const rows = parseAccountImport(
      JSON.stringify({
        format: 'sub2api',
        workspace_id: 'workspace-1',
        token_kind: 'access_token',
        accounts: [
          {
            name: 'subscription-1',
            platform: 'openai',
            type: 'oauth',
            priority: 4,
            concurrency: 8,
            credentials: {
              access_token: codexAccessToken('account-1'),
              expires_at: 2_000_000_000,
              email: 'account@example.com',
            },
            extra: { last_refresh: '2026-07-12T00:00:00Z' },
          },
        ],
      }),
      'single.json',
      defaults
    )

    assert.equal(rows.length, 1)
    assert.equal(rows[0].error, undefined)
    assert.equal(rows[0].account?.adapter_type, 57)
    assert.equal(rows[0].account?.provider, 'openai')
    assert.equal(rows[0].account?.type, 'oauth_json')
    assert.equal(rows[0].account?.concurrency_limit, 8)
    assert.equal(rows[0].account?.expires_at, 2_000_000_000)
    assert.deepEqual(JSON.parse(rows[0].account?.credential ?? '{}'), {
      access_token: codexAccessToken('account-1'),
      account_id: 'account-1',
      email: 'account@example.com',
      expired: '2033-05-18T03:33:20.000Z',
      last_refresh: '2026-07-12T00:00:00Z',
      type: 'codex',
    })
  })

  it('imports every account from a multi-account Sub2API export', () => {
    const rows = parseAccountImport(
      JSON.stringify({
        format: 'sub2api',
        accounts: ['account-1', 'account-2'].map((accountId, index) => ({
          name: `subscription-${index + 1}`,
          platform: 'openai',
          type: 'oauth',
          credentials: { access_token: codexAccessToken(accountId) },
        })),
      }),
      'multiple.json',
      defaults
    )

    assert.equal(rows.length, 2)
    assert.ok(rows.every((row) => row.account?.type === 'oauth_json'))
    assert.deepEqual(
      rows.map((row) => JSON.parse(row.account?.credential ?? '{}').account_id),
      ['account-1', 'account-2']
    )
  })

  it('imports current Sub2API data backups and preserves routing metadata', () => {
    const rows = parseAccountImport(
      JSON.stringify({
        type: 'sub2api-data',
        version: 1,
        exported_at: '2026-07-12T00:00:00Z',
        proxies: [
          {
            proxy_key: 'https|proxy.example.com|443||',
            name: 'primary-proxy',
          },
        ],
        accounts: [
          {
            name: 'api-account',
            notes: 'production',
            platform: 'anthropic',
            type: 'apikey',
            credentials: {
              api_key: 'sk-ant-test',
              base_url: 'https://api.example.com/',
              model_mapping: { claude: 'claude-upstream' },
            },
            extra: { plan_type: 'team' },
            proxy_key: 'https|proxy.example.com|443||',
            concurrency: 6,
            priority: 9,
            expires_at: 2_000_000_000,
          },
        ],
      }),
      'sub2api-data.json',
      defaults
    )

    assert.equal(rows[0].error, undefined)
    assert.equal(rows[0].account?.adapter_type, 14)
    assert.equal(rows[0].account?.base_url, 'https://api.example.com')
    assert.equal(rows[0].account?.model_mapping, '{"claude":"claude-upstream"}')
    assert.equal(rows[0].account?.concurrency_limit, 6)
    assert.equal(rows[0].account?.expires_at, 2_000_000_000)
    assert.deepEqual(JSON.parse(rows[0].account?.metadata ?? '{}'), {
      source_format: 'sub2api',
      source_type: 'sub2api-data',
      source_version: 1,
      exported_at: '2026-07-12T00:00:00Z',
      platform: 'anthropic',
      account_type: 'apikey',
      plan_type: 'team',
      notes: 'production',
      proxy_key: 'https|proxy.example.com|443||',
      extra: { plan_type: 'team' },
    })
  })

  it('accepts legacy Sub2API bundle envelopes', () => {
    const rows = parseAccountImport(
      JSON.stringify({
        type: 'sub2api-bundle',
        accounts: [
          {
            name: 'legacy',
            platform: 'openai',
            type: 'apikey',
            credentials: { api_key: 'legacy-key' },
          },
        ],
      }),
      'legacy.json',
      defaults
    )

    assert.equal(rows[0].account?.credential, 'legacy-key')
    assert.equal(rows[0].account?.adapter_type, 1)
  })

  it('accepts direct Codex OAuth credential objects', () => {
    const rows = parseAccountImport(
      JSON.stringify({
        name: 'direct-codex',
        access_token: codexAccessToken('direct-account'),
        refresh_token: 'refresh-token',
      }),
      'codex.json',
      defaults
    )

    assert.equal(rows[0].account?.type, 'oauth_json')
    assert.equal(rows[0].account?.adapter_type, 57)
    assert.equal(
      JSON.parse(rows[0].account?.credential ?? '{}').account_id,
      'direct-account'
    )
  })

  it('imports mixed raw tokens, session JSON, and JSON arrays for Codex pools', () => {
    const rawToken = codexSessionToken(
      'raw-account',
      'raw-user',
      'raw@example.com'
    )
    const sessionToken = codexSessionToken(
      'session-account',
      'session-user',
      'session@example.com'
    )
    const arrayToken = codexSessionToken(
      'array-account',
      'array-user',
      'array@example.com'
    )
    const rows = parseAccountImport(
      [
        rawToken,
        JSON.stringify({
          user: { name: 'Session User', email: 'session-json@example.com' },
          account: { id: 'session-account', planType: 'team' },
          tokens: {
            accessToken: sessionToken,
            refreshToken: 'session-refresh',
          },
          sessionToken: 'must-not-be-imported',
        }),
        JSON.stringify([arrayToken]),
      ].join('\n'),
      'pasted.txt',
      { ...defaults, type: 'oauth_json' }
    )

    assert.equal(rows.length, 3)
    assert.ok(rows.every((row) => row.error === undefined))
    assert.deepEqual(
      rows.map((row) => JSON.parse(row.account?.credential ?? '{}').account_id),
      ['raw-account', 'session-account', 'array-account']
    )
    const session = JSON.parse(rows[1].account?.credential ?? '{}')
    assert.equal(session.refresh_token, 'session-refresh')
    assert.equal(session.email, 'session-json@example.com')
    assert.equal(session.chatgpt_user_id, 'session-user')
    assert.equal(session.plan_type, 'team')
    assert.equal(session.sessionToken, undefined)
    assert.equal(session.session_token, undefined)
  })

  it('accepts concatenated JSON streams from Sub2API-compatible exports', () => {
    const first = {
      tokens: {
        access_token: codexAccessToken('stream-account-1'),
        refresh_token: 'refresh-1',
      },
      account: { id: 'stream-account-1' },
    }
    const second = {
      accessToken: codexAccessToken('stream-account-2'),
      accountId: 'stream-account-2',
    }
    const rows = parseAccountImport(
      `${JSON.stringify(first)}\n${JSON.stringify(second)}`,
      'stream.json',
      { ...defaults, type: 'oauth_json' }
    )

    assert.equal(rows.length, 2)
    assert.deepEqual(
      rows.map((row) => JSON.parse(row.account?.credential ?? '{}').account_id),
      ['stream-account-1', 'stream-account-2']
    )
    assert.equal(
      JSON.parse(rows[0].account?.credential ?? '{}').refresh_token,
      'refresh-1'
    )
  })

  it('normalizes nested Codex tokens in Sub2API records with type aliases', () => {
    const rows = parseAccountImport(
      JSON.stringify({
        type: 'sub2api-data',
        accounts: [
          {
            name: 'aliased-codex',
            platform: 'codex',
            type: 'session',
            credentials: {
              tokens: {
                accessToken: codexAccessToken('aliased-account'),
                refreshToken: 'aliased-refresh',
              },
              account: { id: 'aliased-account' },
            },
          },
        ],
      }),
      'sub2api-alias.json',
      defaults
    )

    assert.equal(rows[0].account?.type, 'oauth_json')
    assert.equal(rows[0].account?.adapter_type, 57)
    assert.deepEqual(JSON.parse(rows[0].account?.credential ?? '{}'), {
      access_token: codexAccessToken('aliased-account'),
      account_id: 'aliased-account',
      refresh_token: 'aliased-refresh',
      type: 'codex',
    })
  })

  it('preserves CPA Codex fields and normalizes numeric timestamps', () => {
    const rows = parseAccountImport(
      JSON.stringify({
        access_token: codexAccessToken('cpa-account'),
        account_id: 'cpa-account',
        auth_mode: 'personal_access_token',
        disable_cooling: false,
        disabled: false,
        email: 'cpa@example.com',
        excluded_models: ['gpt-hidden'],
        expired: 2_000_000_000,
        id_token: 'id-token',
        last_refresh: 1_900_000_000,
        openai_auth_mode: 'personal_access_token',
        plan_type: 'team',
        proxy_url: 'http://127.0.0.1:7897',
        refresh_token: '',
        token_type: 'Bearer',
        type: 'codex',
      }),
      'codex-cpa.json',
      defaults
    )

    assert.equal(rows[0].error, undefined)
    assert.equal(rows[0].account?.expires_at, 2_000_000_000)
    assert.deepEqual(JSON.parse(rows[0].account?.credential ?? '{}'), {
      access_token: codexAccessToken('cpa-account'),
      account_id: 'cpa-account',
      auth_mode: 'personal_access_token',
      disable_cooling: false,
      disabled: false,
      email: 'cpa@example.com',
      excluded_models: ['gpt-hidden'],
      expired: '2033-05-18T03:33:20.000Z',
      id_token: 'id-token',
      last_refresh: '2030-03-17T17:46:40.000Z',
      openai_auth_mode: 'personal_access_token',
      plan_type: 'team',
      proxy_url: 'http://127.0.0.1:7897',
      refresh_token: '',
      token_type: 'Bearer',
      type: 'codex',
    })
  })

  it('accepts camel-case Codex credential aliases', () => {
    const rows = parseAccountImport(
      JSON.stringify({
        accessToken: codexAccessToken('camel-account'),
        accountId: 'camel-account',
        refreshToken: 'refresh-token',
        idToken: 'id-token',
        lastRefresh: '2026-07-15T00:00:00Z',
        expiresAt: '2026-07-16T00:00:00Z',
      }),
      'camel-codex.json',
      defaults
    )

    assert.equal(rows[0].error, undefined)
    assert.deepEqual(JSON.parse(rows[0].account?.credential ?? '{}'), {
      access_token: codexAccessToken('camel-account'),
      account_id: 'camel-account',
      refresh_token: 'refresh-token',
      id_token: 'id-token',
      last_refresh: '2026-07-15T00:00:00Z',
      expired: '2026-07-16T00:00:00Z',
      type: 'codex',
    })
    assert.equal(rows[0].account?.expires_at, undefined)
  })

  it('imports cookie, session, and setup-token credential fields', () => {
    const rows = parseAccountImport(
      JSON.stringify([
        { name: 'cookie', type: 'cookie', cookie: 'session-cookie' },
        { name: 'session', type: 'custom', session_key: 'session-key' },
        { name: 'setup', type: 'custom', setup_token: 'setup-token' },
      ]),
      'sessions.json',
      defaults
    )

    assert.deepEqual(
      rows.map((row) => row.account?.credential),
      ['session-cookie', 'session-key', 'setup-token']
    )
  })

  it('keeps non-Codex OAuth objects as structured OAuth credentials', () => {
    const rows = parseAccountImport(
      JSON.stringify({
        name: 'generic-oauth',
        access_token: 'opaque-access-token',
        refresh_token: 'opaque-refresh-token',
      }),
      'oauth.json',
      { ...defaults, type: 'oauth_json' }
    )

    assert.equal(rows[0].error, undefined)
    assert.deepEqual(JSON.parse(rows[0].account?.credential ?? '{}'), {
      name: 'generic-oauth',
      access_token: 'opaque-access-token',
      refresh_token: 'opaque-refresh-token',
    })
  })

  it('serializes object metadata from generic JSON imports', () => {
    const rows = parseAccountImport(
      JSON.stringify({ credential: 'key-a', metadata: { plan_type: 'pro' } }),
      'generic.json',
      defaults
    )

    assert.equal(rows[0].account?.metadata, '{"plan_type":"pro"}')
  })
})
