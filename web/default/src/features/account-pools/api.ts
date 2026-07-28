import { api } from '@/lib/api'

import type { ImportedAccount } from './import-parser'
import type {
  AccountPoolChannel,
  AccountPoolDetailResponse,
  AccountPoolListResponse,
  AccountPoolModelsResponse,
  AccountPoolPayload,
  ProviderAccountListResponse,
  ProviderAccountUsageBatchResponse,
} from './types'

export async function getAccountPoolChannels(): Promise<{
  success: boolean
  data?: AccountPoolChannel[]
}> {
  const response = await api.get('/api/account_pool/channels')
  return response.data
}

export async function getAccountPools(
  keyword = ''
): Promise<AccountPoolListResponse> {
  const response = await api.get('/api/account_pool/', {
    params: { p: 1, page_size: 200, keyword },
  })
  return response.data
}

export async function getAccountPool(
  id: number
): Promise<AccountPoolDetailResponse> {
  const response = await api.get(`/api/account_pool/${id}`)
  return response.data
}

export async function getAccountPoolModels(
  id: number
): Promise<AccountPoolModelsResponse> {
  const response = await api.get(`/api/account_pool/${id}/models`, {
    timeout: 300_000,
  })
  return response.data
}

export async function createAccountPool(
  payload: AccountPoolPayload
): Promise<{ success: boolean; message?: string }> {
  const response = await api.post('/api/account_pool/', payload)
  return response.data
}

export async function updateAccountPool(
  id: number,
  payload: AccountPoolPayload
): Promise<{ success: boolean; message?: string }> {
  const response = await api.put(`/api/account_pool/${id}`, payload)
  return response.data
}

export async function deleteAccountPool(
  id: number
): Promise<{ success: boolean; message?: string }> {
  const response = await api.delete(`/api/account_pool/${id}`)
  return response.data
}

export async function deleteProviderAccount(
  id: number
): Promise<{ success: boolean; message?: string }> {
  const response = await api.delete(`/api/account_pool/accounts/${id}`)
  return response.data
}

export async function deleteProviderAccounts(accountIds: number[]): Promise<{
  success: boolean
  message?: string
  data?: { count: number }
}> {
  const response = await api.post('/api/account_pool/accounts/delete', {
    account_ids: accountIds,
  })
  return response.data
}

export async function getProviderAccounts(params: {
  keyword?: string
  poolId?: number
  status?: number
}): Promise<ProviderAccountListResponse> {
  const response = await api.get('/api/account_pool/accounts', {
    params: {
      p: 1,
      page_size: 100,
      keyword: params.keyword || undefined,
      pool_id: params.poolId || undefined,
      status: params.status || undefined,
    },
  })
  return response.data
}

export async function assignProviderAccounts(
  accountIds: number[],
  poolId: number
): Promise<{ success: boolean; message?: string }> {
  const response = await api.post('/api/account_pool/accounts/assign', {
    account_ids: accountIds,
    pool_id: poolId,
  })
  return response.data
}

export async function updateProviderAccountStatuses(
  accountIds: number[],
  status: number
): Promise<{ success: boolean; message?: string }> {
  const response = await api.post('/api/account_pool/accounts/status', {
    account_ids: accountIds,
    status,
  })
  return response.data
}

export async function updateProviderAccountRouting(
  id: number,
  payload: {
    priority: number
    weight: number
    concurrency_limit: number
    cooldown_seconds: number
  }
): Promise<{ success: boolean; message?: string }> {
  const response = await api.put(
    `/api/account_pool/accounts/${id}/routing`,
    payload
  )
  return response.data
}

export async function importProviderAccounts(
  poolId: number,
  accounts: ImportedAccount[]
): Promise<{ success: boolean; message?: string; data?: { count: number } }> {
  const response = await api.post('/api/account_pool/accounts/import', {
    pool_id: poolId,
    accounts,
  })
  return response.data
}

export async function generateGrokOAuthAuthorization(): Promise<{
  success: boolean
  message?: string
  data?: { auth_url: string; session_id: string }
}> {
  const response = await api.post('/api/account_pool/grok/oauth/authorize')
  return response.data
}

export async function exchangeGrokOAuthAuthorization(payload: {
  session_id: string
  callback_url: string
}): Promise<{
  success: boolean
  message?: string
  data?: ImportedAccount
}> {
  const response = await api.post(
    '/api/account_pool/grok/oauth/exchange',
    payload
  )
  return response.data
}

export async function getProviderAccountUsage(id: number): Promise<{
  success: boolean
  message?: string
  upstream_status?: number
  data?: Record<string, unknown>
}> {
  const response = await api.get(`/api/account_pool/accounts/${id}/usage`)
  return response.data
}

export async function refreshProviderAccountUsages(
  accountIds: number[]
): Promise<ProviderAccountUsageBatchResponse> {
  const response = await api.post(
    '/api/account_pool/accounts/usage/refresh',
    { account_ids: accountIds },
    { timeout: 300_000 }
  )
  return response.data
}
