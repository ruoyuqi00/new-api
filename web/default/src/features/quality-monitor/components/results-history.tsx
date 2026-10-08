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
import {
  ArrowDown01Icon,
  ArrowLeft01Icon,
  ArrowRight01Icon,
  Clock01Icon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from '@/components/ui/empty'
import { cn } from '@/lib/utils'

import type { QualityMonitorResult } from '../types'
import { ResultDetails, ResultStatus } from './result-details'

type ResultsHistoryProps = {
  items: QualityMonitorResult[]
  total: number
  page: number
  pageSize: number
  isAdmin: boolean
  isFetching: boolean
  onPageChange: (page: number) => void
}

export function ResultsHistory(props: ResultsHistoryProps) {
  const { t } = useTranslation()
  const [opened, setOpened] = useState<number | null>(null)
  const pages = Math.max(1, Math.ceil(props.total / props.pageSize))
  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('Answer history')}</CardTitle>
        <CardDescription>
          {t(
            'Results reflect each question, not overall model identity or capability. History is kept for 30 days.'
          )}
        </CardDescription>
      </CardHeader>
      <CardContent className='flex flex-col gap-3'>
        {props.items.length === 0 && (
          <Empty>
            <EmptyHeader>
              <EmptyTitle>{t('No probe results yet')}</EmptyTitle>
              <EmptyDescription>
                {t(
                  'Try another group or model, or check again after a probe completes.'
                )}
              </EmptyDescription>
            </EmptyHeader>
          </Empty>
        )}
        {props.items.map((result) => (
          <article key={result.id} className='min-w-0 rounded-xl border'>
            <button
              type='button'
              className='hover:bg-muted/30 focus-visible:ring-ring w-full rounded-xl p-4 text-left transition-colors outline-none focus-visible:ring-2'
              aria-expanded={opened === result.id}
              aria-controls={`result-${result.id}`}
              onClick={() => setOpened(opened === result.id ? null : result.id)}
            >
              <div className='flex flex-wrap items-center gap-2'>
                <span className='text-primary min-w-0 font-semibold break-all'>
                  {result.group}
                </span>
                <span className='text-muted-foreground font-mono text-xs'>
                  {result.model}
                </span>
                <span className='ml-auto'>
                  <ResultStatus status={result.status} />
                </span>
                <HugeiconsIcon
                  icon={ArrowDown01Icon}
                  aria-hidden='true'
                  className={cn(
                    'size-4 shrink-0 transition-transform',
                    opened === result.id && 'rotate-180'
                  )}
                />
              </div>
              <p className='mt-2 line-clamp-2 text-sm [overflow-wrap:anywhere] break-words'>
                {result.prompt}
              </p>
              <div className='text-muted-foreground mt-2 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs'>
                <span>{result.plan_name}</span>
                <time
                  dateTime={new Date(result.created_at * 1000).toISOString()}
                >
                  {new Date(result.created_at * 1000).toLocaleString()}
                </time>
                <span className='inline-flex items-center gap-1'>
                  <HugeiconsIcon
                    icon={Clock01Icon}
                    className='size-3'
                    aria-hidden='true'
                  />
                  {t('{{duration}} s', {
                    duration: (result.duration_ms / 1000).toFixed(2),
                  })}
                </span>
              </div>
            </button>
            {opened === result.id && (
              <div id={`result-${result.id}`} className='px-3 pb-3'>
                <ResultDetails result={result} isAdmin={props.isAdmin} />
              </div>
            )}
          </article>
        ))}
        <div className='flex flex-wrap items-center justify-between gap-3 pt-2'>
          <p className='text-muted-foreground text-xs'>
            {t('Page {{page}} of {{pages}} · {{count}} results', {
              page: props.page,
              pages,
              count: props.total,
            })}
          </p>
          <div className='flex gap-2'>
            <Button
              type='button'
              variant='outline'
              size='sm'
              disabled={props.page <= 1 || props.isFetching}
              onClick={() => props.onPageChange(props.page - 1)}
            >
              <HugeiconsIcon
                icon={ArrowLeft01Icon}
                data-icon='inline-start'
                aria-hidden='true'
              />
              {t('Previous')}
            </Button>
            <Button
              type='button'
              variant='outline'
              size='sm'
              disabled={props.page >= pages || props.isFetching}
              onClick={() => props.onPageChange(props.page + 1)}
            >
              {t('Next')}
              <HugeiconsIcon
                icon={ArrowRight01Icon}
                data-icon='inline-end'
                aria-hidden='true'
              />
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}
