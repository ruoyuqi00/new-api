import { describe, expect, test } from 'bun:test'

import {
  buildTicketListPath,
  getTicketAttachmentPath,
} from '../src/features/tickets/api'

describe('ticket API paths', () => {
  test('builds user and admin list paths with stable filters', () => {
    expect(buildTicketListPath(false, { page: 2, page_size: 25, status: 'open' })).toBe(
      '/api/tickets?page=2&page_size=25&status=open'
    )
    expect(buildTicketListPath(true, { category: 'refund' })).toBe(
      '/api/admin/tickets?category=refund'
    )
  })

  test('builds ownership-scoped attachment paths', () => {
    expect(getTicketAttachmentPath(12, 4, false)).toBe(
      '/api/tickets/12/attachments/4'
    )
    expect(getTicketAttachmentPath(12, 4, true)).toBe(
      '/api/admin/tickets/12/attachments/4'
    )
  })
})
