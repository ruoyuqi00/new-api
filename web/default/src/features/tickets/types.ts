export type TicketCategory = 'general' | 'refund'
export type TicketStatus = 'open' | 'pending_user' | 'pending_admin' | 'closed'
export type TicketPriority = 'normal' | 'high' | 'urgent'

export interface TicketSummary {
  id: number
  user_id?: number
  subject: string
  category: TicketCategory
  status: TicketStatus
  priority: TicketPriority
  message_count: number
  unread_for_user?: number
  unread_for_admin?: number
  created_at: number
  updated_at: number
  last_message_at: number
}

export interface TicketAttachment {
  id: number
  display_name: string
  mime_type: string
  size: number
  created_at: number
}

export interface TicketMessage {
  id: number
  author_role: 'user' | 'admin'
  body: string
  created_at: number
  attachments?: TicketAttachment[]
}

export interface TicketDetail {
  ticket: TicketSummary
  messages: TicketMessage[]
  attachments?: TicketAttachment[]
}

export interface TicketPage {
  items: TicketSummary[]
  total: number
  page: number
  page_size: number
}

export interface TicketCreatePayload {
  subject: string
  category: TicketCategory
  priority: TicketPriority
  body: string
}
