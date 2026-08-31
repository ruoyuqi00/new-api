import type { TicketPriority, TicketStatus } from './types'

export function ticketStatusTranslationKey(status: TicketStatus): string {
  switch (status) {
    case 'pending_user':
      return 'Waiting for User'
    case 'pending_admin':
      return 'Waiting for Support'
    case 'closed':
      return 'Closed'
    default:
      return 'Open'
  }
}

export function ticketPriorityTranslationKey(priority: TicketPriority): string {
  switch (priority) {
    case 'urgent':
      return 'Urgent'
    case 'high':
      return 'High Priority'
    default:
      return 'Normal Priority'
  }
}
