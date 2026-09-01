import { describe, expect, test } from 'bun:test'

import en from '../src/i18n/locales/en.json'
import fr from '../src/i18n/locales/fr.json'
import ja from '../src/i18n/locales/ja.json'
import ru from '../src/i18n/locales/ru.json'
import vi from '../src/i18n/locales/vi.json'
import zh from '../src/i18n/locales/zh.json'

describe('ticket navigation translations', () => {
  test('translates the user ticket center label in every supported locale', () => {
    expect(en.translation['Ticket Center']).toBe('Ticket Center')
    expect(zh.translation['Ticket Center']).toBe('工单中心')
    expect(fr.translation['Ticket Center']).toBe('Centre de tickets')
    expect(ja.translation['Ticket Center']).toBe('チケットセンター')
    expect(ru.translation['Ticket Center']).toBe('Центр тикетов')
    expect(vi.translation['Ticket Center']).toBe('Trung tâm hỗ trợ')
  })
})
