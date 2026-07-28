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
import { describe, expect, it } from 'bun:test'

import {
  buildCCSwitchURL,
  encodeConnectionString,
} from '../src/features/keys/lib/config-export'
import { resolvePublicAddresses } from '../src/lib/public-addresses'

describe('public API configuration exports', () => {
  it('prefers the API URL and normalizes trailing slashes', () => {
    expect(
      resolvePublicAddresses(
        {
          api_url: ' https://api.yuaiapi.com/// ',
          server_address: 'https://yuaiapi.com/',
        },
        'https://fallback.example.com'
      )
    ).toEqual({
      apiAddress: 'https://api.yuaiapi.com',
      serverAddress: 'https://yuaiapi.com',
    })
  })

  it('falls back to the server address for generic deployments', () => {
    expect(
      resolvePublicAddresses(
        { data: { server_address: 'https://site.example.com/' } },
        'https://fallback.example.com'
      )
    ).toEqual({
      apiAddress: 'https://site.example.com',
      serverAddress: 'https://site.example.com',
    })
  })

  it('copies the API address in connection information', () => {
    expect(encodeConnectionString('sk-test', 'https://api.yuaiapi.com')).toBe(
      '{"_type":"newapi_channel_conn","key":"sk-test","url":"https://api.yuaiapi.com"}'
    )
  })

  it('uses the API address for CC Switch and keeps the website homepage', () => {
    const url = buildCCSwitchURL({
      app: 'codex',
      name: 'My Codex',
      models: { model: 'gpt-5.4' },
      apiKey: 'sk-test',
      apiAddress: 'https://api.yuaiapi.com',
      serverAddress: 'https://yuaiapi.com',
    })
    const params = new URL(url).searchParams

    expect(params.get('endpoint')).toBe('https://api.yuaiapi.com/v1')
    expect(params.get('homepage')).toBe('https://yuaiapi.com')
    expect(params.get('apiKey')).toBe('sk-test')
    expect(params.get('model')).toBe('gpt-5.4')
  })

  it('does not duplicate a configured /v1 suffix', () => {
    const url = buildCCSwitchURL({
      app: 'codex',
      name: 'My Codex',
      models: { model: 'gpt-5.4' },
      apiKey: 'sk-test',
      apiAddress: 'https://api.yuaiapi.com/v1',
      serverAddress: 'https://yuaiapi.com',
    })

    expect(new URL(url).searchParams.get('endpoint')).toBe(
      'https://api.yuaiapi.com/v1'
    )
  })
})
