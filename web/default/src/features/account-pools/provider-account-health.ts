import type { ProviderAccountSummary } from './types'

export type ProviderAccountHealth =
  | 'never'
  | 'healthy'
  | 'rate_limited'
  | 'auth_error'
  | 'error'

export function getProviderAccountHealth(
  account: ProviderAccountSummary
): ProviderAccountHealth {
  if (account.usage_checked_at <= 0) return 'never'
  if (account.usage_upstream_status === 429) return 'rate_limited'
  if (
    account.usage_upstream_status === 401 ||
    account.usage_upstream_status === 403
  ) {
    return 'auth_error'
  }
  if (
    account.usage_last_error ||
    account.usage_upstream_status < 200 ||
    account.usage_upstream_status >= 300
  ) {
    return 'error'
  }
  return 'healthy'
}
