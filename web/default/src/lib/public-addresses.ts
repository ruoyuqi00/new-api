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

export type PublicAddresses = {
  apiAddress: string
  serverAddress: string
}

function asRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null
  return value as Record<string, unknown>
}

function normalizeAddress(value: unknown): string {
  if (typeof value !== 'string') return ''
  return value.trim().replace(/\/+$/, '')
}

function readAddress(status: unknown, keys: string[]): string {
  const statusRecord = asRecord(status)
  if (!statusRecord) return ''

  const dataRecord = asRecord(statusRecord.data)
  for (const key of keys) {
    const directValue = normalizeAddress(statusRecord[key])
    if (directValue) return directValue

    const nestedValue = normalizeAddress(dataRecord?.[key])
    if (nestedValue) return nestedValue
  }
  return ''
}

function getBrowserOrigin(): string {
  if (typeof window === 'undefined') return ''
  return normalizeAddress(window.location.origin)
}

export function resolvePublicAddresses(
  status: unknown,
  fallbackOrigin = getBrowserOrigin()
): PublicAddresses {
  const normalizedFallback = normalizeAddress(fallbackOrigin)
  const serverAddress =
    readAddress(status, ['server_address', 'serverAddress']) ||
    normalizedFallback
  const apiAddress =
    readAddress(status, ['api_url', 'api_address', 'apiUrl', 'apiAddress']) ||
    serverAddress

  return { apiAddress, serverAddress }
}

export function getStoredPublicAddresses(): PublicAddresses {
  if (typeof window === 'undefined') {
    return resolvePublicAddresses(null)
  }

  try {
    const raw = window.localStorage.getItem('status')
    return resolvePublicAddresses(raw ? JSON.parse(raw) : null)
  } catch {
    return resolvePublicAddresses(null)
  }
}
