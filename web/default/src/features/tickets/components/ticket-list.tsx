import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { RefreshCw } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

import { listTickets } from '../api'
import type {
  TicketCategory,
  TicketPriority,
  TicketStatus,
  TicketSummary,
} from '../types'
import {
  ticketPriorityTranslationKey,
  ticketStatusTranslationKey,
} from '../lib'
import { TicketCreateDialog } from './ticket-create-dialog'

function formatTicketTime(timestamp: number) {
  return new Intl.DateTimeFormat(undefined, {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(timestamp * 1000))
}

export function TicketList({ isAdmin = false }: { isAdmin?: boolean }) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const [status, setStatus] = useState<TicketStatus | ''>('')
  const [category, setCategory] = useState<TicketCategory | ''>('')
  const [priority, setPriority] = useState<TicketPriority | ''>('')
  const [keyword, setKeyword] = useState('')
  const query = useQuery({
    queryKey: ['tickets', isAdmin, status, category, priority, keyword],
    queryFn: () =>
      listTickets(isAdmin, {
        page: 1,
        page_size: 50,
        status: status || undefined,
        category: category || undefined,
        priority: priority || undefined,
        keyword: keyword || undefined,
      }),
  })
  const items = query.data?.data?.items ?? []

  const openTicket = (ticket: TicketSummary) => {
    if (isAdmin) {
      void navigate({
        to: '/admin-tickets/$ticketId',
        params: { ticketId: String(ticket.id) },
      })
      return
    }
    void navigate({
      to: '/tickets/$ticketId',
      params: { ticketId: String(ticket.id) },
    })
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>
        {isAdmin ? t('Support Tickets') : t('My Tickets')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        {!isAdmin && <TicketCreateDialog />}
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='flex min-h-0 flex-col gap-4'>
          <div className='border-border bg-card flex flex-wrap items-center gap-2 rounded-lg border p-3'>
            <Input
              className='w-full sm:w-64'
              value={keyword}
              onChange={(event) => setKeyword(event.target.value)}
              placeholder={t('Search tickets')}
            />
            <select
              className='border-input bg-background h-8 rounded-lg border px-2.5 text-sm'
              value={status}
              onChange={(event) =>
                setStatus(event.target.value as TicketStatus | '')
              }
              aria-label={t('Status')}
            >
              <option value=''>{t('All Statuses')}</option>
              <option value='open'>{t('Open')}</option>
              <option value='pending_user'>{t('Waiting for User')}</option>
              <option value='pending_admin'>{t('Waiting for Support')}</option>
              <option value='closed'>{t('Closed')}</option>
            </select>
            <select
              className='border-input bg-background h-8 rounded-lg border px-2.5 text-sm'
              value={category}
              onChange={(event) =>
                setCategory(event.target.value as TicketCategory | '')
              }
              aria-label={t('Category')}
            >
              <option value=''>{t('All Categories')}</option>
              <option value='general'>{t('General')}</option>
              <option value='refund'>{t('Manual Refund')}</option>
            </select>
            <select
              className='border-input bg-background h-8 rounded-lg border px-2.5 text-sm'
              value={priority}
              onChange={(event) =>
                setPriority(event.target.value as TicketPriority | '')
              }
              aria-label={t('Priority')}
            >
              <option value=''>{t('All Priorities')}</option>
              <option value='normal'>{t('Normal Priority')}</option>
              <option value='high'>{t('High Priority')}</option>
              <option value='urgent'>{t('Urgent')}</option>
            </select>
            <Button
              variant='outline'
              size='icon'
              onClick={() => void query.refetch()}
              aria-label={t('Refresh')}
            >
              <RefreshCw />
            </Button>
          </div>

          <div className='border-border bg-card overflow-hidden rounded-lg border'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('ID')}</TableHead>
                  {isAdmin && <TableHead>{t('User ID')}</TableHead>}
                  <TableHead>{t('Subject')}</TableHead>
                  <TableHead>{t('Category')}</TableHead>
                  <TableHead>{t('Status')}</TableHead>
                  <TableHead>{t('Priority')}</TableHead>
                  <TableHead>{t('Updated')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {items.map((ticket) => (
                  <TableRow
                    key={ticket.id}
                    className='cursor-pointer'
                    onClick={() => openTicket(ticket)}
                  >
                    <TableCell>{ticket.id}</TableCell>
                    {isAdmin && <TableCell>{ticket.user_id}</TableCell>}
                    <TableCell className='max-w-[28rem] truncate font-medium'>
                      {ticket.subject}
                    </TableCell>
                    <TableCell>
                      {ticket.category === 'refund'
                        ? t('Manual Refund')
                        : t('General')}
                    </TableCell>
                    <TableCell>
                      <Badge
                        variant={
                          ticket.status === 'closed' ? 'outline' : 'secondary'
                        }
                      >
                        {t(ticketStatusTranslationKey(ticket.status))}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      {t(ticketPriorityTranslationKey(ticket.priority))}
                    </TableCell>
                    <TableCell>{formatTicketTime(ticket.updated_at)}</TableCell>
                  </TableRow>
                ))}
                {!query.isLoading && items.length === 0 && (
                  <TableRow>
                    <TableCell
                      colSpan={isAdmin ? 7 : 6}
                      className='text-muted-foreground h-32 text-center'
                    >
                      {t('No tickets found')}
                    </TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </div>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
