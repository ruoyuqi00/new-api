import { describe, expect, test } from 'bun:test'

import {
  ticketPriorityTranslationKey,
  ticketStatusTranslationKey,
} from '../src/features/tickets/lib'

describe('ticket display state', () => {
  test('maps every server status to a translated label key', () => {
    expect(ticketStatusTranslationKey('open')).toBe('Open')
    expect(ticketStatusTranslationKey('pending_user')).toBe('Waiting for User')
    expect(ticketStatusTranslationKey('pending_admin')).toBe('Waiting for Support')
    expect(ticketStatusTranslationKey('closed')).toBe('Closed')
  })

  test('maps priority values to compact display labels', () => {
    expect(ticketPriorityTranslationKey('normal')).toBe('Normal Priority')
    expect(ticketPriorityTranslationKey('high')).toBe('High Priority')
    expect(ticketPriorityTranslationKey('urgent')).toBe('Urgent')
  })
})
