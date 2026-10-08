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
import { z } from 'zod'

const questionSchema = z
  .object({
    id: z
      .string()
      .refine((value) => value.trim().length > 0, 'Question ID is required')
      .refine(
        (value) => new TextEncoder().encode(value).length <= 64,
        'Question ID is too long'
      ),
    prompt: z
      .string()
      .refine(
        (value) =>
          value.trim().length > 0 &&
          new TextEncoder().encode(value).length <= 8192,
        'Enter a question up to 8 KiB'
      ),
    expected_answer: z
      .string()
      .refine(
        (value) => new TextEncoder().encode(value).length <= 8192,
        'Reference answer must be up to 8 KiB'
      ),
    match_type: z.enum(['manual', 'exact', 'contains']),
  })
  .refine(
    (question) =>
      question.match_type === 'manual' ||
      question.expected_answer.trim().length > 0,
    {
      message: 'Reference answer is required for automatic matching',
      path: ['expected_answer'],
    }
  )

export const qualityMonitorPlanSchema = z
  .object({
    name: z
      .string()
      .trim()
      .min(1, 'Plan name is required')
      .refine(
        (value) => [...value].length <= 100,
        'Plan name must be at most 100 characters'
      ),
    enabled: z.boolean(),
    published: z.boolean(),
    groups: z
      .array(z.string().min(1))
      .min(1, 'Select at least one group')
      .max(20, 'Select at most 20 groups')
      .refine(
        (values) => new Set(values).size === values.length,
        'Groups must be unique'
      ),
    models: z
      .array(z.enum(['gpt-6-astra', 'gpt-6.1-sol']))
      .min(1, 'Select at least one model')
      .max(2)
      .refine(
        (values) => new Set(values).size === values.length,
        'Models must be unique'
      ),
    interval_minutes: z
      .number({ error: 'Enter a whole number' })
      .int('Interval must be a whole number')
      .min(1, 'Interval must be between 1 and 10080 minutes')
      .max(10080, 'Interval must be between 1 and 10080 minutes'),
    reasoning_effort: z.enum(['low', 'medium', 'high', 'max']),
    max_output_tokens: z
      .number({ error: 'Enter a whole number' })
      .int('Enter a whole number')
      .min(128, 'Output limit must be between 128 and 8192 tokens')
      .max(8192, 'Output limit must be between 128 and 8192 tokens'),
    timeout_seconds: z
      .number({ error: 'Enter a whole number' })
      .int('Enter a whole number')
      .min(10, 'Timeout must be between 10 and 180 seconds')
      .max(180, 'Timeout must be between 10 and 180 seconds'),
    questions: z
      .array(questionSchema)
      .min(1, 'Add at least one question')
      .max(5, 'Add at most five questions')
      .refine(
        (questions) =>
          new Set(questions.map((question) => question.id)).size ===
          questions.length,
        'Question IDs must be unique'
      ),
  })
  .refine(
    (plan) =>
      plan.groups.length * plan.models.length * plan.questions.length <= 100,
    {
      message: 'A round must contain at most 100 calls',
      path: ['groups'],
    }
  )

export type QualityMonitorPlanInput = z.infer<typeof qualityMonitorPlanSchema>
