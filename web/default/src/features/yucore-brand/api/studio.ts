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
import { api, type ApiRequestConfig } from '@/lib/api'

export interface YucoreMediaAsset {
  id: string
  kind: 'image' | 'video'
  url: string
  thumb_url?: string
  cached_url?: string
  source_url?: string
  label: string
  width?: number
  height?: number
  duration_ms?: number
  mime_type?: string
  metadata?: Record<string, unknown>
}

export interface YucoreMediaTask {
  id: number
  task_id: string
  user_id: number
  session_id: string
  kind: 'image' | 'video'
  mode: string
  model_id: string
  prompt: string
  negative_prompt: string
  aspect_ratio: string
  size: string
  quality: string
  format: string
  count: number
  status: 'pending' | 'processing' | 'completed' | 'failed' | 'canceled'
  progress: number
  cost: number
  assets: YucoreMediaAsset[]
  inputs: unknown[]
  metadata: Record<string, unknown>
  error: string
  created_time: number
  updated_time: number
}

export interface YucoreCanvasRecord {
  id: number
  user_id: number
  title: string
  description: string
  module: string
  snapshot: {
    nodes?: unknown[]
    edges?: unknown[]
    [key: string]: unknown
  }
  viewport: Record<string, unknown>
  revision: number
  created_time: number
  updated_time: number
}

export interface YucoreCanvasVersionRecord {
  id: number
  canvas_id: number
  user_id: number
  revision: number
  title: string
  module: string
  snapshot: {
    nodes?: unknown[]
    edges?: unknown[]
    [key: string]: unknown
  }
  viewport: Record<string, unknown>
  created_time: number
}

export interface YucoreCanvasAgentRun {
  id: number
  run_id: string
  user_id: number
  canvas_id: number
  mode: 'site' | 'local' | string
  prompt: string
  status: 'queued' | 'running' | 'completed' | 'failed' | string
  summary: string
  actions: Array<Record<string, unknown>>
  result_task_id: string
  created_time: number
  updated_time: number
}

export interface YucoreCanvasIdentity {
  user_id: number
  username: string
  identity_session: string
  identity_token: string
  issued_at: number
  expires_at: number
  scopes: string[]
  storage_keys?: {
    identity_token?: string
    identity_session?: string
  }
}

export interface YucoreCanvasAgentExecuteResult {
  run: YucoreCanvasAgentRun
  task: YucoreMediaTask
  identity: YucoreCanvasIdentity
}

export interface YucoreMediaModel {
  id: string
  name: string
  family?: string
  badge?: string
  description?: string
  source?: string
  kind: 'image' | 'video'
  modes: string[]
  sizes: string[]
  size_label?: string
  aspect_ratios?: string[]
  qualities?: string[]
  formats?: string[]
  output_formats?: string[]
  counts?: number[]
  durations?: number[]
  backgrounds?: string[]
  moderations?: string[]
  stream_modes?: string[]
  partial_images?: number[]
  style_presets?: string[]
  input_limits?: {
    max_prompt_chars?: number
    max_reference_images?: number
    max_file_size_mb?: number
  }
  pricing?: {
    unit?: string
    amount?: number
    currency?: string
    display?: string
    [key: string]: unknown
  }
}

export interface YucorePromptTemplate {
  id: string
  title: string
  preview_image_url?: string
  tag?: string
  kind: 'image' | 'video'
  style: string
  model_id?: string
  mode?: string
  prompt: string
  negative_prompt?: string
  aspect_ratio?: string
  duration?: number
}

export interface YucoreMediaBilling {
  active_mode: 'quota_sync' | 'native_wallet'
  available_points: number
  used_points: number
  estimated_unit: string
  settlement: string
}

export interface YucoreMediaHealth {
  adapter: 'mock' | 'openai-compatible' | 'yuapi-channel' | 'uag-proxy' | string
  configured: boolean
  base_url_configured: boolean
  api_key_configured: boolean
  auth_mode?: string
  status?: string
  message?: string
  supports_image: boolean
  supports_video: boolean
  require_real_assets: boolean
  mock_fallback: boolean
  upstream_verified: boolean
  upstream_verification_status?: string
  upstream_verification_message?: string
  real_workflow_ready?: boolean
  verification_blockers?: string[]
}

export interface YucoreMediaReferenceUpload {
  id: string
  name: string
  fileName?: string
  size: number
  mime_type: string
  mimeType?: string
  data_url?: string
  dataUrl?: string
  cached_url?: string
  cachedUrl?: string
  source_url?: string
  sourceUrl?: string
  url?: string
  createdAt?: string
}

interface PageResponse<T> {
  page: number
  page_size: number
  total: number
  items: T[]
}

interface ApiEnvelope<T> {
  success: boolean
  message: string
  data: T
}

function unwrap<T>(payload: ApiEnvelope<T>): T {
  if (!payload.success) {
    throw new Error(payload.message || 'Request failed')
  }
  return payload.data
}

function getYucoreMediaUpstreamToken() {
  if (typeof window === 'undefined') return ''
  const keys = [
    'klein:token',
    'yucore:uag-token',
    'yucore:uag:token',
    'yucoreMediaUagToken',
  ]
  for (const key of keys) {
    const value = window.localStorage.getItem(key)?.trim()
    if (value) return value
  }
  return ''
}

const fallbackCanvasIdentityTokenKey = 'infinite-canvas:identity-token'
const fallbackCanvasIdentitySessionKey = 'infinite-canvas:identity-session'

function getStoredCanvasIdentityToken() {
  if (typeof window === 'undefined') return ''
  return (
    window.localStorage.getItem(fallbackCanvasIdentityTokenKey)?.trim() ?? ''
  )
}

function getStoredCanvasIdentitySession() {
  if (typeof window === 'undefined') return ''
  return (
    window.localStorage.getItem(fallbackCanvasIdentitySessionKey)?.trim() ?? ''
  )
}

export function storeYucoreCanvasIdentity(identity: YucoreCanvasIdentity) {
  if (typeof window === 'undefined') return
  const tokenKey =
    identity.storage_keys?.identity_token ?? fallbackCanvasIdentityTokenKey
  const sessionKey =
    identity.storage_keys?.identity_session ?? fallbackCanvasIdentitySessionKey
  window.localStorage.setItem(tokenKey, identity.identity_token)
  window.localStorage.setItem(sessionKey, identity.identity_session)
  window.localStorage.setItem(
    'yucore:canvas-identity',
    JSON.stringify(identity)
  )
}

function getYucoreMediaRequestConfig(
  config: ApiRequestConfig = {}
): ApiRequestConfig {
  const token = getYucoreMediaUpstreamToken()
  const canvasIdentity = getStoredCanvasIdentityToken()
  const canvasSession = getStoredCanvasIdentitySession()
  if (!token && !canvasIdentity && !canvasSession) return config
  return {
    ...config,
    headers: {
      ...(config.headers as Record<string, string> | undefined),
      ...(token
        ? {
            'X-YuCore-UAG-Authorization': token.includes(' ')
              ? token
              : `Bearer ${token}`,
          }
        : {}),
      ...(canvasIdentity ? { 'X-YuCore-Canvas-Identity': canvasIdentity } : {}),
      ...(canvasSession ? { 'X-YuCore-Canvas-Session': canvasSession } : {}),
    },
  }
}

export async function listYucoreMediaModels() {
  const res = await api.get<ApiEnvelope<YucoreMediaModel[]>>(
    '/api/yucore/media/models',
    getYucoreMediaRequestConfig()
  )
  return unwrap(res.data)
}

export async function listYucorePromptTemplates() {
  const res = await api.get<ApiEnvelope<YucorePromptTemplate[]>>(
    '/api/yucore/media/templates',
    getYucoreMediaRequestConfig()
  )
  return unwrap(res.data)
}

export async function getYucoreMediaBilling() {
  const res = await api.get<ApiEnvelope<YucoreMediaBilling>>(
    '/api/yucore/media/billing',
    getYucoreMediaRequestConfig({
      disableDuplicate: true,
      skipErrorHandler: true,
    })
  )
  return unwrap(res.data)
}

export async function getYucoreMediaHealth() {
  const res = await api.get<ApiEnvelope<YucoreMediaHealth>>(
    '/api/yucore/media/health',
    getYucoreMediaRequestConfig({
      disableDuplicate: true,
      skipErrorHandler: true,
    })
  )
  return unwrap(res.data)
}

export async function uploadYucoreMediaReference(file: File) {
  const formData = new FormData()
  formData.append('file', file)
  const res = await api.post<ApiEnvelope<YucoreMediaReferenceUpload>>(
    '/api/yucore/media/uploads',
    formData,
    getYucoreMediaRequestConfig({
      disableDuplicate: true,
      skipErrorHandler: true,
    })
  )
  return unwrap(res.data)
}

export async function listYucoreMediaTasks(
  params: {
    session_id?: string
    kind?: string
    status?: string
    page_size?: number
  } = {}
) {
  const res = await api.get<ApiEnvelope<PageResponse<YucoreMediaTask>>>(
    '/api/yucore/media/tasks',
    getYucoreMediaRequestConfig({
      params,
      disableDuplicate: true,
      skipErrorHandler: true,
    })
  )
  return unwrap(res.data)
}

export async function listYucoreMediaGallery(
  params: {
    kind?: string
    page_size?: number
  } = {}
) {
  const res = await api.get<ApiEnvelope<PageResponse<YucoreMediaTask>>>(
    '/api/yucore/media/gallery',
    getYucoreMediaRequestConfig({
      params,
      disableDuplicate: true,
      skipErrorHandler: true,
    })
  )
  return unwrap(res.data)
}

export async function createYucoreMediaTask(payload: {
  kind: 'image' | 'video'
  mode: string
  model_id: string
  prompt: string
  negative_prompt?: string
  aspect_ratio?: string
  size?: string
  quality?: string
  format?: string
  count?: number
  session_id?: string
  inputs?: unknown[]
  metadata?: Record<string, unknown>
}) {
  const res = await api.post<ApiEnvelope<YucoreMediaTask>>(
    '/api/yucore/media/tasks',
    payload,
    getYucoreMediaRequestConfig()
  )
  return unwrap(res.data)
}

export async function getYucoreMediaTask(taskId: string) {
  const res = await api.get<ApiEnvelope<YucoreMediaTask>>(
    `/api/yucore/media/tasks/${taskId}`,
    getYucoreMediaRequestConfig({ disableDuplicate: true })
  )
  return unwrap(res.data)
}

export async function cancelYucoreMediaTask(taskId: string) {
  const res = await api.patch<ApiEnvelope<YucoreMediaTask>>(
    `/api/yucore/media/tasks/${taskId}`,
    { action: 'cancel' },
    getYucoreMediaRequestConfig()
  )
  return unwrap(res.data)
}

export async function listYucoreCanvases() {
  const res = await api.get<ApiEnvelope<PageResponse<YucoreCanvasRecord>>>(
    '/api/yucore/canvas',
    {
      params: { page_size: 20 },
      disableDuplicate: true,
      skipErrorHandler: true,
    }
  )
  return unwrap(res.data)
}

export async function getYucoreCanvas(id: number) {
  const res = await api.get<ApiEnvelope<YucoreCanvasRecord>>(
    `/api/yucore/canvas/${id}`,
    { disableDuplicate: true }
  )
  return unwrap(res.data)
}

export async function getYucoreCanvasIdentity() {
  const res = await api.get<ApiEnvelope<YucoreCanvasIdentity>>(
    '/api/yucore/canvas/identity',
    { disableDuplicate: true, skipErrorHandler: true }
  )
  const identity = unwrap(res.data)
  storeYucoreCanvasIdentity(identity)
  return identity
}

export async function createYucoreCanvas(payload: {
  title: string
  description?: string
  module?: string
  snapshot?: Record<string, unknown>
  viewport?: Record<string, unknown>
}) {
  const res = await api.post<ApiEnvelope<YucoreCanvasRecord>>(
    '/api/yucore/canvas',
    payload
  )
  return unwrap(res.data)
}

export async function updateYucoreCanvas(
  id: number,
  payload: {
    title: string
    description?: string
    module?: string
    snapshot?: Record<string, unknown>
    viewport?: Record<string, unknown>
    autosave?: boolean
  }
) {
  const res = await api.put<ApiEnvelope<YucoreCanvasRecord>>(
    `/api/yucore/canvas/${id}`,
    payload
  )
  return unwrap(res.data)
}

export async function deleteYucoreCanvas(id: number) {
  const res = await api.delete<ApiEnvelope<null>>(`/api/yucore/canvas/${id}`)
  return unwrap(res.data)
}

export async function listYucoreCanvasVersions(
  id: number,
  params: { page_size?: number } = {}
) {
  const res = await api.get<
    ApiEnvelope<PageResponse<YucoreCanvasVersionRecord>>
  >(`/api/yucore/canvas/${id}/versions`, {
    params: { page_size: 20, ...params },
    disableDuplicate: true,
    skipErrorHandler: true,
  })
  return unwrap(res.data)
}

export async function listYucoreCanvasAgentRuns(
  canvasId: number,
  params: { page_size?: number } = {}
) {
  const res = await api.get<ApiEnvelope<PageResponse<YucoreCanvasAgentRun>>>(
    `/api/yucore/canvas/${canvasId}/agent-runs`,
    {
      params: { page_size: 20, ...params },
      disableDuplicate: true,
      skipErrorHandler: true,
    }
  )
  return unwrap(res.data)
}

export async function createYucoreCanvasAgentRun(
  canvasId: number,
  payload: {
    mode: 'site' | 'local'
    prompt: string
    status: 'queued' | 'running' | 'completed' | 'failed'
    summary?: string
    actions?: Array<Record<string, unknown>>
    result_task_id?: string
  }
) {
  const res = await api.post<ApiEnvelope<YucoreCanvasAgentRun>>(
    `/api/yucore/canvas/${canvasId}/agent-runs`,
    payload
  )
  return unwrap(res.data)
}

export async function executeYucoreCanvasAgentRun(
  canvasId: number,
  payload: {
    mode: 'site' | 'local'
    prompt: string
    kind: 'image' | 'video'
    media_mode: string
    model_id: string
    negative_prompt?: string
    aspect_ratio?: string
    size?: string
    quality?: string
    format?: string
    count?: number
    session_id?: string
    inputs?: unknown[]
    metadata?: Record<string, unknown>
    agent_prompt_node_id: string
    agent_task_node_id: string
  }
) {
  const res = await api.post<ApiEnvelope<YucoreCanvasAgentExecuteResult>>(
    `/api/yucore/canvas/${canvasId}/agent-runs/execute`,
    payload,
    getYucoreMediaRequestConfig()
  )
  const result = unwrap(res.data)
  storeYucoreCanvasIdentity(result.identity)
  return result
}

export async function updateYucoreCanvasAgentRun(
  canvasId: number,
  runId: string,
  payload: {
    mode: 'site' | 'local' | string
    prompt: string
    status: 'queued' | 'running' | 'completed' | 'failed' | string
    summary?: string
    actions?: Array<Record<string, unknown>>
    result_task_id?: string
  }
) {
  const res = await api.patch<ApiEnvelope<YucoreCanvasAgentRun>>(
    `/api/yucore/canvas/${canvasId}/agent-runs/${runId}`,
    payload
  )
  return unwrap(res.data)
}
