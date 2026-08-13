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
import { describe, test } from 'node:test'

import type { UsageLog } from '../data/schema'
import { formatModelName, getViolationFeeDisplay } from './format'

function usageLog(overrides: Partial<UsageLog>): UsageLog {
  return {
    id: 1,
    user_id: 1,
    created_at: 1,
    type: 2,
    content: '',
    username: '',
    token_name: '',
    model_name: 'public-model',
    quota: 0,
    prompt_tokens: 0,
    completion_tokens: 0,
    use_time: 0,
    is_stream: true,
    channel: 1,
    channel_name: '',
    token_id: 1,
    group: 'default',
    ip: '',
    other: '',
    request_id: 'req-1',
    upstream_request_id: '',
    ...overrides,
  }
}

describe('formatModelName', () => {
  test('keeps request, forwarded, and upstream response models distinct', () => {
    const result = formatModelName(
      usageLog({
        actual_response_model: 'provider-response-model',
        other: JSON.stringify({
          is_model_mapped: true,
          upstream_model_name: 'forwarded-model',
        }),
      })
    )

    assert.deepEqual(result, {
      name: 'public-model',
      isMapped: true,
      forwardedModel: 'forwarded-model',
      actualResponseModel: 'provider-response-model',
    })
  })

  test('omits missing audit values without changing the request model', () => {
    const result = formatModelName(usageLog({}))

    assert.deepEqual(result, {
      name: 'public-model',
      isMapped: false,
      forwardedModel: undefined,
      actualResponseModel: undefined,
    })
  })
})

describe('getViolationFeeDisplay', () => {
  test('labels an unsuccessful charge as a blocked violation with an attempted fee', () => {
    const result = getViolationFeeDisplay(
      {
        violation_fee: true,
        charge_succeeded: false,
        requested_quota: 2500,
      },
      0
    )

    assert.deepEqual(result, {
      statusKey: 'Violation blocked, charge failed',
      amountKey: 'Attempted fee',
      amount: 2500,
    })
  })

  test('falls back to the stored log amount for an older failed charge record', () => {
    const result = getViolationFeeDisplay(
      {
        violation_fee: true,
        charge_succeeded: false,
      },
      900
    )

    assert.equal(result.amount, 900)
  })

  test('shows the charged amount for a successful violation fee', () => {
    const result = getViolationFeeDisplay(
      {
        violation_fee: true,
        charge_succeeded: true,
        charged_quota: 1800,
        fee_quota: 1700,
      },
      1600
    )

    assert.deepEqual(result, {
      statusKey: 'Violation Fee',
      amountKey: 'Fee',
      amount: 1800,
    })
  })

  test('preserves the fee display for legacy violation records', () => {
    const result = getViolationFeeDisplay(
      {
        violation_fee: true,
        fee_quota: 1200,
      },
      1100
    )

    assert.deepEqual(result, {
      statusKey: 'Violation Fee',
      amountKey: 'Fee',
      amount: 1200,
    })
  })
})
