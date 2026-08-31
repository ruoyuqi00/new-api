import { api } from '@/lib/http-client'

import type {
  TicketCreatePayload,
  TicketDetail,
  TicketPage,
  TicketPriority,
  TicketSummary,
  TicketStatus,
} from './types'

export interface TicketListParams {
  page?: number
  page_size?: number
  status?: TicketStatus
  category?: 'general' | 'refund'
  priority?: TicketPriority
  keyword?: string
}

export function buildTicketListPath(
  isAdmin: boolean,
  params: TicketListParams = {}
): string {
  const path = isAdmin ? '/api/admin/tickets' : '/api/tickets'
  const query = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== '') query.set(key, String(value))
  }
  const encoded = query.toString()
  return encoded ? `${path}?${encoded}` : path
}

export function getTicketAttachmentPath(
  ticketId: number,
  attachmentId: number,
  isAdmin: boolean
): string {
  const prefix = isAdmin ? '/api/admin/tickets' : '/api/tickets'
  return `${prefix}/${ticketId}/attachments/${attachmentId}`
}

export async function listTickets(
  isAdmin: boolean,
  params: TicketListParams = {}
): Promise<{ success: boolean; data?: TicketPage; message?: string }> {
  const response = await api.get(buildTicketListPath(isAdmin, params), {
    skipBusinessError: true,
    skipErrorHandler: true,
  })
  return response.data
}

export async function getTicket(
  ticketId: number,
  isAdmin = false
): Promise<{ success: boolean; data?: TicketDetail; message?: string }> {
  const prefix = isAdmin ? '/api/admin/tickets' : '/api/tickets'
  const response = await api.get(`${prefix}/${ticketId}`, {
    skipBusinessError: true,
    skipErrorHandler: true,
  })
  return response.data
}

export async function createTicket(
  payload: TicketCreatePayload
): Promise<{
  success: boolean
  data?: { ticket: TicketSummary; message_id: number }
  message?: string
}> {
  const response = await api.post('/api/tickets', payload)
  return response.data
}

export async function replyTicket(
  ticketId: number,
  body: string,
  isAdmin = false
): Promise<{
  success: boolean
  data?: { id: number }
  message?: string
}> {
  const prefix = isAdmin ? '/api/admin/tickets' : '/api/tickets'
  const response = await api.post(`${prefix}/${ticketId}/messages`, { body })
  return response.data
}

export async function updateTicket(
  ticketId: number,
  payload: { status?: TicketStatus; priority?: TicketPriority }
): Promise<{ success: boolean; data?: unknown; message?: string }> {
  const response = await api.patch(`/api/admin/tickets/${ticketId}`, payload)
  return response.data
}

export async function uploadTicketAttachment(
  ticketId: number,
  messageId: number,
  file: File,
  isAdmin = false
): Promise<{ success: boolean; data?: unknown; message?: string }> {
  const prefix = isAdmin ? '/api/admin/tickets' : '/api/tickets'
  const form = new FormData()
  form.append('message_id', String(messageId))
  form.append('file', file)
  const response = await api.post(`${prefix}/${ticketId}/attachments`, form)
  return response.data
}
