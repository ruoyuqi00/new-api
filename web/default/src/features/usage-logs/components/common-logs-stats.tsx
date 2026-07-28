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
import { useQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { RefreshCw } from 'lucide-react'
import { useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { useIsAdmin } from '@/hooks/use-admin'
import { formatLogQuota } from '@/lib/format'
import {
  getQueryDisplayState,
  getRetainedQueryData,
} from '@/lib/query-display-state'
import { KEEP_CURRENT_PAGE_ON_QUERY_ERROR } from '@/lib/query-error-policy'
import { cn } from '@/lib/utils'

import { getLogStats, getUserLogStats } from '../api'
import { DEFAULT_LOG_STATS } from '../constants'
import { buildApiParams } from '../lib/utils'
import { useUsageLogsContext } from './usage-logs-provider'

const route = getRouteApi('/_authenticated/usage-logs/$section')

function StatBadge(props: {
  label: string
  value: string | number
  accent: string
}) {
  return (
    <span className='border-border/60 bg-muted/25 inline-flex h-7 items-center gap-2 rounded-md border px-2.5 text-xs shadow-xs'>
      <span className={cn('h-3.5 w-0.5 rounded-full', props.accent)} />
      <span className='text-muted-foreground'>{props.label}</span>
      <span className='text-foreground/85 font-mono font-semibold tabular-nums'>
        {props.value}
      </span>
    </span>
  )
}

export function CommonLogsStats() {
  const { t } = useTranslation()
  const isAdmin = useIsAdmin()
  const searchParams = route.useSearch()
  const { sensitiveVisible } = useUsageLogsContext()

  const {
    data: stats,
    isPending,
    isFetching,
    isError,
    refetch,
  } = useQuery({
    queryKey: ['usage-logs-stats', isAdmin, searchParams],
    queryFn: async () => {
      const params = buildApiParams({
        page: 1,
        pageSize: 1,
        searchParams,
        columnFilters: [],
        isAdmin,
      })

      const result = isAdmin
        ? await getLogStats(params)
        : await getUserLogStats(params)

      if (!result.success) {
        throw new Error(result.message || t('Failed to load logs'))
      }
      return result.data || DEFAULT_LOG_STATS
    },
    placeholderData: (previousData) => previousData,
    meta: KEEP_CURRENT_PAGE_ON_QUERY_ERROR,
  })

  const statsDataScope = isAdmin ? 'admin' : 'user'
  const retainedStats = useRef<{
    scope: string
    data: NonNullable<typeof stats>
  }>(undefined)
  useEffect(() => {
    if (stats !== undefined) {
      retainedStats.current = { scope: statsDataScope, data: stats }
    }
  }, [stats, statsDataScope])
  const displayStats = getRetainedQueryData({
    data: stats,
    scope: statsDataScope,
    retainedData: retainedStats.current,
  })

  const statsDisplayState = getQueryDisplayState({
    hasData: displayStats !== undefined,
    isPending,
    isFetching,
    isError,
  })

  if (statsDisplayState === 'initial-loading') {
    return (
      <div className='flex items-center gap-2'>
        <Skeleton className='h-7 w-[150px] rounded-md' />
        <Skeleton className='h-7 w-[100px] rounded-md' />
        <Skeleton className='h-7 w-[120px] rounded-md' />
      </div>
    )
  }

  if (statsDisplayState === 'terminal-error') {
    return (
      <Button variant='outline' size='sm' onClick={() => void refetch()}>
        <RefreshCw data-icon='inline-start' />
        {t('Retry')}
      </Button>
    )
  }

  return (
    <div className='flex flex-wrap items-center gap-2'>
      <StatBadge
        label={t('Usage')}
        value={
          sensitiveVisible ? formatLogQuota(displayStats?.quota || 0) : '••••'
        }
        accent='bg-sky-500/70'
      />
      <StatBadge
        label={t('RPM')}
        value={displayStats?.rpm || 0}
        accent='bg-rose-500/65'
      />
      <StatBadge
        label={t('TPM')}
        value={displayStats?.tpm || 0}
        accent='bg-slate-400/70'
      />
      {statsDisplayState === 'stale-error' && (
        <Button
          variant='outline'
          size='sm'
          className='h-7'
          onClick={() => void refetch()}
        >
          <RefreshCw data-icon='inline-start' />
          {t('Retry')}
        </Button>
      )}
    </div>
  )
}
