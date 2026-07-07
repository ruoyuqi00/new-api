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
import { isAxiosError } from 'axios'
import { Copy, KeyRound, Link2, Loader2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Card, CardContent } from '@/components/ui/card'
import { generateQuotaSyncCode } from '../api'

function getResponseMessage(data: unknown) {
  if (!data || typeof data !== 'object' || !('message' in data)) return ''
  const message = (data as { message?: unknown }).message
  return typeof message === 'string' ? message : ''
}

function getQuotaSyncRequestError(error: unknown, fallback: string) {
  if (isAxiosError(error)) {
    const status = error.response?.status
    const message = getResponseMessage(error.response?.data)
    if (status) {
      return message || `Request failed (${status})`
    }
    return error.message || fallback
  }
  if (error instanceof Error) {
    return error.message || fallback
  }
  return fallback
}

export function QuotaSyncCodeCard() {
  const { t } = useTranslation()
  const [code, setCode] = useState('')
  const [expiresAt, setExpiresAt] = useState(0)
  const [loading, setLoading] = useState(false)

  const handleGenerate = async () => {
    setLoading(true)
    try {
      const res = await generateQuotaSyncCode()
      if (res.success && res.data?.code) {
        setCode(res.data.code)
        setExpiresAt(res.data.expires_time)
        toast.success(t('Quota sync code generated'))
      } else {
        toast.error(res.message || t('Request failed'))
      }
    } catch (error) {
      toast.error(getQuotaSyncRequestError(error, t('Request failed')))
    } finally {
      setLoading(false)
    }
  }

  const handleCopy = async () => {
    if (!code) return
    await navigator.clipboard.writeText(code)
    toast.success(t('Copied'))
  }

  return (
    <Card className='overflow-hidden rounded-lg border'>
      <CardContent className='grid gap-3 p-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-center sm:p-5'>
        <div className='min-w-0'>
          <div className='flex items-center gap-2 font-semibold'>
            <span className='bg-primary/10 text-primary flex h-8 w-8 shrink-0 items-center justify-center rounded-md'>
              <Link2 className='h-4 w-4' />
            </span>
            {t('YUAPI to UAG image site quota sync')}
          </div>
          <p className='text-muted-foreground mt-1 text-xs leading-relaxed'>
            {t('Generate a short-lived code for the UAG image site. Paste it into UAG to bind quota only; accounts stay separate. UAG usage is billed locally first, then debited from YUAPI by the sync job.')}
          </p>
          {code && (
            <div className='mt-3 rounded-md border bg-muted/30 p-3'>
              <div className='flex items-center gap-2'>
                <KeyRound className='text-muted-foreground h-4 w-4 shrink-0' />
                <code className='min-w-0 flex-1 break-all font-mono text-sm'>
                  {code}
                </code>
                <Button
                  variant='outline'
                  size='icon'
                  className='h-8 w-8 shrink-0'
                  onClick={handleCopy}
                >
                  <Copy className='h-4 w-4' />
                </Button>
              </div>
              <p className='text-muted-foreground mt-2 text-xs'>
                {t('Expires at')}:{' '}
                {expiresAt
                  ? new Date(expiresAt * 1000).toLocaleString()
                  : t('Unknown')}
              </p>
            </div>
          )}
        </div>
        <Button
          variant='outline'
          className='h-10 gap-2'
          onClick={handleGenerate}
          disabled={loading}
        >
          {loading ? (
            <Loader2 className='h-4 w-4 animate-spin' />
          ) : (
            <KeyRound className='h-4 w-4' />
          )}
          {t('Generate sync code')}
        </Button>
      </CardContent>
    </Card>
  )
}
