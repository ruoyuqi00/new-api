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
import { Link } from '@tanstack/react-router'
import {
  Background,
  BackgroundVariant,
  Handle,
  MarkerType,
  MiniMap,
  Position,
  ReactFlow,
  addEdge,
  type Connection,
  type Edge,
  type Node,
  type NodeProps,
  type ReactFlowInstance,
  type Viewport,
  useEdgesState,
  useNodesState,
} from '@xyflow/react'

import '@xyflow/react/dist/style.css'
import {
  ArrowRight,
  Bot,
  Boxes,
  Braces,
  CirclePlus,
  Clapperboard,
  Download,
  Eraser,
  FileText,
  FolderOpen,
  Focus,
  GalleryHorizontalEnd,
  History,
  Home,
  ImagePlus,
  Keyboard,
  LayoutGrid,
  LibraryBig,
  Loader2,
  Music2,
  Moon,
  MousePointer2,
  Palette,
  PanelLeft,
  Play,
  Plus,
  RefreshCw,
  Redo2,
  Save,
  Search,
  Send,
  Settings2,
  SlidersHorizontal,
  Sun,
  Type,
  Undo2,
  Upload,
  WandSparkles,
  X,
  ZoomIn,
  ZoomOut,
  type LucideIcon,
} from 'lucide-react'
import {
  type ChangeEvent,
  type DragEvent,
  type ReactNode,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
} from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Switch } from '@/components/ui/switch'
import { cn } from '@/lib/utils'

import {
  cancelYucoreMediaTask,
  createYucoreCanvas,
  createYucoreMediaTask,
  deleteYucoreCanvas,
  executeYucoreCanvasAgentRun,
  getYucoreCanvas,
  getYucoreCanvasIdentity,
  getYucoreMediaCatalog,
  getYucoreMediaBilling,
  getYucoreMediaHealth,
  getYucoreMediaTask,
  listYucoreCanvasAgentRuns,
  listYucoreCanvasVersions,
  listYucoreCanvases,
  listYucoreMediaGallery,
  listYucoreMediaTasks,
  listYucorePromptTemplates,
  updateYucoreCanvasAgentRun,
  updateYucoreCanvas,
  uploadYucoreMediaReference,
  type YucoreCanvasRecord,
  type YucoreCanvasAgentRun,
  type YucoreCanvasIdentity,
  type YucoreCanvasVersionRecord,
  type YucoreMediaAsset,
  type YucoreMediaBilling,
  type YucoreMediaCatalog,
  type YucoreMediaHealth,
  type YucoreMediaModel,
  type YucoreMediaTask,
  type YucorePromptTemplate,
} from '../api/studio'
import { YUCORE_STUDIO_NAME } from '../data/content'
import { modelsForKind, resolveMediaSelection } from '../lib/media-catalog'
import { YucoreBrandMark } from './yucore-brand-mark'
import { YucorePageShell } from './yucore-page-shell'

type StudioView = 'home' | 'canvas' | 'image' | 'video' | 'prompts' | 'assets'
type CanvasAgentTab = 'connect' | 'chat' | 'history' | 'logs'
type CanvasBackgroundMode = 'net' | 'lines' | 'dots' | 'blank'
type CanvasToolId =
  | 'move'
  | 'save'
  | 'undo'
  | 'redo'
  | 'text'
  | 'image'
  | 'video'
  | 'audio'
  | 'settings'
  | 'upload'
  | 'assets'
  | 'organize'
  | 'appearance'
  | 'clear'

interface StudioNavItem {
  id: StudioView
  label: string
  icon: typeof Home
}

interface CanvasMenuItem {
  label: string
  icon: LucideIcon
  action: () => void
  danger?: boolean
  disabled?: boolean
  separatorBefore?: boolean
  shortcut?: string
}

interface ReferenceAssetDraft {
  id: string
  name: string
  size: number
  mimeType: string
  previewUrl: string
  dataUrl?: string
  cachedUrl?: string
  sourceUrl?: string
  url?: string
  isUploading?: boolean
  uploadError?: string
}

interface CanvasNodeData extends Record<string, unknown> {
  label: string
  sublabel?: string
  kind?: 'prompt' | 'image' | 'video' | 'text'
  assetUrl?: string
  thumbUrl?: string
  prompt?: string
  status?: string
}

type StudioCanvasNode = Node<CanvasNodeData>

interface CanvasHistoryEntry {
  nodes: StudioCanvasNode[]
  edges: Edge[]
  viewport?: Viewport
  backgroundMode: CanvasBackgroundMode
}

interface CanvasClipboard {
  nodes: StudioCanvasNode[]
  edges: Edge[]
}

interface ImportedCanvasPayload {
  title?: string
  description?: string
  module?: string
  snapshot?: {
    nodes?: unknown[]
    edges?: unknown[]
    backgroundMode?: unknown
    [key: string]: unknown
  }
  nodes?: unknown[]
  edges?: unknown[]
  connections?: unknown[]
  viewport?: unknown
  backgroundMode?: unknown
  background_mode?: unknown
}

const modeLabels: Record<string, string> = {
  'text-to-image': '文生图',
  'image-to-image': '图生图',
  'text-to-video': '文生视频',
  'image-to-video': '图生视频',
  'reference-to-video': '参考图生视频',
}

const styleLabels: Record<string, string> = {
  auto: '自动',
  commercial: '商业精修',
  cinematic: '电影感',
  editorial: '杂志质感',
  product: '产品棚拍',
  anime: '二次元',
  realistic: '写实',
}

const streamModeLabels: Record<string, string> = {
  final: '按次返回最终结果',
  partial: '流式预览',
  poll: '异步任务轮询',
}

const outputFormatLabels: Record<string, string> = {
  png: 'PNG',
  jpeg: 'JPEG',
  webp: 'WebP',
  url: '临时 URL',
  b64_json: 'Base64',
  'image/png': 'PNG',
  'image/jpeg': 'JPEG',
}

const backgroundLabels: Record<string, string> = {
  auto: '自动',
  opaque: '不透明',
}

const moderationLabels: Record<string, string> = {
  auto: '标准',
  low: '低限制',
}

const visibilityLabels: Record<string, string> = {
  private: '仅自己可见',
  link: '链接可见',
}

const studioNavItems: StudioNavItem[] = [
  { id: 'home', label: '首页', icon: Home },
  { id: 'canvas', label: '我的画布', icon: GalleryHorizontalEnd },
  { id: 'image', label: '生图工作台', icon: ImagePlus },
  { id: 'video', label: '视频创作台', icon: Clapperboard },
  { id: 'prompts', label: '提示词库', icon: FileText },
  { id: 'assets', label: '我的素材', icon: LibraryBig },
]

const defaultTemplates: YucorePromptTemplate[] = [
  {
    id: 'direct-flash-editorial',
    title: '直闪胶片人像',
    preview_image_url: '/yucore/prompt-library/direct-flash-editorial.webp',
    kind: 'image',
    style: 'CCD / direct flash',
    aspect_ratio: '3:4',
    prompt:
      '真实直闪胶片质感，室内生活快照，人物自然动作，硬边阴影，轻微颗粒和暗角，保留真实皮肤纹理。',
  },
  {
    id: 'premium-product-core',
    title: '高端产品核心图',
    preview_image_url: '/yucore/prompt-library/premium-product-core.webp',
    kind: 'image',
    style: 'product / cinematic',
    aspect_ratio: '1:1',
    prompt:
      '高级黑色产品主视觉，精确边缘光，玻璃与金属细节，干净背景，商业级构图。',
  },
]

const baseCanvasNodes: StudioCanvasNode[] = [
  {
    id: 'brief',
    type: 'media',
    position: { x: -280, y: -80 },
    data: {
      kind: 'text',
      label: '创意简报',
      sublabel: '品牌方向 / 画面目标',
      prompt: '把目标人群、投放渠道、视觉关键词先放到这里。',
    },
    style: {
      width: 190,
      border: '1px solid rgb(103 232 249 / 0.32)',
      borderRadius: 16,
      background: 'rgb(5 9 16 / 0.88)',
      color: 'white',
      padding: 14,
      whiteSpace: 'pre-line',
    },
  },
  {
    id: 'prompt',
    type: 'media',
    position: { x: 0, y: -160 },
    data: {
      kind: 'prompt',
      label: '提示词堆栈',
      sublabel: '主体 / 光线 / 构图',
      prompt: '用提示词、反向提示词和风格模板组织一次生成。',
    },
    style: {
      width: 190,
      border: '1px solid rgb(253 230 138 / 0.32)',
      borderRadius: 16,
      background: 'rgb(5 9 16 / 0.88)',
      color: 'white',
      padding: 14,
      whiteSpace: 'pre-line',
    },
  },
  {
    id: 'reference',
    type: 'media',
    position: { x: 0, y: 84 },
    data: {
      kind: 'image',
      label: '参考素材',
      sublabel: '图片 / 风格 / 版本',
      prompt: '参考图、品牌素材和历史结果可以拖入画布复用。',
    },
    style: {
      width: 190,
      border: '1px solid rgb(110 231 183 / 0.3)',
      borderRadius: 16,
      background: 'rgb(5 9 16 / 0.88)',
      color: 'white',
      padding: 14,
      whiteSpace: 'pre-line',
    },
  },
  {
    id: 'render',
    type: 'media',
    position: { x: 310, y: -34 },
    data: {
      kind: 'video',
      label: '生成队列',
      sublabel: '图像 / 视频 / 导出',
      prompt: '把画布内容送入生图或视频任务，结果再回流到素材库。',
    },
    style: {
      width: 190,
      border: '1px solid rgb(196 181 253 / 0.32)',
      borderRadius: 16,
      background: 'rgb(5 9 16 / 0.88)',
      color: 'white',
      padding: 14,
      whiteSpace: 'pre-line',
    },
  },
]

const baseCanvasEdges: Edge[] = [
  {
    id: 'brief-prompt',
    source: 'brief',
    target: 'prompt',
    animated: true,
    type: 'smoothstep',
    markerEnd: { type: MarkerType.ArrowClosed, color: '#67e8f9' },
    style: { stroke: '#67e8f9', opacity: 0.7 },
  },
  {
    id: 'brief-reference',
    source: 'brief',
    target: 'reference',
    animated: true,
    type: 'smoothstep',
    markerEnd: { type: MarkerType.ArrowClosed, color: '#6ee7b7' },
    style: { stroke: '#6ee7b7', opacity: 0.68 },
  },
  {
    id: 'prompt-render',
    source: 'prompt',
    target: 'render',
    animated: true,
    type: 'smoothstep',
    markerEnd: { type: MarkerType.ArrowClosed, color: '#fde68a' },
    style: { stroke: '#fde68a', opacity: 0.72 },
  },
  {
    id: 'reference-render',
    source: 'reference',
    target: 'render',
    animated: true,
    type: 'smoothstep',
    markerEnd: { type: MarkerType.ArrowClosed, color: '#c4b5fd' },
    style: { stroke: '#c4b5fd', opacity: 0.72 },
  },
]

function getStudioSessionId() {
  if (typeof window === 'undefined') return 'studio-session'
  const key = 'yucore-studio-session-id'
  const existing = window.localStorage.getItem(key)
  if (existing) return existing
  const next = `studio_${Date.now().toString(36)}_${Math.random()
    .toString(36)
    .slice(2, 8)}`
  window.localStorage.setItem(key, next)
  return next
}

function formatTime(value?: number) {
  if (!value) return '刚刚'
  return new Date(value * 1000).toLocaleString('zh-CN', {
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  })
}

function firstOption<T>(items: T[] | undefined, fallback: T): T {
  return items && items.length > 0 ? items[0] : fallback
}

function keepOption<T>(current: T, items: T[] | undefined, fallback: T): T {
  return items?.includes(current) ? current : firstOption(items, fallback)
}

function getModelById(
  models: YucoreMediaModel[],
  modelId: string,
  kind: 'image' | 'video'
) {
  return (
    models.find((model) => model.id === modelId) ??
    models.find((model) => model.kind === kind)
  )
}

function getAspectRatios(model?: YucoreMediaModel) {
  return model?.aspect_ratios ?? emptyStringOptions
}

function getSizes(model?: YucoreMediaModel) {
  return model?.sizes ?? emptyStringOptions
}

function getFormats(model?: YucoreMediaModel) {
  if (model?.output_formats?.length) return model.output_formats
  if (model?.formats?.length) return model.formats
  return emptyStringOptions
}

const emptyMediaCatalog: YucoreMediaCatalog = {
  default_group: '',
  groups: [],
}
const emptyStringOptions: string[] = []
const emptyNumberOptions: number[] = []

function inferMediaHealthFromModels(
  modelRows: YucoreMediaModel[]
): YucoreMediaHealth | null {
  const hasUAGModels = modelRows.some((model) => model.source === 'uag-proxy')
  if (!hasUAGModels) return null
  return {
    adapter: 'uag-proxy',
    configured: true,
    base_url_configured: true,
    api_key_configured: false,
    status: 'configured',
    message:
      'UAG model directory is available, but real upstream provider verification is still pending.',
    supports_image: modelRows.some((model) => model.kind === 'image'),
    supports_video: modelRows.some((model) => model.kind === 'video'),
    require_real_assets: true,
    mock_fallback: false,
    upstream_verified: false,
    upstream_verification_status: 'inferred_unverified',
    upstream_verification_message:
      'UAG model directory is available, but real upstream provider verification is still pending.',
    real_workflow_ready: false,
    verification_blockers: [
      'An ordinary-user Studio generation has not been verified against a real upstream provider.',
    ],
  }
}

function fallbackMediaHealth(modelRows: YucoreMediaModel[]): YucoreMediaHealth {
  return (
    inferMediaHealthFromModels(modelRows) ?? {
      adapter: 'mock',
      configured: true,
      base_url_configured: false,
      api_key_configured: false,
      status: 'development',
      message:
        'Local development media is using mock assets. Configure UAG/OpenAI-compatible upstream to switch to real assets.',
      supports_image: true,
      supports_video: true,
      require_real_assets: false,
      mock_fallback: true,
      upstream_verified: false,
      upstream_verification_status: 'not_applicable',
      upstream_verification_message:
        'Local mock media does not verify an upstream provider.',
      real_workflow_ready: false,
      verification_blockers: [
        'YuCore media adapter is still using mock assets.',
        'yucore_media.require_real_assets must stay enabled for real workflow verification.',
      ],
    }
  )
}

function mediaEngineLabel(health: YucoreMediaHealth | null) {
  if (!health) return '加载中'
  if (health.adapter === 'mock') return '本地模拟'
  if (!health.configured) return '等待上游配置'
  if (health.real_workflow_ready) return '真实工作流已验证'
  if (health.adapter === 'uag-proxy') return 'UAG 待真实验证'
  if (health.upstream_verified) return '上游已标记验证'
  return '上游待验证'
}

function mediaAssetPolicyLabel(health: YucoreMediaHealth | null) {
  if (!health) return '加载中'
  if (health.real_workflow_ready) return '真实资产链路就绪'
  if (health.require_real_assets) return '强制真实资产 / 待验'
  if (health.mock_fallback) return '允许本地回退'
  return '按上游返回'
}

function getCanvasNodeTitle(data: CanvasNodeData) {
  return data.label || '未命名节点'
}

function normalizeCanvasViewport(value: unknown): Viewport | undefined {
  if (!value || typeof value !== 'object') return undefined
  const row = value as Record<string, unknown>
  const x = Number(row.x)
  const y = Number(row.y)
  const zoom = Number(row.zoom)
  if (
    !Number.isFinite(x) ||
    !Number.isFinite(y) ||
    !Number.isFinite(zoom) ||
    zoom <= 0
  ) {
    return undefined
  }
  return { x, y, zoom }
}

function normalizeCanvasBackgroundMode(value: unknown): CanvasBackgroundMode {
  return value === 'lines' ||
    value === 'dots' ||
    value === 'blank' ||
    value === 'net'
    ? value
    : 'net'
}

function clampCanvasZoom(value: number) {
  return Math.min(5, Math.max(0.05, value))
}

function cloneCanvasNodes(items: StudioCanvasNode[]) {
  return structuredClone(items)
}

function cloneCanvasEdges(items: Edge[]) {
  return structuredClone(items)
}

function isEditableKeyboardTarget(target: EventTarget | null) {
  if (!(target instanceof HTMLElement)) return false
  return (
    target.isContentEditable ||
    target.matches('input, textarea, select, [role="textbox"]')
  )
}

function cloneCanvasViewport(viewport?: Viewport) {
  return viewport ? ({ ...viewport } as Viewport) : undefined
}

function buildCanvasHistoryEntry(
  nodes: StudioCanvasNode[],
  edges: Edge[],
  viewport: Viewport | undefined,
  backgroundMode: CanvasBackgroundMode
): CanvasHistoryEntry {
  return {
    nodes: cloneCanvasNodes(nodes),
    edges: cloneCanvasEdges(edges),
    viewport: cloneCanvasViewport(viewport),
    backgroundMode,
  }
}

function safeCanvasFileName(value: string) {
  return (
    value
      .trim()
      .replaceAll(/[\\/:*?"<>|]+/g, '-')
      .replaceAll(/\s+/g, '-')
      .slice(0, 72) || 'yucore-canvas'
  )
}

function isPlainRecord(value: unknown): value is Record<string, unknown> {
  return Boolean(value && typeof value === 'object' && !Array.isArray(value))
}

function importedReferenceNodeToFlowNode(
  row: Record<string, unknown>,
  index: number
): StudioCanvasNode {
  const id = String(row.id || `imported_${Date.now().toString(36)}_${index}`)
  const position = isPlainRecord(row.position) ? row.position : {}
  const metadata = isPlainRecord(row.metadata) ? row.metadata : {}
  const rawType = String(row.type || metadata.generationMode || 'text')
  const kind: CanvasNodeData['kind'] =
    rawType === 'image' || rawType === 'video' || rawType === 'prompt'
      ? rawType
      : 'text'
  const prompt = String(
    metadata.prompt ||
      metadata.composerContent ||
      metadata.content ||
      row.prompt ||
      ''
  )
  return {
    id,
    type: 'media',
    position: {
      x: Number(position.x) || Math.round((index % 4) * 280 - 320),
      y: Number(position.y) || Math.round(Math.floor(index / 4) * 220 - 120),
    },
    data: {
      kind,
      label: String(row.title || row.label || id),
      sublabel: rawType === 'config' ? '生成配置' : rawType,
      prompt,
      status: 'imported',
    },
    style: {
      width: Number(row.width) || 220,
      border: '1px solid rgb(103 232 249 / 0.26)',
      borderRadius: 16,
      background: 'rgb(5 9 16 / 0.9)',
      color: 'white',
      padding: 14,
      whiteSpace: 'pre-line',
      boxShadow: '0 18px 52px rgb(0 0 0 / 0.28)',
    },
  }
}

function normalizeImportedCanvasNodes(value: unknown): StudioCanvasNode[] {
  if (!Array.isArray(value)) return []
  return value.filter(isPlainRecord).map((row, index) => {
    if (isPlainRecord(row.data) && isPlainRecord(row.position)) {
      return {
        ...row,
        id: String(row.id || `node_${Date.now().toString(36)}_${index}`),
        type: 'media',
        data: {
          label: String(row.data.label || row.id || `节点 ${index + 1}`),
          ...row.data,
        },
      } as StudioCanvasNode
    }
    return importedReferenceNodeToFlowNode(row, index)
  })
}

function normalizeImportedCanvasEdges(value: unknown): Edge[] {
  if (!Array.isArray(value)) return []
  return value.filter(isPlainRecord).flatMap((row, index) => {
    const source = String(row.source || row.fromNodeId || '')
    const target = String(row.target || row.toNodeId || '')
    if (!source || !target) return []
    return [
      {
        id: String(row.id || `edge_${Date.now().toString(36)}_${index}`),
        source,
        target,
        animated: true,
        type: 'smoothstep',
        markerEnd: { type: MarkerType.ArrowClosed, color: '#67e8f9' },
        style: { stroke: '#67e8f9', opacity: 0.72 },
      },
    ]
  })
}

function normalizeImportedCanvasPayload(value: unknown) {
  if (!isPlainRecord(value)) return null
  const payload = value as ImportedCanvasPayload
  const snapshot = isPlainRecord(payload.snapshot)
    ? payload.snapshot
    : undefined
  const nodes = normalizeImportedCanvasNodes(snapshot?.nodes ?? payload.nodes)
  const edges = normalizeImportedCanvasEdges(
    snapshot?.edges ?? payload.edges ?? payload.connections
  )
  if (nodes.length === 0 && edges.length === 0) return null
  return {
    title: typeof payload.title === 'string' ? payload.title : undefined,
    description:
      typeof payload.description === 'string' ? payload.description : undefined,
    module: typeof payload.module === 'string' ? payload.module : undefined,
    nodes,
    edges,
    viewport: normalizeCanvasViewport(payload.viewport),
    backgroundMode: normalizeCanvasBackgroundMode(
      snapshot?.backgroundMode ??
        payload.backgroundMode ??
        payload.background_mode
    ),
  }
}

function metadataString(
  metadata: Record<string, unknown> | undefined,
  key: string
) {
  const value = metadata?.[key]
  return typeof value === 'string' && value.trim() ? value.trim() : undefined
}

function metadataNumber(
  metadata: Record<string, unknown> | undefined,
  key: string
) {
  const value = metadata?.[key]
  if (typeof value === 'number' && Number.isFinite(value)) return value
  if (typeof value === 'string' && value.trim()) {
    const next = Number(value)
    if (Number.isFinite(next)) return next
  }
  return undefined
}

function isTerminalMediaTask(task: YucoreMediaTask) {
  return (
    task.status === 'completed' ||
    task.status === 'failed' ||
    task.status === 'canceled'
  )
}

function isActiveMediaTask(task: YucoreMediaTask) {
  return !isTerminalMediaTask(task)
}

function mediaTaskRouteLabel(task: YucoreMediaTask) {
  return task.group ? `${task.group} / ${task.model_id}` : task.model_id
}

function compactTaskError(error: string) {
  const trimmed = error.trim()
  return trimmed.length > 140 ? `${trimmed.slice(0, 137)}...` : trimmed
}

function buildCanvasTaskStatus(task: YucoreMediaTask) {
  if (task.status === 'completed') {
    const assetLabel = `${task.assets.length} asset${task.assets.length === 1 ? '' : 's'}`
    return `${task.task_id} / completed / ${assetLabel}`
  }
  if (task.status === 'canceled') return `${task.task_id} / canceled`
  return `${task.task_id} / failed${task.error ? ` / ${compactTaskError(task.error)}` : ''}`
}

function buildAgentRunSummary(task: YucoreMediaTask) {
  if (task.status === 'completed') {
    const assetLabel = `${task.assets.length} asset${task.assets.length === 1 ? '' : 's'}`
    return `Media task ${task.task_id} completed with ${assetLabel}.`
  }
  if (task.status === 'canceled') {
    return `Media task ${task.task_id} was canceled.`
  }
  return `Media task ${task.task_id} failed${task.error ? `: ${compactTaskError(task.error)}` : '.'}`
}

function buildAgentRunActions(
  run: YucoreCanvasAgentRun | undefined,
  task: YucoreMediaTask
) {
  const resultStatus = task.status === 'completed' ? 'completed' : 'failed'
  const resultAssets = task.assets.map((asset) => ({
    id: asset.id,
    kind: asset.kind,
    url: asset.url,
    thumb_url: asset.thumb_url,
  }))
  const generationAction = {
    tool: 'canvas_run_generation',
    status: resultStatus,
    task_id: task.task_id,
    assets: resultAssets,
    error: task.status === 'failed' ? task.error : undefined,
  }
  let patchedGeneration = false
  let hasApplyResult = false
  const actions = (run?.actions?.length ? run.actions : []).map((action) => {
    if (
      action.tool === 'canvas_apply_result' &&
      action.task_id === task.task_id
    ) {
      hasApplyResult = true
    }
    if (
      action.tool === 'canvas_run_generation' ||
      (!action.tool && action.task_id === task.task_id)
    ) {
      patchedGeneration = true
      return { ...action, ...generationAction }
    }
    return action
  })

  if (!patchedGeneration) actions.push(generationAction)
  if (
    task.status === 'completed' &&
    resultAssets.length > 0 &&
    !hasApplyResult
  ) {
    actions.push({
      tool: 'canvas_apply_result',
      status: 'completed',
      task_id: task.task_id,
      asset_ids: resultAssets.map((asset) => asset.id),
    })
  }
  return actions
}

function getAssetExtension(asset: YucoreMediaAsset) {
  const mime = asset.mime_type?.split(';')[0]?.trim().toLowerCase()
  const byMime: Record<string, string> = {
    'image/gif': 'gif',
    'image/jpeg': 'jpg',
    'image/png': 'png',
    'image/svg+xml': 'svg',
    'image/webp': 'webp',
    'video/mp4': 'mp4',
    'video/quicktime': 'mov',
    'video/webm': 'webm',
  }
  if (mime && byMime[mime]) return byMime[mime]

  try {
    const baseUrl =
      typeof window === 'undefined' ? 'http://localhost' : window.location.href
    const pathname = new URL(asset.url, baseUrl).pathname
    const extension = pathname.match(/\.([a-z0-9]{2,5})$/i)?.[1]
    if (extension) return extension.toLowerCase()
  } catch {
    // Ignore malformed or relative URLs and fall back by asset kind.
  }

  return asset.kind === 'video' ? 'mp4' : 'png'
}

function getAssetDownloadName(asset: YucoreMediaAsset) {
  const base = (asset.label || asset.id)
    .trim()
    .replaceAll(/[\\/:*?"<>|]+/g, '-')
    .replaceAll(/\s+/g, '-')
    .slice(0, 72)

  return `${base || asset.id}.${getAssetExtension(asset)}`
}

function AssetPreview({
  asset,
  title,
}: {
  asset: YucoreMediaAsset
  title: string
}) {
  if (asset.kind === 'video') {
    return (
      <video
        src={asset.url}
        poster={asset.thumb_url}
        title={title}
        className='h-full w-full object-contain transition duration-500 group-hover:scale-[1.02]'
        controls
        muted
        playsInline
        preload='metadata'
      />
    )
  }

  return (
    <img
      src={asset.thumb_url || asset.url}
      alt={title}
      className='h-full w-full object-contain transition duration-500 group-hover:scale-[1.02]'
    />
  )
}

function MediaCanvasNode({ data }: NodeProps<StudioCanvasNode>) {
  const kind = data.kind ?? 'text'
  return (
    <div className='yucore-media-canvas-node'>
      <Handle type='target' position={Position.Left} />
      {data.assetUrl && (
        <div className='mb-2 aspect-[4/3] overflow-hidden rounded-lg border border-white/10 bg-black/50'>
          {kind === 'video' ? (
            <video
              src={data.assetUrl}
              poster={data.thumbUrl}
              title={getCanvasNodeTitle(data)}
              className='h-full w-full object-contain'
              controls
              muted
              playsInline
              preload='metadata'
            />
          ) : (
            <img
              src={data.thumbUrl || data.assetUrl}
              alt={getCanvasNodeTitle(data)}
              className='h-full w-full object-contain'
            />
          )}
        </div>
      )}
      <div className='flex items-start justify-between gap-3'>
        <div className='min-w-0'>
          <div className='truncate text-[13px] font-semibold text-white'>
            {getCanvasNodeTitle(data)}
          </div>
          {data.sublabel && (
            <div className='mt-1 truncate text-[11px] text-white/42'>
              {data.sublabel}
            </div>
          )}
        </div>
        <span
          className={cn(
            'rounded-md border px-1.5 py-0.5 text-[10px]',
            kind === 'image' && 'border-emerald-200/20 text-emerald-100',
            kind === 'video' && 'border-violet-200/20 text-violet-100',
            kind !== 'image' &&
              kind !== 'video' &&
              'border-cyan-200/20 text-cyan-100'
          )}
        >
          {kind}
        </span>
      </div>
      {data.prompt && (
        <p className='mt-2 line-clamp-3 text-[11px] leading-5 text-white/48'>
          {data.prompt}
        </p>
      )}
      {data.status && (
        <div className='mt-2 text-[10px] text-white/34'>{data.status}</div>
      )}
      <Handle type='source' position={Position.Right} />
    </div>
  )
}

const canvasNodeTypes = {
  media: MediaCanvasNode,
}

function TaskStatusBadge({ task }: { task: YucoreMediaTask }) {
  let label = '处理中'
  if (task.status === 'completed') label = '已完成'
  if (task.status === 'failed') label = '失败'
  if (task.status === 'canceled') label = '已取消'
  if (task.status === 'pending') label = '排队中'

  return (
    <span
      className={cn(
        'inline-flex h-6 items-center rounded-md border px-2 text-[11px] font-medium',
        task.status === 'completed' &&
          'border-emerald-200/25 bg-emerald-300/10 text-emerald-100',
        task.status === 'failed' &&
          'border-rose-200/25 bg-rose-300/10 text-rose-100',
        task.status !== 'completed' &&
          task.status !== 'failed' &&
          'border-cyan-200/25 bg-cyan-300/10 text-cyan-100'
      )}
    >
      {label}
    </span>
  )
}

function ProviderBadge({
  task,
  asset,
}: {
  task: YucoreMediaTask
  asset?: YucoreMediaAsset
}) {
  const adapter = String(
    asset?.metadata?.adapter ?? task.metadata?.adapter ?? 'mock'
  )
  const isMock = adapter === 'mock' || asset?.metadata?.mock === true
  let label = '真实上游'
  if (isMock) label = '本地模拟'
  if (!isMock && adapter === 'uag-proxy') label = 'UAG 上游'
  if (!isMock && adapter === 'openai-compatible') label = 'OpenAI 兼容'

  return (
    <span
      className={cn(
        'inline-flex h-6 items-center rounded-md border px-2 text-[11px] font-medium',
        isMock
          ? 'border-amber-200/25 bg-amber-300/10 text-amber-100'
          : 'border-violet-200/25 bg-violet-300/10 text-violet-100'
      )}
    >
      {label}
    </span>
  )
}

function SegmentedButton(props: {
  active: boolean
  children: ReactNode
  onClick: () => void
  className?: string
}) {
  return (
    <button
      type='button'
      onClick={props.onClick}
      className={cn(
        'h-9 rounded-lg border px-3 text-xs font-medium transition',
        props.active
          ? 'border-white/25 bg-white text-black shadow-[0_0_24px_rgba(255,255,255,0.16)]'
          : 'border-white/10 bg-white/[0.035] text-white/62 hover:border-cyan-200/25 hover:bg-cyan-300/10 hover:text-white',
        props.className
      )}
    >
      {props.children}
    </button>
  )
}

function AssetGrid({
  tasks,
  compact = false,
  onSendToCanvas,
}: {
  tasks: YucoreMediaTask[]
  compact?: boolean
  onSendToCanvas?: (task: YucoreMediaTask, asset: YucoreMediaAsset) => void
}) {
  const assets = tasks.flatMap((task) =>
    task.assets.map((asset) => ({ asset, task }))
  )

  if (assets.length === 0) {
    return (
      <div className='grid min-h-[20rem] place-items-center rounded-2xl border border-dashed border-white/12 bg-white/[0.025] text-center'>
        <div>
          <ImagePlus className='mx-auto mb-3 size-9 text-white/32' />
          <div className='text-sm font-semibold text-white'>还没有生成素材</div>
          <div className='mt-1 text-xs text-white/42'>
            从生图工作台提交后会自动沉淀到这里。
          </div>
        </div>
      </div>
    )
  }

  return (
    <div
      className={cn(
        'grid gap-3',
        compact ? 'grid-cols-2' : 'grid-cols-2 xl:grid-cols-3'
      )}
    >
      {assets.map(({ asset, task }) => (
        <article
          key={asset.id}
          className='group overflow-hidden rounded-2xl border border-white/10 bg-white/[0.035]'
        >
          <div className='aspect-[4/3] overflow-hidden bg-black/45'>
            <AssetPreview asset={asset} title={asset.label} />
          </div>
          <div className='space-y-2 p-3'>
            <div className='flex items-center justify-between gap-2'>
              <span className='truncate text-xs font-semibold text-white'>
                {asset.label}
              </span>
              <div className='flex shrink-0 items-center gap-1'>
                <ProviderBadge task={task} asset={asset} />
                <TaskStatusBadge task={task} />
              </div>
            </div>
            <p className='line-clamp-2 text-xs leading-5 text-white/45'>
              {task.prompt}
            </p>
            <div className='flex items-center gap-2 pt-1'>
              {onSendToCanvas && (
                <Button
                  size='sm'
                  variant='outline'
                  className='h-7 rounded-lg border-white/10 bg-white/[0.035] px-2 text-[11px] text-white hover:bg-white/10'
                  onClick={() => onSendToCanvas(task, asset)}
                >
                  <Send data-icon='inline-start' />
                  放入画布
                </Button>
              )}
              <Button
                size='sm'
                variant='outline'
                className='h-7 rounded-lg border-white/10 bg-white/[0.035] px-2 text-[11px] text-white hover:bg-white/10'
                render={
                  <a href={asset.url} download={getAssetDownloadName(asset)} />
                }
              >
                <Download data-icon='inline-start' />
                下载
              </Button>
            </div>
          </div>
        </article>
      ))}
    </div>
  )
}

export function YucoreStudioWorkspace({
  initialView = 'home',
}: {
  initialView?: StudioView
}) {
  const { t } = useTranslation()
  const [view, setView] = useState<StudioView>(initialView)
  const [sessionId] = useState(getStudioSessionId)
  const [mediaCatalog, setMediaCatalog] =
    useState<YucoreMediaCatalog>(emptyMediaCatalog)
  const [selectedGroup, setSelectedGroup] = useState('')
  const [isMediaCatalogLoading, setIsMediaCatalogLoading] = useState(true)
  const [templates, setTemplates] =
    useState<YucorePromptTemplate[]>(defaultTemplates)
  const [billing, setBilling] = useState<YucoreMediaBilling | null>(null)
  const [mediaHealth, setMediaHealth] = useState<YucoreMediaHealth | null>(null)
  const [tasks, setTasks] = useState<YucoreMediaTask[]>([])
  const [galleryTasks, setGalleryTasks] = useState<YucoreMediaTask[]>([])
  const [canvases, setCanvases] = useState<YucoreCanvasRecord[]>([])
  const [agentRuns, setAgentRuns] = useState<YucoreCanvasAgentRun[]>([])
  const [activeCanvas, setActiveCanvas] = useState<YucoreCanvasRecord | null>(
    null
  )
  const activeCanvasRef = useRef<YucoreCanvasRecord | null>(null)
  const [canvasVersions, setCanvasVersions] = useState<
    YucoreCanvasVersionRecord[]
  >([])
  const [canvasIdentity, setCanvasIdentity] =
    useState<YucoreCanvasIdentity | null>(null)
  const [nodes, setNodes, onNodesChange] = useNodesState(baseCanvasNodes)
  const [edges, setEdges, onEdgesChange] = useEdgesState(baseCanvasEdges)
  const fileInputRef = useRef<HTMLInputElement | null>(null)
  const canvasImportInputRef = useRef<HTMLInputElement | null>(null)
  const canvasMediaInputRef = useRef<HTMLInputElement | null>(null)
  const referenceFilesRef = useRef<ReferenceAssetDraft[]>([])
  const appliedTaskBackflowRef = useRef(new Set<string>())
  const isApplyingCanvasHistoryRef = useRef(false)
  const canvasClipboardRef = useRef<CanvasClipboard | null>(null)
  const reactFlowInstanceRef = useRef<ReactFlowInstance<
    StudioCanvasNode,
    Edge
  > | null>(null)
  const [prompt, setPrompt] = useState(
    '直闪胶片质感的成年人生活方式人像，室内真实快照，硬闪光，轻微颗粒，暗部保留噪点，自然动作。'
  )
  const [negativePrompt, setNegativePrompt] = useState(
    '塑料皮肤，畸形手指，多余肢体，过度 HDR，商业棚拍感'
  )
  const [kind, setKind] = useState<'image' | 'video'>('image')
  const [imageModelId, setImageModelId] = useState('')
  const [videoModelId, setVideoModelId] = useState('')
  const [mode, setMode] = useState('text-to-image')
  const [aspectRatio, setAspectRatio] = useState('auto')
  const [size, setSize] = useState('1k')
  const [format, setFormat] = useState('png')
  const [quality, setQuality] = useState('high')
  const [stylePreset, setStylePreset] = useState('auto')
  const [background, setBackground] = useState('auto')
  const [moderation, setModeration] = useState('auto')
  const [streamMode, setStreamMode] = useState('final')
  const [partialImages, setPartialImages] = useState(0)
  const [duration, setDuration] = useState(8)
  const [generateAudio, setGenerateAudio] = useState(true)
  const [visibility, setVisibility] = useState('private')
  const [count, setCount] = useState(3)
  const [referenceFiles, setReferenceFiles] = useState<ReferenceAssetDraft[]>(
    []
  )
  const [activeTask, setActiveTask] = useState<YucoreMediaTask | null>(null)
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [isSavingCanvas, setIsSavingCanvas] = useState(false)
  const [isCanvasDirty, setIsCanvasDirty] = useState(false)
  const [isCanvasMediaUploading, setIsCanvasMediaUploading] = useState(false)
  const [isCanvasMenuOpen, setIsCanvasMenuOpen] = useState(false)
  const [isShortcutsOpen, setIsShortcutsOpen] = useState(false)
  const [isLoadingCanvasVersions, setIsLoadingCanvasVersions] = useState(false)
  const [isAgentPanelOpen, setIsAgentPanelOpen] = useState(
    () =>
      initialView === 'canvas' &&
      (typeof window === 'undefined' || window.innerWidth >= 768)
  )
  const [agentTab, setAgentTab] = useState<CanvasAgentTab>('chat')
  const [agentMode, setAgentMode] = useState<'site' | 'local'>('site')
  const [toolConfirmationEnabled, setToolConfirmationEnabled] = useState(true)
  const [agentPrompt, setAgentPrompt] = useState('')
  const [selectedCanvasTool, setSelectedCanvasTool] =
    useState<CanvasToolId>('move')
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null)
  const [canvasViewport, setCanvasViewport] = useState<Viewport | undefined>()
  const [canvasMiniMapOpen, setCanvasMiniMapOpen] = useState(
    () => typeof window === 'undefined' || window.innerWidth >= 768
  )
  const [canvasBackgroundMode, setCanvasBackgroundMode] =
    useState<CanvasBackgroundMode>('net')
  const [canvasPast, setCanvasPast] = useState<CanvasHistoryEntry[]>([])
  const [canvasFuture, setCanvasFuture] = useState<CanvasHistoryEntry[]>([])
  const [reactFlowInstance, setReactFlowInstance] = useState<ReactFlowInstance<
    StudioCanvasNode,
    Edge
  > | null>(null)

  const models = useMemo(
    () =>
      mediaCatalog.groups.find((group) => group.id === selectedGroup)?.models ??
      [],
    [mediaCatalog.groups, selectedGroup]
  )
  const imageModels = useMemo(
    () => modelsForKind(mediaCatalog, selectedGroup, 'image'),
    [mediaCatalog, selectedGroup]
  )
  const videoModels = useMemo(
    () => modelsForKind(mediaCatalog, selectedGroup, 'video'),
    [mediaCatalog, selectedGroup]
  )
  const imageModel = useMemo(
    () => getModelById(models, imageModelId, 'image'),
    [imageModelId, models]
  )
  const videoModel = useMemo(
    () => getModelById(models, videoModelId, 'video'),
    [models, videoModelId]
  )
  const activeModel =
    view === 'video' || kind === 'video' ? videoModel : imageModel
  const activeModelId = activeModel?.id ?? ''
  const availableModels =
    view === 'video' || kind === 'video' ? videoModels : imageModels
  const activeMediaKind =
    view === 'video' || kind === 'video' ? 'video' : 'image'
  let mediaModelsEmptyMessage = t(
    'No image models are available in this group.'
  )
  if (activeMediaKind === 'video') {
    mediaModelsEmptyMessage = t('No video models are available in this group.')
  }
  if (isMediaCatalogLoading) {
    mediaModelsEmptyMessage = t('Loading media catalog')
  }
  const activeModes = activeModel?.modes ?? emptyStringOptions
  const activeAspectRatios = getAspectRatios(activeModel)
  const activeSizes = getSizes(activeModel)
  const activeFormats = getFormats(activeModel)
  const activeQualities = activeModel?.qualities ?? emptyStringOptions
  const activeStylePresets = activeModel?.style_presets ?? emptyStringOptions
  const activeStreamModes = activeModel?.stream_modes ?? emptyStringOptions
  const activeBackgrounds = activeModel?.backgrounds ?? emptyStringOptions
  const activeModerations = activeModel?.moderations ?? emptyStringOptions
  const activeDurations = activeModel?.durations ?? emptyNumberOptions
  const activeCounts = activeModel?.counts ?? emptyNumberOptions
  const maxReferenceImages =
    activeModel?.input_limits?.max_reference_images ?? 0
  const isReferenceUploading = referenceFiles.some((file) => file.isUploading)
  const referenceUploadErrors = referenceFiles.filter(
    (file) => file.uploadError
  )
  const isSubmitDisabled =
    isSubmitting ||
    isReferenceUploading ||
    referenceUploadErrors.length > 0 ||
    !prompt.trim() ||
    !selectedGroup ||
    !activeModel
  const visibleTasks = useMemo(
    () =>
      tasks.filter((task) =>
        view === 'video' ? task.kind === 'video' : task.kind === 'image'
      ),
    [tasks, view]
  )
  const completedTasks = useMemo(() => {
    const seen = new Set<string>()
    return [
      ...galleryTasks,
      ...tasks.filter((task) => task.status === 'completed'),
    ].filter((task) => {
      if (seen.has(task.task_id)) return false
      seen.add(task.task_id)
      return true
    })
  }, [galleryTasks, tasks])
  const selectedNode = useMemo(
    () =>
      selectedNodeId
        ? (nodes.find((node) => node.id === selectedNodeId) as
            | StudioCanvasNode
            | undefined)
        : undefined,
    [nodes, selectedNodeId]
  )
  const effectiveCanvasViewport = useMemo(
    () => canvasViewport ?? { x: 0, y: 0, zoom: 1 },
    [canvasViewport]
  )
  let canvasSyncLabel = '本地草稿'
  if (isSavingCanvas) {
    canvasSyncLabel = '正在同步画布'
  } else if (isCanvasDirty) {
    canvasSyncLabel = activeCanvas
      ? '未保存更改，稍后自动同步'
      : '本地草稿，保存后刷新可保留'
  } else if (activeCanvas) {
    canvasSyncLabel = `已同步 v${activeCanvas.revision}`
  }
  const canvasZoomPercent = Math.round(effectiveCanvasViewport.zoom * 100)
  const canUndoCanvas = canvasPast.length > 0
  const canRedoCanvas = canvasFuture.length > 0
  const canvasBackgroundModes: Array<{
    icon: LucideIcon
    id: CanvasBackgroundMode
    label: string
  }> = [
    { id: 'net', icon: Boxes, label: '能量网' },
    { id: 'lines', icon: Braces, label: '线网' },
    { id: 'dots', icon: CirclePlus, label: '点阵' },
    { id: 'blank', icon: Eraser, label: '空白' },
  ]

  const refreshTasks = useCallback(async () => {
    const payload = await listYucoreMediaTasks({
      session_id: sessionId,
      page_size: 40,
    })
    setTasks(payload.items)
    return payload.items
  }, [sessionId])

  const refreshGallery = useCallback(async () => {
    const payload = await listYucoreMediaGallery({ page_size: 40 })
    setGalleryTasks(payload.items)
  }, [])

  const applyCanvasRecordToEditor = useCallback(
    (canvas: YucoreCanvasRecord | null) => {
      const snapshotNodes = Array.isArray(canvas?.snapshot?.nodes)
        ? (canvas.snapshot.nodes as StudioCanvasNode[])
        : []
      const snapshotEdges = Array.isArray(canvas?.snapshot?.edges)
        ? (canvas.snapshot.edges as Edge[])
        : []
      setNodes(snapshotNodes.length > 0 ? snapshotNodes : baseCanvasNodes)
      setEdges(snapshotEdges.length > 0 ? snapshotEdges : baseCanvasEdges)
      const nextViewport = normalizeCanvasViewport(canvas?.viewport)
      setCanvasViewport(nextViewport)
      if (nextViewport) {
        void reactFlowInstanceRef.current?.setViewport(nextViewport)
      } else {
        void reactFlowInstanceRef.current?.fitView({ padding: 0.32 })
      }
      setCanvasBackgroundMode(
        normalizeCanvasBackgroundMode(canvas?.snapshot?.backgroundMode)
      )
      setCanvasPast([])
      setCanvasFuture([])
      setSelectedNodeId(null)
      setIsCanvasDirty(false)
    },
    [setEdges, setNodes]
  )

  useEffect(() => {
    activeCanvasRef.current = activeCanvas
  }, [activeCanvas])

  const refreshCanvases = useCallback(async () => {
    const payload = await listYucoreCanvases()
    setCanvases(payload.items)
    const currentCanvas = activeCanvasRef.current
    const nextCanvas = currentCanvas
      ? (payload.items.find((item) => item.id === currentCanvas.id) ??
        payload.items[0] ??
        null)
      : (payload.items[0] ?? null)
    setActiveCanvas(nextCanvas)
    if (currentCanvas?.id !== nextCanvas?.id) {
      applyCanvasRecordToEditor(nextCanvas)
    }
  }, [applyCanvasRecordToEditor])

  const refreshAgentRuns = useCallback(
    async (canvasId = activeCanvas?.id) => {
      if (!canvasId) {
        setAgentRuns([])
        return []
      }
      const payload = await listYucoreCanvasAgentRuns(canvasId, {
        page_size: 20,
      })
      setAgentRuns(payload.items)
      return payload.items
    },
    [activeCanvas?.id]
  )

  const refreshCanvasVersions = useCallback(
    async (canvasId = activeCanvas?.id) => {
      if (!canvasId) {
        setCanvasVersions([])
        return []
      }
      setIsLoadingCanvasVersions(true)
      try {
        const payload = await listYucoreCanvasVersions(canvasId, {
          page_size: 12,
        })
        setCanvasVersions(payload.items)
        return payload.items
      } finally {
        setIsLoadingCanvasVersions(false)
      }
    },
    [activeCanvas?.id]
  )

  const openCanvasRecord = useCallback(
    async (canvas: YucoreCanvasRecord) => {
      setActiveCanvas(canvas)
      applyCanvasRecordToEditor(canvas)
      setIsCanvasMenuOpen(false)
      try {
        const latest = await getYucoreCanvas(canvas.id)
        setActiveCanvas(latest)
        applyCanvasRecordToEditor(latest)
        setCanvases((items) =>
          items.map((item) => (item.id === latest.id ? latest : item))
        )
        void refreshAgentRuns(latest.id)
        void refreshCanvasVersions(latest.id)
      } catch (error) {
        toast.error(error instanceof Error ? error.message : '打开画布失败')
      }
    },
    [applyCanvasRecordToEditor, refreshAgentRuns, refreshCanvasVersions]
  )

  async function handleRefreshCanvas() {
    if (!activeCanvas) {
      await refreshCanvases()
      toast.success('画布库已刷新')
      return
    }
    try {
      const latest = await getYucoreCanvas(activeCanvas.id)
      setActiveCanvas(latest)
      applyCanvasRecordToEditor(latest)
      setCanvases((items) =>
        items.map((item) => (item.id === latest.id ? latest : item))
      )
      void refreshAgentRuns(latest.id)
      void refreshCanvasVersions(latest.id)
      toast.success('画布已从服务端刷新')
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '刷新画布失败')
    }
  }

  async function handleRenameCanvas() {
    if (!activeCanvas) {
      toast.info('请先保存一个画布')
      return
    }
    const nextTitle = window
      .prompt('输入新的画布名称', activeCanvas.title)
      ?.trim()
    if (!nextTitle || nextTitle === activeCanvas.title) return
    try {
      const renamed = await updateYucoreCanvas(activeCanvas.id, {
        title: nextTitle,
        description: activeCanvas.description,
        module: activeCanvas.module,
        snapshot: activeCanvas.snapshot,
        viewport: activeCanvas.viewport,
        autosave: true,
      })
      setActiveCanvas(renamed)
      setCanvases((items) =>
        items.map((item) => (item.id === renamed.id ? renamed : item))
      )
      toast.success('画布已重命名')
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '重命名失败')
    }
  }

  async function handleDeleteCanvas() {
    if (!activeCanvas) {
      toast.info('当前还是本地草稿')
      return
    }
    const confirmed = window.confirm(
      `删除画布「${activeCanvas.title}」？这个操作不会删除已生成素材。`
    )
    if (!confirmed) return
    try {
      await deleteYucoreCanvas(activeCanvas.id)
      const nextCanvases = canvases.filter(
        (item) => item.id !== activeCanvas.id
      )
      setCanvases(nextCanvases)
      const nextCanvas = nextCanvases[0] ?? null
      setActiveCanvas(nextCanvas)
      applyCanvasRecordToEditor(nextCanvas)
      setCanvasVersions([])
      setAgentRuns([])
      toast.success('画布已删除')
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '删除画布失败')
    }
  }

  async function handleRestoreCanvasVersion(
    version: YucoreCanvasVersionRecord
  ) {
    if (!activeCanvas) return
    const confirmed = window.confirm(
      `恢复到 v${version.revision}？当前画布会保存一个新的服务端版本。`
    )
    if (!confirmed) return
    try {
      const restored = await updateYucoreCanvas(activeCanvas.id, {
        title: activeCanvas.title,
        description: activeCanvas.description,
        module: version.module || activeCanvas.module,
        snapshot: version.snapshot,
        viewport: version.viewport,
        autosave: false,
      })
      setActiveCanvas(restored)
      applyCanvasRecordToEditor(restored)
      setCanvases((items) =>
        items.map((item) => (item.id === restored.id ? restored : item))
      )
      await refreshCanvasVersions(restored.id)
      toast.success(`已恢复到 v${version.revision}`)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '恢复版本失败')
    }
  }

  function handleExportCanvas() {
    const payload = {
      app: 'yucore-canvas',
      version: 1,
      exported_at: new Date().toISOString(),
      title: activeCanvas?.title ?? 'YuCore Canvas',
      description: activeCanvas?.description ?? '',
      module: activeCanvas?.module ?? '无限画布',
      snapshot: {
        nodes: nodes as StudioCanvasNode[],
        edges,
        backgroundMode: canvasBackgroundMode,
      },
      viewport: effectiveCanvasViewport,
    }
    const blob = new Blob([JSON.stringify(payload, null, 2)], {
      type: 'application/json;charset=utf-8',
    })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `${safeCanvasFileName(payload.title)}.json`
    document.body.appendChild(link)
    link.click()
    link.remove()
    URL.revokeObjectURL(url)
    toast.success('画布已导出')
  }

  async function handleCanvasImportChange(
    event: ChangeEvent<HTMLInputElement>
  ) {
    const file = event.target.files?.[0]
    event.target.value = ''
    if (!file) return
    try {
      const imported = normalizeImportedCanvasPayload(
        JSON.parse(await file.text())
      )
      if (!imported) {
        toast.error('没有读取到可导入的画布节点')
        return
      }
      commitCanvasHistory()
      setNodes(imported.nodes)
      setEdges(imported.edges)
      setCanvasViewport(imported.viewport)
      setCanvasBackgroundMode(imported.backgroundMode)
      setSelectedNodeId(null)
      setIsCanvasMenuOpen(false)
      setView('canvas')
      markCanvasDirty()
      const synced = await persistCanvasSnapshot(
        imported.nodes,
        imported.edges,
        '导入画布',
        {
          autosave: false,
          createIfMissing: true,
          viewport: imported.viewport,
          backgroundMode: imported.backgroundMode,
        }
      )
      if (synced && imported.title && synced.title !== imported.title) {
        const renamed = await updateYucoreCanvas(synced.id, {
          title: imported.title,
          description: imported.description ?? synced.description,
          module: imported.module ?? synced.module,
          snapshot: {
            nodes: imported.nodes,
            edges: imported.edges,
            backgroundMode: imported.backgroundMode,
          },
          viewport: imported.viewport ?? {},
          autosave: false,
        })
        setActiveCanvas(renamed)
        setCanvases((items) =>
          items.map((item) => (item.id === renamed.id ? renamed : item))
        )
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '导入画布失败')
    }
  }

  function handleCanvasBackgroundModeChange(mode: CanvasBackgroundMode) {
    if (mode === canvasBackgroundMode) return
    commitCanvasHistory()
    setCanvasBackgroundMode(mode)
    markCanvasDirty()
  }

  function organizeCanvas() {
    const sourceNodes = nodes as StudioCanvasNode[]
    if (sourceNodes.length === 0) {
      toast.info('当前画布没有可整理的节点')
      return
    }
    commitCanvasHistory()
    const columns = Math.max(1, Math.ceil(Math.sqrt(sourceNodes.length)))
    const startX = -Math.round(((columns - 1) * 280) / 2)
    const nextNodes = sourceNodes.map((node, index) => ({
      ...node,
      position: {
        x: startX + (index % columns) * 280,
        y: -160 + Math.floor(index / columns) * 220,
      },
    }))
    setNodes(nextNodes)
    setSelectedNodeId(null)
    markCanvasDirty()
    void reactFlowInstance?.fitView({ padding: 0.34, duration: 260 })
    toast.success('画布已整理')
  }

  useEffect(() => {
    let mounted = true

    async function load() {
      const [
        catalogResult,
        templatesResult,
        billingResult,
        healthResult,
        identityResult,
      ] = await Promise.allSettled([
        getYucoreMediaCatalog(),
        listYucorePromptTemplates(),
        getYucoreMediaBilling(),
        getYucoreMediaHealth(),
        getYucoreCanvasIdentity(),
      ])
      if (!mounted) return
      let loadedModels: YucoreMediaModel[] = []

      if (catalogResult.status === 'fulfilled') {
        const catalog = catalogResult.value
        const nextGroup = catalog.default_group || catalog.groups[0]?.id || ''
        const imageSelection = resolveMediaSelection(
          catalog,
          nextGroup,
          '',
          'image'
        )
        const videoSelection = resolveMediaSelection(
          catalog,
          nextGroup,
          '',
          'video'
        )
        loadedModels =
          catalog.groups.find((group) => group.id === nextGroup)?.models ?? []
        setMediaCatalog(catalog)
        setSelectedGroup(nextGroup)
        setImageModelId(imageSelection.modelId)
        setVideoModelId(videoSelection.modelId)
      } else {
        toast.error(
          catalogResult.reason instanceof Error
            ? catalogResult.reason.message
            : t('Failed to load media catalog')
        )
      }
      setIsMediaCatalogLoading(false)

      if (templatesResult.status === 'fulfilled') {
        setTemplates(
          templatesResult.value.length > 0
            ? templatesResult.value
            : defaultTemplates
        )
      }
      if (billingResult.status === 'fulfilled') {
        setBilling(billingResult.value)
      }
      if (healthResult.status === 'fulfilled') {
        setMediaHealth(healthResult.value)
      } else {
        setMediaHealth({
          adapter: 'mock',
          configured: true,
          base_url_configured: false,
          api_key_configured: false,
          status: 'development',
          message:
            '本地开发环境使用模拟素材；配置 UAG/OpenAI-compatible 后会切换为真实上游。',
          supports_image: true,
          supports_video: true,
          require_real_assets: false,
          mock_fallback: true,
          upstream_verified: false,
          upstream_verification_status: 'not_applicable',
          upstream_verification_message:
            'Local mock media does not verify an upstream provider.',
          real_workflow_ready: false,
          verification_blockers: [
            'YuCore media adapter is still using mock assets.',
            'yucore_media.require_real_assets must stay enabled for real workflow verification.',
          ],
        })
      }
      if (healthResult.status !== 'fulfilled') {
        setMediaHealth(fallbackMediaHealth(loadedModels))
      }
      if (identityResult.status === 'fulfilled') {
        setCanvasIdentity(identityResult.value)
      } else {
        setCanvasIdentity(null)
      }

      const [tasksResult, galleryResult, canvasesResult] =
        await Promise.allSettled([
          refreshTasks(),
          refreshGallery(),
          refreshCanvases(),
        ])
      if (!mounted) return
      if (tasksResult.status === 'rejected') setTasks([])
      if (galleryResult.status === 'rejected') setGalleryTasks([])
      if (canvasesResult.status === 'rejected') setCanvases([])
    }

    load()
    return () => {
      mounted = false
    }
  }, [refreshCanvases, refreshGallery, refreshTasks, t])

  useEffect(() => {
    if (!activeModel) return
    if (activeModes.length > 0) {
      setMode((current) => keepOption(current, activeModes, activeModes[0]))
    }
    if (activeAspectRatios.length > 0) {
      setAspectRatio((current) =>
        keepOption(current, activeAspectRatios, activeAspectRatios[0])
      )
    }
    if (activeSizes.length > 0) {
      setSize((current) => keepOption(current, activeSizes, activeSizes[0]))
    }
    if (activeFormats.length > 0) {
      setFormat((current) =>
        keepOption(current, activeFormats, activeFormats[0])
      )
    }
    if (activeQualities.length > 0) {
      setQuality((current) =>
        keepOption(current, activeQualities, activeQualities[0])
      )
    }
    if (activeStylePresets.length > 0) {
      setStylePreset((current) =>
        keepOption(current, activeStylePresets, activeStylePresets[0])
      )
    }
    if (activeBackgrounds.length > 0) {
      setBackground((current) =>
        keepOption(current, activeBackgrounds, activeBackgrounds[0])
      )
    }
    if (activeModerations.length > 0) {
      setModeration((current) =>
        keepOption(current, activeModerations, activeModerations[0])
      )
    }
    if (activeStreamModes.length > 0) {
      setStreamMode((current) =>
        keepOption(current, activeStreamModes, activeStreamModes[0])
      )
    }
    if (activeDurations.length > 0) {
      setDuration((current) =>
        keepOption(current, activeDurations, activeDurations[0])
      )
    }
    if (activeCounts.length > 0) {
      setCount((current) => keepOption(current, activeCounts, activeCounts[0]))
    }
    if (!activeModel.supports_audio) {
      setGenerateAudio(true)
    }
    setReferenceFiles((items) => items.slice(0, maxReferenceImages))
  }, [
    activeAspectRatios,
    activeBackgrounds,
    activeCounts,
    activeDurations,
    activeFormats,
    activeModel,
    activeModerations,
    activeModes,
    activeQualities,
    activeSizes,
    activeStreamModes,
    activeStylePresets,
    maxReferenceImages,
  ])

  useEffect(() => {
    void refreshAgentRuns(activeCanvas?.id)
  }, [activeCanvas?.id, refreshAgentRuns])

  useEffect(() => {
    void refreshCanvasVersions(activeCanvas?.id)
  }, [activeCanvas?.id, refreshCanvasVersions])

  useEffect(() => {
    if (selectedNodeId && !selectedNode) {
      setSelectedNodeId(null)
    }
  }, [selectedNode, selectedNodeId])

  const markCanvasDirty = useCallback(() => {
    setIsCanvasDirty(true)
  }, [])

  const commitCanvasHistory = useCallback(() => {
    if (isApplyingCanvasHistoryRef.current) return
    const entry = buildCanvasHistoryEntry(
      nodes as StudioCanvasNode[],
      edges,
      canvasViewport,
      canvasBackgroundMode
    )
    setCanvasPast((items) => [...items.slice(-49), entry])
    setCanvasFuture([])
  }, [canvasBackgroundMode, canvasViewport, edges, nodes])

  const restoreCanvasHistoryEntry = useCallback(
    (entry: CanvasHistoryEntry) => {
      isApplyingCanvasHistoryRef.current = true
      setNodes(cloneCanvasNodes(entry.nodes))
      setEdges(cloneCanvasEdges(entry.edges))
      setCanvasViewport(cloneCanvasViewport(entry.viewport))
      setCanvasBackgroundMode(entry.backgroundMode)
      setSelectedNodeId(null)
      setIsCanvasMenuOpen(false)
      markCanvasDirty()
      window.setTimeout(() => {
        isApplyingCanvasHistoryRef.current = false
      }, 0)
    },
    [markCanvasDirty, setEdges, setNodes]
  )

  const undoCanvas = useCallback(() => {
    setCanvasPast((items) => {
      const entry = items.at(-1)
      if (!entry) return items
      const current = buildCanvasHistoryEntry(
        nodes as StudioCanvasNode[],
        edges,
        canvasViewport,
        canvasBackgroundMode
      )
      setCanvasFuture((future) => [current, ...future.slice(0, 49)])
      restoreCanvasHistoryEntry(entry)
      return items.slice(0, -1)
    })
  }, [
    canvasBackgroundMode,
    canvasViewport,
    edges,
    nodes,
    restoreCanvasHistoryEntry,
  ])

  const redoCanvas = useCallback(() => {
    setCanvasFuture((items) => {
      const entry = items[0]
      if (!entry) return items
      const current = buildCanvasHistoryEntry(
        nodes as StudioCanvasNode[],
        edges,
        canvasViewport,
        canvasBackgroundMode
      )
      setCanvasPast((past) => [...past.slice(-49), current])
      restoreCanvasHistoryEntry(entry)
      return items.slice(1)
    })
  }, [
    canvasBackgroundMode,
    canvasViewport,
    edges,
    nodes,
    restoreCanvasHistoryEntry,
  ])

  const handleNodesChange = useCallback(
    (changes: Parameters<typeof onNodesChange>[0]) => {
      const shouldCommit = changes.some(
        (change) =>
          change.type !== 'select' &&
          (!('dragging' in change) || change.dragging === false)
      )
      if (shouldCommit) {
        commitCanvasHistory()
      }
      onNodesChange(changes)
      if (changes.some((change) => change.type !== 'select')) {
        markCanvasDirty()
      }
    },
    [commitCanvasHistory, markCanvasDirty, onNodesChange]
  )

  const handleEdgesChange = useCallback(
    (changes: Parameters<typeof onEdgesChange>[0]) => {
      if (changes.some((change) => change.type !== 'select')) {
        commitCanvasHistory()
      }
      onEdgesChange(changes)
      if (changes.some((change) => change.type !== 'select')) {
        markCanvasDirty()
      }
    },
    [commitCanvasHistory, markCanvasDirty, onEdgesChange]
  )

  const handleConnect = useCallback(
    (connection: Connection) => {
      commitCanvasHistory()
      setEdges((items) =>
        addEdge(
          {
            ...connection,
            id: `edge_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 6)}`,
            animated: true,
            type: 'smoothstep',
            markerEnd: { type: MarkerType.ArrowClosed, color: '#67e8f9' },
            style: { stroke: '#67e8f9', opacity: 0.72 },
          },
          items
        )
      )
      markCanvasDirty()
    },
    [commitCanvasHistory, markCanvasDirty, setEdges]
  )

  const handleCanvasMoveEnd = useCallback(
    (_event: MouseEvent | TouchEvent | null, viewport: Viewport) => {
      const changed =
        !canvasViewport ||
        Math.abs(canvasViewport.x - viewport.x) > 0.5 ||
        Math.abs(canvasViewport.y - viewport.y) > 0.5 ||
        Math.abs(canvasViewport.zoom - viewport.zoom) > 0.005
      setCanvasViewport(viewport)
      if (changed) {
        markCanvasDirty()
      }
    },
    [canvasViewport, markCanvasDirty]
  )

  const handleCanvasMove = useCallback(
    (_event: MouseEvent | TouchEvent | null, viewport: Viewport) => {
      setCanvasViewport(viewport)
    },
    []
  )

  const applyCanvasViewport = useCallback(
    (viewport: Viewport, options: { markDirty?: boolean } = {}) => {
      if (options.markDirty) {
        commitCanvasHistory()
      }
      setCanvasViewport(viewport)
      void reactFlowInstance?.setViewport(viewport, { duration: 180 })
      if (options.markDirty) {
        markCanvasDirty()
      }
    },
    [commitCanvasHistory, markCanvasDirty, reactFlowInstance]
  )

  const handleCanvasZoomChange = useCallback(
    (zoom: number) => {
      applyCanvasViewport(
        {
          ...effectiveCanvasViewport,
          zoom: clampCanvasZoom(zoom),
        },
        { markDirty: true }
      )
    },
    [applyCanvasViewport, effectiveCanvasViewport]
  )

  const handleCanvasZoomStep = useCallback(
    (direction: -1 | 1) => {
      handleCanvasZoomChange(
        effectiveCanvasViewport.zoom * (direction > 0 ? 1.18 : 1 / 1.18)
      )
    },
    [effectiveCanvasViewport.zoom, handleCanvasZoomChange]
  )

  const handleCanvasFitView = useCallback(() => {
    commitCanvasHistory()
    void reactFlowInstance?.fitView({ padding: 0.32, duration: 240 })
    markCanvasDirty()
  }, [commitCanvasHistory, markCanvasDirty, reactFlowInstance])

  const updateSelectedCanvasNode = useCallback(
    (patch: Partial<CanvasNodeData>) => {
      if (!selectedNodeId) return
      commitCanvasHistory()
      setNodes((items) =>
        items.map((node) =>
          node.id === selectedNodeId
            ? ({
                ...node,
                data: {
                  ...node.data,
                  ...patch,
                },
              } as StudioCanvasNode)
            : node
        )
      )
      markCanvasDirty()
    },
    [commitCanvasHistory, markCanvasDirty, selectedNodeId, setNodes]
  )

  const deleteSelectedCanvasNode = useCallback(() => {
    const selectedIds = new Set(
      (nodes as StudioCanvasNode[])
        .filter((node) => node.selected || node.id === selectedNodeId)
        .map((node) => node.id)
    )
    if (selectedIds.size === 0) return
    const confirmed = window.confirm(
      selectedIds.size === 1
        ? '删除当前选中的画布节点？'
        : `删除选中的 ${selectedIds.size} 个画布节点？`
    )
    if (!confirmed) return
    commitCanvasHistory()
    setNodes((items) => items.filter((node) => !selectedIds.has(node.id)))
    setEdges((items) =>
      items.filter(
        (edge) => !selectedIds.has(edge.source) && !selectedIds.has(edge.target)
      )
    )
    setSelectedNodeId(null)
    markCanvasDirty()
  }, [
    commitCanvasHistory,
    markCanvasDirty,
    nodes,
    selectedNodeId,
    setEdges,
    setNodes,
  ])

  const duplicateSelectedCanvasNode = useCallback(() => {
    if (!selectedNode) return
    commitCanvasHistory()
    const id = `${selectedNode.data.kind ?? 'node'}_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 6)}`
    setNodes((items) => [
      ...items,
      {
        ...selectedNode,
        id,
        selected: false,
        position: {
          x: selectedNode.position.x + 36,
          y: selectedNode.position.y + 36,
        },
        data: {
          ...selectedNode.data,
          label: `${getCanvasNodeTitle(selectedNode.data)} 副本`,
        },
      },
    ])
    setSelectedNodeId(id)
    markCanvasDirty()
  }, [commitCanvasHistory, markCanvasDirty, selectedNode, setNodes])

  const copySelectedCanvasNodes = useCallback(() => {
    const selectedIds = new Set(
      (nodes as StudioCanvasNode[])
        .filter((node) => node.selected || node.id === selectedNodeId)
        .map((node) => node.id)
    )
    if (selectedIds.size === 0) return
    canvasClipboardRef.current = {
      nodes: cloneCanvasNodes(
        (nodes as StudioCanvasNode[]).filter((node) => selectedIds.has(node.id))
      ),
      edges: cloneCanvasEdges(
        edges.filter(
          (edge) => selectedIds.has(edge.source) && selectedIds.has(edge.target)
        )
      ),
    }
    toast.success(`已复制 ${selectedIds.size} 个节点`)
  }, [edges, nodes, selectedNodeId])

  const pasteCanvasNodes = useCallback(() => {
    const clipboard = canvasClipboardRef.current
    if (!clipboard || clipboard.nodes.length === 0) return
    commitCanvasHistory()
    const stamp = Date.now().toString(36)
    const idMap = new Map(
      clipboard.nodes.map((node, index) => [
        node.id,
        `${node.data.kind ?? 'node'}_${stamp}_${index}`,
      ])
    )
    const pastedNodes = cloneCanvasNodes(clipboard.nodes).map((node) => ({
      ...node,
      id: idMap.get(node.id) ?? node.id,
      selected: true,
      position: {
        x: node.position.x + 48,
        y: node.position.y + 48,
      },
    }))
    const pastedEdges = cloneCanvasEdges(clipboard.edges).map(
      (edge, index) => ({
        ...edge,
        id: `edge_${stamp}_${index}`,
        source: idMap.get(edge.source) ?? edge.source,
        target: idMap.get(edge.target) ?? edge.target,
      })
    )
    setNodes((items) => [
      ...items.map((node) => ({ ...node, selected: false })),
      ...pastedNodes,
    ])
    setEdges((items) => [...items, ...pastedEdges])
    setSelectedNodeId(pastedNodes[0]?.id ?? null)
    markCanvasDirty()
    toast.success(`已粘贴 ${pastedNodes.length} 个节点`)
  }, [commitCanvasHistory, markCanvasDirty, setEdges, setNodes])

  const selectAllCanvasNodes = useCallback(() => {
    setNodes((items) => items.map((node) => ({ ...node, selected: true })))
    setSelectedNodeId(nodes[0]?.id ?? null)
  }, [nodes, setNodes])

  async function generateFromSelectedCanvasNode() {
    if (!selectedNode) return
    const nodePrompt = String(
      selectedNode.data.prompt || selectedNode.data.label || ''
    ).trim()
    if (!nodePrompt) {
      toast.error('这个节点还没有可用于生成的提示词')
      return
    }
    let canvasForTask = activeCanvas
    if (!canvasForTask) {
      try {
        const createdCanvas = await createYucoreCanvas({
          title: `YuCore Canvas ${canvases.length + 1}`,
          description: '图像、提示词、素材与生成节点的无限画布。',
          module: '无限画布',
          snapshot: { nodes, edges, backgroundMode: canvasBackgroundMode },
          viewport: canvasViewport ?? {},
        })
        canvasForTask = createdCanvas
        setCanvases((items) => [createdCanvas, ...items])
        setActiveCanvas(createdCanvas)
        setIsCanvasDirty(false)
      } catch (error) {
        toast.error(error instanceof Error ? error.message : '创建画布失败')
        return
      }
    }
    const taskKind = selectedNode.data.kind === 'video' ? 'video' : 'image'
    const nodeModel = taskKind === 'video' ? videoModel : imageModel
    setPrompt(nodePrompt)
    if (taskKind === 'video') {
      setKind('video')
      setMode((current) =>
        current.includes('video') ? current : 'text-to-video'
      )
    } else {
      setKind('image')
      setMode((current) =>
        current.includes('image') ? current : 'text-to-image'
      )
    }
    const task = await handleSubmitTask(taskKind, nodePrompt, {
      stayOnCanvas: true,
      metadata: {
        surface: 'yucore-studio',
        canvas_id: canvasForTask.id,
        agent_mode: 'node',
        agent_prompt_node_id: selectedNode.id,
        agent_task_node_id: selectedNode.id,
        canvas_node_id: selectedNode.id,
        canvas_node_kind: selectedNode.data.kind ?? taskKind,
        model_name: nodeModel?.name,
        model_family: nodeModel?.family,
      },
    })
    if (!task) return

    const nextNodes = (nodes as StudioCanvasNode[]).map((node) =>
      node.id === selectedNode.id
        ? ({
            ...node,
            data: {
              ...node.data,
              kind: task.kind,
              sublabel: mediaTaskRouteLabel(task),
              prompt: task.prompt,
              status: `${task.task_id} / ${task.status}`,
              resultTaskId: task.task_id,
            },
            style: {
              ...node.style,
              width: 230,
              border: '1px solid rgb(253 230 138 / 0.3)',
              padding: node.style?.padding ?? 14,
              boxShadow: '0 22px 70px rgb(0 0 0 / 0.36)',
            },
          } as StudioCanvasNode)
        : node
    )
    commitCanvasHistory()
    setNodes(nextNodes)
    setSelectedNodeId(selectedNode.id)
    setView('canvas')
    markCanvasDirty()

    try {
      const syncedCanvas = await updateYucoreCanvas(canvasForTask.id, {
        title: canvasForTask.title,
        description: canvasForTask.description,
        module: canvasForTask.module,
        snapshot: {
          nodes: nextNodes,
          edges,
          backgroundMode: canvasBackgroundMode,
        },
        viewport: canvasViewport ?? {},
        autosave: true,
      })
      setActiveCanvas(syncedCanvas)
      setCanvases((items) =>
        items.map((item) => (item.id === syncedCanvas.id ? syncedCanvas : item))
      )
      setIsCanvasDirty(false)
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : '画布任务状态同步失败'
      )
    }
  }

  useEffect(() => {
    referenceFilesRef.current = referenceFiles
  }, [referenceFiles])

  useEffect(() => {
    return () => {
      referenceFilesRef.current.forEach((item) =>
        URL.revokeObjectURL(item.previewUrl)
      )
    }
  }, [])

  useEffect(() => {
    if (view !== 'canvas') {
      setIsCanvasMenuOpen(false)
    }
  }, [view])

  async function handleSubmitTask(
    nextKind: 'image' | 'video' = kind,
    promptOverride?: string,
    options: {
      stayOnCanvas?: boolean
      metadata?: Record<string, unknown>
    } = {}
  ) {
    const taskPrompt = (promptOverride ?? prompt).trim()
    if (!taskPrompt) {
      toast.error('请先输入提示词')
      return null
    }
    if (isReferenceUploading) {
      toast.error('参考图还在上传，请稍等')
      return null
    }
    if (referenceUploadErrors.length > 0) {
      toast.error('有参考图上传失败，请重新选择后再生成')
      return null
    }
    const referenceAssets = referenceFiles
      .map(
        (file) => file.sourceUrl || file.cachedUrl || file.url || file.dataUrl
      )
      .filter(Boolean)
    if (
      referenceFiles.length > 0 &&
      referenceAssets.length !== referenceFiles.length
    ) {
      toast.error('参考图素材尚未准备完成')
      return null
    }

    setIsSubmitting(true)
    try {
      const currentModel = nextKind === 'video' ? videoModel : imageModel
      if (!selectedGroup || !currentModel) {
        toast.error(t('Select a media model before submitting.'))
        return null
      }
      const submitModes = currentModel.modes
      const submitMode = submitModes.includes(mode) ? mode : submitModes[0]
      const task = await createYucoreMediaTask({
        group: selectedGroup,
        kind: nextKind,
        mode: submitMode,
        model_id: currentModel.id,
        prompt: taskPrompt,
        negative_prompt: negativePrompt,
        ...(currentModel.aspect_ratios?.includes(aspectRatio)
          ? { aspect_ratio: aspectRatio }
          : {}),
        ...(currentModel.sizes?.includes(size) ? { size } : {}),
        ...(currentModel.qualities?.includes(quality) ? { quality } : {}),
        ...((currentModel.output_formats ?? currentModel.formats)?.includes(
          format
        )
          ? { format }
          : {}),
        count:
          nextKind === 'video' || !currentModel.counts?.includes(count)
            ? 1
            : count,
        session_id: sessionId,
        inputs: referenceFiles.map((file) => ({
          id: file.id,
          name: file.name,
          size: file.size,
          mime_type: file.mimeType,
          dataUrl: file.dataUrl,
          cachedUrl: file.cachedUrl,
          sourceUrl: file.sourceUrl,
          url: file.url || file.sourceUrl || file.cachedUrl || file.dataUrl,
        })),
        metadata: {
          model_name: currentModel.name,
          model_family: currentModel.family,
          ...(currentModel.style_presets?.includes(stylePreset)
            ? { style_preset: stylePreset }
            : {}),
          ...(currentModel.backgrounds?.includes(background)
            ? { background }
            : {}),
          ...(currentModel.moderations?.includes(moderation)
            ? { moderation }
            : {}),
          ...(currentModel.stream_modes?.includes(streamMode)
            ? { stream_mode: streamMode }
            : {}),
          ...(currentModel.partial_images?.includes(partialImages)
            ? { partial_images: partialImages }
            : {}),
          ...(currentModel.durations?.includes(duration) ? { duration } : {}),
          ...(currentModel.supports_audio
            ? { generate_audio: generateAudio }
            : {}),
          visibility,
          reference_count: referenceFiles.length,
          surface: 'yucore-studio',
          canvas_identity_session: canvasIdentity?.identity_session,
          canvas_identity_expires_at: canvasIdentity?.expires_at,
          ...options.metadata,
        },
      })
      setKind(nextKind)
      setActiveTask(task)
      setTasks((items) => [task, ...items])
      void getYucoreMediaBilling()
        .then(setBilling)
        .catch(() => undefined)
      if (!options.stayOnCanvas) {
        setView(nextKind === 'video' ? 'video' : 'image')
      }
      toast.success('生成任务已进入 YuCore 队列')
      return task
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '创建任务失败')
      return null
    } finally {
      setIsSubmitting(false)
    }
  }

  async function handleCancelTask(task: YucoreMediaTask) {
    try {
      const next = await cancelYucoreMediaTask(task.task_id)
      setTasks((items) => [
        next,
        ...items.filter((item) => item.task_id !== next.task_id),
      ])
      if (activeTask?.task_id === next.task_id) setActiveTask(next)
      toast.success('任务已取消')
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '取消任务失败')
    }
  }

  async function handleCreateCanvas() {
    try {
      const canvas = await createYucoreCanvas({
        title: `YuCore Canvas ${canvases.length + 1}`,
        description: '图像、提示词、素材与生成节点的无限画布。',
        module: '无限画布',
        snapshot: { nodes, edges, backgroundMode: canvasBackgroundMode },
        viewport: canvasViewport ?? {},
      })
      setCanvases((items) => [canvas, ...items])
      setActiveCanvas(canvas)
      setIsCanvasDirty(false)
      setView('canvas')
      toast.success('画布已创建')
    } catch (error) {
      toast.error(error instanceof Error ? error.message : '创建画布失败')
    }
  }

  const persistCanvasSnapshot = useCallback(
    async (
      nextNodes: StudioCanvasNode[],
      nextEdges: Edge[],
      reason: string,
      options: {
        autosave?: boolean
        createIfMissing?: boolean
        quiet?: boolean
        viewport?: Viewport
        backgroundMode?: CanvasBackgroundMode
        canvasOverride?: YucoreCanvasRecord | null
      } = {}
    ) => {
      const autosave = options.autosave ?? true
      const nextViewport = options.viewport ?? canvasViewport
      const nextBackgroundMode = options.backgroundMode ?? canvasBackgroundMode
      const targetCanvas = options.canvasOverride ?? activeCanvas
      setIsSavingCanvas(true)
      try {
        let canvas: YucoreCanvasRecord | null = null
        if (targetCanvas) {
          canvas = await updateYucoreCanvas(targetCanvas.id, {
            title: targetCanvas.title,
            description: targetCanvas.description,
            module: targetCanvas.module,
            snapshot: {
              nodes: nextNodes,
              edges: nextEdges,
              backgroundMode: nextBackgroundMode,
            },
            viewport: nextViewport ?? {},
            autosave,
          })
        } else if (options.createIfMissing) {
          canvas = await createYucoreCanvas({
            title: `YuCore Canvas ${canvases.length + 1}`,
            description: '图像、提示词、素材与生成节点的无限画布。',
            module: '无限画布',
            snapshot: {
              nodes: nextNodes,
              edges: nextEdges,
              backgroundMode: nextBackgroundMode,
            },
            viewport: nextViewport ?? {},
          })
        }

        if (!canvas) {
          setIsCanvasDirty(true)
          if (!options.quiet) {
            toast.info(`${reason}已更新，请保存画布以便刷新后保留`)
          }
          return null
        }

        setActiveCanvas(canvas)
        setCanvases((items) => {
          const exists = items.some((item) => item.id === canvas.id)
          return exists
            ? items.map((item) => (item.id === canvas.id ? canvas : item))
            : [canvas, ...items]
        })
        setIsCanvasDirty(false)
        if (!options.quiet) {
          toast.success(autosave ? `${reason}已同步到画布` : '画布已保存')
        }
        return canvas
      } catch (error) {
        setIsCanvasDirty(true)
        toast.error(
          error instanceof Error ? error.message : `${reason}保存失败`
        )
        return null
      } finally {
        setIsSavingCanvas(false)
      }
    },
    [activeCanvas, canvasBackgroundMode, canvasViewport, canvases.length]
  )

  const handleMediaTaskBackflow = useCallback(
    async (task: YucoreMediaTask) => {
      if (!isTerminalMediaTask(task)) return
      const canvasId = metadataNumber(task.metadata, 'canvas_id')
      const agentRunId = metadataString(task.metadata, 'agent_run_id')
      const promptNodeId = metadataString(task.metadata, 'agent_prompt_node_id')
      const taskNodeId = metadataString(task.metadata, 'agent_task_node_id')
      if (!canvasId && !agentRunId && !taskNodeId) return

      const firstAsset = task.assets[0]
      const backflowKey = `${task.task_id}:${task.status}:${task.updated_time}:${firstAsset?.id ?? 'none'}`
      if (appliedTaskBackflowRef.current.has(backflowKey)) return
      appliedTaskBackflowRef.current.add(backflowKey)

      try {
        if (task.status === 'completed') {
          await refreshGallery()
        }

        if (canvasId && agentRunId) {
          const run = agentRuns.find((item) => item.run_id === agentRunId)
          const nextRun = await updateYucoreCanvasAgentRun(
            canvasId,
            agentRunId,
            {
              mode:
                run?.mode ??
                metadataString(task.metadata, 'agent_mode') ??
                'site',
              prompt: run?.prompt || task.prompt,
              status: task.status === 'completed' ? 'completed' : 'failed',
              summary: buildAgentRunSummary(task),
              result_task_id: task.task_id,
              actions: buildAgentRunActions(run, task),
            }
          )
          if (!activeCanvas || activeCanvas.id === canvasId) {
            setAgentRuns((items) => [
              nextRun,
              ...items.filter((item) => item.run_id !== nextRun.run_id),
            ])
            setAgentTab('history')
          }
        }

        if (canvasId && taskNodeId && activeCanvas?.id === canvasId) {
          const sourceNodes = nodes as StudioCanvasNode[]
          const nextNodes = sourceNodes.map((node) => {
            if (node.id === taskNodeId) {
              const nextData: CanvasNodeData = {
                ...node.data,
                kind: task.kind,
                label:
                  task.status === 'completed'
                    ? firstAsset?.label ||
                      node.data.label ||
                      'Generation result'
                    : node.data.label,
                sublabel: mediaTaskRouteLabel(task),
                prompt: task.prompt,
                status: buildCanvasTaskStatus(task),
                resultTaskId: task.task_id,
                error: task.error || undefined,
              }
              if (task.status === 'completed' && firstAsset) {
                nextData.assetUrl =
                  firstAsset.url ||
                  firstAsset.cached_url ||
                  firstAsset.source_url
                nextData.thumbUrl =
                  firstAsset.thumb_url ||
                  firstAsset.cached_url ||
                  firstAsset.source_url
              }

              return {
                ...node,
                data: nextData,
                style: {
                  ...node.style,
                  width: 230,
                  border:
                    task.status === 'completed'
                      ? '1px solid rgb(103 232 249 / 0.32)'
                      : '1px solid rgb(251 113 133 / 0.32)',
                  padding:
                    task.status === 'completed' && firstAsset
                      ? 0
                      : (node.style?.padding ?? 14),
                  boxShadow: '0 22px 70px rgb(0 0 0 / 0.36)',
                },
              } as StudioCanvasNode
            }
            if (promptNodeId && node.id === promptNodeId) {
              return {
                ...node,
                data: {
                  ...node.data,
                  status: `linked ${task.task_id} / ${task.status}`,
                },
              } as StudioCanvasNode
            }
            return node
          })

          const changed = nextNodes.some(
            (node, index) => node !== sourceNodes[index]
          )
          if (changed) {
            setNodes(nextNodes)
            await persistCanvasSnapshot(nextNodes, edges, 'Agent result', {
              autosave: true,
              createIfMissing: false,
              quiet: true,
              viewport: canvasViewport,
            })
          }
        }
      } catch (error) {
        appliedTaskBackflowRef.current.delete(backflowKey)
        toast.error(
          error instanceof Error
            ? error.message
            : 'Agent result backflow failed'
        )
      }
    },
    [
      activeCanvas,
      agentRuns,
      canvasViewport,
      edges,
      nodes,
      persistCanvasSnapshot,
      refreshGallery,
      setNodes,
    ]
  )

  const activeTaskId = activeTask?.task_id
  const activeTaskIsRunning = activeTask ? isActiveMediaTask(activeTask) : false

  useEffect(() => {
    if (!activeTaskId || !activeTaskIsRunning) return

    const timer = window.setInterval(async () => {
      try {
        const next = await getYucoreMediaTask(activeTaskId)
        setActiveTask(next)
        setTasks((items) => [
          next,
          ...items.filter((item) => item.task_id !== next.task_id),
        ])
      } catch {
        window.clearInterval(timer)
      }
    }, 1300)

    return () => window.clearInterval(timer)
  }, [activeTaskId, activeTaskIsRunning])

  useEffect(() => {
    if (!activeTask || !isTerminalMediaTask(activeTask)) return
    void handleMediaTaskBackflow(activeTask)
    void getYucoreMediaBilling()
      .then(setBilling)
      .catch(() => undefined)
  }, [activeTask, handleMediaTaskBackflow])

  const handleSaveCanvas = useCallback(async () => {
    setIsSavingCanvas(true)
    try {
      await persistCanvasSnapshot(nodes as StudioCanvasNode[], edges, '画布', {
        autosave: false,
        createIfMissing: true,
        viewport: canvasViewport,
        backgroundMode: canvasBackgroundMode,
      })
    } finally {
      setIsSavingCanvas(false)
    }
  }, [
    canvasBackgroundMode,
    canvasViewport,
    edges,
    nodes,
    persistCanvasSnapshot,
  ])

  useEffect(() => {
    if (!activeCanvas || !isCanvasDirty || isSavingCanvas) return

    const timer = window.setTimeout(() => {
      void persistCanvasSnapshot(
        nodes as StudioCanvasNode[],
        edges,
        '画布变更',
        {
          autosave: true,
          quiet: true,
          viewport: canvasViewport,
          backgroundMode: canvasBackgroundMode,
        }
      )
    }, 1200)

    return () => window.clearTimeout(timer)
  }, [
    activeCanvas,
    canvasBackgroundMode,
    canvasViewport,
    edges,
    isCanvasDirty,
    isSavingCanvas,
    nodes,
    persistCanvasSnapshot,
  ])

  function addCanvasNode(type: 'text' | 'image' | 'video') {
    commitCanvasHistory()
    const id = `${type}_${Date.now().toString(36)}`
    let label = '视频节点'
    let sublabel = '镜头 / 运动 / 时长'
    if (type === 'text') {
      label = '文本节点'
      sublabel = '新的提示词片段'
    } else if (type === 'image') {
      label = '图片节点'
      sublabel = '素材 / 生成结果'
    }
    setNodes((items) => [
      ...items,
      {
        id,
        type: 'media',
        position: {
          x: Math.round(Math.random() * 360 - 120),
          y: Math.round(Math.random() * 260 - 80),
        },
        data: {
          kind: type,
          label,
          sublabel,
          prompt:
            type === 'text'
              ? prompt.slice(0, 160)
              : '从素材库拖入内容，或把生成结果发送到画布。',
        },
        style: {
          width: 190,
          border: '1px solid rgb(255 255 255 / 0.16)',
          borderRadius: 16,
          background: 'rgb(5 9 16 / 0.9)',
          color: 'white',
          padding: 14,
          whiteSpace: 'pre-line',
          boxShadow: '0 18px 50px rgb(0 0 0 / 0.25)',
        },
      },
    ])
    markCanvasDirty()
  }

  function addAssetToCanvas(task: YucoreMediaTask, asset: YucoreMediaAsset) {
    commitCanvasHistory()
    const id = `${asset.kind}_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 6)}`
    const nextNode: StudioCanvasNode = {
      id,
      type: 'media',
      position: {
        x: Math.round(Math.random() * 520 - 160),
        y: Math.round(Math.random() * 320 - 80),
      },
      data: {
        kind: asset.kind,
        label: asset.label,
        sublabel: mediaTaskRouteLabel(task),
        assetUrl: asset.url,
        thumbUrl: asset.thumb_url,
        prompt: task.prompt,
        status: `${formatTime(task.updated_time)} / ${task.cost} 点`,
      },
      style: {
        width: 230,
        border: '1px solid rgb(103 232 249 / 0.28)',
        borderRadius: 18,
        background: 'rgb(5 9 16 / 0.92)',
        color: 'white',
        padding: 0,
        boxShadow: '0 22px 70px rgb(0 0 0 / 0.35)',
      },
    }
    const nextNodes = [...(nodes as StudioCanvasNode[]), nextNode]
    setNodes(nextNodes)
    markCanvasDirty()
    setView('canvas')
    void persistCanvasSnapshot(nextNodes, edges, '素材放入画布', {
      autosave: true,
      createIfMissing: true,
      backgroundMode: canvasBackgroundMode,
    })
  }

  async function addCanvasMediaFiles(
    sourceFiles: File[],
    dropPosition?: { x: number; y: number }
  ) {
    const files = sourceFiles
      .filter(
        (file) =>
          file.type.startsWith('image/') || file.type.startsWith('video/')
      )
      .slice(0, 12)
    if (files.length === 0) {
      toast.error('请选择图片或视频文件')
      return
    }

    setIsCanvasMediaUploading(true)
    const uploadToast = toast.loading(`正在上传 ${files.length} 个素材`)
    try {
      const uploadResults = await Promise.allSettled(
        files.map(async (file) => ({
          file,
          upload: await uploadYucoreMediaReference(file),
        }))
      )
      const stamp = Date.now().toString(36)
      const uploadedNodes: StudioCanvasNode[] = uploadResults.flatMap(
        (result, index) => {
          if (result.status === 'rejected') return []
          const assetUrl =
            result.value.upload.url ||
            result.value.upload.source_url ||
            result.value.upload.sourceUrl ||
            result.value.upload.cached_url ||
            result.value.upload.cachedUrl ||
            result.value.upload.data_url ||
            result.value.upload.dataUrl
          if (!assetUrl) return []
          const file = result.value.file
          const nodeKind: 'image' | 'video' = file.type.startsWith('video/')
            ? 'video'
            : 'image'

          return [
            {
              id: `${nodeKind}_${stamp}_${index}`,
              type: 'media',
              position: {
                x: (dropPosition?.x ?? -120) + (index % 4) * 270,
                y: (dropPosition?.y ?? -80) + Math.floor(index / 4) * 230,
              },
              selected: index === 0,
              data: {
                kind: nodeKind,
                label: file.name,
                sublabel: `${file.type || nodeKind} / ${(file.size / 1024 / 1024).toFixed(1)} MB`,
                assetUrl,
                thumbUrl: nodeKind === 'image' ? assetUrl : undefined,
                status: '已上传',
              },
              style: {
                width: 240,
                border: '1px solid rgb(103 232 249 / 0.24)',
                borderRadius: 18,
                background: 'rgb(5 9 16 / 0.92)',
                color: 'white',
                padding: 0,
                overflow: 'hidden',
                boxShadow: '0 22px 70px rgb(0 0 0 / 0.36)',
              },
            },
          ]
        }
      )

      if (uploadedNodes.length === 0) {
        toast.error('素材上传失败，请检查服务端上传接口', {
          id: uploadToast,
        })
        return
      }
      commitCanvasHistory()
      setNodes((items) => [
        ...items.map((node) => ({ ...node, selected: false })),
        ...uploadedNodes,
      ])
      setSelectedNodeId(uploadedNodes[0]?.id ?? null)
      setView('canvas')
      markCanvasDirty()
      const failedCount = files.length - uploadedNodes.length
      let successMessage = `已加入 ${uploadedNodes.length} 个素材`
      if (failedCount > 0) {
        successMessage += `，${failedCount} 个上传失败`
      }
      toast.success(successMessage, { id: uploadToast })
    } finally {
      setIsCanvasMediaUploading(false)
    }
  }

  function handleCanvasMediaInputChange(event: ChangeEvent<HTMLInputElement>) {
    const files = [...(event.target.files ?? [])]
    event.target.value = ''
    void addCanvasMediaFiles(files)
  }

  function handleCanvasMediaDrop(event: DragEvent<HTMLDivElement>) {
    event.preventDefault()
    if (isCanvasMediaUploading) return
    const files = [...event.dataTransfer.files]
    const dropPosition = reactFlowInstanceRef.current?.screenToFlowPosition({
      x: event.clientX,
      y: event.clientY,
    })
    void addCanvasMediaFiles(files, dropPosition)
  }

  async function handleReferenceChange(event: ChangeEvent<HTMLInputElement>) {
    const files = [...(event.target.files ?? [])].slice(0, maxReferenceImages)
    referenceFilesRef.current.forEach((item) =>
      URL.revokeObjectURL(item.previewUrl)
    )
    const drafts = files.map((file) => ({
      id: `${file.name}_${file.lastModified}_${file.size}`,
      name: file.name,
      size: file.size,
      mimeType: file.type || 'application/octet-stream',
      previewUrl: URL.createObjectURL(file),
      isUploading: true,
    }))
    setReferenceFiles(drafts)
    event.target.value = ''
    if (
      drafts.length > 0 &&
      kind === 'image' &&
      activeModes.includes('image-to-image')
    ) {
      setMode('image-to-image')
    }
    if (
      drafts.length > 0 &&
      kind === 'video' &&
      activeModes.includes('image-to-video')
    ) {
      setMode('image-to-video')
    }

    if (drafts.length === 0) return

    const uploaded: ReferenceAssetDraft[] = await Promise.all(
      drafts.map(async (draft, index) => {
        try {
          const upload = await uploadYucoreMediaReference(files[index])
          return {
            ...draft,
            id: upload.id || draft.id,
            name: upload.name || upload.fileName || draft.name,
            size: upload.size || draft.size,
            mimeType: upload.mime_type || upload.mimeType || draft.mimeType,
            dataUrl: upload.data_url || upload.dataUrl,
            cachedUrl: upload.cached_url || upload.cachedUrl,
            sourceUrl: upload.source_url || upload.sourceUrl,
            url: upload.url,
            isUploading: false,
          }
        } catch (error) {
          return {
            ...draft,
            isUploading: false,
            uploadError: error instanceof Error ? error.message : '上传失败',
          }
        }
      })
    )

    setReferenceFiles((current) => {
      const stillSameSelection =
        current.length === drafts.length &&
        current.every(
          (item, index) => item.previewUrl === drafts[index]?.previewUrl
        )
      return stillSameSelection ? uploaded : current
    })

    const failedCount = uploaded.filter((item) => item.uploadError).length
    if (failedCount > 0) {
      toast.error(`${failedCount} 张参考图上传失败`)
    } else {
      toast.success('参考图已准备好')
    }
  }

  function clearReferenceFiles() {
    setReferenceFiles((items) => {
      items.forEach((item) => URL.revokeObjectURL(item.previewUrl))
      return []
    })
  }

  function clearCanvas() {
    const confirmed = window.confirm('清空当前画布上的所有节点和连线？')
    if (!confirmed) return
    commitCanvasHistory()
    setNodes([])
    setEdges([])
    markCanvasDirty()
  }

  function applyTemplate(template: YucorePromptTemplate) {
    setPrompt(template.prompt)
    setNegativePrompt(template.negative_prompt ?? '')
    setKind(template.kind)
    if (template.model_id) {
      const modelIsAvailable = models.some(
        (model) =>
          model.id === template.model_id && model.kind === template.kind
      )
      if (modelIsAvailable && template.kind === 'video') {
        setVideoModelId(template.model_id)
      } else if (modelIsAvailable) {
        setImageModelId(template.model_id)
      }
    }
    if (template.mode) setMode(template.mode)
    if (template.style) setStylePreset(template.style)
    if (template.duration) setDuration(template.duration)
    setAspectRatio(template.aspect_ratio ?? 'auto')
    if (template.kind === 'video' && template.preview_image_url) {
      const previewUrl = new URL(
        template.preview_image_url,
        window.location.origin
      ).toString()
      setReferenceFiles((items) => {
        items.forEach((item) => {
          if (item.previewUrl.startsWith('blob:')) {
            URL.revokeObjectURL(item.previewUrl)
          }
        })
        return [
          {
            id: `template-${template.id}`,
            name: `${template.id}.webp`,
            size: 0,
            mimeType: 'image/webp',
            previewUrl,
            sourceUrl: previewUrl,
            url: previewUrl,
          },
        ]
      })
    }
    setView(template.kind === 'video' ? 'video' : 'image')
  }

  function openCanvasView(nextView: StudioView) {
    setView(nextView)
    setIsCanvasMenuOpen(false)
  }

  function handleMediaGroupChange(nextGroup: string) {
    const imageSelection = resolveMediaSelection(
      mediaCatalog,
      nextGroup,
      imageModelId,
      'image'
    )
    const videoSelection = resolveMediaSelection(
      mediaCatalog,
      nextGroup,
      videoModelId,
      'video'
    )
    setSelectedGroup(nextGroup)
    setImageModelId(imageSelection.modelId)
    setVideoModelId(videoSelection.modelId)
  }

  async function handleAgentSubmit() {
    const nextPrompt = agentPrompt.trim()
    if (!nextPrompt) return
    if (isReferenceUploading) {
      toast.error('Reference uploads are still running.')
      return
    }
    if (referenceUploadErrors.length > 0) {
      toast.error(
        'Some reference uploads failed. Please replace them before running Agent.'
      )
      return
    }
    const referenceAssets = referenceFiles
      .map(
        (file) => file.sourceUrl || file.cachedUrl || file.url || file.dataUrl
      )
      .filter(Boolean)
    if (
      referenceFiles.length > 0 &&
      referenceAssets.length !== referenceFiles.length
    ) {
      toast.error('Reference assets are not ready yet.')
      return
    }
    const agentTaskInputs = referenceFiles.map((file) => ({
      id: file.id,
      name: file.name,
      size: file.size,
      mime_type: file.mimeType,
      dataUrl: file.dataUrl,
      cachedUrl: file.cachedUrl,
      sourceUrl: file.sourceUrl,
      url: file.url || file.sourceUrl || file.cachedUrl || file.dataUrl,
    }))
    const agentMediaKind = kind
    const agentMediaModel = agentMediaKind === 'video' ? videoModel : imageModel
    if (!selectedGroup || !agentMediaModel) {
      toast.error(t('Select a media model before submitting.'))
      return
    }
    const agentMediaModes = agentMediaModel.modes
    let agentMediaMode = agentMediaModes[0]
    const textMode =
      agentMediaKind === 'video' ? 'text-to-video' : 'text-to-image'
    const referenceMode =
      agentMediaKind === 'video' ? 'image-to-video' : 'image-to-image'
    if (agentMediaModes.includes(textMode)) {
      agentMediaMode = textMode
    }
    if (referenceFiles.length > 0 && agentMediaModes.includes(referenceMode)) {
      agentMediaMode = referenceMode
    }
    const agentMediaModelId = agentMediaModel.id
    setPrompt(nextPrompt)
    setAgentPrompt('')
    setMode(agentMediaMode)
    const seed = Date.now().toString(36)
    const agentNodeId = `agent_prompt_${seed}`
    const taskNodeId = `agent_task_${seed}`
    const agentNode: StudioCanvasNode = {
      id: agentNodeId,
      type: 'media',
      position: {
        x: Math.round(Math.random() * 260 - 180),
        y: Math.round(Math.random() * 220 - 110),
      },
      data: {
        kind: 'prompt',
        label: 'Agent 指令',
        sublabel: `${agentMode === 'site' ? 'website' : 'local'} / tool plan`,
        prompt: nextPrompt,
        status: 'canvas_create_generation_flow',
      },
      style: {
        width: 230,
        border: '1px solid rgb(103 232 249 / 0.3)',
        borderRadius: 18,
        background: 'rgb(5 9 16 / 0.92)',
        color: 'white',
        padding: 14,
        boxShadow: '0 22px 70px rgb(0 0 0 / 0.36)',
      },
    }
    const taskNode: StudioCanvasNode = {
      id: taskNodeId,
      type: 'media',
      position: { x: agentNode.position.x + 300, y: agentNode.position.y + 24 },
      data: {
        kind: agentMediaKind,
        label: '生成任务',
        sublabel: `${selectedGroup} / ${agentMediaModel.name}`,
        prompt: nextPrompt,
        status: 'waiting for media task',
      },
      style: {
        width: 230,
        border: '1px solid rgb(253 230 138 / 0.28)',
        borderRadius: 18,
        background: 'rgb(5 9 16 / 0.92)',
        color: 'white',
        padding: 14,
        boxShadow: '0 22px 70px rgb(0 0 0 / 0.36)',
      },
    }
    const agentEdge: Edge = {
      id: `${agentNodeId}-${taskNodeId}`,
      source: agentNodeId,
      target: taskNodeId,
      animated: true,
      type: 'smoothstep',
      markerEnd: { type: MarkerType.ArrowClosed, color: '#fde68a' },
      style: { stroke: '#fde68a', opacity: 0.72 },
    }
    const plannedNodes = [...(nodes as StudioCanvasNode[]), agentNode, taskNode]
    const plannedEdges = [...edges, agentEdge]
    setNodes(plannedNodes)
    setEdges(plannedEdges)
    markCanvasDirty()
    setView('canvas')

    const canvasForRun = await persistCanvasSnapshot(
      plannedNodes,
      plannedEdges,
      'Agent flow',
      {
        autosave: true,
        createIfMissing: true,
        quiet: true,
        viewport: canvasViewport,
        backgroundMode: canvasBackgroundMode,
      }
    )
    if (!canvasForRun) {
      setAgentTab('logs')
      return
    }

    setIsSubmitting(true)
    try {
      const result = await executeYucoreCanvasAgentRun(canvasForRun.id, {
        mode: agentMode,
        prompt: nextPrompt,
        group: selectedGroup,
        kind: agentMediaKind,
        media_mode: agentMediaMode,
        model_id: agentMediaModelId,
        negative_prompt: negativePrompt,
        ...(agentMediaModel.aspect_ratios?.includes(aspectRatio)
          ? { aspect_ratio: aspectRatio }
          : {}),
        ...(agentMediaModel.sizes?.includes(size) ? { size } : {}),
        ...(agentMediaModel.qualities?.includes(quality) ? { quality } : {}),
        ...((
          agentMediaModel.output_formats ?? agentMediaModel.formats
        )?.includes(format)
          ? { format }
          : {}),
        count:
          agentMediaKind === 'video' || !agentMediaModel.counts?.includes(count)
            ? 1
            : count,
        session_id: sessionId,
        inputs: agentTaskInputs,
        metadata: {
          model_name: agentMediaModel.name,
          model_family: agentMediaModel.family,
          ...(agentMediaModel.style_presets?.includes(stylePreset)
            ? { style_preset: stylePreset }
            : {}),
          ...(agentMediaModel.backgrounds?.includes(background)
            ? { background }
            : {}),
          ...(agentMediaModel.moderations?.includes(moderation)
            ? { moderation }
            : {}),
          ...(agentMediaModel.stream_modes?.includes(streamMode)
            ? { stream_mode: streamMode }
            : {}),
          ...(agentMediaModel.partial_images?.includes(partialImages)
            ? { partial_images: partialImages }
            : {}),
          ...(agentMediaModel.durations?.includes(duration)
            ? { duration }
            : {}),
          ...(agentMediaModel.supports_audio
            ? { generate_audio: generateAudio }
            : {}),
          visibility,
          reference_count: referenceFiles.length,
          surface: 'yucore-studio',
          canvas_identity_session: canvasIdentity?.identity_session,
          canvas_identity_expires_at: canvasIdentity?.expires_at,
        },
        agent_prompt_node_id: agentNodeId,
        agent_task_node_id: taskNodeId,
      })
      setCanvasIdentity(result.identity)
      setActiveTask(result.task)
      setTasks((items) => [
        result.task,
        ...items.filter((item) => item.task_id !== result.task.task_id),
      ])
      void getYucoreMediaBilling()
        .then(setBilling)
        .catch(() => undefined)
      setAgentRuns((items) => [
        result.run,
        ...items.filter((item) => item.run_id !== result.run.run_id),
      ])
      const syncedNodes = plannedNodes.map((node) =>
        node.id === taskNodeId
          ? ({
              ...node,
              data: {
                ...node.data,
                sublabel: `${result.task.group} / ${result.task.model_id}`,
                status: `${result.task.task_id} / ${result.task.status}`,
                resultTaskId: result.task.task_id,
              },
            } as StudioCanvasNode)
          : node
      )
      setNodes(syncedNodes)
      await persistCanvasSnapshot(syncedNodes, plannedEdges, 'Agent run', {
        autosave: true,
        createIfMissing: true,
        canvasOverride: canvasForRun,
        quiet: true,
        viewport: canvasViewport,
        backgroundMode: canvasBackgroundMode,
      })
      setAgentTab('history')
      toast.success('Agent execution created by backend runner.')
    } catch (error) {
      setAgentTab('logs')
      toast.error(
        error instanceof Error ? error.message : 'Agent execution failed'
      )
    } finally {
      setIsSubmitting(false)
    }
  }

  const canvasAgentTabs: Array<{
    id: CanvasAgentTab
    label: string
    icon: LucideIcon
  }> = [
    { id: 'connect', label: '连接配置', icon: SlidersHorizontal },
    { id: 'chat', label: '对话', icon: Bot },
    { id: 'history', label: '历史', icon: History },
    { id: 'logs', label: '日志', icon: Keyboard },
  ]

  const canvasMenuItems: CanvasMenuItem[] = [
    { icon: Home, label: '首页', action: () => openCanvasView('home') },
    {
      icon: GalleryHorizontalEnd,
      label: '我的画布',
      action: () => openCanvasView('canvas'),
    },
    {
      icon: ImagePlus,
      label: '生图工作台',
      action: () => openCanvasView('image'),
    },
    {
      icon: Clapperboard,
      label: '视频创作台',
      action: () => openCanvasView('video'),
    },
    {
      icon: FileText,
      label: '提示词库',
      action: () => openCanvasView('prompts'),
    },
    {
      icon: LibraryBig,
      label: '我的素材',
      action: () => openCanvasView('assets'),
    },
    { icon: Plus, label: '新建画布', action: handleCreateCanvas },
    {
      icon: RefreshCw,
      label: '刷新画布',
      action: () => void handleRefreshCanvas(),
    },
    {
      icon: FileText,
      label: '重命名画布',
      action: () => void handleRenameCanvas(),
      disabled: !activeCanvas,
    },
    { icon: Save, label: '保存画布', action: () => void handleSaveCanvas() },
    {
      icon: Upload,
      label: '导入画布',
      action: () => canvasImportInputRef.current?.click(),
    },
    { icon: Download, label: '导出画布', action: handleExportCanvas },
    {
      icon: Undo2,
      label: '撤销',
      shortcut: 'Ctrl Z',
      disabled: !canUndoCanvas,
      action: undoCanvas,
      separatorBefore: true,
    },
    {
      icon: Redo2,
      label: '重做',
      shortcut: 'Ctrl Shift Z',
      disabled: !canRedoCanvas,
      action: redoCanvas,
    },
    {
      icon: Eraser,
      label: '清空画布',
      danger: true,
      action: clearCanvas,
      separatorBefore: true,
    },
    {
      icon: Eraser,
      label: '删除服务端画布',
      danger: true,
      disabled: !activeCanvas,
      action: () => void handleDeleteCanvas(),
    },
  ]

  const canvasToolbarItems: Array<{
    id: CanvasToolId
    icon: LucideIcon
    label: string
    action: () => void
    disabled?: boolean
  }> = [
    {
      id: 'move',
      icon: MousePointer2,
      label: '移动/选择',
      action: () => setSelectedCanvasTool('move'),
    },
    {
      id: 'save',
      icon: Save,
      label: '保存画布',
      action: () => void handleSaveCanvas(),
    },
    {
      id: 'undo',
      icon: Undo2,
      label: '撤销',
      disabled: !canUndoCanvas,
      action: undoCanvas,
    },
    {
      id: 'redo',
      icon: Redo2,
      label: '重做',
      disabled: !canRedoCanvas,
      action: redoCanvas,
    },
    {
      id: 'text',
      icon: Type,
      label: '文本',
      action: () => addCanvasNode('text'),
    },
    {
      id: 'image',
      icon: ImagePlus,
      label: '图片',
      action: () => addCanvasNode('image'),
    },
    {
      id: 'video',
      icon: Clapperboard,
      label: '视频',
      action: () => addCanvasNode('video'),
    },
    {
      id: 'audio',
      icon: Music2,
      label: '音频',
      action: () => toast.info('音频素材节点将在视频链路中接入'),
    },
    {
      id: 'settings',
      icon: SlidersHorizontal,
      label: '生成配置',
      action: () => setView('image'),
    },
    {
      id: 'upload',
      icon: Upload,
      label: isCanvasMediaUploading ? '素材上传中' : '上传图片/视频',
      disabled: isCanvasMediaUploading,
      action: () => canvasMediaInputRef.current?.click(),
    },
    {
      id: 'assets',
      icon: FolderOpen,
      label: '我的素材',
      action: () => setView('assets'),
    },
    {
      id: 'organize',
      icon: LayoutGrid,
      label: '整理画布',
      action: organizeCanvas,
    },
    {
      id: 'appearance',
      icon: Palette,
      label: '画布外观',
      action: () =>
        handleCanvasBackgroundModeChange(
          canvasBackgroundMode === 'net' ? 'lines' : 'net'
        ),
    },
    { id: 'clear', icon: Eraser, label: '清空画布', action: clearCanvas },
  ]

  useEffect(() => {
    if (view !== 'canvas') return

    const handleCanvasKeyDown = (event: KeyboardEvent) => {
      if (isEditableKeyboardTarget(event.target)) return
      const key = event.key.toLowerCase()
      const modifier = event.ctrlKey || event.metaKey

      if (modifier && key === 's') {
        event.preventDefault()
        void handleSaveCanvas()
        return
      }
      if (modifier && key === 'a') {
        event.preventDefault()
        selectAllCanvasNodes()
        return
      }
      if (modifier && key === 'c') {
        event.preventDefault()
        copySelectedCanvasNodes()
        return
      }
      if (modifier && key === 'v') {
        event.preventDefault()
        pasteCanvasNodes()
        return
      }
      if (modifier && key === 'd') {
        event.preventDefault()
        duplicateSelectedCanvasNode()
        return
      }
      if (modifier && key === 'z') {
        event.preventDefault()
        if (event.shiftKey) {
          redoCanvas()
        } else {
          undoCanvas()
        }
        return
      }
      if (modifier && key === 'y') {
        event.preventDefault()
        redoCanvas()
        return
      }
      if (key === 'delete' || key === 'backspace') {
        event.preventDefault()
        deleteSelectedCanvasNode()
        return
      }
      if (key === 'escape') {
        setNodes((items) => items.map((node) => ({ ...node, selected: false })))
        setSelectedNodeId(null)
        setIsCanvasMenuOpen(false)
        setIsShortcutsOpen(false)
        return
      }
      if (key === '+' || key === '=') {
        event.preventDefault()
        handleCanvasZoomStep(1)
        return
      }
      if (key === '-') {
        event.preventDefault()
        handleCanvasZoomStep(-1)
        return
      }
      if (key === '0') {
        event.preventDefault()
        handleCanvasFitView()
      }
    }

    window.addEventListener('keydown', handleCanvasKeyDown)
    return () => window.removeEventListener('keydown', handleCanvasKeyDown)
  }, [
    copySelectedCanvasNodes,
    deleteSelectedCanvasNode,
    duplicateSelectedCanvasNode,
    handleCanvasFitView,
    handleCanvasZoomStep,
    handleSaveCanvas,
    pasteCanvasNodes,
    redoCanvas,
    selectAllCanvasNodes,
    setNodes,
    undoCanvas,
    view,
  ])

  return (
    <YucorePageShell
      intensity='workbench'
      paddedTop={false}
      showBackground={false}
      className='yucore-studio-shell h-full min-h-0 !bg-transparent'
      contentClassName='flex h-full min-h-0 max-w-none flex-col gap-0 overflow-hidden p-0'
    >
      <input
        ref={canvasImportInputRef}
        type='file'
        accept='application/json,.json'
        className='hidden'
        onChange={handleCanvasImportChange}
      />
      <input
        ref={canvasMediaInputRef}
        type='file'
        accept='image/*,video/*'
        multiple
        className='hidden'
        onChange={handleCanvasMediaInputChange}
      />
      <div className='flex h-full min-h-0 flex-1 flex-col bg-black/10'>
        {view !== 'canvas' && (
          <header className='z-20 flex h-14 shrink-0 items-center justify-between gap-3 border-b border-white/10 bg-black/62 px-4 backdrop-blur-2xl'>
            <div className='flex min-w-0 items-center gap-3'>
              <button
                type='button'
                className='flex size-9 items-center justify-center rounded-lg border border-white/10 bg-white/[0.035] text-white/72 hover:bg-white/10'
                aria-label='打开侧栏'
                title='打开侧栏'
              >
                <PanelLeft className='size-4' />
              </button>
              <YucoreBrandMark />
              <div className='hidden h-5 w-px bg-white/10 sm:block' />
              <div className='hidden min-w-0 sm:block'>
                <div className='truncate text-sm font-semibold text-white'>
                  YuCore Pixel
                </div>
                <div className='truncate text-[11px] text-white/38'>
                  {YUCORE_STUDIO_NAME} / 用户端创作工作区
                </div>
              </div>
            </div>

            <nav className='hidden min-w-0 flex-1 items-center gap-1 overflow-x-auto lg:flex'>
              {studioNavItems.map((item) => {
                const Icon = item.icon
                return (
                  <button
                    key={item.id}
                    type='button'
                    onClick={() => setView(item.id)}
                    className={cn(
                      'flex h-10 shrink-0 items-center gap-2 rounded-lg border px-3 text-sm whitespace-nowrap transition',
                      view === item.id
                        ? 'border-cyan-100/24 bg-cyan-300/14 text-white shadow-[0_0_24px_rgba(34,211,238,0.1)]'
                        : 'border-transparent text-white/70 hover:bg-white/[0.07] hover:text-white'
                    )}
                  >
                    <Icon className='size-4' />
                    <span>{item.label}</span>
                  </button>
                )
              })}
            </nav>

            <div className='flex items-center gap-2'>
              <button
                type='button'
                className='hidden h-9 items-center gap-2 rounded-lg border border-white/10 bg-white/[0.035] px-3 text-xs text-white/62 hover:bg-white/10 md:flex'
              >
                <Search className='size-4' />
                搜索
              </button>
              <button
                type='button'
                className='flex size-9 items-center justify-center rounded-lg border border-white/10 bg-white/[0.035] text-white/62 hover:bg-white/10'
                aria-label='配置'
                title='配置'
              >
                <Settings2 className='size-4' />
              </button>
              <button
                type='button'
                className='flex size-9 items-center justify-center rounded-lg border border-white/10 bg-white/[0.035] text-white/62 hover:bg-white/10'
                aria-label='切换主题'
                title='切换主题'
              >
                <Moon className='size-4' />
              </button>
            </div>
          </header>
        )}

        {view !== 'canvas' && (
          <nav className='flex shrink-0 gap-1 overflow-x-auto border-b border-white/10 bg-black/72 px-2 py-2 backdrop-blur-xl lg:hidden'>
            {studioNavItems.map((item) => {
              const Icon = item.icon
              return (
                <button
                  key={item.id}
                  type='button'
                  onClick={() => setView(item.id)}
                  className={cn(
                    'flex h-9 shrink-0 items-center gap-2 rounded-lg border px-3 text-xs whitespace-nowrap transition',
                    view === item.id
                      ? 'border-cyan-100/24 bg-cyan-300/14 text-white shadow-[0_0_24px_rgba(34,211,238,0.12)]'
                      : 'border-white/10 bg-white/[0.035] text-white/70 hover:border-white/18 hover:bg-white/[0.075] hover:text-white'
                  )}
                >
                  <Icon className='size-4' />
                  <span>{item.label}</span>
                </button>
              )
            })}
          </nav>
        )}

        <div className='flex min-h-0 flex-1'>
          {view !== 'canvas' && (
            <aside className='hidden w-14 shrink-0 border-r border-white/10 bg-black/46 px-2 py-3 backdrop-blur-xl md:block'>
              <div className='grid gap-2'>
                {studioNavItems.map((item) => {
                  const Icon = item.icon
                  return (
                    <button
                      key={item.id}
                      type='button'
                      onClick={() => setView(item.id)}
                      className={cn(
                        'flex size-10 items-center justify-center rounded-xl border text-white/58 transition',
                        view === item.id
                          ? 'border-cyan-100/24 bg-cyan-300/14 text-white'
                          : 'border-transparent hover:bg-white/[0.07] hover:text-white'
                      )}
                      aria-label={item.label}
                      title={item.label}
                    >
                      <Icon className='size-4' />
                    </button>
                  )
                })}
              </div>
            </aside>
          )}

          <main className='min-h-0 flex-1 overflow-hidden'>
            {view === 'home' && (
              <section className='yucore-pixel-grid relative h-full overflow-y-auto p-6 lg:p-8'>
                <div className='mx-auto grid min-h-[calc(100svh-8rem)] max-w-7xl content-center gap-8'>
                  <div className='max-w-4xl'>
                    <p className='mb-4 text-xs font-semibold tracking-[0.24em] text-cyan-100/55 uppercase'>
                      YuCore Pixel Core
                    </p>
                    <h1 className='max-w-5xl text-6xl font-semibold tracking-normal text-white sm:text-7xl lg:text-8xl'>
                      光合像素
                    </h1>
                    <p className='mt-6 max-w-3xl text-lg leading-8 text-white/58'>
                      在 YuCore
                      中生成、连接和重组图片、文字与图形，让创作从一次生成变成连续推演。
                    </p>
                    <div className='mt-8 flex flex-wrap gap-3'>
                      <Button
                        className='h-11 rounded-xl bg-white px-5 text-black hover:bg-cyan-50'
                        onClick={() => setView('image')}
                      >
                        开始使用
                        <ArrowRight data-icon='inline-end' />
                      </Button>
                      <Button
                        variant='outline'
                        className='h-11 rounded-xl border-white/15 bg-white/[0.035] px-5 text-white hover:bg-white/10'
                        onClick={() => setView('canvas')}
                      >
                        打开画布
                      </Button>
                    </div>
                  </div>

                  <div className='grid gap-4 lg:grid-cols-[1.15fr_0.85fr]'>
                    <section className='rounded-2xl border border-white/10 bg-black/42 p-5 backdrop-blur-xl'>
                      <div className='mb-4 flex items-center justify-between gap-3'>
                        <div>
                          <h2 className='text-2xl font-semibold text-white'>
                            沉淀每一次好结果
                          </h2>
                          <p className='mt-1 text-sm text-white/48'>
                            收藏稳定出图的提示词、参考风格和结果图片。
                          </p>
                        </div>
                        <Button
                          variant='outline'
                          className='h-9 rounded-xl border-white/15 bg-white/[0.035] text-white hover:bg-white/10'
                          onClick={() => setView('prompts')}
                        >
                          查看提示词库
                        </Button>
                      </div>
                      <AssetGrid
                        tasks={completedTasks.slice(0, 4)}
                        compact
                        onSendToCanvas={addAssetToCanvas}
                      />
                    </section>

                    <section className='grid gap-3'>
                      {studioNavItems.slice(1).map((item) => {
                        const Icon = item.icon
                        return (
                          <button
                            key={item.id}
                            type='button'
                            onClick={() => setView(item.id)}
                            className='flex items-center justify-between gap-4 rounded-2xl border border-white/10 bg-white/[0.035] p-4 text-left transition hover:border-cyan-200/22 hover:bg-cyan-300/10'
                          >
                            <span className='flex min-w-0 items-center gap-3'>
                              <span className='flex size-10 shrink-0 items-center justify-center rounded-xl border border-white/10 bg-black/40 text-cyan-100'>
                                <Icon className='size-4' />
                              </span>
                              <span className='min-w-0'>
                                <span className='block text-sm font-semibold text-white'>
                                  {item.label}
                                </span>
                                <span className='mt-1 block truncate text-xs text-white/42'>
                                  任务、素材和画布状态会按用户保存
                                </span>
                              </span>
                            </span>
                            <ArrowRight className='size-4 shrink-0 text-white/34' />
                          </button>
                        )
                      })}
                    </section>
                  </div>
                </div>
              </section>
            )}

            {view === 'canvas' && (
              <section
                className={cn(
                  'yucore-canvas-motion-stage relative h-full min-h-0 overflow-hidden bg-[#050b0e]/62',
                  `yucore-canvas-motion-stage-${canvasBackgroundMode}`
                )}
              >
                <div
                  className='yucore-canvas-particle-field absolute inset-0'
                  aria-hidden='true'
                />
                <ReactFlow
                  key={activeCanvas?.id ?? 'draft-canvas'}
                  className='yucore-flow-canvas yucore-studio-full-canvas relative z-[2]'
                  nodes={nodes}
                  edges={edges}
                  nodeTypes={canvasNodeTypes}
                  onInit={(instance) => {
                    reactFlowInstanceRef.current = instance
                    setReactFlowInstance(instance)
                    if (canvasViewport) {
                      void instance.setViewport(canvasViewport)
                    }
                  }}
                  onNodesChange={handleNodesChange}
                  onEdgesChange={handleEdgesChange}
                  onConnect={handleConnect}
                  onDragOver={(event) => {
                    event.preventDefault()
                    event.dataTransfer.dropEffect = 'copy'
                  }}
                  onDrop={handleCanvasMediaDrop}
                  onMove={handleCanvasMove}
                  onMoveEnd={handleCanvasMoveEnd}
                  onNodeClick={(_, node) => {
                    setSelectedNodeId(node.id)
                    setIsCanvasMenuOpen(false)
                  }}
                  onPaneClick={() => {
                    setSelectedNodeId(null)
                    setIsCanvasMenuOpen(false)
                  }}
                  defaultViewport={effectiveCanvasViewport}
                  fitView={!canvasViewport}
                  fitViewOptions={{ padding: 0.32 }}
                  minZoom={0.05}
                  maxZoom={5}
                  panOnScroll
                  zoomOnScroll
                >
                  {canvasBackgroundMode !== 'blank' && (
                    <Background
                      variant={
                        canvasBackgroundMode === 'dots'
                          ? BackgroundVariant.Dots
                          : BackgroundVariant.Lines
                      }
                      gap={canvasBackgroundMode === 'dots' ? 30 : 48}
                      size={canvasBackgroundMode === 'dots' ? 1.5 : 1}
                      color={
                        canvasBackgroundMode === 'net'
                          ? 'rgba(103,232,249,0.12)'
                          : 'rgba(255,255,255,0.105)'
                      }
                    />
                  )}
                  {canvasMiniMapOpen && (
                    <MiniMap
                      nodeColor={() => 'rgba(235,235,224,0.72)'}
                      maskColor='rgba(0,0,0,0.56)'
                      position='top-right'
                      pannable
                      zoomable
                    />
                  )}
                </ReactFlow>

                <div className='absolute inset-x-0 top-0 z-20 flex h-14 items-center justify-between border-b border-white/10 bg-[#071014]/86 px-4 backdrop-blur-xl'>
                  <div className='flex min-w-0 items-center gap-3'>
                    <button
                      type='button'
                      className={cn(
                        'flex size-9 items-center justify-center rounded-lg text-white/72 transition hover:bg-white/10 hover:text-white',
                        isCanvasMenuOpen && 'bg-white/10 text-white'
                      )}
                      aria-label='打开画布菜单'
                      title='打开画布菜单'
                      onClick={() => setIsCanvasMenuOpen((open) => !open)}
                    >
                      <PanelLeft className='size-4' />
                    </button>
                    <button
                      type='button'
                      className='truncate text-sm font-semibold text-white'
                      onClick={() => setIsCanvasMenuOpen((open) => !open)}
                    >
                      {activeCanvas?.title ?? '光合像素 1'}
                    </button>
                    <span className='hidden text-[11px] text-white/34 sm:inline'>
                      {canvasSyncLabel}
                    </span>
                  </div>
                  <div className='flex items-center gap-1.5'>
                    <button
                      type='button'
                      className='flex size-8 items-center justify-center rounded-lg text-white/58 hover:bg-white/10 hover:text-white'
                      aria-label='配置'
                      title='配置'
                    >
                      <SlidersHorizontal className='size-4' />
                    </button>
                    <button
                      type='button'
                      className='flex size-8 items-center justify-center rounded-lg text-white/58 hover:bg-white/10 hover:text-white'
                      aria-label='切换到浅色主题'
                      title='切换到浅色主题'
                    >
                      <Sun className='size-4' />
                    </button>
                    <button
                      type='button'
                      className='hidden size-8 items-center justify-center rounded-lg text-white/58 hover:bg-white/10 hover:text-white sm:flex'
                      aria-label='快捷键'
                      title='快捷键'
                      aria-pressed={isShortcutsOpen}
                      onClick={() => setIsShortcutsOpen(true)}
                    >
                      <Keyboard className='size-4' />
                    </button>
                    <Button
                      className={cn(
                        'h-9 rounded-xl px-3 text-xs',
                        isAgentPanelOpen
                          ? 'bg-white text-black hover:bg-cyan-50'
                          : 'border border-white/10 bg-white/[0.055] text-white hover:bg-white/10'
                      )}
                      onClick={() => setIsAgentPanelOpen((open) => !open)}
                    >
                      <Bot data-icon='inline-start' />
                      Agent
                    </Button>
                  </div>
                </div>

                {isShortcutsOpen && (
                  <div className='absolute inset-0 z-50 flex items-center justify-center p-4'>
                    <button
                      type='button'
                      className='absolute inset-0 bg-black/72 backdrop-blur-sm'
                      aria-label='关闭快捷键说明'
                      onClick={() => setIsShortcutsOpen(false)}
                    />
                    <section
                      role='dialog'
                      aria-modal='true'
                      aria-label='画布快捷键'
                      className='relative w-full max-w-xl rounded-3xl border border-white/12 bg-[#081216]/98 p-5 shadow-[0_32px_100px_rgb(0_0_0/0.62)]'
                    >
                      <div className='flex items-start justify-between gap-4'>
                        <div>
                          <div className='text-lg font-semibold text-white'>
                            画布快捷键
                          </div>
                          <p className='mt-1 text-xs leading-5 text-white/42'>
                            对齐 infinite-canvas 的核心选择、编辑和视图操作。
                          </p>
                        </div>
                        <button
                          type='button'
                          className='flex size-8 items-center justify-center rounded-lg text-white/55 hover:bg-white/10 hover:text-white'
                          aria-label='关闭快捷键说明'
                          onClick={() => setIsShortcutsOpen(false)}
                        >
                          <X className='size-4' />
                        </button>
                      </div>
                      <div className='mt-5 grid gap-2 sm:grid-cols-2'>
                        {[
                          ['Ctrl / Cmd + S', '保存画布'],
                          ['Ctrl / Cmd + A', '全选节点'],
                          ['Ctrl / Cmd + C', '复制节点与连线'],
                          ['Ctrl / Cmd + V', '粘贴节点与连线'],
                          ['Ctrl / Cmd + D', '复制当前节点'],
                          ['Ctrl / Cmd + Z', '撤销'],
                          ['Ctrl / Cmd + Shift + Z', '重做'],
                          ['Delete / Backspace', '删除选中节点'],
                          ['Esc', '取消选择与关闭浮层'],
                          ['+ / - / 0', '缩放与适配视图'],
                        ].map(([shortcut, label]) => (
                          <div
                            key={shortcut}
                            className='flex items-center justify-between gap-3 rounded-xl border border-white/8 bg-black/28 px-3 py-2.5'
                          >
                            <span className='text-xs text-white/58'>
                              {label}
                            </span>
                            <kbd className='rounded-md border border-white/12 bg-white/[0.055] px-2 py-1 font-mono text-[10px] text-cyan-100'>
                              {shortcut}
                            </kbd>
                          </div>
                        ))}
                      </div>
                    </section>
                  </div>
                )}

                {isCanvasMenuOpen && (
                  <div className='yucore-canvas-menu absolute top-16 left-4 z-30 w-64 rounded-2xl border border-white/10 bg-[#081216]/95 p-2 shadow-[0_28px_80px_rgb(0_0_0/0.42)] backdrop-blur-2xl'>
                    {canvasMenuItems.map((item, index) => {
                      const Icon = item.icon
                      const hasDivider = item.separatorBefore || index === 6
                      return (
                        <div key={item.label}>
                          {hasDivider && (
                            <div className='my-1 h-px bg-white/10' />
                          )}
                          <button
                            type='button'
                            disabled={item.disabled}
                            onClick={() => {
                              if (item.disabled) return
                              item.action()
                            }}
                            className={cn(
                              'flex h-10 w-full items-center justify-between gap-3 rounded-xl px-3 text-left text-sm transition',
                              item.danger
                                ? 'text-rose-200 hover:bg-rose-400/10'
                                : 'text-white/74 hover:bg-white/[0.075] hover:text-white',
                              item.disabled &&
                                'cursor-not-allowed opacity-35 hover:bg-transparent'
                            )}
                          >
                            <span className='flex min-w-0 items-center gap-3'>
                              <Icon className='size-4 shrink-0' />
                              <span className='truncate'>{item.label}</span>
                            </span>
                            {item.shortcut && (
                              <span className='font-mono text-[11px] text-white/28'>
                                {item.shortcut}
                              </span>
                            )}
                          </button>
                        </div>
                      )
                    })}
                  </div>
                )}

                {selectedNode && (
                  <aside className='absolute top-20 left-5 z-20 flex max-h-[calc(100%-7rem)] w-[min(22rem,calc(100vw-2.5rem))] flex-col overflow-hidden rounded-2xl border border-white/10 bg-[#071116]/92 text-white shadow-[0_24px_90px_rgb(0_0_0/0.46)] backdrop-blur-2xl'>
                    <div className='flex shrink-0 items-center justify-between gap-3 border-b border-white/10 px-4 py-3'>
                      <div className='min-w-0'>
                        <div className='truncate text-sm font-semibold'>
                          节点检查器
                        </div>
                        <div className='mt-1 truncate text-[11px] text-white/38'>
                          {selectedNode.id}
                        </div>
                      </div>
                      <button
                        type='button'
                        className='flex size-8 shrink-0 items-center justify-center rounded-lg text-white/58 hover:bg-white/10 hover:text-white'
                        aria-label='关闭节点检查器'
                        title='关闭节点检查器'
                        onClick={() => setSelectedNodeId(null)}
                      >
                        <X className='size-4' />
                      </button>
                    </div>
                    <div className='min-h-0 flex-1 overflow-y-auto p-4'>
                      <label className='block text-xs font-medium text-white/52'>
                        标题
                        <input
                          value={selectedNode.data.label}
                          onChange={(event) =>
                            updateSelectedCanvasNode({
                              label: event.target.value,
                            })
                          }
                          className='mt-2 h-10 w-full rounded-xl border border-white/10 bg-black/34 px-3 text-sm text-white transition outline-none placeholder:text-white/28 focus:border-cyan-200/45'
                        />
                      </label>
                      <label className='mt-4 block text-xs font-medium text-white/52'>
                        副标题
                        <input
                          value={selectedNode.data.sublabel ?? ''}
                          onChange={(event) =>
                            updateSelectedCanvasNode({
                              sublabel: event.target.value,
                            })
                          }
                          className='mt-2 h-10 w-full rounded-xl border border-white/10 bg-black/34 px-3 text-sm text-white transition outline-none placeholder:text-white/28 focus:border-cyan-200/45'
                        />
                      </label>
                      <div className='mt-4'>
                        <div className='mb-2 text-xs font-medium text-white/52'>
                          节点类型
                        </div>
                        <div className='grid grid-cols-4 gap-1 rounded-xl border border-white/10 bg-black/28 p-1'>
                          {(['text', 'prompt', 'image', 'video'] as const).map(
                            (item) => (
                              <button
                                key={item}
                                type='button'
                                onClick={() =>
                                  updateSelectedCanvasNode({ kind: item })
                                }
                                className={cn(
                                  'h-8 rounded-lg text-[11px] font-medium transition',
                                  (selectedNode.data.kind ?? 'text') === item
                                    ? 'bg-white text-black'
                                    : 'text-white/58 hover:bg-white/10 hover:text-white'
                                )}
                              >
                                {item}
                              </button>
                            )
                          )}
                        </div>
                      </div>
                      <label className='mt-4 block text-xs font-medium text-white/52'>
                        提示词 / 节点说明
                        <textarea
                          value={selectedNode.data.prompt ?? ''}
                          onChange={(event) =>
                            updateSelectedCanvasNode({
                              prompt: event.target.value,
                            })
                          }
                          className='mt-2 min-h-28 w-full resize-none rounded-xl border border-white/10 bg-black/34 px-3 py-2 text-sm leading-6 text-white transition outline-none placeholder:text-white/28 focus:border-cyan-200/45'
                        />
                      </label>
                      {selectedNode.data.assetUrl && (
                        <label className='mt-4 block text-xs font-medium text-white/52'>
                          素材地址
                          <input
                            value={selectedNode.data.assetUrl}
                            readOnly
                            className='mt-2 h-10 w-full rounded-xl border border-white/10 bg-black/34 px-3 font-mono text-[11px] text-white/58 outline-none'
                          />
                        </label>
                      )}
                      {selectedNode.data.status && (
                        <div className='mt-4 rounded-xl border border-white/10 bg-white/[0.035] p-3 text-xs leading-5 text-white/48'>
                          {selectedNode.data.status}
                        </div>
                      )}
                    </div>
                    <div className='grid shrink-0 grid-cols-2 gap-2 border-t border-white/10 p-3'>
                      <Button
                        variant='outline'
                        className='h-9 rounded-xl border-white/10 bg-white/[0.035] text-white hover:bg-white/10'
                        onClick={duplicateSelectedCanvasNode}
                      >
                        <CirclePlus data-icon='inline-start' />
                        复制
                      </Button>
                      <Button
                        variant='outline'
                        className='h-9 rounded-xl border-rose-200/18 bg-rose-400/10 text-rose-100 hover:bg-rose-400/16'
                        onClick={deleteSelectedCanvasNode}
                      >
                        <Eraser data-icon='inline-start' />
                        删除
                      </Button>
                      <Button
                        className='col-span-2 h-10 rounded-xl bg-white text-black hover:bg-cyan-50'
                        disabled={isSubmitting}
                        onClick={generateFromSelectedCanvasNode}
                      >
                        {isSubmitting ? (
                          <Loader2
                            data-icon='inline-start'
                            className='animate-spin'
                          />
                        ) : (
                          <WandSparkles data-icon='inline-start' />
                        )}
                        从节点生成
                      </Button>
                    </div>
                  </aside>
                )}

                <div className='absolute bottom-20 left-5 z-20 flex max-w-[calc(100vw-2.5rem)] items-center gap-2 overflow-x-auto rounded-2xl border border-white/10 bg-black/64 px-3 py-2 text-white/70 backdrop-blur-2xl xl:bottom-5 xl:max-w-[calc(100vw-10rem)]'>
                  <button
                    type='button'
                    className={cn(
                      'flex size-8 items-center justify-center rounded-xl hover:bg-white/10',
                      canvasMiniMapOpen && 'bg-white/12 text-white'
                    )}
                    aria-label='Toggle minimap'
                    title='Toggle minimap'
                    onClick={() => setCanvasMiniMapOpen((open) => !open)}
                  >
                    <MousePointer2 className='size-4' />
                  </button>
                  <div className='mx-1 flex items-center gap-1 rounded-xl border border-white/10 bg-white/[0.035] p-1'>
                    {canvasBackgroundModes.map((item) => {
                      const Icon = item.icon

                      return (
                        <button
                          key={item.id}
                          type='button'
                          className={cn(
                            'flex size-7 items-center justify-center rounded-lg text-[11px] transition sm:w-auto sm:px-2',
                            canvasBackgroundMode === item.id
                              ? 'bg-white text-black'
                              : 'text-white/52 hover:bg-white/10 hover:text-white'
                          )}
                          aria-label={item.label}
                          title={item.label}
                          onClick={() =>
                            handleCanvasBackgroundModeChange(item.id)
                          }
                        >
                          <Icon className='size-3.5 shrink-0' />
                          <span className='hidden sm:ml-1 sm:inline'>
                            {item.label}
                          </span>
                        </button>
                      )
                    })}
                  </div>
                  <button
                    type='button'
                    className='flex size-8 items-center justify-center rounded-xl hover:bg-white/10'
                    aria-label='缩小画布'
                    title='缩小画布'
                    onClick={() => handleCanvasZoomStep(-1)}
                  >
                    <ZoomOut className='size-4' />
                  </button>
                  <input
                    aria-label='画布缩放'
                    className='h-1 w-24 accent-white'
                    type='range'
                    min='5'
                    max='500'
                    value={canvasZoomPercent}
                    onChange={(event) =>
                      handleCanvasZoomChange(Number(event.target.value) / 100)
                    }
                  />
                  <span className='font-mono text-xs text-white/76'>
                    {canvasZoomPercent}%
                  </span>
                  <button
                    type='button'
                    className='flex size-8 items-center justify-center rounded-xl hover:bg-white/10'
                    aria-label='放大画布'
                    title='放大画布'
                    onClick={() => handleCanvasZoomStep(1)}
                  >
                    <ZoomIn className='size-4' />
                  </button>
                  <button
                    type='button'
                    className='flex size-8 items-center justify-center rounded-xl hover:bg-white/10'
                    aria-label='Fit view'
                    title='Fit view'
                    onClick={handleCanvasFitView}
                  >
                    <Focus className='size-4' />
                  </button>
                </div>

                <div className='yucore-studio-floating-toolbar absolute bottom-5 left-1/2 z-10 flex max-w-[calc(100vw-2rem)] -translate-x-1/2 items-center gap-1 overflow-x-auto rounded-2xl border border-white/10 bg-[#071116]/88 p-2 backdrop-blur-2xl'>
                  {canvasToolbarItems.map(
                    ({ id, icon: Icon, label, action, disabled }) => (
                      <button
                        key={id}
                        type='button'
                        disabled={disabled}
                        onClick={() => {
                          if (disabled) return
                          setSelectedCanvasTool(id)
                          action()
                        }}
                        className={cn(
                          'flex size-9 shrink-0 items-center justify-center rounded-xl text-white/58 transition hover:bg-white/10 hover:text-white',
                          selectedCanvasTool === id && 'bg-white/12 text-white',
                          disabled &&
                            'cursor-not-allowed opacity-32 hover:bg-transparent hover:text-white/58'
                        )}
                        aria-label={label}
                        title={label}
                      >
                        <Icon className='size-4' />
                      </button>
                    )
                  )}
                </div>

                {isAgentPanelOpen && (
                  <aside className='yucore-canvas-agent-panel absolute inset-y-0 right-0 z-30 flex w-[calc(100vw-1rem)] flex-col border-l border-white/10 bg-[#071116]/76 text-white shadow-[-18px_0_70px_rgb(0_0_0/0.36)] backdrop-blur-xl sm:w-[min(34rem,calc(100vw-4rem))] md:w-[min(34rem,calc(100vw-18rem))]'>
                    <div className='flex h-14 shrink-0 items-center justify-between border-b border-white/10 px-4'>
                      <div className='flex items-center gap-3'>
                        <span className='flex size-8 items-center justify-center rounded-lg border border-white/10 bg-white/[0.035] text-cyan-100'>
                          <Bot className='size-4' />
                        </span>
                        <div>
                          <div className='text-sm font-semibold'>Agent</div>
                          <div className='text-[11px] text-white/40'>
                            画布助手
                          </div>
                        </div>
                      </div>
                      <div className='flex items-center gap-2'>
                        <div className='hidden rounded-xl border border-white/10 bg-black/35 p-1 sm:flex'>
                          {(['site', 'local'] as const).map((item) => (
                            <button
                              key={item}
                              type='button'
                              onClick={() => setAgentMode(item)}
                              className={cn(
                                'h-7 rounded-lg px-3 text-xs transition',
                                agentMode === item
                                  ? 'bg-white text-black'
                                  : 'text-white/58 hover:bg-white/10 hover:text-white'
                              )}
                            >
                              {item === 'site' ? '网站' : '本机'}
                            </button>
                          ))}
                        </div>
                        <label className='hidden items-center gap-2 text-xs text-white/58 sm:flex'>
                          <input
                            type='checkbox'
                            checked={toolConfirmationEnabled}
                            onChange={(event) =>
                              setToolConfirmationEnabled(event.target.checked)
                            }
                            className='size-4 accent-white'
                          />
                          工具确认
                        </label>
                        <button
                          type='button'
                          className='flex size-8 items-center justify-center rounded-lg text-white/58 hover:bg-white/10 hover:text-white'
                          aria-label='关闭 Agent'
                          title='关闭 Agent'
                          onClick={() => setIsAgentPanelOpen(false)}
                        >
                          <X className='size-4' />
                        </button>
                      </div>
                    </div>

                    <div className='flex h-12 shrink-0 items-center gap-1 overflow-x-auto border-b border-white/10 px-3'>
                      {canvasAgentTabs.map((item) => {
                        const Icon = item.icon
                        return (
                          <button
                            key={item.id}
                            type='button'
                            onClick={() => setAgentTab(item.id)}
                            className={cn(
                              'flex h-9 shrink-0 items-center gap-2 rounded-lg px-3 text-sm transition',
                              agentTab === item.id
                                ? 'bg-white text-black'
                                : 'text-white/58 hover:bg-white/10 hover:text-white'
                            )}
                          >
                            <Icon className='size-4' />
                            <span>{item.label}</span>
                          </button>
                        )
                      })}
                    </div>

                    <div className='min-h-0 flex-1 overflow-y-auto p-5'>
                      {agentTab === 'connect' && (
                        <div className='grid gap-3'>
                          <div className='rounded-2xl border border-white/10 bg-white/[0.035] p-4'>
                            <div className='text-sm font-semibold'>
                              媒体引擎
                            </div>
                            <div className='mt-2 text-sm text-white/58'>
                              {mediaEngineLabel(mediaHealth)}
                            </div>
                            {mediaHealth?.message && (
                              <p className='mt-3 text-xs leading-5 text-amber-100/70'>
                                {mediaHealth.message}
                              </p>
                            )}
                            {mediaHealth?.verification_blockers?.length ? (
                              <div className='mt-3 grid gap-1.5'>
                                {mediaHealth.verification_blockers
                                  .slice(0, 4)
                                  .map((blocker) => (
                                    <div
                                      key={blocker}
                                      className='rounded-lg border border-amber-200/12 bg-amber-300/8 px-2.5 py-2 text-xs leading-5 text-amber-100/68'
                                    >
                                      {blocker}
                                    </div>
                                  ))}
                              </div>
                            ) : null}
                          </div>
                          <div className='rounded-2xl border border-white/10 bg-white/[0.035] p-4'>
                            <div className='flex items-center justify-between gap-3'>
                              <div className='text-sm font-semibold'>
                                身份桥
                              </div>
                              <span
                                className={cn(
                                  'rounded-md border px-2 py-1 text-[11px]',
                                  canvasIdentity
                                    ? 'border-emerald-200/20 bg-emerald-300/10 text-emerald-100'
                                    : 'border-amber-200/20 bg-amber-300/10 text-amber-100'
                                )}
                              >
                                {canvasIdentity ? '已签发' : '未签发'}
                              </span>
                            </div>
                            <div className='mt-3 grid gap-2 text-xs text-white/46'>
                              <div className='flex items-center justify-between gap-3'>
                                <span>session</span>
                                <span className='max-w-[12rem] truncate font-mono text-white/62'>
                                  {canvasIdentity?.identity_session ?? 'none'}
                                </span>
                              </div>
                              <div className='flex items-center justify-between gap-3'>
                                <span>expires</span>
                                <span className='font-mono text-white/62'>
                                  {canvasIdentity
                                    ? formatTime(canvasIdentity.expires_at)
                                    : 'none'}
                                </span>
                              </div>
                            </div>
                          </div>
                          <div className='rounded-2xl border border-white/10 bg-white/[0.035] p-4'>
                            <div className='flex items-center justify-between gap-3'>
                              <div>
                                <div className='text-sm font-semibold'>
                                  画布库
                                </div>
                                <div className='mt-1 text-[11px] text-white/38'>
                                  服务端项目、快照与版本记录
                                </div>
                              </div>
                              <div className='flex items-center gap-1'>
                                <button
                                  type='button'
                                  className='flex size-8 items-center justify-center rounded-lg text-white/56 hover:bg-white/10 hover:text-white'
                                  aria-label='刷新画布库'
                                  title='刷新画布库'
                                  onClick={() => void handleRefreshCanvas()}
                                >
                                  <RefreshCw className='size-4' />
                                </button>
                                <button
                                  type='button'
                                  className='flex size-8 items-center justify-center rounded-lg text-white/56 hover:bg-white/10 hover:text-white disabled:cursor-not-allowed disabled:opacity-35'
                                  aria-label='重命名画布'
                                  title='重命名画布'
                                  disabled={!activeCanvas}
                                  onClick={() => void handleRenameCanvas()}
                                >
                                  <FileText className='size-4' />
                                </button>
                              </div>
                            </div>
                            <div className='mt-3 grid gap-2'>
                              {canvases.length === 0 ? (
                                <div className='rounded-xl border border-dashed border-white/12 p-3 text-xs text-white/42'>
                                  当前是本地草稿，保存后会出现在这里。
                                </div>
                              ) : (
                                canvases.slice(0, 6).map((canvas) => (
                                  <button
                                    key={canvas.id}
                                    type='button'
                                    onClick={() =>
                                      void openCanvasRecord(canvas)
                                    }
                                    className={cn(
                                      'rounded-xl border p-3 text-left text-sm transition',
                                      activeCanvas?.id === canvas.id
                                        ? 'border-cyan-200/25 bg-cyan-300/10'
                                        : 'border-white/10 bg-black/28 hover:bg-white/[0.06]'
                                    )}
                                  >
                                    <div className='truncate font-semibold'>
                                      {canvas.title}
                                    </div>
                                    <div className='mt-1 text-[11px] text-white/38'>
                                      {Array.isArray(canvas.snapshot?.nodes)
                                        ? canvas.snapshot.nodes.length
                                        : 0}{' '}
                                      节点 / v{canvas.revision}
                                    </div>
                                  </button>
                                ))
                              )}
                            </div>
                            <div className='mt-3 grid grid-cols-2 gap-2'>
                              <Button
                                variant='outline'
                                className='h-9 rounded-xl border-white/10 bg-white/[0.035] text-xs text-white hover:bg-white/10'
                                onClick={() => void handleSaveCanvas()}
                              >
                                <Save data-icon='inline-start' />
                                保存
                              </Button>
                              <Button
                                variant='outline'
                                className='h-9 rounded-xl border-rose-200/18 bg-rose-400/10 text-xs text-rose-100 hover:bg-rose-400/16'
                                disabled={!activeCanvas}
                                onClick={() => void handleDeleteCanvas()}
                              >
                                <Eraser data-icon='inline-start' />
                                删除
                              </Button>
                            </div>
                          </div>
                          <div className='rounded-2xl border border-white/10 bg-white/[0.035] p-4'>
                            <div className='flex items-center justify-between gap-3'>
                              <div>
                                <div className='text-sm font-semibold'>
                                  版本记录
                                </div>
                                <div className='mt-1 text-[11px] text-white/38'>
                                  手动保存会生成可恢复版本
                                </div>
                              </div>
                              {activeCanvas && (
                                <span className='rounded-md border border-white/10 bg-black/28 px-2 py-1 font-mono text-[11px] text-white/48'>
                                  v{activeCanvas.revision}
                                </span>
                              )}
                            </div>
                            <div className='mt-3 grid gap-2'>
                              {!activeCanvas && (
                                <div className='rounded-xl border border-dashed border-white/12 p-3 text-xs text-white/42'>
                                  保存画布后会显示版本。
                                </div>
                              )}
                              {activeCanvas && isLoadingCanvasVersions && (
                                <div className='rounded-xl border border-white/10 bg-black/28 p-3 text-xs text-white/42'>
                                  正在加载版本记录...
                                </div>
                              )}
                              {activeCanvas &&
                                !isLoadingCanvasVersions &&
                                canvasVersions.length === 0 && (
                                  <div className='rounded-xl border border-dashed border-white/12 p-3 text-xs text-white/42'>
                                    暂无版本记录。
                                  </div>
                                )}
                              {activeCanvas &&
                                !isLoadingCanvasVersions &&
                                canvasVersions.length > 0 &&
                                canvasVersions.slice(0, 6).map((version) => (
                                  <button
                                    key={version.id}
                                    type='button'
                                    className='rounded-xl border border-white/10 bg-black/28 p-3 text-left transition hover:border-cyan-200/22 hover:bg-cyan-300/10'
                                    onClick={() =>
                                      void handleRestoreCanvasVersion(version)
                                    }
                                  >
                                    <div className='flex items-center justify-between gap-3'>
                                      <span className='font-mono text-xs text-cyan-100'>
                                        v{version.revision}
                                      </span>
                                      <span className='text-[11px] text-white/34'>
                                        {formatTime(version.created_time)}
                                      </span>
                                    </div>
                                    <div className='mt-1 text-[11px] text-white/42'>
                                      {Array.isArray(version.snapshot?.nodes)
                                        ? version.snapshot.nodes.length
                                        : 0}{' '}
                                      节点 / {version.module || '无限画布'}
                                    </div>
                                  </button>
                                ))}
                            </div>
                          </div>
                        </div>
                      )}

                      {agentTab === 'chat' && (
                        <div className='grid h-full place-items-center text-center'>
                          <div>
                            <YucoreBrandMark />
                            <h2 className='mt-6 text-4xl font-semibold'>
                              光合像素
                            </h2>
                            <p className='mt-3 font-serif text-lg text-white/50 italic'>
                              像素绽放，灵感流动
                            </p>
                            <p className='mx-auto mt-5 max-w-md text-sm leading-6 text-white/44'>
                              描述你想让 Agent
                              如何操作画布。当前会把指令作为生图任务提交，并把结果回流到素材与画布。
                            </p>
                          </div>
                        </div>
                      )}

                      {agentTab === 'history' && (
                        <div className='grid gap-2'>
                          {agentRuns.slice(0, 8).map((run) => (
                            <button
                              key={run.run_id}
                              type='button'
                              onClick={() => {
                                if (!run.result_task_id) return
                                const task = tasks.find(
                                  (item) => item.task_id === run.result_task_id
                                )
                                if (task) setActiveTask(task)
                              }}
                              className='rounded-2xl border border-white/10 bg-white/[0.035] p-3 text-left transition hover:bg-white/[0.06]'
                            >
                              <div className='mb-2 flex items-center justify-between gap-2'>
                                <span className='rounded-md border border-cyan-200/20 bg-cyan-300/10 px-2 py-1 text-[11px] text-cyan-100'>
                                  {run.status}
                                </span>
                                <span className='font-mono text-[11px] text-white/34'>
                                  {run.run_id.slice(-8)}
                                </span>
                              </div>
                              <div className='line-clamp-2 text-sm text-white/68'>
                                {run.prompt}
                              </div>
                              <div className='mt-2 text-[11px] text-white/34'>
                                {run.summary || 'Agent 工具运行'} /{' '}
                                {formatTime(run.updated_time)}
                              </div>
                            </button>
                          ))}
                          {agentRuns.length === 0 &&
                          visibleTasks.length === 0 ? (
                            <div className='rounded-2xl border border-dashed border-white/12 p-5 text-sm text-white/44'>
                              暂无 Agent 任务历史。
                            </div>
                          ) : (
                            visibleTasks.slice(0, 8).map((task) => (
                              <button
                                key={task.task_id}
                                type='button'
                                onClick={() => setActiveTask(task)}
                                className='rounded-2xl border border-white/10 bg-white/[0.035] p-3 text-left transition hover:bg-white/[0.06]'
                              >
                                <div className='mb-2 flex items-center gap-2'>
                                  <ProviderBadge task={task} />
                                  <TaskStatusBadge task={task} />
                                </div>
                                <div className='line-clamp-2 text-sm text-white/68'>
                                  {task.prompt}
                                </div>
                                <div className='mt-2 text-[11px] text-white/34'>
                                  {formatTime(task.updated_time)}
                                </div>
                              </button>
                            ))
                          )}
                        </div>
                      )}

                      {agentTab === 'logs' && (
                        <div className='rounded-2xl border border-white/10 bg-black/38 p-4 font-mono text-xs leading-6 text-white/54'>
                          <div>
                            [canvas] nodes={nodes.length} edges={edges.length}
                          </div>
                          <div>[sync] {canvasSyncLabel}</div>
                          <div>
                            [mode] agent={agentMode} confirmation=
                            {toolConfirmationEnabled ? 'on' : 'off'}
                          </div>
                          <div>
                            [media] adapter={mediaHealth?.adapter ?? 'loading'}{' '}
                            ready=
                            {mediaHealth?.real_workflow_ready
                              ? 'true'
                              : 'false'}{' '}
                            status=
                            {mediaHealth?.upstream_verification_status ??
                              'loading'}
                          </div>
                          {agentRuns.slice(0, 3).map((run) => (
                            <div
                              key={run.run_id}
                              className='mt-3 border-t border-white/10 pt-3'
                            >
                              <div>
                                [agent] {run.run_id} {run.status}
                              </div>
                              <div>[task] {run.result_task_id || 'none'}</div>
                              {run.actions.slice(-8).map((action) => (
                                <div
                                  key={`${run.run_id}-${String(action.id ?? action.action_id ?? action.task_id ?? action.tool ?? 'action')}-${String(action.status ?? '')}`}
                                >
                                  [tool] {String(action.tool ?? 'unknown')}:{' '}
                                  {String(action.status ?? 'pending')}
                                </div>
                              ))}
                            </div>
                          ))}
                        </div>
                      )}
                    </div>

                    <div className='shrink-0 border-t border-white/10 p-4'>
                      <div className='rounded-2xl border border-white/10 bg-black/38 p-3'>
                        <div className='mb-3 grid gap-2 sm:grid-cols-3'>
                          <label className='grid gap-1 text-[11px] text-white/48'>
                            <span>{t('Billing group')}</span>
                            <select
                              value={selectedGroup}
                              onChange={(event) =>
                                handleMediaGroupChange(event.target.value)
                              }
                              disabled={mediaCatalog.groups.length === 0}
                              className='h-9 min-w-0 rounded-md border border-white/12 bg-black/45 px-2 text-xs text-white outline-none focus:border-cyan-200/35'
                            >
                              {mediaCatalog.groups.map((group) => (
                                <option key={group.id} value={group.id}>
                                  {group.description || group.id} ({group.id})
                                </option>
                              ))}
                            </select>
                          </label>
                          <label className='grid gap-1 text-[11px] text-white/48'>
                            <span>{t('Media type')}</span>
                            <select
                              value={kind}
                              onChange={(event) =>
                                setKind(event.target.value as 'image' | 'video')
                              }
                              className='h-9 min-w-0 rounded-md border border-white/12 bg-black/45 px-2 text-xs text-white outline-none focus:border-cyan-200/35'
                            >
                              <option value='image'>{t('Image')}</option>
                              <option value='video'>{t('Video')}</option>
                            </select>
                          </label>
                          <label className='grid gap-1 text-[11px] text-white/48'>
                            <span>
                              {kind === 'video'
                                ? t('Video model')
                                : t('Image model')}
                            </span>
                            <select
                              value={activeModelId}
                              onChange={(event) => {
                                if (kind === 'video') {
                                  setVideoModelId(event.target.value)
                                } else {
                                  setImageModelId(event.target.value)
                                }
                              }}
                              disabled={availableModels.length === 0}
                              className='h-9 min-w-0 rounded-md border border-white/12 bg-black/45 px-2 text-xs text-white outline-none focus:border-cyan-200/35'
                            >
                              {availableModels.map((model) => (
                                <option key={model.id} value={model.id}>
                                  {model.id}
                                </option>
                              ))}
                            </select>
                          </label>
                        </div>
                        {availableModels.length === 0 && (
                          <p className='mb-3 text-xs text-amber-100/72'>
                            {mediaModelsEmptyMessage}
                          </p>
                        )}
                        <textarea
                          value={agentPrompt}
                          onChange={(event) =>
                            setAgentPrompt(event.target.value)
                          }
                          placeholder='描述你想让 Agent 如何操作画布'
                          className='min-h-24 w-full resize-none bg-transparent text-sm leading-6 text-white outline-none placeholder:text-white/28'
                        />
                        <div className='mt-3 flex items-center justify-between gap-3'>
                          <div className='flex items-center gap-2 text-xs text-white/42'>
                            <LibraryBig className='size-4' />
                            <span>
                              {activeModel
                                ? `${selectedGroup} / ${activeModel.id}`
                                : t('Select a media model before submitting.')}
                            </span>
                          </div>
                          <Button
                            className='size-10 rounded-full bg-white p-0 text-black hover:bg-cyan-50'
                            disabled={!agentPrompt.trim() || isSubmitDisabled}
                            onClick={() => void handleAgentSubmit()}
                            aria-label='发送'
                            title='发送'
                          >
                            {isSubmitting ? (
                              <Loader2 className='size-4 animate-spin' />
                            ) : (
                              <ArrowRight className='size-4' />
                            )}
                          </Button>
                        </div>
                      </div>
                    </div>
                  </aside>
                )}
              </section>
            )}

            {(view === 'image' || view === 'video') && (
              <section className='grid h-full min-h-0 gap-3 overflow-hidden p-3 xl:grid-cols-[18rem_23rem_minmax(0,1fr)]'>
                <aside className='hidden min-h-0 overflow-hidden rounded-2xl border border-white/10 bg-black/40 xl:block'>
                  <div className='flex items-center justify-between border-b border-white/10 p-4'>
                    <h2 className='text-sm font-semibold text-white'>
                      生成记录
                    </h2>
                    <span className='rounded-md border border-white/10 bg-white/[0.035] px-2 py-1 text-xs text-white/52'>
                      {visibleTasks.length}
                    </span>
                  </div>
                  <div className='flex gap-2 border-b border-white/10 p-3'>
                    <Button
                      size='sm'
                      className='h-8 rounded-lg bg-white text-black hover:bg-cyan-50'
                      disabled={isSubmitDisabled}
                      onClick={() =>
                        handleSubmitTask(view === 'video' ? 'video' : 'image')
                      }
                    >
                      <Plus data-icon='inline-start' />
                      新建
                    </Button>
                    <Button
                      size='sm'
                      variant='outline'
                      className='h-8 rounded-lg border-white/10 bg-white/[0.035] text-white hover:bg-white/10'
                      onClick={refreshTasks}
                    >
                      <RefreshCw data-icon='inline-start' />
                      刷新
                    </Button>
                  </div>
                  <div className='grid max-h-[calc(100svh-13rem)] gap-2 overflow-y-auto p-3'>
                    {visibleTasks.length === 0 ? (
                      <div className='grid min-h-44 place-items-center rounded-xl border border-dashed border-white/10 text-center text-xs text-white/38'>
                        暂无生成记录
                      </div>
                    ) : (
                      visibleTasks.map((task) => (
                        <button
                          key={task.task_id}
                          type='button'
                          onClick={() => setActiveTask(task)}
                          className={cn(
                            'rounded-xl border p-3 text-left transition hover:bg-white/[0.06]',
                            activeTask?.task_id === task.task_id
                              ? 'border-cyan-200/24 bg-cyan-300/10'
                              : 'border-white/10 bg-white/[0.035]'
                          )}
                        >
                          <div className='mb-2 flex items-center justify-between gap-2'>
                            <div className='flex items-center gap-1'>
                              <ProviderBadge task={task} />
                              <TaskStatusBadge task={task} />
                            </div>
                            <span className='text-[11px] text-white/34'>
                              {task.progress}%
                            </span>
                          </div>
                          <div className='line-clamp-2 text-xs leading-5 text-white/62'>
                            {task.prompt}
                          </div>
                          <div className='mt-2 text-[11px] text-white/32'>
                            {formatTime(task.created_time)}
                          </div>
                        </button>
                      ))
                    )}
                  </div>
                </aside>

                <section className='min-h-0 overflow-y-auto rounded-2xl border border-white/10 bg-black/42 p-4 backdrop-blur-xl'>
                  <div className='mb-4 flex items-start justify-between gap-3'>
                    <div>
                      <h1 className='text-2xl font-semibold text-white'>
                        {view === 'video' ? '视频创作台' : '生图工作台'}
                      </h1>
                      <p className='mt-1 text-xs text-white/42'>
                        提示词、参考图、模型、尺寸和审核策略集中在一个用户端工作台。
                      </p>
                    </div>
                    <WandSparkles className='size-5 text-cyan-100' />
                  </div>

                  <div className='mb-4 grid grid-cols-2 gap-2 rounded-xl border border-white/10 bg-white/[0.035] p-3 text-xs'>
                    <div>
                      <div className='text-white/34'>可用点数</div>
                      <div className='mt-1 font-mono text-sm text-white'>
                        {billing?.available_points ?? '--'}
                      </div>
                    </div>
                    <div>
                      <div className='text-white/34'>结算模式</div>
                      <div className='mt-1 text-sm text-white'>
                        {billing?.active_mode === 'native_wallet'
                          ? '站内钱包'
                          : '额度同步'}
                      </div>
                    </div>
                    <div>
                      <div className='text-white/34'>媒体引擎</div>
                      <div className='mt-1 text-sm text-white'>
                        {mediaEngineLabel(mediaHealth)}
                      </div>
                    </div>
                    <div>
                      <div className='text-white/34'>资产策略</div>
                      <div className='mt-1 text-sm text-white'>
                        {mediaAssetPolicyLabel(mediaHealth)}
                      </div>
                    </div>
                  </div>
                  {mediaHealth?.message && (
                    <div className='mb-4 rounded-xl border border-amber-200/15 bg-amber-300/[0.055] px-3 py-2 text-xs leading-5 text-amber-50/78'>
                      {mediaHealth.message}
                    </div>
                  )}
                  {mediaHealth?.verification_blockers?.length ? (
                    <div className='mb-4 rounded-xl border border-rose-200/15 bg-rose-400/[0.055] px-3 py-2 text-xs leading-5 text-rose-50/78'>
                      <div className='font-semibold text-rose-50'>
                        真实工作流阻断项
                      </div>
                      <ul className='mt-1 list-inside list-disc space-y-1'>
                        {mediaHealth.verification_blockers.map((blocker) => (
                          <li key={blocker}>{blocker}</li>
                        ))}
                      </ul>
                    </div>
                  ) : null}

                  <div className='grid gap-4'>
                    <div className='grid gap-2'>
                      <div className='flex items-center justify-between gap-2'>
                        <label
                          htmlFor='yucore-prompt'
                          className='text-sm font-semibold text-white'
                        >
                          提示词
                        </label>
                        <div className='flex gap-2'>
                          <Button
                            size='sm'
                            variant='outline'
                            className='h-8 rounded-lg border-white/10 bg-white/[0.035] text-white hover:bg-white/10'
                            onClick={() => setView('prompts')}
                          >
                            提示词库
                          </Button>
                          <Button
                            size='sm'
                            variant='outline'
                            className='h-8 rounded-lg border-white/10 bg-white/[0.035] text-white hover:bg-white/10'
                            onClick={() => setView('assets')}
                          >
                            我的素材
                          </Button>
                        </div>
                      </div>
                      <textarea
                        id='yucore-prompt'
                        value={prompt}
                        onChange={(event) => setPrompt(event.target.value)}
                        placeholder='描述画面主体、风格、构图、光线和用途'
                        className='min-h-32 resize-y rounded-xl border border-white/10 bg-white/[0.035] p-3 text-sm leading-6 text-white outline-none placeholder:text-white/28 focus:border-cyan-200/35'
                      />
                    </div>

                    <div className='grid gap-2'>
                      <label
                        htmlFor='yucore-negative'
                        className='text-sm font-semibold text-white'
                      >
                        负面提示词
                      </label>
                      <textarea
                        id='yucore-negative'
                        value={negativePrompt}
                        onChange={(event) =>
                          setNegativePrompt(event.target.value)
                        }
                        className='min-h-20 resize-y rounded-xl border border-white/10 bg-white/[0.035] p-3 text-sm leading-6 text-white outline-none placeholder:text-white/28 focus:border-cyan-200/35'
                      />
                    </div>

                    {maxReferenceImages > 0 && (
                      <div className='rounded-xl border border-dashed border-white/12 bg-white/[0.025] p-4'>
                        <div className='mb-3 flex items-center justify-between gap-2'>
                          <div>
                            <div className='text-sm font-semibold text-white'>
                              参考图
                            </div>
                            <div className='mt-1 text-[11px] text-white/34'>
                              最多 {maxReferenceImages} 张，当前{' '}
                              {referenceFiles.length} 张
                            </div>
                          </div>
                          <div className='flex gap-2'>
                            {referenceFiles.length > 0 && (
                              <Button
                                size='sm'
                                variant='outline'
                                className='h-8 rounded-lg border-white/10 bg-white/[0.035] text-white hover:bg-white/10'
                                onClick={clearReferenceFiles}
                              >
                                清空
                              </Button>
                            )}
                            <Button
                              size='sm'
                              variant='outline'
                              className='h-8 rounded-lg border-white/10 bg-white/[0.035] text-white hover:bg-white/10'
                              disabled={isReferenceUploading}
                              onClick={() => fileInputRef.current?.click()}
                            >
                              <Upload data-icon='inline-start' />
                              上传
                            </Button>
                          </div>
                        </div>
                        <input
                          ref={fileInputRef}
                          type='file'
                          accept='image/*'
                          multiple
                          className='hidden'
                          onChange={handleReferenceChange}
                        />
                        {referenceFiles.length > 0 ? (
                          <div className='grid grid-cols-3 gap-2'>
                            {referenceFiles.map((file) => (
                              <div
                                key={file.id}
                                className={cn(
                                  'overflow-hidden rounded-xl border bg-black/35',
                                  file.uploadError && 'border-rose-300/35',
                                  !file.uploadError &&
                                    file.isUploading &&
                                    'border-cyan-200/30',
                                  !file.uploadError &&
                                    !file.isUploading &&
                                    'border-white/10'
                                )}
                              >
                                <div className='relative'>
                                  <img
                                    src={file.previewUrl}
                                    alt={file.name}
                                    className='aspect-square w-full object-cover'
                                  />
                                  {file.isUploading && (
                                    <div className='absolute inset-0 grid place-items-center bg-black/62 text-[11px] font-medium text-white'>
                                      <span className='inline-flex items-center gap-1.5'>
                                        <Loader2 className='size-3 animate-spin' />
                                        上传中
                                      </span>
                                    </div>
                                  )}
                                  {file.uploadError && (
                                    <div className='absolute inset-0 grid place-items-center bg-rose-950/62 px-2 text-center text-[11px] font-medium text-rose-100'>
                                      上传失败
                                    </div>
                                  )}
                                </div>
                                <div className='truncate px-2 py-1.5 text-[11px] text-white/45'>
                                  {file.name}
                                </div>
                              </div>
                            ))}
                          </div>
                        ) : (
                          <button
                            type='button'
                            onClick={() => fileInputRef.current?.click()}
                            className='grid h-24 w-full place-items-center rounded-xl border border-dashed border-white/12 text-white/38 hover:border-cyan-200/24 hover:text-white'
                          >
                            <span>
                              <CirclePlus className='mx-auto mb-1 size-6' />
                              添加参考图
                            </span>
                          </button>
                        )}
                      </div>
                    )}

                    <div className='grid gap-3'>
                      <label className='grid gap-2 text-sm font-semibold text-white'>
                        <span>{t('Billing group')}</span>
                        <select
                          value={selectedGroup}
                          onChange={(event) =>
                            handleMediaGroupChange(event.target.value)
                          }
                          disabled={mediaCatalog.groups.length === 0}
                          className='h-10 min-w-0 rounded-md border border-white/12 bg-black/45 px-3 text-sm font-normal text-white outline-none focus:border-cyan-200/35'
                        >
                          {mediaCatalog.groups.map((group) => (
                            <option key={group.id} value={group.id}>
                              {group.description || group.id} ({group.id})
                            </option>
                          ))}
                        </select>
                        {!isMediaCatalogLoading &&
                          mediaCatalog.groups.length === 0 && (
                            <span className='text-xs font-normal text-amber-100/72'>
                              {t('No media groups are available.')}
                            </span>
                          )}
                      </label>

                      <div>
                        <div className='mb-2 text-sm font-semibold text-white'>
                          {view === 'video'
                            ? t('Video model')
                            : t('Image model')}
                        </div>
                        <div className='grid gap-2'>
                          {availableModels.length === 0 ? (
                            <div className='rounded-xl border border-white/10 bg-white/[0.035] p-3 text-sm text-white/58'>
                              {mediaModelsEmptyMessage}
                            </div>
                          ) : (
                            availableModels.map((model) => (
                              <button
                                key={model.id}
                                type='button'
                                onClick={() => {
                                  if (model.kind === 'video') {
                                    setVideoModelId(model.id)
                                    setKind('video')
                                  } else {
                                    setImageModelId(model.id)
                                    setKind('image')
                                  }
                                }}
                                className={cn(
                                  'rounded-xl border p-3 text-left transition',
                                  activeModelId === model.id
                                    ? 'border-cyan-200/25 bg-cyan-300/10'
                                    : 'border-white/10 bg-white/[0.035] hover:bg-white/[0.06]'
                                )}
                              >
                                <div className='flex items-center justify-between gap-2'>
                                  <span className='text-sm font-semibold text-white'>
                                    {model.name}
                                  </span>
                                  <span className='flex shrink-0 items-center gap-1.5'>
                                    {model.pricing?.display && (
                                      <span className='rounded-md border border-cyan-200/18 bg-cyan-300/8 px-2 py-0.5 text-[10px] text-cyan-100/72'>
                                        {model.pricing.display}
                                      </span>
                                    )}
                                    {model.badge && (
                                      <span className='rounded-md border border-white/10 px-2 py-0.5 text-[10px] text-white/45'>
                                        {model.badge}
                                      </span>
                                    )}
                                  </span>
                                </div>
                                {model.description && (
                                  <p className='mt-1 line-clamp-2 text-[11px] leading-5 text-white/42'>
                                    {model.description}
                                  </p>
                                )}
                              </button>
                            ))
                          )}
                        </div>
                      </div>

                      <div>
                        <div className='mb-2 text-sm font-semibold text-white'>
                          模式
                        </div>
                        <div className='flex flex-wrap gap-2'>
                          {activeModes.map((item) => (
                            <SegmentedButton
                              key={item}
                              active={mode === item}
                              onClick={() => setMode(item)}
                            >
                              {modeLabels[item] ?? item}
                            </SegmentedButton>
                          ))}
                        </div>
                      </div>

                      {activeAspectRatios.length > 0 && (
                        <div>
                          <div className='mb-2 text-sm font-semibold text-white'>
                            宽高比
                          </div>
                          <div className='flex flex-wrap gap-2'>
                            {activeAspectRatios.map((item) => (
                              <SegmentedButton
                                key={item}
                                active={aspectRatio === item}
                                onClick={() => setAspectRatio(item)}
                              >
                                {item === 'auto' ? '自动' : item}
                              </SegmentedButton>
                            ))}
                          </div>
                        </div>
                      )}

                      {activeSizes.length > 0 && (
                        <div>
                          <div className='mb-2 text-sm font-semibold text-white'>
                            {activeModel?.size_label ?? '尺寸'}
                          </div>
                          <div className='flex flex-wrap gap-2'>
                            {activeSizes.map((item) => (
                              <SegmentedButton
                                key={item}
                                active={size === item}
                                onClick={() => setSize(item)}
                              >
                                {item === 'custom' ? '自定义' : item}
                              </SegmentedButton>
                            ))}
                          </div>
                        </div>
                      )}

                      {view === 'image' && activeFormats.length > 0 && (
                        <div>
                          <div className='mb-2 text-sm font-semibold text-white'>
                            格式
                          </div>
                          <div className='flex flex-wrap gap-2'>
                            {activeFormats.map((item) => (
                              <SegmentedButton
                                key={item}
                                active={format === item}
                                onClick={() => setFormat(item)}
                              >
                                {outputFormatLabels[item] ?? item.toUpperCase()}
                              </SegmentedButton>
                            ))}
                          </div>
                        </div>
                      )}

                      {activeQualities.length > 0 && (
                        <div>
                          <div className='mb-2 text-sm font-semibold text-white'>
                            质量
                          </div>
                          <div className='flex flex-wrap gap-2'>
                            {activeQualities.map((item) => (
                              <SegmentedButton
                                key={item}
                                active={quality === item}
                                onClick={() => setQuality(item)}
                              >
                                {item === 'auto' ? '自动' : item}
                              </SegmentedButton>
                            ))}
                          </div>
                        </div>
                      )}

                      {activeStylePresets.length > 0 && (
                        <div>
                          <div className='mb-2 text-sm font-semibold text-white'>
                            风格预设
                          </div>
                          <div className='flex flex-wrap gap-2'>
                            {activeStylePresets.map((item) => (
                              <SegmentedButton
                                key={item}
                                active={stylePreset === item}
                                onClick={() => setStylePreset(item)}
                              >
                                {styleLabels[item] ?? item}
                              </SegmentedButton>
                            ))}
                          </div>
                        </div>
                      )}

                      {view === 'image' && activeBackgrounds.length > 0 && (
                        <div>
                          <div className='mb-2 text-sm font-semibold text-white'>
                            背景
                          </div>
                          <div className='flex flex-wrap gap-2'>
                            {activeBackgrounds.map((item) => (
                              <SegmentedButton
                                key={item}
                                active={background === item}
                                onClick={() => setBackground(item)}
                              >
                                {backgroundLabels[item] ?? item}
                              </SegmentedButton>
                            ))}
                          </div>
                        </div>
                      )}

                      {view === 'image' && activeModerations.length > 0 && (
                        <div>
                          <div className='mb-2 text-sm font-semibold text-white'>
                            内容审核
                          </div>
                          <div className='flex flex-wrap gap-2'>
                            {activeModerations.map((item) => (
                              <SegmentedButton
                                key={item}
                                active={moderation === item}
                                onClick={() => setModeration(item)}
                              >
                                {moderationLabels[item] ?? item}
                              </SegmentedButton>
                            ))}
                          </div>
                        </div>
                      )}

                      {activeStreamModes.length > 0 && (
                        <div>
                          <div className='mb-2 text-sm font-semibold text-white'>
                            执行方式
                          </div>
                          <div className='flex flex-wrap gap-2'>
                            {activeStreamModes.map((item) => (
                              <SegmentedButton
                                key={item}
                                active={streamMode === item}
                                onClick={() => setStreamMode(item)}
                              >
                                {streamModeLabels[item] ?? item}
                              </SegmentedButton>
                            ))}
                          </div>
                        </div>
                      )}

                      {view === 'image' &&
                        streamMode === 'partial' &&
                        Boolean(activeModel?.partial_images?.length) && (
                          <div>
                            <div className='mb-2 text-sm font-semibold text-white'>
                              预览阶段
                            </div>
                            <div className='flex flex-wrap gap-2'>
                              {activeModel?.partial_images?.map((item) => (
                                <SegmentedButton
                                  key={item}
                                  active={partialImages === item}
                                  onClick={() => setPartialImages(item)}
                                >
                                  {item === 0 ? '关闭' : `${item} 段`}
                                </SegmentedButton>
                              ))}
                            </div>
                          </div>
                        )}

                      {view === 'video' && activeDurations.length > 0 && (
                        <div>
                          <div className='mb-2 text-sm font-semibold text-white'>
                            时长
                          </div>
                          <div className='flex flex-wrap gap-2'>
                            {activeDurations.map((item) => (
                              <SegmentedButton
                                key={item}
                                active={duration === item}
                                onClick={() => setDuration(item)}
                              >
                                {item} 秒
                              </SegmentedButton>
                            ))}
                          </div>
                        </div>
                      )}

                      {view === 'video' && activeModel?.supports_audio && (
                        <div className='flex items-center justify-between gap-4 border-t border-white/10 pt-3'>
                          <div className='min-w-0'>
                            <label
                              htmlFor='studio-generate-audio'
                              className='text-sm font-semibold text-white'
                            >
                              {t('Generate native audio')}
                            </label>
                            <p
                              id='studio-generate-audio-description'
                              className='mt-1 text-xs leading-5 text-white/45'
                            >
                              {t(
                                'Include model-generated audio in the video.'
                              )}
                            </p>
                          </div>
                          <Switch
                            id='studio-generate-audio'
                            aria-describedby='studio-generate-audio-description'
                            className='shrink-0 data-checked:bg-cyan-300'
                            checked={generateAudio}
                            onCheckedChange={setGenerateAudio}
                          />
                        </div>
                      )}

                      <div>
                        <div className='mb-2 text-sm font-semibold text-white'>
                          可见性
                        </div>
                        <div className='flex flex-wrap gap-2'>
                          {Object.keys(visibilityLabels).map((item) => (
                            <SegmentedButton
                              key={item}
                              active={visibility === item}
                              onClick={() => setVisibility(item)}
                            >
                              {visibilityLabels[item]}
                            </SegmentedButton>
                          ))}
                        </div>
                      </div>

                      {view === 'image' && activeCounts.length > 0 && (
                        <div>
                          <div className='mb-2 text-sm font-semibold text-white'>
                            生成张数
                          </div>
                          <div className='flex items-center gap-2'>
                            {activeCounts.map((item) => (
                              <SegmentedButton
                                key={item}
                                active={count === item}
                                onClick={() => setCount(item)}
                              >
                                {item}
                              </SegmentedButton>
                            ))}
                          </div>
                        </div>
                      )}
                    </div>

                    <Button
                      className='h-11 rounded-xl bg-white text-black hover:bg-cyan-50'
                      disabled={isSubmitDisabled}
                      onClick={() =>
                        handleSubmitTask(view === 'video' ? 'video' : 'image')
                      }
                    >
                      {isSubmitting ? (
                        <Loader2
                          data-icon='inline-start'
                          className='animate-spin'
                        />
                      ) : (
                        <Play data-icon='inline-start' />
                      )}
                      开始生成
                    </Button>
                  </div>
                </section>

                <section className='min-h-0 overflow-y-auto rounded-2xl border border-white/10 bg-black/42 p-4 backdrop-blur-xl'>
                  <div className='mb-4 flex items-center justify-between gap-3'>
                    <h2 className='flex items-center gap-2 text-lg font-semibold text-white'>
                      <ImagePlus className='size-5 text-cyan-100' />
                      生成结果
                    </h2>
                    {activeTask && (
                      <div className='flex items-center gap-1'>
                        <ProviderBadge task={activeTask} />
                        <TaskStatusBadge task={activeTask} />
                      </div>
                    )}
                  </div>

                  {!activeTask && (
                    <div className='grid min-h-[34rem] place-items-center rounded-2xl border border-dashed border-white/12 bg-white/[0.025] text-center'>
                      <div>
                        <ImagePlus className='mx-auto mb-4 size-12 text-white/30' />
                        <div className='text-sm font-semibold text-white'>
                          还没有生成图片
                        </div>
                        <div className='mt-1 text-xs text-white/42'>
                          提交任务后会在这里显示进度和结果。
                        </div>
                      </div>
                    </div>
                  )}
                  {activeTask && isActiveMediaTask(activeTask) && (
                    <div className='grid min-h-[34rem] place-items-center rounded-2xl border border-cyan-200/14 bg-cyan-300/[0.045] text-center'>
                      <div className='w-full max-w-sm'>
                        <Loader2 className='mx-auto mb-4 size-10 animate-spin text-cyan-100' />
                        <div className='text-sm font-semibold text-white'>
                          {activeTask.status === 'pending'
                            ? '任务排队中'
                            : '任务处理中'}
                        </div>
                        <div className='mt-2 h-2 overflow-hidden rounded-full bg-white/10'>
                          <span
                            className='block h-full rounded-full bg-linear-to-r from-cyan-200 via-amber-100 to-rose-200'
                            style={{ width: `${activeTask.progress}%` }}
                          />
                        </div>
                        <div className='mt-2 text-xs text-white/42'>
                          {activeTask.task_id}
                        </div>
                        <Button
                          variant='outline'
                          className='mt-5 h-9 rounded-xl border-white/15 bg-white/[0.035] text-white hover:bg-white/10'
                          onClick={() => handleCancelTask(activeTask)}
                        >
                          取消任务
                        </Button>
                      </div>
                    </div>
                  )}
                  {activeTask && !isActiveMediaTask(activeTask) && (
                    <AssetGrid
                      tasks={[activeTask]}
                      onSendToCanvas={addAssetToCanvas}
                    />
                  )}

                  {visibleTasks.length > 0 && (
                    <div className='mt-5'>
                      <h3 className='mb-3 text-sm font-semibold text-white'>
                        本次会话任务
                      </h3>
                      <div className='grid gap-2'>
                        {visibleTasks.slice(0, 4).map((task) => (
                          <button
                            key={task.task_id}
                            type='button'
                            onClick={() => setActiveTask(task)}
                            className='grid grid-cols-[auto_minmax(0,1fr)_auto] items-center gap-3 rounded-xl border border-white/10 bg-white/[0.035] p-3 text-left hover:bg-white/[0.06]'
                          >
                            <span className='flex items-center gap-1'>
                              <ProviderBadge task={task} />
                              <TaskStatusBadge task={task} />
                            </span>
                            <span className='min-w-0'>
                              <span className='block truncate text-sm font-medium text-white'>
                                {mediaTaskRouteLabel(task)}
                              </span>
                              <span className='mt-1 block truncate text-xs text-white/38'>
                                {task.prompt}
                              </span>
                            </span>
                            <span className='font-mono text-xs text-white/42'>
                              {task.cost} 点
                            </span>
                          </button>
                        ))}
                      </div>
                    </div>
                  )}
                </section>
              </section>
            )}

            {view === 'prompts' && (
              <section className='h-full overflow-y-auto p-5 lg:p-8'>
                <div className='mb-6 flex flex-wrap items-end justify-between gap-4'>
                  <div>
                    <p className='text-xs tracking-[0.2em] text-white/35'>
                      提示词库
                    </p>
                    <h1 className='mt-2 text-3xl font-semibold text-white'>
                      提示词库
                    </h1>
                    <p className='mt-2 max-w-2xl text-sm text-white/48'>
                      把可复用的提示词、参数和风格变成下一次生成的起点。
                    </p>
                  </div>
                  <Button
                    className='h-10 rounded-xl bg-white text-black hover:bg-cyan-50'
                    onClick={() => setView('image')}
                  >
                    <WandSparkles data-icon='inline-start' />
                    回到工作台
                  </Button>
                </div>
                <div className='grid gap-4 md:grid-cols-2 xl:grid-cols-3'>
                  {templates.map((template) => (
                    <article
                      key={template.id}
                      className='overflow-hidden rounded-2xl border border-white/10 bg-white/[0.035]'
                    >
                      {template.preview_image_url && (
                        <img
                          src={template.preview_image_url}
                          alt={template.title}
                          loading='lazy'
                          decoding='async'
                          className='aspect-[4/3] w-full bg-black/35 object-cover'
                        />
                      )}
                      <div className='p-4'>
                        <div className='mb-3 flex items-center justify-between gap-2'>
                          <span className='rounded-lg border border-white/10 bg-black/35 px-2 py-1 text-[11px] text-white/48'>
                            {template.kind === 'video' ? '视频' : '图片'} /{' '}
                            {modeLabels[template.mode ?? ''] ?? template.style}
                          </span>
                          <Button
                            size='sm'
                            variant='outline'
                            className='h-8 rounded-lg border-white/10 bg-white/[0.035] text-white hover:bg-white/10'
                            onClick={() => applyTemplate(template)}
                          >
                            使用
                          </Button>
                        </div>
                        <h2 className='text-lg font-semibold text-white'>
                          {template.title}
                        </h2>
                        <div className='mt-1 text-xs text-cyan-100/52'>
                          {template.tag ?? template.style}
                          {template.model_id ? ` · ${template.model_id}` : ''}
                        </div>
                        <p className='mt-3 line-clamp-6 text-sm leading-6 text-white/52'>
                          {template.prompt}
                        </p>
                      </div>
                    </article>
                  ))}
                </div>
              </section>
            )}

            {view === 'assets' && (
              <section className='h-full overflow-y-auto p-5 lg:p-8'>
                <div className='mb-6 flex flex-wrap items-end justify-between gap-4'>
                  <div>
                    <p className='text-xs tracking-[0.2em] text-white/35'>
                      素材库
                    </p>
                    <h1 className='mt-2 text-3xl font-semibold text-white'>
                      我的素材
                    </h1>
                    <p className='mt-2 max-w-2xl text-sm text-white/48'>
                      生成结果、参考资产和画布实验会在同一用户端沉淀。
                    </p>
                  </div>
                  <div className='flex gap-2'>
                    <Button
                      variant='outline'
                      className='h-10 rounded-xl border-white/15 bg-white/[0.035] text-white hover:bg-white/10'
                      onClick={refreshGallery}
                    >
                      <RefreshCw data-icon='inline-start' />
                      刷新
                    </Button>
                    <Button
                      className='h-10 rounded-xl bg-white text-black hover:bg-cyan-50'
                      onClick={() => setView('image')}
                    >
                      新建生成
                    </Button>
                  </div>
                </div>
                <AssetGrid
                  tasks={completedTasks}
                  onSendToCanvas={addAssetToCanvas}
                />
              </section>
            )}
          </main>
        </div>

        {view !== 'canvas' && (
          <footer className='flex h-9 shrink-0 items-center justify-between border-t border-white/10 bg-black/56 px-4 text-[11px] text-white/38'>
            <span>YuCore Pixel / session {sessionId.slice(-8)}</span>
            <span className='hidden items-center gap-3 sm:flex'>
              <span>画布已同步</span>
              <span>媒体队列已接入</span>
              <span>网关底座</span>
            </span>
            <span className='flex items-center gap-2'>
              <Button
                variant='outline'
                className='h-7 rounded-lg border-white/10 bg-white/[0.035] px-2 text-[11px] text-white hover:bg-white/10'
                render={<Link to='/pricing' />}
              >
                <Boxes data-icon='inline-start' />
                模型广场
              </Button>
              <Button
                variant='outline'
                className='h-7 rounded-lg border-white/10 bg-white/[0.035] px-2 text-[11px] text-white hover:bg-white/10'
                render={<Link to='/playground' />}
              >
                <Braces data-icon='inline-start' />
                API
              </Button>
            </span>
          </footer>
        )}
      </div>
    </YucorePageShell>
  )
}
