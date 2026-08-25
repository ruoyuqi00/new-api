import { Activity, RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Skeleton } from '@/components/ui/skeleton'
import { cn } from '@/lib/utils'
import {
  getAvailabilityCardStatus,
} from './group-availability-monitor-utils'
import type { GroupAvailabilityItem } from '../types'

type GroupAvailabilityMonitorProps = {
  items: GroupAvailabilityItem[]
  isLoading: boolean
  isFetching: boolean
  onRefresh: () => void
}

const statusClassName = {
  stable: 'border-emerald-500/30 bg-emerald-500/5 text-emerald-500',
  degraded: 'border-amber-500/30 bg-amber-500/5 text-amber-500',
  unavailable: 'border-red-500/30 bg-red-500/5 text-red-500',
  no_data: 'border-muted-foreground/20 bg-muted/30 text-muted-foreground',
}

const statusLabelKey = {
  stable: 'Stable',
  degraded: 'Degraded',
  unavailable: 'Unavailable',
  no_data: 'No data',
} as const

export function GroupAvailabilityMonitor(props: GroupAvailabilityMonitorProps) {
  const { t } = useTranslation()
  if (props.isLoading) {
    return (
      <div className='grid gap-3 md:grid-cols-2 xl:grid-cols-3'>
        {[0, 1, 2].map((item) => (
          <Skeleton key={item} className='h-36 rounded-xl' />
        ))}
      </div>
    )
  }
  if (props.items.length === 0) return null

  return (
    <section aria-labelledby='group-availability-title' className='space-y-3'>
      <div className='flex items-center justify-between gap-3'>
        <div className='min-w-0'>
          <div className='flex items-center gap-2'>
            <Activity className='text-primary size-4' aria-hidden='true' />
            <h2 id='group-availability-title' className='text-sm font-semibold'>
              {t('Group availability')}
            </h2>
          </div>
          <p className='text-muted-foreground mt-1 text-xs'>
            {t('Recent request success only')}
          </p>
        </div>
        <Button
          type='button'
          variant='ghost'
          size='icon'
          aria-label={t('Refresh')}
          title={t('Refresh')}
          onClick={props.onRefresh}
          disabled={props.isFetching}
        >
          <RefreshCw className={cn('size-4', props.isFetching && 'animate-spin')} />
        </Button>
      </div>

      <div className='grid gap-3 md:grid-cols-2 xl:grid-cols-3'>
        {props.items.map((item) => {
          const status = getAvailabilityCardStatus(item)
          return (
            <Card key={item.group} size='sm' className={cn('border', statusClassName[status])}>
              <CardHeader className='gap-2'>
                <div className='flex items-start justify-between gap-3'>
                  <div className='min-w-0'>
                    <CardTitle className='truncate text-sm'>{item.group}</CardTitle>
                    {item.description && (
                      <CardDescription className='truncate text-xs'>
                        {item.description}
                      </CardDescription>
                    )}
                  </div>
                  <span className='shrink-0 text-xs font-medium'>
                    {t(statusLabelKey[status])}
                  </span>
                </div>
              </CardHeader>
              <CardContent className='grid grid-cols-2 gap-3'>
                <div>
                  <div className='text-muted-foreground text-xs'>{t('Requests')}</div>
                  <div className='mt-1 text-lg font-semibold tabular-nums'>
                    {item.request_count}
                  </div>
                </div>
                <div>
                  <div className='text-muted-foreground text-xs'>{t('Success rate')}</div>
                  <div className='mt-1 text-lg font-semibold tabular-nums'>
                    {item.request_count > 0 ? `${item.success_rate.toFixed(2)}%` : '—'}
                  </div>
                </div>
              </CardContent>
            </Card>
          )
        })}
      </div>
    </section>
  )
}
