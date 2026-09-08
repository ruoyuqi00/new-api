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
import {
  formatModelName,
  getThinkingTokenBreakdown,
  getTieredBillingSummary,
  getToolFeeBreakdown,
  getViolationFeeDisplay,
} from './format'

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

describe('getTieredBillingSummary', () => {
  test('parses a per-call expression using its matched context tier', () => {
    const expression = 'len <= 128000 ? tier("short", 0.05) : tier("long", 0.1)'

    const result = getTieredBillingSummary({
      billing_mode: 'per_call_expr',
      expr_b64: Buffer.from(expression).toString('base64'),
      matched_tier: 'long',
    })

    assert.equal(result?.tier.label, 'long')
    assert.equal(result?.tier.fixedPrice, 0.1)
    assert.deepEqual(result?.priceEntries, [])
  })

  test('uses the frozen estimated tier when terminal usage is unavailable', () => {
    const expression = 'len <= 128000 ? tier("short", 0.05) : tier("long", 0.1)'

    const result = getTieredBillingSummary({
      billing_mode: 'per_call_expr',
      expr_b64: Buffer.from(expression).toString('base64'),
      estimated_tier: 'short',
      usage_unconfirmed: true,
      settled_from_reservation: true,
    })

    assert.equal(result?.tier.label, 'short')
    assert.equal(result?.tier.fixedPrice, 0.05)
  })
})

describe('getThinkingTokenBreakdown', () => {
  test('treats thinking as an included subset of authoritative output', () => {
    assert.deepEqual(
      getThinkingTokenBreakdown(10_000, {
        thinking_tokens: 8_000,
        text_output_tokens: 2_000,
        thinking_tokens_included_in_output: true,
      }),
      {
        outputTokens: 10_000,
        thinkingTokens: 8_000,
        textOutputTokens: 2_000,
        includedInOutput: true,
      }
    )
  })

  test('clamps malformed thinking details without inflating output', () => {
    assert.deepEqual(
      getThinkingTokenBreakdown(100, {
        thinking_tokens: 120,
        thinking_tokens_included_in_output: true,
      }),
      {
        outputTokens: 100,
        thinkingTokens: 100,
        textOutputTokens: 0,
        includedInOutput: true,
      }
    )
  })
})

describe('getToolFeeBreakdown', () => {
  test('calculates the additional fee from executed calls and effective ratio', () => {
    assert.deepEqual(getToolFeeBreakdown(2, 10, 0.15), {
      callCount: 2,
      pricePerThousand: 10,
      effectiveGroupRatio: 0.15,
      feeUSD: 0.003,
    })
  })

  test('omits declared tools when no call executed', () => {
    assert.equal(getToolFeeBreakdown(0, 10, 0.15), null)
  })
})
