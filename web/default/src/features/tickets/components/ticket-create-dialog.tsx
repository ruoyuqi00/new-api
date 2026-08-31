import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Paperclip, Plus } from 'lucide-react'
import { useState, type FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'

import { createTicket, uploadTicketAttachment } from '../api'
import type { TicketCategory, TicketPriority } from '../types'

export function TicketCreateDialog() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [open, setOpen] = useState(false)
  const [category, setCategory] = useState<TicketCategory>('general')
  const [priority, setPriority] = useState<TicketPriority>('normal')
  const [subject, setSubject] = useState('')
  const [body, setBody] = useState('')
  const [files, setFiles] = useState<File[]>([])

  const createMutation = useMutation({
    mutationFn: async () => {
      const result = await createTicket({ subject, category, priority, body })
      if (!result.success || !result.data) {
        throw new Error(result.message || t('Ticket could not be created'))
      }
      for (const file of files.slice(0, 5)) {
        const upload = await uploadTicketAttachment(
          result.data.ticket.id,
          result.data.message_id,
          file
        )
        if (!upload.success) {
          throw new Error(upload.message || t('Attachment upload failed'))
        }
      }
      return result.data.ticket
    },
    onSuccess: () => {
      toast.success(t('Ticket created'))
      setOpen(false)
      setSubject('')
      setBody('')
      setFiles([])
      void queryClient.invalidateQueries({ queryKey: ['tickets'] })
    },
    onError: (error) => toast.error(error.message),
  })

  const submit = (event: FormEvent) => {
    event.preventDefault()
    if (!subject.trim() || !body.trim()) {
      toast.error(t('Subject and description are required'))
      return
    }
    createMutation.mutate()
  }

  return (
    <>
      <Button onClick={() => setOpen(true)}>
        <Plus data-icon='inline-start' />
        {t('New Ticket')}
      </Button>
      <Dialog open={open} onOpenChange={setOpen}>
        <DialogContent className='yucore-app-shell max-h-[calc(100dvh-2rem)] overflow-y-auto sm:max-w-2xl'>
          <form onSubmit={submit}>
            <DialogHeader>
              <DialogTitle>{t('New Ticket')}</DialogTitle>
              <DialogDescription>
                {t(
                  'Describe an issue or request a manual refund review. Refunds are handled offline by an administrator.'
                )}
              </DialogDescription>
            </DialogHeader>
            <div className='grid gap-4 py-5'>
              <div className='grid grid-cols-2 gap-2'>
                <Button
                  type='button'
                  variant={category === 'general' ? 'default' : 'outline'}
                  onClick={() => setCategory('general')}
                >
                  {t('General')}
                </Button>
                <Button
                  type='button'
                  variant={category === 'refund' ? 'default' : 'outline'}
                  onClick={() => setCategory('refund')}
                >
                  {t('Manual Refund')}
                </Button>
              </div>
              <div className='grid gap-2'>
                <Label htmlFor='ticket-subject'>{t('Subject')}</Label>
                <Input
                  id='ticket-subject'
                  value={subject}
                  maxLength={255}
                  onChange={(event) => setSubject(event.target.value)}
                  placeholder={t('Summarize the issue in one sentence')}
                />
              </div>
              <div className='grid gap-2'>
                <Label htmlFor='ticket-priority'>{t('Priority')}</Label>
                <select
                  id='ticket-priority'
                  className='border-input bg-background h-8 w-full rounded-lg border px-2.5 text-sm'
                  value={priority}
                  onChange={(event) =>
                    setPriority(event.target.value as TicketPriority)
                  }
                >
                  <option value='normal'>{t('Normal Priority')}</option>
                  <option value='high'>{t('High Priority')}</option>
                  <option value='urgent'>{t('Urgent')}</option>
                </select>
              </div>
              <div className='grid gap-2'>
                <Label htmlFor='ticket-body'>{t('Description')}</Label>
                <Textarea
                  id='ticket-body'
                  value={body}
                  maxLength={32768}
                  onChange={(event) => setBody(event.target.value)}
                  placeholder={t('Describe the problem in detail')}
                  className='min-h-28'
                />
              </div>
              <div className='grid gap-2'>
                <Label htmlFor='ticket-files'>{t('Attachments')}</Label>
                <label
                  htmlFor='ticket-files'
                  className='border-input hover:bg-muted flex min-h-16 cursor-pointer items-center gap-3 rounded-lg border border-dashed px-4 py-3 text-sm'
                >
                  <Paperclip className='size-4' />
                  <span>
                    {files.length
                      ? t('{{count}} files selected', { count: files.length })
                      : t('Up to 5 files, 50 MB each')}
                  </span>
                </label>
                <Input
                  id='ticket-files'
                  type='file'
                  multiple
                  className='sr-only'
                  onChange={(event) =>
                    setFiles([...(event.target.files ?? [])].slice(0, 5))
                  }
                />
              </div>
            </div>
            <DialogFooter>
              <Button
                type='button'
                variant='outline'
                onClick={() => setOpen(false)}
              >
                {t('Cancel')}
              </Button>
              <Button type='submit' disabled={createMutation.isPending}>
                {createMutation.isPending ? t('Submitting...') : t('Submit')}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </>
  )
}
