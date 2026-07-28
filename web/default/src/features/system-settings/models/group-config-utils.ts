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
import type { GroupCatalogItem } from '../types'
import { safeJsonParse } from '../utils/json-parser'

export type GroupPricingRow = {
  _id: string
  name: string
  ratio: number
  public: boolean
  description: string
}

export type GroupCoverageState = 'ready' | 'missing' | 'unsaved'

let groupPricingIdCounter = 0

function createGroupPricingId(): string {
  groupPricingIdCounter += 1
  return `gpr_${groupPricingIdCounter}`
}

export function normalizeGroupRatio(value: unknown): number {
  const parsed = Number(value)
  return Number.isFinite(parsed) ? parsed : 1
}

export function buildGroupPricingRows(
  groupRatio: string,
  userUsableGroups: string
): GroupPricingRow[] {
  const ratioMap = safeJsonParse<Record<string, number>>(groupRatio, {
    fallback: {},
    context: 'group ratios',
  })
  const usableMap = safeJsonParse<Record<string, string>>(userUsableGroups, {
    fallback: {},
    context: 'user usable groups',
  })
  const names = new Set([...Object.keys(ratioMap), ...Object.keys(usableMap)])

  return [...names].map((name) => ({
    _id: createGroupPricingId(),
    name,
    ratio: normalizeGroupRatio(ratioMap[name]),
    public: Object.hasOwn(usableMap, name),
    description: String(usableMap[name] ?? ''),
  }))
}

export function serializeGroupPricingRows(rows: GroupPricingRow[]) {
  const groupRatio: Record<string, number> = {}
  const userUsableGroups: Record<string, string> = {}

  for (const row of rows) {
    const name = row.name.trim()
    if (!name) continue
    groupRatio[name] = normalizeGroupRatio(row.ratio)
    if (row.public) {
      userUsableGroups[name] = row.description
    }
  }

  return {
    GroupRatio: JSON.stringify(groupRatio, null, 2),
    UserUsableGroups: JSON.stringify(userUsableGroups, null, 2),
  }
}

export function groupPricingSignature(rows: GroupPricingRow[]): string {
  const serialized = serializeGroupPricingRows(rows)
  return JSON.stringify({
    groupRatio: safeJsonParse(serialized.GroupRatio, {
      fallback: {},
      silent: true,
    }),
    userUsableGroups: safeJsonParse(serialized.UserUsableGroups, {
      fallback: {},
      silent: true,
    }),
  })
}

export function sourceGroupPricingSignature(
  groupRatio: string,
  userUsableGroups: string
): string {
  return JSON.stringify({
    groupRatio: safeJsonParse(groupRatio, { fallback: {}, silent: true }),
    userUsableGroups: safeJsonParse(userUsableGroups, {
      fallback: {},
      silent: true,
    }),
  })
}

export function getGroupCoverageState(
  groupName: string,
  catalog: GroupCatalogItem[]
): GroupCoverageState {
  const savedGroup = catalog.find((item) => item.name === groupName.trim())
  if (!savedGroup) return 'unsaved'
  return savedGroup.active_model_count > 0 ? 'ready' : 'missing'
}

export function getSpecialRuleSources(
  groupName: string,
  specialUsableGroups: string
): string[] {
  const rules = safeJsonParse<Record<string, Record<string, string>>>(
    specialUsableGroups,
    { fallback: {}, silent: true }
  )
  const sources: string[] = []

  for (const [sourceGroup, targets] of Object.entries(rules)) {
    const referencesGroup = Object.keys(targets).some((rawTarget) => {
      const target =
        rawTarget.startsWith('+:') || rawTarget.startsWith('-:')
          ? rawTarget.slice(2)
          : rawTarget
      return target === groupName
    })
    if (referencesGroup) sources.push(sourceGroup)
  }

  return sources.sort((left, right) => left.localeCompare(right))
}
