import type { GroupAvailabilityItem } from '../types'

export type { GroupAvailabilityItem }

export type AvailabilityCardStatus =
  | 'stable'
  | 'degraded'
  | 'unavailable'
  | 'no_data'

export function getAvailabilityCardStatus(
  item: GroupAvailabilityItem
): AvailabilityCardStatus {
  if (item.status === 'stable') return 'stable'
  if (item.status === 'degraded') return 'degraded'
  if (item.status === 'unavailable') return 'unavailable'
  return 'no_data'
}
