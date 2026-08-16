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
export type ApiDocsLocale = 'zh-CN' | 'zh-TW' | 'en'

export interface ApiDocsLocaleConfig {
  labelKey: 'Simplified Chinese' | 'Traditional Chinese' | 'English'
  path: string
}

interface StorageLike {
  getItem(key: string): string | null
  setItem(key: string, value: string): void
}

export const API_DOCS_LOCALE_STORAGE_KEY = 'yuapi:api-docs-locale:v1'

export const API_DOCS_LOCALES: Record<
  ApiDocsLocale,
  ApiDocsLocaleConfig
> = {
  'zh-CN': {
    labelKey: 'Simplified Chinese',
    path: '/developer-docs/yucore-api.md',
  },
  'zh-TW': {
    labelKey: 'Traditional Chinese',
    path: '/developer-docs/yucore-api.zh-TW.md',
  },
  en: {
    labelKey: 'English',
    path: '/developer-docs/yucore-api.en.md',
  },
}

function mapApiDocsLocale(language?: string | null): ApiDocsLocale | null {
  const normalized = language?.trim().replace('_', '-').toLowerCase()
  if (!normalized) return null
  if (
    normalized === 'zh-tw' ||
    normalized === 'zh-hk' ||
    normalized.startsWith('zh-hant')
  ) {
    return 'zh-TW'
  }
  if (normalized === 'zh' || normalized.startsWith('zh-')) return 'zh-CN'
  if (normalized === 'en' || normalized.startsWith('en-')) return 'en'
  return null
}

export function resolveApiDocsLocale(
  remembered: string | null,
  siteLanguage: string | null,
  browserLanguages: readonly string[]
): ApiDocsLocale {
  if (remembered && remembered in API_DOCS_LOCALES) {
    return remembered as ApiDocsLocale
  }
  const siteLocale = mapApiDocsLocale(siteLanguage)
  if (siteLocale) return siteLocale
  for (const language of browserLanguages) {
    const browserLocale = mapApiDocsLocale(language)
    if (browserLocale) return browserLocale
  }
  return 'zh-CN'
}

export function readApiDocsLocale(
  storage?: Pick<StorageLike, 'getItem'>
): ApiDocsLocale | null {
  try {
    const target =
      storage ?? (typeof window === 'undefined' ? null : window.localStorage)
    if (!target) return null
    const value = target.getItem(API_DOCS_LOCALE_STORAGE_KEY)
    return value && value in API_DOCS_LOCALES
      ? (value as ApiDocsLocale)
      : null
  } catch {
    return null
  }
}

export function writeApiDocsLocale(
  locale: ApiDocsLocale,
  storage?: Pick<StorageLike, 'setItem'>
): void {
  try {
    const target =
      storage ?? (typeof window === 'undefined' ? null : window.localStorage)
    target?.setItem(API_DOCS_LOCALE_STORAGE_KEY, locale)
  } catch {
    // The selector still works for this tab when storage is unavailable.
  }
}
