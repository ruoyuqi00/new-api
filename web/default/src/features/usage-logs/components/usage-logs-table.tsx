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
import type { ColumnDef } from '@tanstack/react-table'
import { RefreshCw, TriangleAlert } from 'lucide-react'
import { useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'

import {
  DataTablePage,
  DataTableRow,
  useDataTable,
} from '@/components/data-table'
import {
  Alert,
  AlertAction,
  AlertDescription,
  AlertTitle,
} from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { useMediaQuery } from '@/hooks'
import { useIsAdmin } from '@/hooks/use-admin'
import { useTableUrlState } from '@/hooks/use-table-url-state'
import {
  getQueryDisplayState,
  getRetainedQueryData,
} from '@/lib/query-display-state'
import { KEEP_CURRENT_PAGE_ON_QUERY_ERROR } from '@/lib/query-error-policy'
import { cn } from '@/lib/utils'

import {
  DEFAULT_LOGS_DATA,
  LOG_TYPE_ALL_VALUE,
  LOG_TYPE_ENUM,
} from '../constants'
import { useColumnsByCategory } from '../lib/columns'
import { fetchLogsByCategory } from '../lib/utils'
import type { LogCategory } from '../types'
import { CommonLogsFilterBar } from './common-logs-filter-bar'
import { TaskLogsFilterBar } from './task-logs-filter-bar'
import { UsageLogsMobileList } from './usage-logs-mobile-card'

const route = getRouteApi('/_authenticated/usage-logs/$section')

const logTypeRowTint: Record<number, string> = {
  [LOG_TYPE_ENUM.ERROR]: 'bg-rose-50/40 dark:bg-rose-950/20',
  [LOG_TYPE_ENUM.REFUND]: 'bg-blue-50/30 dark:bg-blue-950/15',
}

function getColumnVisibilityStorageKey(
  logCategory: LogCategory,
  isAdmin: boolean
): string {
  return `usage-logs:${logCategory}:${isAdmin ? 'admin' : 'user'}:column-visibility`
}

function deserializeLogTypeFilter(value: unknown): unknown[] {
  let values: unknown[] = []
  if (Array.isArray(value)) {
    values = value
  } else if (value) {
    values = [value]
  }
  return values.filter((item) => String(item) !== LOG_TYPE_ALL_VALUE)
}

interface UsageLogsTableProps {
  logCategory: LogCategory
}

export function UsageLogsTable({ logCategory }: UsageLogsTableProps) {
  const { t } = useTranslation()
  const isAdmin = useIsAdmin()
  const isMobile = useMediaQuery('(max-width: 640px)')
  const searchParams = route.useSearch()

  const {
    columnFilters,
    onColumnFiltersChange,
    pagination,
    onPaginationChange,
    ensurePageInRange,
  } = useTableUrlState({
    search: route.useSearch(),
    navigate: route.useNavigate(),
    pagination: { defaultPage: 1, defaultPageSize: isMobile ? 20 : 30 },
    globalFilter: { enabled: false },
    columnFilters: [
      {
        columnId: 'created_at',
        searchKey: 'type',
        type: 'array' as const,
        deserialize: deserializeLogTypeFilter,
      },
      { columnId: 'model_name', searchKey: 'model', type: 'string' as const },
      { columnId: 'token_name', searchKey: 'token', type: 'string' as const },
      { columnId: 'group', searchKey: 'group', type: 'string' as const },
      ...(isAdmin
        ? [
            {
              columnId: 'channel',
              searchKey: 'channel',
              type: 'string' as const,
            },
            {
              columnId: 'username',
              searchKey: 'username',
              type: 'string' as const,
            },
          ]
        : []),
    ],
  })

  const { data, isPending, isFetching, isError, refetch } = useQuery({
    queryKey: [
      'logs',
      logCategory,
      isAdmin,
      pagination.pageIndex + 1,
      pagination.pageSize,
      columnFilters,
      searchParams,
      t,
    ],
    queryFn: async () => {
      const result = await fetchLogsByCategory({
        logCategory,
        isAdmin,
        page: pagination.pageIndex + 1,
        pageSize: pagination.pageSize,
        searchParams,
        columnFilters,
      })

      if (!result?.success) {
        throw new Error(result?.message || t('Failed to load logs'))
      }

      return result.data || DEFAULT_LOGS_DATA
    },
    placeholderData: (previousData, previousQuery) => {
      if (previousQuery?.queryKey[1] === logCategory) {
        return previousData
      }
      return undefined
    },
    meta: KEEP_CURRENT_PAGE_ON_QUERY_ERROR,
  })

  const logsDataScope = `${logCategory}:${isAdmin ? 'admin' : 'user'}`
  const retainedLogsData = useRef<{
    scope: string
    data: NonNullable<typeof data>
  }>(undefined)
  useEffect(() => {
    if (data !== undefined) {
      retainedLogsData.current = { scope: logsDataScope, data }
    }
  }, [data, logsDataScope])
  const displayData = getRetainedQueryData({
    data,
    scope: logsDataScope,
    retainedData: retainedLogsData.current,
  })

  const logs = Array.isArray(displayData?.items) ? displayData.items : []
  const columns = useColumnsByCategory(logCategory, isAdmin)
  const logsDisplayState = getQueryDisplayState({
    hasData: displayData !== undefined,
    isPending,
    isFetching,
    isError,
  })
  const isLoadingData = logsDisplayState === 'initial-loading'
  const showLogsError =
    logsDisplayState === 'stale-error' || logsDisplayState === 'terminal-error'

  const { table } = useDataTable({
    data: logs as Record<string, unknown>[],
    columns: columns as ColumnDef<Record<string, unknown>>[],
    columnFilters,
    columnVisibilityStorageKey: getColumnVisibilityStorageKey(
      logCategory,
      isAdmin
    ),
    pagination,
    enableRowSelection: false,
    onPaginationChange,
    onColumnFiltersChange,
    manualPagination: true,
    manualFiltering: true,
    totalCount: displayData?.total || 0,
    ensurePageInRange,
  })

  const isCommon = logCategory === 'common'

  return (
    <div className='flex h-full min-h-0 flex-col gap-2'>
      {showLogsError && (
        <Alert variant='destructive' className='bg-destructive/8 shrink-0'>
          <TriangleAlert aria-hidden='true' />
          <AlertTitle>{t('Failed to load logs')}</AlertTitle>
          {logsDisplayState === 'stale-error' && (
            <AlertDescription>
              {t('Showing the last available data.')}
            </AlertDescription>
          )}
          <AlertAction>
            <Button variant='outline' size='sm' onClick={() => void refetch()}>
              <RefreshCw data-icon='inline-start' />
              {t('Retry')}
            </Button>
          </AlertAction>
        </Alert>
      )}

      <DataTablePage
        className='min-h-0 flex-1'
        table={table}
        columns={columns as ColumnDef<Record<string, unknown>>[]}
        isLoading={isLoadingData}
        isFetching={isFetching}
        emptyTitle={t('No Logs Found')}
        emptyDescription={t(
          'No usage logs available. Logs will appear here once API calls are made.'
        )}
        skeletonKeyPrefix='usage-log-skeleton'
        applyHeaderSize
        tableClassName={cn(
          '[&_[data-slot=table]]:text-[13px] [&_[data-slot=table]_td]:text-[13px] [&_[data-slot=table]_td_*]:text-[13px] [&_[data-slot=table]_th]:text-[13px] [&_[data-slot=table]_th_*]:text-[13px]'
        )}
        mobile={
          <UsageLogsMobileList
            table={table}
            isLoading={isLoadingData}
            logCategory={logCategory}
          />
        }
        toolbar={
          isCommon ? (
            <CommonLogsFilterBar table={table} />
          ) : (
            <TaskLogsFilterBar table={table} logCategory={logCategory} />
          )
        }
        renderRow={(row) => {
          const logType = (row.original as Record<string, unknown>).type as
            | number
            | undefined
          const tintClass =
            isCommon && logType != null ? (logTypeRowTint[logType] ?? '') : ''

          return (
            <DataTableRow
              key={row.id}
              row={row}
              className={cn('transition-colors', tintClass)}
              getColumnClassName={() => (isCommon ? 'py-2' : 'py-3.5')}
            />
          )
        }}
      />
    </div>
  )
}
