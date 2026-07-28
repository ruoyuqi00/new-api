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

type BuildCCSwitchURLParams = {
  app: string
  name: string
  models: Record<string, string>
  apiKey: string
  apiAddress: string
  serverAddress: string
}

function appendV1(address: string): string {
  return /\/v1$/i.test(address) ? address : `${address}/v1`
}

export function encodeConnectionString(key: string, apiAddress: string) {
  return JSON.stringify({
    _type: 'newapi_channel_conn',
    key,
    url: apiAddress,
  })
}

export function buildCCSwitchURL(params: BuildCCSwitchURLParams): string {
  const endpoint =
    params.app === 'codex' ? appendV1(params.apiAddress) : params.apiAddress
  const searchParams = new URLSearchParams()
  searchParams.set('resource', 'provider')
  searchParams.set('app', params.app)
  searchParams.set('name', params.name)
  searchParams.set('endpoint', endpoint)
  searchParams.set('apiKey', params.apiKey)
  for (const [key, value] of Object.entries(params.models)) {
    if (value) searchParams.set(key, value)
  }
  searchParams.set('homepage', params.serverAddress)
  searchParams.set('enabled', 'true')
  return `ccswitch://v1/import?${searchParams.toString()}`
}
