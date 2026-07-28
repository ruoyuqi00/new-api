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
export type OverviewPanelPlan = {
  left: Array<'performance' | 'api-info' | 'announcements' | 'faq'>
  uptime: boolean
}

export function getOverviewPanelPlan(input: {
  isAdmin: boolean
  apiInfo: boolean
  announcements: boolean
  faq: boolean
  uptime: boolean
}): OverviewPanelPlan {
  const left: OverviewPanelPlan['left'] = []
  if (input.isAdmin) left.push('performance')
  if (input.apiInfo) left.push('api-info')
  if (input.announcements) left.push('announcements')
  if (input.faq) left.push('faq')
  return { left, uptime: input.uptime }
}
