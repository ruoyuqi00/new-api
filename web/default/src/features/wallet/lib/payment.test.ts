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

import { createElement } from 'react'
import { renderToString } from 'react-dom/server'

import { api } from '@/lib/api'

import { usePayment } from '../hooks/use-payment'
import { useWaffoPancakePayment } from '../hooks/use-waffo-pancake-payment'
import { useWaffoPayment } from '../hooks/use-waffo-payment'
import { parseOrdinaryTopUpAmount } from './payment'

test('ordinary recharge accepts only positive safe integers without truncation', () => {
  for (const value of [
    '',
    '10.5',
    '-1',
    '1e3',
    '0',
    '9007199254740992',
    ' 10',
    '+10',
    '10 ',
    'NaN',
    'Infinity',
    1.5,
    0,
    -1,
    Number.NaN,
    Infinity,
    9007199254740992,
  ]) {
    assert.equal(parseOrdinaryTopUpAmount(value), null, String(value))
  }
  assert.equal(parseOrdinaryTopUpAmount('10'), 10)
  assert.equal(parseOrdinaryTopUpAmount(10), 10)
  assert.equal(parseOrdinaryTopUpAmount('9007199254740991'), 9007199254740991)
})

test('ordinary payment hooks reject invalid amounts before quoting or creating orders', async () => {
  let hooks:
    | {
        payment: ReturnType<typeof usePayment>
        waffo: ReturnType<typeof useWaffoPayment>
        pancake: ReturnType<typeof useWaffoPancakePayment>
      }
    | undefined
  function Harness() {
    hooks = {
      payment: usePayment(),
      waffo: useWaffoPayment(),
      pancake: useWaffoPancakePayment(),
    }
    return null
  }
  renderToString(createElement(Harness))
  assert.ok(hooks)
  const originalAdapter = api.defaults.adapter
  const requests: { url?: string; data: unknown }[] = []
  api.defaults.adapter = async (config) => {
    requests.push({ url: config.url, data: JSON.parse(config.data) })
    return {
      config,
      status: 200,
      statusText: 'OK',
      headers: {},
      data: { success: false },
    }
  }
  try {
    for (const amount of [
      10.5,
      0,
      -1,
      Number.NaN,
      Infinity,
      9007199254740992,
    ]) {
      assert.equal(
        await hooks.payment.calculatePaymentAmount(amount, 'stripe'),
        0
      )
      assert.equal(await hooks.payment.processPayment(amount, 'stripe'), false)
      assert.equal(await hooks.payment.processPayment(amount, 'alipay'), false)
      assert.equal(await hooks.waffo.processWaffoPayment(amount, 0), false)
      assert.equal(
        await hooks.pancake.processWaffoPancakePayment(amount),
        false
      )
    }
    assert.deepEqual(requests, [])
    await hooks.payment.calculatePaymentAmount(10, 'stripe')
    await hooks.payment.calculatePaymentAmount(10, 'alipay')
    await hooks.payment.calculatePaymentAmount(10, 'waffo_pancake')
    await hooks.payment.processPayment(10, 'stripe')
    await hooks.payment.processPayment(10, 'alipay')
    await hooks.waffo.processWaffoPayment(10, 2)
    await hooks.pancake.processWaffoPancakePayment(10)
    assert.deepEqual(requests, [
      { url: '/api/user/stripe/amount', data: { amount: 10 } },
      { url: '/api/user/amount', data: { amount: 10 } },
      { url: '/api/user/waffo-pancake/amount', data: { amount: 10 } },
      {
        url: '/api/user/stripe/pay',
        data: { amount: 10, payment_method: 'stripe' },
      },
      { url: '/api/user/pay', data: { amount: 10, payment_method: 'alipay' } },
      { url: '/api/user/waffo/pay', data: { amount: 10, pay_method_index: 2 } },
      { url: '/api/user/waffo-pancake/pay', data: { amount: 10 } },
    ])
  } finally {
    api.defaults.adapter = originalAdapter
  }
})
