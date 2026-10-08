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
import { api } from '@/lib/api'

import type { QualityMonitorPlanInput } from './lib/plan-schema'
import type {
  QualityMonitorFilters,
  QualityMonitorOptions,
  QualityMonitorPlan,
  QualityMonitorResults,
} from './types'

type Response<T> = { success: boolean; message: string; data: T }
const endpoint = '/api/quality-monitor'

export async function getQualityMonitorOptions(): Promise<QualityMonitorOptions> {
  const response = await api.get<Response<QualityMonitorOptions>>(
    `${endpoint}/options`
  )
  return response.data.data
}

export async function getQualityMonitorPlans(): Promise<QualityMonitorPlan[]> {
  const response = await api.get<Response<QualityMonitorPlan[]>>(
    `${endpoint}/plans`
  )
  return response.data.data
}

export async function getQualityMonitorResults(
  filters: QualityMonitorFilters
): Promise<QualityMonitorResults> {
  const response = await api.get<Response<QualityMonitorResults>>(
    `${endpoint}/results`,
    { params: filters }
  )
  return response.data.data
}

export async function saveQualityMonitorPlan(input: {
  id?: number
  plan: QualityMonitorPlanInput
}): Promise<QualityMonitorPlan> {
  const response = input.id
    ? await api.put<Response<QualityMonitorPlan>>(
        `${endpoint}/plans/${input.id}`,
        input.plan
      )
    : await api.post<Response<QualityMonitorPlan>>(
        `${endpoint}/plans`,
        input.plan
      )
  return response.data.data
}

export async function deleteQualityMonitorPlan(id: number): Promise<void> {
  await api.delete(`${endpoint}/plans/${id}`)
}

export async function runQualityMonitorPlan(
  id: number
): Promise<{ task_id: string }> {
  const response = await api.post<Response<{ task_id: string }>>(
    `${endpoint}/plans/${id}/run`
  )
  return response.data.data
}
