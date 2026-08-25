export type UserGroupRatioRow = {
  userId: string
  group: string
  ratio: number
}

export function flattenUserGroupRatios(value: string): UserGroupRatioRow[] {
  let parsed: unknown
  try {
    parsed = JSON.parse(value || '{}')
  } catch {
    return []
  }
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return []

  const rows: UserGroupRatioRow[] = []
  for (const [userId, groups] of Object.entries(parsed)) {
    if (!groups || typeof groups !== 'object' || Array.isArray(groups)) continue
    for (const [group, ratio] of Object.entries(groups)) {
      if (typeof ratio !== 'number' || !Number.isFinite(ratio)) continue
      rows.push({ userId, group, ratio })
    }
  }
  return rows.sort(
    (left, right) =>
      left.userId.localeCompare(right.userId) ||
      left.group.localeCompare(right.group)
  )
}

export function serializeUserGroupRatios(rows: UserGroupRatioRow[]): string {
  const result: Record<string, Record<string, number>> = {}
  for (const row of rows) {
    const userId = row.userId.trim()
    const group = row.group.trim()
    if (!userId || !group || !Number.isFinite(row.ratio) || row.ratio < 0) continue
    result[userId] ??= {}
    result[userId][group] = row.ratio
  }
  const sortedResult: Record<string, Record<string, number>> = {}
  for (const userId of Object.keys(result).sort()) {
    sortedResult[userId] = {}
    for (const group of Object.keys(result[userId]).sort()) {
      sortedResult[userId][group] = result[userId][group]
    }
  }
  return JSON.stringify(sortedResult, null, 2)
}
