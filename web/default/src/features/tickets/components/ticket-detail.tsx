import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { ArrowLeft, Paperclip, Send } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { cn } from '@/lib/utils'

import {
  getTicket,
  getTicketAttachmentPath,
  replyTicket,
  updateTicket,
  uploadTicketAttachment,
} from '../api'

export function TicketDetail({
  ticketId,
  isAdmin = false,
}: {
  ticketId: number
  isAdmin?: boolean
}) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const [reply, setReply] = useState('')
  const [files, setFiles] = useState<File[]>([])
  const query = useQuery({
    queryKey: ['ticket', ticketId, isAdmin],
    queryFn: () => getTicket(ticketId, isAdmin),
  })
  const detail = query.data?.data
  const ticket = detail?.ticket

  const replyMutation = useMutation({
    mutationFn: async () => {
      const result = await replyTicket(ticketId, reply, isAdmin)
      if (!result.success || !result.data) {
        throw new Error(result.message || t('Reply could not be sent'))
      }
      for (const file of files.slice(0, 5)) {
        const upload = await uploadTicketAttachment(
          ticketId,
          result.data.id,
          file,
          isAdmin
        )
        if (!upload.success) {
          throw new Error(upload.message || t('Attachment upload failed'))
        }
      }
    },
    onSuccess: () => {
      setReply('')
      setFiles([])
      toast.success(t('Reply sent'))
      void queryClient.invalidateQueries({ queryKey: ['ticket', ticketId] })
      void queryClient.invalidateQueries({ queryKey: ['tickets'] })
    },
    onError: (error) => toast.error(error.message),
  })

  const stateMutation = useMutation({
    mutationFn: (status: 'closed' | 'pending_admin') =>
      updateTicket(ticketId, { status }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['ticket', ticketId] })
      void queryClient.invalidateQueries({ queryKey: ['tickets'] })
    },
    onError: (error) => toast.error(error.message),
  })

  const submitReply = (event: FormEvent) => {
    event.preventDefault()
    if (!reply.trim()) return
    replyMutation.mutate()
  }

  const back = () => void navigate({ to: isAdmin ? '/admin-tickets' : '/tickets' })

  if (query.isLoading) {
    return (
      <SectionPageLayout>
        <SectionPageLayout.Content>
          <div className='text-muted-foreground p-6'>{t('Loading...')}</div>
        </SectionPageLayout.Content>
      </SectionPageLayout>
    )
  }

  if (!ticket || !detail) {
    return (
      <SectionPageLayout>
        <SectionPageLayout.Content>
          <div className='p-6'>{t('Ticket not found')}</div>
        </SectionPageLayout.Content>
      </SectionPageLayout>
    )
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{ticket.subject}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button variant='outline' onClick={back}>
          <ArrowLeft data-icon='inline-start' />
          {t('Back')}
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='mx-auto flex w-full max-w-5xl flex-col gap-4'>
          <div className='border-border bg-card flex flex-wrap items-center gap-2 rounded-lg border p-4'>
            <Badge variant='secondary'>
              {ticket.category === 'refund' ? t('Manual Refund') : t('General')}
            </Badge>
            <Badge variant={ticket.status === 'closed' ? 'outline' : 'default'}>
              {t(ticket.status === 'closed' ? 'Closed' : 'In Progress')}
            </Badge>
            <span className='text-muted-foreground ml-auto text-sm'>
              #{ticket.id}
              {isAdmin && ticket.user_id
                ? ` · ${t('User ID')} ${ticket.user_id}`
                : ''}
            </span>
          </div>

          <div className='border-border bg-card space-y-3 rounded-lg border p-4'>
            {detail.messages.map((message) => (
              <div
                key={message.id}
                className={cn(
                  'max-w-[88%] rounded-lg border px-4 py-3',
                  message.author_role === 'admin'
                    ? 'bg-muted mr-auto'
                    : 'border-primary/30 bg-primary/5 ml-auto'
                )}
              >
                <div className='mb-1 flex items-center gap-2 text-xs font-medium'>
                  {message.author_role === 'admin' ? t('Support') : t('User')}
                  <span className='text-muted-foreground'>
                    {new Date(message.created_at * 1000).toLocaleString()}
                  </span>
                </div>
                <p className='whitespace-pre-wrap break-words text-sm'>
                  {message.body}
                </p>
                {message.attachments?.map((attachment) => (
                  <a
                    key={attachment.id}
                    className='text-primary mt-2 flex items-center gap-1 text-xs hover:underline'
                    href={getTicketAttachmentPath(ticket.id, attachment.id, isAdmin)}
                  >
                    <Paperclip className='size-3' />
                    {attachment.display_name}
                  </a>
                ))}
              </div>
            ))}
          </div>

          <form
            onSubmit={submitReply}
            className='border-border bg-card rounded-lg border p-4'
          >
            <div className='mb-3 flex items-center justify-between gap-2'>
              <h2 className='text-base font-semibold'>{t('Reply to Ticket')}</h2>
              {isAdmin && (
                <Button
                  type='button'
                  variant='outline'
                  onClick={() =>
                    stateMutation.mutate(
                      ticket.status === 'closed' ? 'pending_admin' : 'closed'
                    )
                  }
                >
                  {ticket.status === 'closed' ? t('Reopen') : t('Close Ticket')}
                </Button>
              )}
            </div>
            {ticket.status === 'closed' && !isAdmin ? (
              <p className='text-muted-foreground text-sm'>
                {t('This ticket is closed. Contact support to reopen it.')}
              </p>
            ) : (
              <div className='grid gap-3'>
                <Textarea
                  value={reply}
                  onChange={(event) => setReply(event.target.value)}
                  placeholder={t('Write a reply')}
                  className='min-h-28'
                />
                <div className='flex flex-wrap items-center justify-between gap-2'>
                  <label
                    htmlFor='ticket-reply-files'
                    className='border-input hover:bg-muted flex cursor-pointer items-center gap-2 rounded-lg border px-3 py-2 text-sm'
                  >
                    <Paperclip className='size-4' />
                    {files.length
                      ? t('{{count}} files selected', { count: files.length })
                      : t('Attachments')}
                  </label>
                  <Input
                    id='ticket-reply-files'
                    type='file'
                    multiple
                    className='sr-only'
                    onChange={(event) =>
                      setFiles([...(event.target.files ?? [])].slice(0, 5))
                    }
                  />
                  <Button type='submit' disabled={replyMutation.isPending}>
                    <Send data-icon='inline-start' />
                    {replyMutation.isPending ? t('Sending...') : t('Send Reply')}
                  </Button>
                </div>
              </div>
            )}
          </form>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
