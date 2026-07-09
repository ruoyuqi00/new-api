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
import {
  Activity,
  CreditCard,
  FileText,
  Gauge,
  KeyRound,
  ListChecks,
  RadioTower,
  Route,
  ShieldCheck,
  Sparkles,
  TerminalSquare,
  WandSparkles,
  type LucideIcon,
} from 'lucide-react'

import type { TopNavLink } from '@/components/layout/types'

export const YUCORE_BRAND_NAME = 'YuCore'
export const YUCORE_STUDIO_NAME = 'YuCore Studio'

export const yucoreBrandPrompt =
  'Operate model gateway, media studio, billing, quota visibility, and production routing from one YuCore account foundation.'

type YucoreSignal = {
  label: string
  value: string
  icon: LucideIcon
}

type YucoreMetric = {
  label: string
  value: string
}

type YucoreCapability = {
  title: string
  description: string
  signal: string
  icon: LucideIcon
}

export type YucoreStudioAccent = 'cyan' | 'violet' | 'emerald' | 'amber'

type YucoreStudioModule = {
  title: string
  description: string
  accent: YucoreStudioAccent
  icon: LucideIcon
}

export const yucorePublicNavLinks: TopNavLink[] = [
  { title: 'Home', href: '/' },
  { title: 'Model Square', href: '/pricing' },
  { title: 'Rankings', href: '/rankings' },
  { title: 'About', href: '/about' },
]

export const yucoreHeroSignals: YucoreSignal[] = [
  { label: 'Gateway', value: 'Route active', icon: Route },
  { label: 'Studio', value: 'Assets synced', icon: Sparkles },
  { label: 'Quota', value: 'Ledger ready', icon: CreditCard },
]

export const yucoreModelHubSignals: YucoreSignal[] = [
  { label: 'Route mesh', value: '40+ providers', icon: RadioTower },
  { label: 'Pricing', value: 'Quota-aware', icon: Gauge },
  { label: 'Access', value: 'Key scoped', icon: KeyRound },
]

export const yucoreMetrics: YucoreMetric[] = [
  { value: '1', label: 'Account core' },
  { value: 'API', label: 'OpenAI-compatible' },
  { value: '24h', label: 'Usage audit' },
  { value: 'Studio', label: 'Creative workflows' },
]

export const yucoreCapabilities: YucoreCapability[] = [
  {
    title: 'Gateway routing',
    description:
      'Route OpenAI-compatible requests across text, code, image, video, and provider pools from one gateway.',
    signal: 'Route mesh',
    icon: Route,
  },
  {
    title: 'Billing visibility',
    description:
      'Keep wallet, quota, subscriptions, model pricing, cache savings, and request logs in one operational view.',
    signal: 'Quota linked',
    icon: CreditCard,
  },
  {
    title: 'Access control',
    description:
      'Create API keys, model scopes, request limits, and user groups without splitting the account foundation.',
    signal: 'Keys scoped',
    icon: ShieldCheck,
  },
  {
    title: 'Studio workflows',
    description:
      'Bridge image, video, canvas, prompt, and asset workflows without leaving the gateway foundation.',
    signal: 'Media ready',
    icon: WandSparkles,
  },
]

export const yucoreStudioModules: YucoreStudioModule[] = [
  {
    title: 'Image Render',
    description: 'Generate, compare, and reuse image outputs with quota trace.',
    accent: 'cyan',
    icon: Sparkles,
  },
  {
    title: 'Video Render',
    description:
      'Prepare motion, duration, and model settings for media tasks.',
    accent: 'violet',
    icon: Activity,
  },
  {
    title: 'Prompt Library',
    description:
      'Store reusable prompts, negative prompts, and reference notes.',
    accent: 'emerald',
    icon: FileText,
  },
  {
    title: 'Canvas Workflow',
    description:
      'Arrange assets, task states, and review notes on one surface.',
    accent: 'amber',
    icon: ListChecks,
  },
]

export const yucoreDashboardActions = [
  { title: 'Create API Key', href: '/keys', icon: KeyRound },
  { title: 'Open Studio', href: '/playground/studio', icon: WandSparkles },
  { title: 'Usage Logs', href: '/usage-logs/common', icon: ListChecks },
] satisfies Array<{
  title: string
  href: string
  icon: LucideIcon
}>

export const yucoreTerminalLines = [
  "curl -X POST '/v1/responses' \\",
  "  -H 'Authorization: Bearer sk-yucore' \\",
  "  -d '{",
  '    "model": "gpt-5.5-Fast",',
  '    "input": "Ship the next workflow"',
  "  }'",
  'response: 200 ok | stream:sse | usage tracked',
]

export const yucoreBrandSignals = [
  { label: 'Gateway', value: 'online', icon: TerminalSquare },
  { label: 'Models', value: 'synced', icon: RadioTower },
  { label: 'Quota', value: 'linked', icon: CreditCard },
] satisfies YucoreSignal[]
