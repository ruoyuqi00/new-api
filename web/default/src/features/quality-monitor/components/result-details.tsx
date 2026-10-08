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
import { Copy01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { copyToClipboard } from '@/lib/copy-to-clipboard'
import { cn } from '@/lib/utils'

import type { QualityMonitorResult } from '../types'

const statusLabels = {
  passed: 'Passed',
  failed: 'Wrong answer',
  ungraded: 'Needs review',
  error: 'Call failed',
  skipped: 'Skipped',
} as const

const statusStyles = {
  passed: 'border-success/30 bg-success/10 text-success',
  failed: 'border-warning/30 bg-warning/10 text-warning',
  ungraded: 'border-info/30 bg-info/10 text-info',
  error: 'border-destructive/30 bg-destructive/10 text-destructive',
  skipped: 'border-muted-foreground/25 text-muted-foreground',
}

export function ResultStatus(props: {
  status: QualityMonitorResult['status']
}) {
  const { t } = useTranslation()
  return (
    <Badge
      variant='outline'
      className={cn('shrink-0', statusStyles[props.status])}
    >
      {t(statusLabels[props.status])}
    </Badge>
  )
}

export function ResultDetails(props: {
  result: QualityMonitorResult
  isAdmin: boolean
}) {
  const { t } = useTranslation()
  const result = props.result
  return (
    <div className='bg-muted/25 grid min-w-0 gap-4 rounded-xl border p-4'>
      <div className='flex flex-wrap items-center justify-between gap-2'>
        <span className='text-muted-foreground text-xs'>
          {t('Reasoning effort')}: {result.reasoning_effort}
        </span>
        <Button
          type='button'
          variant='outline'
          size='sm'
          disabled={!result.answer}
          onClick={async () => {
            const copied = await copyToClipboard(result.answer)
            if (copied) toast.success(t('Copied'))
            else toast.error(t('Failed to copy'))
          }}
        >
          <HugeiconsIcon
            icon={Copy01Icon}
            data-icon='inline-start'
            aria-hidden='true'
          />
          {t('Copy answer')}
        </Button>
      </div>
      <dl className='grid min-w-0 gap-4'>
        <div>
          <dt className='text-muted-foreground mb-1.5 text-xs font-medium'>
            {t('Question')}
          </dt>
          <dd className='[overflow-wrap:anywhere] break-words whitespace-pre-wrap'>
            {result.prompt}
          </dd>
        </div>
        <div>
          <dt className='text-muted-foreground mb-1.5 text-xs font-medium'>
            {t('Reference answer')}
          </dt>
          <dd className='[overflow-wrap:anywhere] break-words whitespace-pre-wrap'>
            {result.expected_answer || t('Not provided')}
          </dd>
        </div>
        <div>
          <dt className='text-muted-foreground mb-1.5 text-xs font-medium'>
            {t('Actual answer')}
          </dt>
          <dd className='bg-background/50 max-h-96 overflow-y-auto rounded-lg border p-3 font-mono text-xs leading-relaxed [overflow-wrap:anywhere] break-words whitespace-pre-wrap'>
            {result.answer || t('No answer returned')}
          </dd>
        </div>
      </dl>
      {result.answer_truncated && (
        <p className='text-warning text-xs'>
          {t('Answer truncated at the storage limit')}
        </p>
      )}
      {props.isAdmin && (
        <dl className='text-muted-foreground grid gap-2 text-xs sm:grid-cols-3'>
          <div>
            <dt>{t('Channel ID')}</dt>
            <dd className='text-foreground'>{result.channel_id ?? '—'}</dd>
          </div>
          <div>
            <dt>{t('Reported model')}</dt>
            <dd className='text-foreground break-all'>
              {result.actual_response_model || '—'}
            </dd>
          </div>
          <div>
            <dt>{t('Estimated quota')}</dt>
            <dd className='text-foreground'>{result.estimated_quota ?? '—'}</dd>
          </div>
          {result.error && (
            <div className='sm:col-span-3'>
              <dt>{t('Internal error')}</dt>
              <dd className='text-foreground mt-1 [overflow-wrap:anywhere] break-words whitespace-pre-wrap'>
                {result.error}
              </dd>
            </div>
          )}
        </dl>
      )}
    </div>
  )
}
