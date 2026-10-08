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
import type { QualityMonitorPlanInput } from './lib/plan-schema'

export type QualityMonitorQuestion =
  QualityMonitorPlanInput['questions'][number]

export type QualityMonitorPlan = QualityMonitorPlanInput & {
  id: number
  last_run_at: number
  next_run_at: number
  created_at: number
  updated_at: number
}

export type QualityMonitorOptions = {
  models: string[]
  groups: { name: string; models: string[] }[]
}

export type QualityMonitorResult = {
  id: number
  plan_id: number
  plan_name: string
  group: string
  model: string
  question_id: string
  prompt: string
  expected_answer: string
  answer: string
  status: 'passed' | 'failed' | 'ungraded' | 'error' | 'skipped'
  duration_ms: number
  created_at: number
  answer_truncated: boolean
  reasoning_effort: string
  channel_id?: number
  error?: string
  actual_response_model?: string
  estimated_quota?: number
}

export type QualityMonitorResults = {
  items: QualityMonitorResult[]
  total: number
}

export type QualityMonitorFilters = {
  group: string
  model: string
  plan_id?: number
  page: number
  page_size: number
}
