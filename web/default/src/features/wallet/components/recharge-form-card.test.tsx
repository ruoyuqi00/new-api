/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import assert from 'node:assert/strict'
import { test } from 'node:test'

import { renderToStaticMarkup } from 'react-dom/server'

import i18n from '@/i18n/config'

import { RechargeFormCard } from './recharge-form-card'

test('USDT recharge entry shows the payment method rather than the provider name', async () => {
  await i18n.changeLanguage('zh')
  const markup = renderToStaticMarkup(
    <RechargeFormCard
      topupInfo={{
        enable_tokenpay_topup: true,
        enable_online_topup: false,
        enable_stripe_topup: false,
        pay_methods: [],
        min_topup: 1,
        stripe_min_topup: 1,
        amount_options: [],
        discount: {},
        enable_redemption: false,
      }}
      presetAmounts={[]}
      selectedPreset={null}
      onSelectPreset={() => {}}
      topupAmount={0}
      onTopupAmountChange={() => {}}
      paymentAmount={0}
      calculating={false}
      onPaymentMethodSelect={() => {}}
      paymentLoading={null}
      redemptionCode=''
      onRedemptionCodeChange={() => {}}
      onRedeem={() => {}}
      redeeming={false}
      onTokenPaySelect={() => {}}
    />
  )

  assert.match(markup, /USDT 充值/)
  assert.doesNotMatch(markup, /TokenPay USDT/)
})
