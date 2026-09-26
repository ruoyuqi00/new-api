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
import assert from 'node:assert/strict'
import { test } from 'node:test'

import { QueryClient } from '@tanstack/react-query'

import { api } from '@/lib/api'

import { requestSHKeeperPayment, submitSHKeeperTransaction } from '../api'
import { shkeeperOrderQueryOptions } from '../hooks/use-shkeeper-payment'
import type { SHKeeperInvoice, SHKeeperOrderStatus } from '../types'
import {
  claimSHKeeperCredit,
  isSHKeeperTerminal,
  canRecoverSHKeeperTransaction,
  sortSHKeeperPackages,
} from './shkeeper-payment-model'

const invoice: SHKeeperInvoice = {
  trade_no: 'USDT123',
  network: 'BSC (BEP20)',
  crypto: 'BNB-USDT',
  usdt_amount: '10',
  balance_amount: '66',
  address: '0x123',
  qr_payload: '0x123',
  status: 'unpaid',
  expires_at: 1900000000,
  received_usdt: '0',
  credited_balance: '0',
  transaction_ids: [],
}

test('sorts fixed packages without mutating API data', () => {
  const source = [
    { usdt: 50, balance: '330', label: 'Common' },
    { usdt: 10, balance: '66' },
  ]
  assert.deepEqual(
    sortSHKeeperPackages(source).map((item) => item.usdt),
    [10, 50]
  )
  assert.deepEqual(
    source.map((item) => item.usdt),
    [50, 10]
  )
})

test('polling and recovery follow server statuses, not the local expiry clock', () => {
  const cases: [SHKeeperOrderStatus, boolean, boolean][] = [
    ['pending_provider', false, false],
    ['unpaid', false, true],
    ['partial', false, true],
    ['confirming', false, true],
    ['paid', true, false],
    ['overpaid', true, false],
    ['late', true, false],
    ['failed', true, false],
  ]
  for (const [status, terminal, recovery] of cases) {
    assert.equal(isSHKeeperTerminal(status), terminal)
    assert.equal(
      canRecoverSHKeeperTransaction({ ...invoice, status, expires_at: 1 }),
      recovery
    )
  }
  assert.equal(
    canRecoverSHKeeperTransaction({ ...invoice, address: '' }),
    false
  )
})

test('partial payment never refreshes credit and positive credit refreshes once per order', () => {
  const refreshed = new Set<string>()
  assert.equal(
    claimSHKeeperCredit(
      { ...invoice, status: 'partial', received_usdt: '5' },
      refreshed
    ),
    false
  )
  assert.equal(
    claimSHKeeperCredit(
      {
        ...invoice,
        status: 'overpaid',
        received_usdt: '12',
        credited_balance: '66',
      },
      refreshed
    ),
    true
  )
  assert.equal(
    claimSHKeeperCredit(
      {
        ...invoice,
        status: 'overpaid',
        received_usdt: '15',
        credited_balance: '66',
      },
      refreshed
    ),
    false
  )
})

test('business failure retains invoice and same-order refetch recovers without another creation', async () => {
  const originalAdapter = api.defaults.adapter
  const client = new QueryClient()
  const requests: { url?: string; method?: string; data: unknown }[] = []
  let fail = false
  api.defaults.adapter = async (config) => {
    requests.push({
      url: config.url,
      method: config.method,
      data: config.data ? JSON.parse(config.data) : undefined,
    })
    const data = fail
      ? { success: false, message: 'Provider unavailable' }
      : { success: true, data: invoice }
    return { config, status: 200, statusText: 'OK', headers: {}, data }
  }
  try {
    const created = await requestSHKeeperPayment({
      usdt_amount: 10,
      crypto: 'BNB-USDT',
    })
    assert.deepEqual(requests[0], {
      url: '/api/user/shkeeper/pay',
      method: 'post',
      data: { usdt_amount: 10, crypto: 'BNB-USDT' },
    })
    const options = shkeeperOrderQueryOptions(created.trade_no)
    client.setQueryData(options.queryKey, created)
    fail = true
    await assert.rejects(
      client.fetchQuery({ ...options, retryDelay: 0 }),
      /Provider unavailable/
    )
    assert.deepEqual(client.getQueryData(options.queryKey), invoice)
    assert.equal(
      requests.length,
      3,
      'one create and two bounded status attempts'
    )
    fail = false
    assert.deepEqual(await client.fetchQuery(options), invoice)
    assert.equal(requests.filter((r) => r.method === 'post').length, 1)
    assert.ok(
      requests
        .slice(1)
        .every((r) => r.url === '/api/user/shkeeper/order/USDT123')
    )
    await submitSHKeeperTransaction('USDT123', '0xhash')
    assert.deepEqual(requests.at(-1), {
      url: '/api/user/shkeeper/order/USDT123/transaction',
      method: 'post',
      data: { txid: '0xhash' },
    })
  } finally {
    client.clear()
    api.defaults.adapter = originalAdapter
  }
})
