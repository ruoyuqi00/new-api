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
import { describe, expect, test } from 'bun:test'

import {
  API_DOCS_LOCALES,
  readApiDocsLocale,
  resolveApiDocsLocale,
  writeApiDocsLocale,
} from '../src/features/docs/document-locale'

describe('API documentation locale', () => {
  test('prefers a valid remembered choice', () => {
    expect(resolveApiDocsLocale('zh-TW', 'en', ['zh-CN'])).toBe('zh-TW')
  })

  test('maps the supported site language before browser languages', () => {
    expect(resolveApiDocsLocale(null, 'en-US', ['zh-TW'])).toBe('en')
  })

  test('uses browser language when the site locale has no docs edition', () => {
    expect(resolveApiDocsLocale(null, 'fr', ['zh-HK', 'en'])).toBe('zh-TW')
  })

  test('falls back to Simplified Chinese', () => {
    expect(resolveApiDocsLocale('invalid', 'ja', ['ru'])).toBe('zh-CN')
  })

  test('keeps the legacy Simplified Chinese URL', () => {
    expect(API_DOCS_LOCALES['zh-CN'].path).toBe(
      '/developer-docs/yucore-api.md'
    )
    expect(API_DOCS_LOCALES['zh-TW'].path).toBe(
      '/developer-docs/yucore-api.zh-TW.md'
    )
    expect(API_DOCS_LOCALES.en.path).toBe(
      '/developer-docs/yucore-api.en.md'
    )
  })

  test('contains storage failures and persists only a locale code', () => {
    const values = new Map<string, string>()
    const storage = {
      getItem: (key: string) => values.get(key) ?? null,
      setItem: (key: string, value: string) => values.set(key, value),
    }
    writeApiDocsLocale('en', storage)
    expect(readApiDocsLocale(storage)).toBe('en')
    expect([...values.values()]).toEqual(['en'])

    const broken = {
      getItem: () => {
        throw new Error('disabled')
      },
      setItem: () => {
        throw new Error('disabled')
      },
    }
    expect(readApiDocsLocale(broken)).toBeNull()
    expect(() => writeApiDocsLocale('zh-TW', broken)).not.toThrow()
  })
})
