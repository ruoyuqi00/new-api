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

import {
  canResumeTokenPayRecord,
  canPayTokenPayInvoice,
  claimTokenPayCredit,
  isTokenPayTerminal,
  sortTokenPayPackages,
  tokenPayNetworkLabel,
} from './tokenpay-payment-model'

test('TokenPay never offers an address or checkout again after payment', () => {
  const invoice = {
    status: 'unpaid' as const,
    address: 'Taddress',
    payment_url: 'https://pay.example.com/Pay?Id=1',
    expires_at: 2000000000,
  }
  const beforeExpiry = 1999999999000
  assert.equal(canPayTokenPayInvoice(invoice, beforeExpiry), true)
  assert.equal(
    canPayTokenPayInvoice({ ...invoice, status: 'paid' }, beforeExpiry),
    false
  )
  assert.equal(
    canPayTokenPayInvoice({ ...invoice, status: 'failed' }, beforeExpiry),
    false
  )
  assert.equal(
    canPayTokenPayInvoice(
      { ...invoice, status: 'pending_provider' },
      beforeExpiry
    ),
    false
  )
  assert.equal(
    canPayTokenPayInvoice({ ...invoice, address: '' }, beforeExpiry),
    false
  )
  assert.equal(canPayTokenPayInvoice(invoice, 2000000000000), false)
})

test('Only the owner can resume an unfinished TokenPay order from billing history', () => {
  const pending = {
    payment_method: 'tokenpay',
    status: 'pending' as const,
    user_id: 7,
  }
  assert.equal(canResumeTokenPayRecord(pending, 7), true)
  assert.equal(canResumeTokenPayRecord({ ...pending, user_id: 8 }, 7), false)
  assert.equal(
    canResumeTokenPayRecord({ ...pending, status: 'success' }, 7),
    false
  )
  assert.equal(
    canResumeTokenPayRecord({ ...pending, payment_method: 'shkeeper' }, 7),
    false
  )
})

test('TokenPay sorts packages without changing configured order', () => {
  const configured = [
    { usdt: 20, balance: '132' },
    { usdt: 10, balance: '66' },
  ]
  assert.deepEqual(
    sortTokenPayPackages(configured).map((item) => item.usdt),
    [10, 20]
  )
  assert.equal(configured[0]?.usdt, 20)
})

test('TokenPay labels the selected chain', () => {
  assert.equal(tokenPayNetworkLabel('USDT_TRC20'), 'TRON (TRC20)')
  assert.equal(tokenPayNetworkLabel('EVM_BSC_USDT_BEP20'), 'BSC (BEP20)')
  assert.equal(tokenPayNetworkLabel('EVM_Polygon_USDT_ERC20'), 'Polygon')
})

test('TokenPay refreshes balance once only after paid state', () => {
  const refreshed = new Set<string>()
  assert.equal(isTokenPayTerminal('unpaid'), false)
  assert.equal(isTokenPayTerminal('paid'), true)
  assert.equal(claimTokenPayCredit('order-1', 'unpaid', refreshed), false)
  assert.equal(claimTokenPayCredit('order-1', 'paid', refreshed), true)
  assert.equal(claimTokenPayCredit('order-1', 'paid', refreshed), false)
  assert.equal(claimTokenPayCredit('order-2', 'failed', refreshed), false)
})
