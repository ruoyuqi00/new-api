import type { GroupAvailabilityItem } from '../types'

export type { GroupAvailabilityItem }

export type AvailabilityCardStatus =
  | 'stable'
  | 'degraded'
  | 'unavailable'
  | 'observing'
  | 'no_data'

export type AvailabilityBar = {
  successPercent: number
  failurePercent: number
}

export function getAvailabilityCardStatus(
  item: GroupAvailabilityItem
): AvailabilityCardStatus {
  if (item.status === 'stable') return 'stable'
  if (item.status === 'degraded') return 'degraded'
  if (item.status === 'unavailable') return 'unavailable'
  if (item.status === 'observing') return 'observing'
  return 'no_data'
}

export function getAvailabilityBar(
  item: GroupAvailabilityItem
): AvailabilityBar {
  if (item.request_count <= 0) {
    return { successPercent: 0, failurePercent: 0 }
  }
  const successPercent = Math.min(
    100,
    Math.max(0, (item.success_count / item.request_count) * 100)
  )
  return {
    successPercent,
    failurePercent: 100 - successPercent,
  }
}
