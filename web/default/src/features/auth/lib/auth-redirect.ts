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
import type { AuthUser } from '@/stores/auth-store'

const allowedRedirectProtocols = new Set(['http:', 'https:'])
const oauthLoginRedirectKeyPrefix = 'oauth:login:redirect:'
const oauthFlowTokenPattern = /^[A-Za-z0-9_-]{32,128}$/

interface AuthRedirectStorage {
  getItem: (key: string) => string | null
  setItem: (key: string, value: string) => unknown
  removeItem: (key: string) => unknown
}

export function getSavedLanguage(user: AuthUser): string | undefined {
  if (typeof user.language === 'string') {
    return user.language
  }

  if (user.setting && typeof user.setting === 'object') {
    return typeof user.setting.language === 'string'
      ? user.setting.language
      : undefined
  }

  if (typeof user.setting !== 'string') {
    return undefined
  }

  try {
    const setting = JSON.parse(user.setting) as { language?: unknown }
    return typeof setting.language === 'string' ? setting.language : undefined
  } catch {
    return undefined
  }
}

export function sanitizeAuthRedirect(
  value: unknown,
  origin: string
): string | null {
  if (typeof value !== 'string') return null

  const target = value.trim()
  if (!target || target.includes('\\') || target.startsWith('//')) return null

  let trustedOrigin: URL
  try {
    trustedOrigin = new URL(origin)
  } catch {
    return null
  }
  if (!allowedRedirectProtocols.has(trustedOrigin.protocol)) return null

  let redirectURL: URL
  try {
    redirectURL = target.startsWith('/')
      ? new URL(target, trustedOrigin.origin)
      : new URL(target)
  } catch {
    return null
  }

  if (
    !allowedRedirectProtocols.has(redirectURL.protocol) ||
    redirectURL.origin !== trustedOrigin.origin
  ) {
    return null
  }

  return `${redirectURL.pathname}${redirectURL.search}${redirectURL.hash}`
}

export function createAuthRedirectSearch(
  value: unknown,
  origin: string
): { redirect?: string } {
  const redirect = sanitizeAuthRedirect(value, origin)
  return redirect ? { redirect } : {}
}

export function rememberOAuthLoginRedirect(
  flowToken: string,
  value: unknown,
  origin: string,
  storage: AuthRedirectStorage
): boolean {
  if (!oauthFlowTokenPattern.test(flowToken)) return false
  const redirect = sanitizeAuthRedirect(value, origin)
  if (!redirect) return false

  try {
    storage.setItem(`${oauthLoginRedirectKeyPrefix}${flowToken}`, redirect)
    return true
  } catch {
    return false
  }
}

export function takeOAuthLoginRedirect(
  flowToken: string,
  origin: string,
  storage: AuthRedirectStorage
): string | null {
  if (!oauthFlowTokenPattern.test(flowToken)) return null
  const key = `${oauthLoginRedirectKeyPrefix}${flowToken}`
  try {
    const redirect = storage.getItem(key)
    storage.removeItem(key)
    return sanitizeAuthRedirect(redirect, origin)
  } catch {
    return null
  }
}
