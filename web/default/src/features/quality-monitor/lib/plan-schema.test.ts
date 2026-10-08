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

import { qualityMonitorPlanSchema } from './plan-schema'

const validPlan = {
  name: 'Reasoning checks',
  enabled: false,
  published: false,
  groups: ['standard'],
  models: ['gpt-6-astra', 'gpt-6.1-sol'],
  interval_minutes: 60,
  reasoning_effort: 'high',
  max_output_tokens: 2048,
  timeout_seconds: 120,
  questions: [
    {
      id: 'arithmetic',
      prompt: 'What is 6 times 7?',
      expected_answer: '42',
      match_type: 'exact',
    },
  ],
}

describe('quality monitor plan validation', () => {
  test('counts plan names by Unicode characters and question IDs by UTF-8 bytes', () => {
    assert.equal(
      qualityMonitorPlanSchema.safeParse({
        ...validPlan,
        name: '🧠'.repeat(100),
      }).success,
      true
    )
    assert.equal(
      qualityMonitorPlanSchema.safeParse({
        ...validPlan,
        name: '🧠'.repeat(101),
      }).success,
      false
    )
    assert.equal(
      qualityMonitorPlanSchema.safeParse({
        ...validPlan,
        questions: [{ ...validPlan.questions[0], id: '题'.repeat(22) }],
      }).success,
      false
    )
  })
  test('accepts explicit multiple groups, both supported models and all question rules', () => {
    const result = qualityMonitorPlanSchema.safeParse({
      ...validPlan,
      groups: ['standard', 'premium'],
      questions: [
        ...validPlan.questions,
        {
          id: 'manual',
          prompt: 'Explain your reasoning.',
          expected_answer: '',
          match_type: 'manual',
        },
        {
          id: 'contains',
          prompt: 'Name the capital of France.',
          expected_answer: 'Paris',
          match_type: 'contains',
        },
      ],
    })
    assert.equal(result.success, true)
  })

  test('rejects absent targets and unsupported models before saving', () => {
    for (const patch of [
      { groups: [] },
      { models: [] },
      { models: ['other-model'] },
      { questions: [] },
    ]) {
      assert.equal(
        qualityMonitorPlanSchema.safeParse({ ...validPlan, ...patch }).success,
        false
      )
    }
  })

  test('rejects intervals and budgets outside the bounded probe contract', () => {
    for (const patch of [
      { interval_minutes: 0 },
      { interval_minutes: 10081 },
      { interval_minutes: 1.5 },
      { max_output_tokens: 127 },
      { max_output_tokens: 8193 },
      { timeout_seconds: 9 },
      { timeout_seconds: 181 },
    ]) {
      assert.equal(
        qualityMonitorPlanSchema.safeParse({ ...validPlan, ...patch }).success,
        false
      )
    }
  })

  test('rejects more than twenty groups, five questions or one hundred calls', () => {
    const groups = Array.from({ length: 20 }, (_, i) => `group-${i}`)
    const questions = Array.from({ length: 5 }, (_, i) => ({
      ...validPlan.questions[0],
      id: `question-${i}`,
    }))
    assert.equal(
      qualityMonitorPlanSchema.safeParse({
        ...validPlan,
        groups: [...groups, 'extra'],
      }).success,
      false
    )
    assert.equal(
      qualityMonitorPlanSchema.safeParse({
        ...validPlan,
        questions: [...questions, { ...questions[0], id: 'extra' }],
      }).success,
      false
    )
    assert.equal(
      qualityMonitorPlanSchema.safeParse({ ...validPlan, groups, questions })
        .success,
      false
    )
    assert.equal(
      qualityMonitorPlanSchema.safeParse({
        ...validPlan,
        groups,
        questions,
        models: ['gpt-6-astra'],
      }).success,
      true
    )
  })

  test('requires a nonempty expected answer for automatic matching and bounds UTF-8 prompts', () => {
    for (const question of [
      { ...validPlan.questions[0], expected_answer: '   ' },
      { ...validPlan.questions[0], prompt: '   ' },
      { ...validPlan.questions[0], prompt: '问'.repeat(2731) },
    ]) {
      assert.equal(
        qualityMonitorPlanSchema.safeParse({
          ...validPlan,
          questions: [question],
        }).success,
        false
      )
    }
  })

  test('rejects duplicate group/model targets and question identifiers', () => {
    for (const patch of [
      { groups: ['standard', 'standard'] },
      { models: ['gpt-6-astra', 'gpt-6-astra'] },
      { questions: [validPlan.questions[0], validPlan.questions[0]] },
    ]) {
      assert.equal(
        qualityMonitorPlanSchema.safeParse({ ...validPlan, ...patch }).success,
        false
      )
    }
  })
})
