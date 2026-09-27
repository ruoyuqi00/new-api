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
import { useInfiniteQuery } from '@tanstack/react-query'
import { RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Spinner } from '@/components/ui/spinner'

import { getTokenPayRecoveryClaims } from '../api'

export function TokenPayRecoveryClaims() {
  const { t } = useTranslation()
  const query = useInfiniteQuery({
    queryKey: ['tokenpay-recovery-claims'],
    initialPageParam: 0,
    queryFn: ({ pageParam, signal }) =>
      getTokenPayRecoveryClaims(pageParam, signal),
    getNextPageParam: (lastPage) => {
      if (!lastPage.success || !lastPage.data || lastPage.data.length < 50) {
        return undefined
      }
      return lastPage.data.at(-1)?.claim_id
    },
    refetchOnWindowFocus: false,
  })
  const pages = query.data?.pages ?? []
  const claims = pages.flatMap((page) => page.data ?? [])
  const responseFailed = pages.some((page) => !page.success)

  return (
    <section className='space-y-3 border-t pt-5'>
      <div className='flex items-center justify-between gap-3'>
        <h4 className='text-sm font-semibold'>{t('Manual payment review')}</h4>
        <Button
          type='button'
          variant='outline'
          size='icon-sm'
          title={t('Refresh')}
          aria-label={t('Refresh')}
          disabled={query.isFetching}
          onClick={() => void query.refetch()}
        >
          <RefreshCw className={query.isFetching ? 'animate-spin' : ''} />
        </Button>
      </div>

      {query.isLoading && (
        <div className='flex items-center gap-2 text-sm'>
          <Spinner />
          {t('Loading...')}
        </div>
      )}
      {(query.isError || responseFailed) && (
        <Alert variant='destructive'>
          <AlertDescription>
            {t('Unable to load transaction reviews')}
          </AlertDescription>
        </Alert>
      )}
      {pages.length > 0 && !responseFailed && claims.length === 0 && (
        <p className='text-muted-foreground text-sm'>
          {t('No transaction hashes submitted')}
        </p>
      )}
      {claims.length > 0 && (
        <div className='divide-y border-y'>
          {claims.map((claim) => (
            <div
              key={claim.claim_id}
              className='grid gap-3 py-3 sm:grid-cols-[minmax(0,1fr)_auto]'
            >
              <div className='min-w-0 space-y-2'>
                <div className='flex min-w-0 items-center gap-1'>
                  <span className='text-muted-foreground text-xs'>
                    {t('Order number')}
                  </span>
                  <code className='min-w-0 truncate text-xs'>
                    {claim.trade_no}
                  </code>
                  <CopyButton
                    value={claim.trade_no}
                    className='size-6'
                    iconClassName='size-3'
                  />
                </div>
                <p className='text-muted-foreground flex flex-wrap gap-x-3 text-xs'>
                  <span>
                    {t('User')} #{claim.user_id}
                  </span>
                  <span>{claim.network}</span>
                  <span>{claim.requested_usdt} USDT</span>
                </p>
                <div className='flex min-w-0 items-center gap-2 text-xs'>
                  <span className='text-muted-foreground shrink-0'>
                    {t('Payment address')}
                  </span>
                  <code className='min-w-0 flex-1 break-all'>
                    {claim.receive_address}
                  </code>
                  <CopyButton
                    value={claim.receive_address}
                    className='size-6'
                    iconClassName='size-3'
                  />
                </div>
                <div className='flex min-w-0 items-center gap-2 text-xs'>
                  <span className='text-muted-foreground shrink-0'>
                    {t('Transaction hash')}
                  </span>
                  <code className='min-w-0 flex-1 break-all'>
                    {claim.transaction_id}
                  </code>
                  <CopyButton
                    value={claim.transaction_id}
                    className='size-6'
                    iconClassName='size-3'
                  />
                </div>
              </div>
              <div className='flex items-center gap-2 sm:flex-col sm:items-end'>
                <Badge
                  variant={
                    claim.order_status === 'paid' ? 'default' : 'secondary'
                  }
                >
                  {claim.order_status === 'paid'
                    ? t('Paid')
                    : t('Awaiting payment')}
                </Badge>
                <time className='text-muted-foreground text-xs tabular-nums'>
                  {new Date(claim.submitted_at * 1000).toLocaleString()}
                </time>
              </div>
            </div>
          ))}
        </div>
      )}
      {query.hasNextPage && (
        <div className='flex justify-center'>
          <Button
            type='button'
            variant='outline'
            size='sm'
            disabled={query.isFetchingNextPage}
            onClick={() => void query.fetchNextPage()}
          >
            {query.isFetchingNextPage && <Spinner />}
            {t('Load older')}
          </Button>
        </div>
      )}
    </section>
  )
}
