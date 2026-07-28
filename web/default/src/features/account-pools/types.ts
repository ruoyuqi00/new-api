export type AccountPoolSummary = {
  id: number
  name: string
  provider: string
  adapter_type: number
  group: string
  status: number
  priority: number
  weight: number
  remark: string
  created_time: number
  updated_time: number
  account_count: number
  enabled_account_count: number
  channel_count: number
}

export type AccountPoolChannel = {
  id: number
  name: string
  group: string
  status: number
  type: number
}

export type ProviderAccount = {
  id: number
  pool_id: number
  name: string
  type: string
  credential?: string
  credential_set: boolean
  credential_preview: string
  base_url: string
  model_mapping: string
  status: number
  priority: number
  weight: number
  concurrency_limit: number
  cooldown_seconds: number
  expires_at: number
  last_used_at: number
  last_error: string
  metadata: string
  plan_type: string
  primary_usage_percent: number | null
  secondary_usage_percent: number | null
  usage_updated_at: number
  usage_last_error: string
  usage_error_code: string
  usage_upstream_status: number
  usage_checked_at: number
  created_time: number
  updated_time: number
}

export type ProviderAccountSummary = ProviderAccount & {
  pool_name: string
  pool_group: string
  pool_adapter_type: number
  pool_status: number
  channel_count: number
}

export type AccountPoolDetail = {
  pool: AccountPoolSummary
  accounts: ProviderAccount[]
  channel_ids: number[]
}

export type AccountPoolPayload = {
  id?: number
  name: string
  provider: string
  adapter_type: number
  group: string
  status: number
  priority: number
  weight: number
  remark: string
  accounts: Array<{
    id?: number
    name: string
    type: string
    credential?: string
    base_url: string
    model_mapping: string
    status: number
    priority: number
    weight: number
    concurrency_limit: number
    cooldown_seconds: number
    expires_at: number
    metadata: string
  }>
  channel_ids: number[]
}

export type AccountPoolListResponse = {
  success: boolean
  message?: string
  data?: {
    items: AccountPoolSummary[]
    total: number
    page: number
    page_size: number
  }
}

export type AccountPoolDetailResponse = {
  success: boolean
  message?: string
  data?: AccountPoolDetail
}

export type ProviderAccountListResponse = {
  success: boolean
  message?: string
  data?: {
    items: ProviderAccountSummary[]
    total: number
    page: number
    page_size: number
  }
}

export type ProviderAccountUsageRefreshResult = {
  account_id: number
  account_name: string
  success: boolean
  supported: boolean
  message?: string
  error_code?: string
  upstream_status: number
  token_refreshed: boolean
  checked_at: number
}

export type ProviderAccountUsageBatchResponse = {
  success: boolean
  message?: string
  data?: {
    total: number
    succeeded: number
    failed: number
    unsupported: number
    results: ProviderAccountUsageRefreshResult[]
  }
}

export type ProviderAccountModelDiscovery = {
  account_id: number
  account_name: string
  success: boolean
  models: string[]
  message?: string
}

export type ProviderModelCoverage = {
  model: string
  support_count: number
}

export type AccountPoolModelDiscovery = {
  pool_id: number
  pool_name: string
  channel_id: number
  channel_name: string
  total_accounts: number
  succeeded_accounts: number
  failed_accounts: number
  complete: boolean
  common_models: string[]
  coverage: ProviderModelCoverage[]
  accounts: ProviderAccountModelDiscovery[]
}

export type AccountPoolModelsResponse = {
  success: boolean
  message?: string
  data?: AccountPoolModelDiscovery
}
