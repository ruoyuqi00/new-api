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

import { renderToStaticMarkup } from 'react-dom/server'

import i18n from '@/i18n/config'

import type { QualityMonitorResult } from '../types'
import { ResultDetails, ResultStatus } from './result-details'

const result: QualityMonitorResult = {
  id: 1,
  plan_id: 2,
  plan_name: 'Arithmetic',
  group: 'standard',
  model: 'gpt-6-astra',
  question_id: 'q1',
  prompt: 'What is 6 times 7?',
  expected_answer: '42',
  answer: '<script>alert("x")</script>\n41',
  status: 'failed',
  duration_ms: 1250,
  created_at: 1700000000,
  answer_truncated: true,
  reasoning_effort: 'high',
  channel_id: 7,
  error: 'private upstream error',
  actual_response_model: 'reported-model',
  estimated_quota: 12,
}

test('renders wrong answer separately from invocation failure and manual review', async () => {
  await i18n.changeLanguage('en')
  assert.match(
    renderToStaticMarkup(<ResultStatus status='failed' />),
    /Wrong answer/
  )
  assert.match(
    renderToStaticMarkup(<ResultStatus status='error' />),
    /Call failed/
  )
  assert.match(
    renderToStaticMarkup(<ResultStatus status='ungraded' />),
    /Needs review/
  )
})

test('shows complete prompt, reference and escaped actual answer with truncation notice', () => {
  const markup = renderToStaticMarkup(
    <ResultDetails result={result} isAdmin={false} />
  )
  assert.match(markup, /What is 6 times 7\?/)
  assert.match(markup, /42/)
  assert.match(
    markup,
    /&lt;script&gt;alert\(&quot;x&quot;\)&lt;\/script&gt;\n41/
  )
  assert.match(markup, /Answer truncated/)
  assert.doesNotMatch(markup, /<script>/)
  assert.doesNotMatch(
    markup,
    /private upstream error|reported-model|Estimated quota/
  )
})

test('exposes channel, reported model, estimate and internal error only to administrators', () => {
  const markup = renderToStaticMarkup(<ResultDetails result={result} isAdmin />)
  assert.match(markup, /private upstream error/)
  assert.match(markup, /reported-model/)
  assert.match(markup, /Estimated quota/)
})
