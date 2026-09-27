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
import { queryOptions, useMutation, useQuery } from '@tanstack/react-query'
import { useEffect, useRef } from 'react'

import { KEEP_CURRENT_PAGE_ON_QUERY_ERROR } from '@/lib/query-error-policy'

import {
  getTokenPayOrder,
  requestTokenPayPayment,
  submitTokenPayTransaction,
} from '../api'
import {
  claimTokenPayCredit,
  isTokenPayTerminal,
} from '../lib/tokenpay-payment-model'
import type { TokenPayPaymentRequest } from '../types'

export function tokenPayOrderQueryOptions(tradeNo: string) {
  return queryOptions({
    queryKey: ['wallet', 'tokenpay', tradeNo],
    queryFn: ({ signal }) => getTokenPayOrder(tradeNo, signal),
    enabled: Boolean(tradeNo),
    retry: 1,
    gcTime: 0,
    refetchOnWindowFocus: false,
    refetchInterval: (query) =>
      query.state.data && !isTokenPayTerminal(query.state.data.status)
        ? 3000
        : false,
    meta: KEEP_CURRENT_PAGE_ON_QUERY_ERROR,
  })
}

export function useTokenPayPayment(
  onCredited: () => void | Promise<void>,
  initialTradeNo = ''
) {
  const attempted = useRef(false)
  const refreshed = useRef(new Set<string>())
  const create = useMutation({
    mutationFn: requestTokenPayPayment,
    retry: false,
    meta: KEEP_CURRENT_PAGE_ON_QUERY_ERROR,
    onError: () => {},
  })
  const tradeNo = create.data?.trade_no || initialTradeNo
  const order = useQuery({
    ...tokenPayOrderQueryOptions(tradeNo),
    initialData: create.data,
  })
  const invoice = order.data ?? create.data
  const recovery = useMutation({
    mutationFn: (transactionID: string) =>
      submitTokenPayTransaction(tradeNo, transactionID),
    retry: false,
    meta: KEEP_CURRENT_PAGE_ON_QUERY_ERROR,
    onError: () => {},
  })

  useEffect(() => {
    if (
      invoice &&
      claimTokenPayCredit(invoice.trade_no, invoice.status, refreshed.current)
    ) {
      void onCredited()
    }
  }, [invoice, onCredited])

  const createInvoice = (request: TokenPayPaymentRequest) => {
    if (attempted.current) return
    attempted.current = true
    create.mutate(request)
  }

  return {
    invoice,
    createInvoice,
    creating: create.isPending,
    createError: create.error,
    pollingError: order.error,
    retryOrder: order.refetch,
    fetching: order.isFetching,
    submitTransaction: recovery.mutateAsync,
    recovering: recovery.isPending,
  }
}
