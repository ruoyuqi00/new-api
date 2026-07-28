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
import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import {
  Activity,
  ArrowRight,
  BookOpen,
  Check,
  ChevronDown,
  ChevronUp,
  Circle,
  Copy,
  CreditCard,
  FileText,
  Gauge,
  KeyRound,
  ListChecks,
  RadioTower,
  ShieldCheck,
  Sparkles,
  TerminalSquare,
  Timer,
  WandSparkles,
  type LucideIcon,
} from 'lucide-react'
import { motion, useReducedMotion } from 'motion/react'
import { lazy, Suspense, useMemo, useState } from 'react'
import { toast } from 'sonner'

import {
  CardStaggerContainer,
  CardStaggerItem,
} from '@/components/page-transition'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { fetchTokenKey, getApiKeys } from '@/features/keys/api'
import type { ApiKey } from '@/features/keys/types'
import {
  YUCORE_BRAND_NAME,
  YucoreBrandMark,
  YucoreOpsPulse,
  YucoreStudioEntry,
} from '@/features/yucore-brand'
import { useYucoreTranslation } from '@/features/yucore-brand/i18n/use-yucore-translation'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'
import { getUserModels } from '@/lib/api'
import { MOTION_TRANSITION } from '@/lib/motion'
import { ROLE } from '@/lib/roles'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'

import {
  useApiInfo,
  useDashboardContentVisibility,
} from '../../hooks/use-status-data'
import {
  getOverviewPanelPlan,
  type OverviewPanelPlan,
} from './overview-panel-plan'

const LazyOverviewSecondaryPanels = lazy(() =>
  import('./overview-secondary-panels').then((module) => ({
    default: module.OverviewSecondaryPanels,
  }))
)

const SETUP_GUIDE_VISIBILITY_STORAGE_KEY =
  'dashboard_overview_setup_guide_expanded'

const SETUP_GUIDE_CODE_PATTERN = [
  'const request = await client.responses.create({',
  "  model: 'gpt-4.1-mini',",
  "  input: 'Start routing traffic',",
  '})',
  '',
  'if (request.output_text) {',
  '  console.log(request.output_text)',
  '}',
].join('\n')

type DashboardActionPath =
  | '/keys'
  | '/wallet'
  | '/playground'
  | '/channels'
  | '/usage-logs'
  | '/pricing'
  | '/playground/studio'

interface StartStep {
  title: string
  description: string
  to: DashboardActionPath
  icon: LucideIcon
  completed: boolean
}

interface QuickAction {
  title: string
  description: string
  to: DashboardActionPath
  icon: LucideIcon
  adminOnly?: boolean
}

interface RequestExample {
  endpoint: string
  model: string
  keyName: string
  keyId?: number
  displayKey: string
  ready: boolean
}

interface HeroSignal {
  label: string
  value: string
  icon: LucideIcon
}

function OverviewSecondaryPanelsFallback(props: { plan: OverviewPanelPlan }) {
  const showContentPanels = props.plan.left.length > 0 || props.plan.uptime

  return (
    <div className='space-y-4' aria-hidden='true'>
      <Skeleton className='h-64 w-full rounded-lg' />
      {showContentPanels && <Skeleton className='h-72 w-full rounded-lg' />}
    </div>
  )
}

function getSavedSetupGuideExpanded(): boolean | null {
  if (typeof window === 'undefined') return null
  const saved = window.localStorage.getItem(SETUP_GUIDE_VISIBILITY_STORAGE_KEY)
  if (saved === 'expanded') return true
  if (saved === 'collapsed') return false
  return null
}

function saveSetupGuideExpanded(expanded: boolean): void {
  if (typeof window === 'undefined') return
  window.localStorage.setItem(
    SETUP_GUIDE_VISIBILITY_STORAGE_KEY,
    expanded ? 'expanded' : 'collapsed'
  )
}

function getCurrentOrigin(): string {
  if (typeof window === 'undefined') return ''
  return window.location.origin
}

function normalizeEndpoint(sourceUrl?: string): string {
  const fallback = `${getCurrentOrigin()}/v1/chat/completions`
  const trimmed = sourceUrl?.trim()
  if (!trimmed) return fallback

  const withoutTrailingSlash = trimmed.replace(/\/+$/, '')
  if (withoutTrailingSlash.endsWith('/v1/chat/completions')) {
    return withoutTrailingSlash
  }
  if (withoutTrailingSlash.endsWith('/v1')) {
    return `${withoutTrailingSlash}/chat/completions`
  }
  return `${withoutTrailingSlash}/v1/chat/completions`
}

function getPreferredKey(keys: ApiKey[]): ApiKey | null {
  return keys.find((item) => item.status === 1) ?? keys[0] ?? null
}

function formatDisplayKey(key?: string): string {
  if (!key) return 'sk-...'
  if (key.length <= 14) return key
  return `${key.slice(0, 7)}...${key.slice(-4)}`
}

function buildCurlCommand(args: {
  endpoint: string
  apiKey: string
  model: string
}): string {
  return [
    `curl ${args.endpoint} \\`,
    '  -H "Content-Type: application/json" \\',
    `  -H "Authorization: Bearer ${args.apiKey}" \\`,
    `  -d '{"model":"${args.model}","messages":[{"role":"user","content":"Say hello in one sentence."}]}'`,
  ].join('\n')
}

function SetupGuideBackdrop(props: { compact?: boolean }) {
  return (
    <>
      <div
        className={cn(
          'yucore-dashboard-setup-backdrop pointer-events-none absolute inset-0 bg-[radial-gradient(ellipse_58%_128%_at_78%_0%,rgba(34,211,238,0.16)_0%,transparent_62%),radial-gradient(ellipse_42%_90%_at_12%_92%,rgba(250,204,21,0.1)_0%,transparent_58%),linear-gradient(112deg,rgba(4,7,14,0.96)_0%,rgba(8,12,22,0.92)_48%,rgba(5,7,12,0.98)_100%)]',
          props.compact
            ? '[mask-image:linear-gradient(90deg,black_0%,black_56%,transparent_82%)] opacity-90'
            : 'opacity-100'
        )}
        aria-hidden='true'
      />
      <div
        className='yucore-grid pointer-events-none absolute inset-0 opacity-25'
        aria-hidden='true'
      />
      <div
        className={cn(
          'pointer-events-none absolute inset-y-0 right-0 hidden overflow-hidden font-mono text-white/[0.06] sm:block',
          props.compact ? 'w-1/2 opacity-45' : 'w-[58%] opacity-75'
        )}
        aria-hidden='true'
      >
        <pre
          className={cn(
            'absolute right-3 [mask-image:linear-gradient(90deg,transparent_0%,black_30%,black_82%,transparent_100%)] text-right tracking-[0.38em] whitespace-pre',
            props.compact
              ? '-top-6 text-[9px] leading-4'
              : 'top-1 text-[11px] leading-5'
          )}
        >
          {SETUP_GUIDE_CODE_PATTERN}
        </pre>
      </div>
      <div
        className='yucore-dashboard-setup-vignette pointer-events-none absolute inset-0 bg-linear-to-b from-black/5 via-transparent to-black/65'
        aria-hidden='true'
      />
    </>
  )
}

function DashboardSignalItem(props: { signal: HeroSignal }) {
  const Icon = props.signal.icon

  return (
    <div className='rounded-lg border border-white/10 bg-black/35 px-3 py-2.5 backdrop-blur'>
      <div className='flex min-w-0 items-center gap-2 text-[11px] font-medium tracking-[0.06em] text-white/46 uppercase'>
        <Icon className='size-3.5 text-cyan-100' aria-hidden='true' />
        <span className='truncate'>{props.signal.label}</span>
      </div>
      <div className='mt-1 truncate text-sm font-semibold text-white/86'>
        {props.signal.value}
      </div>
    </div>
  )
}

function ReadinessMeter(props: { completed: number; total: number }) {
  const { t } = useYucoreTranslation()
  const progress = props.total > 0 ? (props.completed / props.total) * 100 : 0

  return (
    <div className='rounded-lg border border-white/10 bg-black/35 p-3 backdrop-blur'>
      <div className='mb-2 flex items-center justify-between gap-3 text-xs'>
        <span className='flex items-center gap-2 font-medium text-white/64'>
          <Gauge className='size-3.5 text-amber-100' aria-hidden='true' />
          {t('Setup integrity')}
        </span>
        <span className='font-mono text-white/78'>
          {props.completed}/{props.total}
        </span>
      </div>
      <div className='h-1.5 overflow-hidden rounded-full bg-white/10'>
        <span
          className='block h-full rounded-full bg-linear-to-r from-cyan-200 via-amber-100 to-emerald-200'
          style={{ width: `${progress}%` }}
        />
      </div>
    </div>
  )
}

function StartStepItem(props: {
  step: StartStep
  index: number
  isLast: boolean
}) {
  const Icon = props.step.icon
  const StatusIcon = props.step.completed ? Check : Circle

  return (
    <li className='relative flex gap-3 pb-2.5 last:pb-0'>
      {!props.isLast && (
        <span
          className='absolute top-9 bottom-0 left-4 w-px bg-white/10'
          aria-hidden='true'
        />
      )}
      <span
        className={cn(
          'relative z-10 flex size-8 shrink-0 items-center justify-center rounded-lg border border-white/10 bg-black/50 shadow-xs',
          props.step.completed && 'border-emerald-200/30 bg-emerald-300/10'
        )}
      >
        <StatusIcon
          className={
            props.step.completed
              ? 'size-4 text-emerald-200'
              : 'size-4 text-white/45'
          }
          aria-hidden='true'
        />
      </span>

      <Link
        to={props.step.to}
        className='focus-visible:ring-ring group flex min-w-0 flex-1 items-center justify-between gap-3 rounded-lg border border-white/10 bg-black/30 px-3 py-2.5 text-left text-white shadow-xs transition-colors outline-none hover:border-cyan-200/25 hover:bg-cyan-300/10 focus-visible:ring-2'
      >
        <span className='flex min-w-0 items-start gap-2.5'>
          <span className='mt-0.5 flex size-7 shrink-0 items-center justify-center rounded-lg border border-cyan-200/15 bg-cyan-200/10'>
            <Icon className='size-3.5' aria-hidden='true' />
          </span>
          <span className='flex min-w-0 flex-col gap-0.5'>
            <span className='flex items-center gap-2 text-sm font-medium'>
              <span className='font-mono text-xs text-white/42 tabular-nums'>
                {props.index + 1}.
              </span>
              <span className='truncate'>{props.step.title}</span>
            </span>
            <span className='line-clamp-1 text-xs text-white/46'>
              {props.step.description}
            </span>
          </span>
        </span>
        <ArrowRight
          className='size-4 shrink-0 text-white/38 transition group-hover:translate-x-0.5 group-hover:text-cyan-100'
          aria-hidden='true'
        />
      </Link>
    </li>
  )
}

function RequestPreview(props: {
  example: RequestExample
  signals: HeroSignal[]
}) {
  const { t } = useYucoreTranslation()
  const shouldReduceMotion = useReducedMotion()
  const [isCopying, setIsCopying] = useState(false)
  const { copyToClipboard } = useCopyToClipboard({ notify: false })
  const previewCurl = buildCurlCommand({
    endpoint: props.example.endpoint,
    apiKey: props.example.displayKey,
    model: props.example.model,
  })
  const previewLines = previewCurl.split('\n')
  const handleCopyRequest = async () => {
    if (!props.example.keyId || isCopying) return

    setIsCopying(true)
    try {
      const result = await fetchTokenKey(props.example.keyId)
      const key = result.success && result.data?.key ? result.data.key : ''
      if (!key) {
        toast.error(result.message || t('Failed to copy to clipboard'))
        return
      }

      const realCurl = buildCurlCommand({
        endpoint: props.example.endpoint,
        apiKey: `sk-${key}`,
        model: props.example.model,
      })
      const copied = await copyToClipboard(realCurl)
      if (copied) {
        toast.success(t('Copied to clipboard'))
      } else {
        toast.error(t('Failed to copy to clipboard'))
      }
    } finally {
      setIsCopying(false)
    }
  }

  return (
    <motion.div
      initial={shouldReduceMotion ? false : { opacity: 0, y: 10, scale: 0.98 }}
      animate={shouldReduceMotion ? undefined : { opacity: 1, y: 0, scale: 1 }}
      transition={MOTION_TRANSITION.slow}
      className='relative overflow-hidden rounded-lg border border-white/10 bg-black/45 p-3 shadow-sm backdrop-blur'
    >
      {!shouldReduceMotion && (
        <motion.div
          className='pointer-events-none absolute inset-x-0 top-0 h-px bg-linear-to-r from-transparent via-cyan-100/45 to-transparent'
          animate={{ x: ['-100%', '100%'] }}
          transition={{ duration: 3.2, repeat: Infinity, ease: 'easeInOut' }}
          aria-hidden='true'
        />
      )}

      <div className='flex items-center justify-between gap-3 border-b border-white/10 pb-3'>
        <div className='flex min-w-0 items-center gap-2'>
          <span className='flex size-8 shrink-0 items-center justify-center rounded-lg border border-cyan-200/15 bg-cyan-200/10 text-cyan-100'>
            <TerminalSquare className='size-4' aria-hidden='true' />
          </span>
          <div className='min-w-0'>
            <div className='truncate text-sm font-medium text-white'>
              {t('First API request')}
            </div>
            <div className='truncate text-xs text-white/45'>
              {props.example.ready
                ? props.example.keyName
                : t('Create an API key to unlock the real request')}
            </div>
          </div>
        </div>
        {props.example.ready ? (
          <Button
            variant='outline'
            size='sm'
            className='h-7 gap-1.5 rounded-lg border-white/15 bg-white/[0.035] px-2 text-xs text-white hover:bg-white/10'
            disabled={isCopying}
            onClick={handleCopyRequest}
            aria-label={t('Copy ready-to-run curl')}
          >
            <Copy data-icon='inline-start' />
            {isCopying ? t('Loading') : t('Copy')}
          </Button>
        ) : (
          <Button
            size='sm'
            variant='outline'
            className='border-white/15 bg-white/[0.035] text-white hover:bg-white/10'
            render={<Link to='/keys' />}
          >
            {t('Create API Key')}
          </Button>
        )}
      </div>

      <div className='my-3 rounded-xl border border-white/10 bg-black/50 p-3 font-mono text-xs'>
        <div className='mb-2 flex items-center gap-1.5'>
          <span className='bg-destructive size-2 rounded-full' />
          <span className='bg-warning size-2 rounded-full' />
          <span className='bg-success size-2 rounded-full' />
        </div>
        <div className='flex flex-col gap-1 overflow-hidden'>
          {previewLines.map((line) => (
            <code
              key={line || 'request-spacer'}
              className='truncate text-cyan-50/58'
              title={line}
            >
              {line}
            </code>
          ))}
        </div>
      </div>

      <div className='grid gap-2'>
        {props.signals.map((signal) => {
          const Icon = signal.icon

          return (
            <div
              key={signal.label}
              className='flex items-center justify-between gap-3 rounded-xl border border-white/10 bg-white/[0.035] px-3 py-2'
            >
              <span className='flex min-w-0 items-center gap-2'>
                <Icon
                  className='size-3.5 shrink-0 text-cyan-100'
                  aria-hidden='true'
                />
                <span className='truncate text-xs font-medium'>
                  {signal.label}
                </span>
              </span>
              <span className='shrink-0 text-xs text-white/45'>
                {signal.value}
              </span>
            </div>
          )
        })}
      </div>
    </motion.div>
  )
}

function QuickActionItem(props: { action: QuickAction }) {
  const Icon = props.action.icon

  return (
    <Button
      variant='outline'
      className='group h-auto justify-start rounded-lg border-white/10 bg-black/30 px-3 py-3 text-left text-white hover:border-cyan-200/25 hover:bg-cyan-300/10'
      render={<Link to={props.action.to} />}
    >
      <span className='flex size-9 shrink-0 items-center justify-center rounded-xl border border-cyan-200/15 bg-cyan-200/10 text-cyan-100'>
        <Icon className='size-4' aria-hidden='true' />
      </span>
      <span className='flex min-w-0 flex-1 flex-col gap-0.5'>
        <span className='truncate text-sm font-medium'>
          {props.action.title}
        </span>
        <span className='line-clamp-2 text-xs leading-relaxed text-white/45'>
          {props.action.description}
        </span>
      </span>
    </Button>
  )
}

function CompactQuickAction(props: { action: QuickAction }) {
  const Icon = props.action.icon

  return (
    <Button
      variant='outline'
      size='sm'
      className='h-8 min-w-24 gap-1.5 rounded-xl border-white/10 bg-white/[0.035] px-2.5 text-white hover:border-cyan-200/25 hover:bg-cyan-300/10'
      render={<Link to={props.action.to} />}
    >
      <Icon data-icon='inline-start' />
      <span>{props.action.title}</span>
    </Button>
  )
}

export function OverviewDashboard() {
  const { t } = useYucoreTranslation()
  const user = useAuthStore((state) => state.auth.user)
  const { items: apiInfoItems } = useApiInfo()
  const {
    apiInfo: showApiInfoPanel,
    announcements: showAnnouncementsPanel,
    faq: showFAQPanel,
    uptimeKuma: showUptimePanel,
  } = useDashboardContentVisibility()
  const [manualSetupGuideExpanded, setManualSetupGuideExpanded] = useState<
    boolean | null
  >(() => getSavedSetupGuideExpanded())

  const requestCount = Number(user?.request_count ?? 0)
  const remainQuota = Number(user?.quota ?? 0)
  const usedQuota = Number(user?.used_quota ?? 0)
  const isAdmin = Boolean(user?.role && user.role >= ROLE.ADMIN)

  const apiKeysQuery = useQuery({
    queryKey: ['dashboard', 'overview', 'api-keys'],
    queryFn: async () => {
      const result = await getApiKeys({ p: 1, size: 10 })
      return result.success ? (result.data?.items ?? []) : []
    },
    staleTime: 60 * 1000,
  })

  const modelsQuery = useQuery({
    queryKey: ['dashboard', 'overview', 'user-models'],
    queryFn: async () => {
      const result = await getUserModels()
      return result.success ? (result.data ?? []) : []
    },
    staleTime: 5 * 60 * 1000,
  })

  const preferredKey = useMemo(
    () => getPreferredKey(apiKeysQuery.data ?? []),
    [apiKeysQuery.data]
  )

  const startSteps = useMemo<StartStep[]>(
    () => [
      {
        title: t('Create API Key'),
        description: t('Create a key for your app or service'),
        to: '/keys',
        icon: KeyRound,
        completed: Boolean(preferredKey),
      },
      {
        title: t('Add credits'),
        description: t('Keep enough balance before production traffic'),
        to: '/wallet',
        icon: CreditCard,
        completed: remainQuota > 0 || usedQuota > 0,
      },
      {
        title: t('Send a request'),
        description: t('Verify routing with Playground or your client'),
        to: '/playground',
        icon: TerminalSquare,
        completed: requestCount > 0,
      },
    ],
    [preferredKey, remainQuota, requestCount, t, usedQuota]
  )

  const quickActions = useMemo<QuickAction[]>(
    () => [
      {
        title: t('API Keys'),
        description: t('Create a key for your app or service'),
        to: '/keys',
        icon: KeyRound,
      },
      {
        title: t('Channels'),
        description: t('Configure upstream providers and routing.'),
        to: '/channels',
        icon: RadioTower,
        adminOnly: true,
      },
      {
        title: t('Usage Logs'),
        description: t('Inspect requests, errors, and billing details'),
        to: '/usage-logs',
        icon: FileText,
      },
      {
        title: t('YuCore Studio'),
        description: t(
          'Open image, video, canvas, prompt, and asset workflows'
        ),
        to: '/playground/studio',
        icon: WandSparkles,
      },
      {
        title: t('Pricing'),
        description: t('Review model rates before scaling traffic'),
        to: '/pricing',
        icon: BookOpen,
      },
    ],
    [t]
  )

  const visibleQuickActions = useMemo(
    () => quickActions.filter((action) => !action.adminOnly || isAdmin),
    [isAdmin, quickActions]
  )

  const heroSignals = useMemo<HeroSignal[]>(
    () => [
      {
        label: t('Route active'),
        value: apiInfoItems.length > 0 ? t('Online') : t('Current domain'),
        icon: RadioTower,
      },
      {
        label: t('Auth configured'),
        value: preferredKey ? t('Secured') : t('Needs API key'),
        icon: ShieldCheck,
      },
      {
        label: t('Model selected'),
        value: modelsQuery.data?.[0] ?? t('Loading'),
        icon: Timer,
      },
    ],
    [apiInfoItems.length, modelsQuery.data, preferredKey, t]
  )

  const requestExample = useMemo<RequestExample>(() => {
    const endpoint = normalizeEndpoint(apiInfoItems[0]?.url)
    const model = modelsQuery.data?.[0] ?? 'gpt-4o-mini'
    const keyName = preferredKey?.name ?? t('No API key yet')
    const ready = Boolean(preferredKey?.id && model)

    return {
      endpoint,
      model,
      keyName,
      keyId: preferredKey?.id,
      displayKey: preferredKey
        ? formatDisplayKey(`sk-${preferredKey.key}`)
        : 'sk-...',
      ready,
    }
  }, [apiInfoItems, modelsQuery.data, preferredKey, t])

  const completedStepCount = startSteps.filter((step) => step.completed).length
  const setupComplete = completedStepCount === startSteps.length
  const setupStatusReady = apiKeysQuery.isFetched && Boolean(user)
  const setupGuideExpanded =
    manualSetupGuideExpanded ?? (setupStatusReady && !setupComplete)
  const panelPlan = useMemo(
    () =>
      getOverviewPanelPlan({
        isAdmin,
        apiInfo: showApiInfoPanel,
        announcements: showAnnouncementsPanel,
        faq: showFAQPanel,
        uptime: showUptimePanel,
      }),
    [
      isAdmin,
      showAnnouncementsPanel,
      showApiInfoPanel,
      showFAQPanel,
      showUptimePanel,
    ]
  )

  const handleSetupGuideToggle = () => {
    const nextExpanded = !setupGuideExpanded
    setManualSetupGuideExpanded(nextExpanded)
    saveSetupGuideExpanded(nextExpanded)
  }

  return (
    <div className='yucore-dashboard-overview flex flex-col gap-4'>
      <CardStaggerContainer className='grid gap-4 xl:grid-cols-[minmax(0,1fr)_28rem]'>
        <CardStaggerItem className='yucore-dashboard-hero relative overflow-hidden rounded-lg border border-white/10 bg-[#05070c]/92 p-5 text-white shadow-sm sm:p-6'>
          <div className='yucore-dashboard-hero-wash absolute inset-0 bg-[linear-gradient(118deg,rgba(34,211,238,0.18),transparent_34%),linear-gradient(242deg,rgba(167,139,250,0.14),transparent_32%),linear-gradient(135deg,#030406,#0d1221)]' />
          <div className='yucore-grid absolute inset-0 opacity-25' />
          <div className='relative'>
            <div className='min-w-0'>
              <div className='mb-7 flex flex-wrap items-center justify-between gap-3'>
                <YucoreBrandMark />
                <span className='inline-flex items-center gap-1.5 rounded-full border border-emerald-300/20 bg-emerald-300/10 px-3 py-1 text-xs font-medium text-emerald-100'>
                  <Activity className='size-3' aria-hidden='true' />
                  {t('Gateway online')}
                </span>
              </div>
              <div className='max-w-2xl'>
                <p className='text-xs font-medium tracking-[0.22em] text-cyan-100/55 uppercase'>
                  {YUCORE_BRAND_NAME}
                </p>
                <h2 className='mt-2 text-3xl leading-tight font-semibold tracking-tight text-white sm:text-4xl'>
                  {t('Your model command core is ready.')}
                </h2>
                <p className='mt-4 max-w-xl text-sm leading-7 text-white/58'>
                  {t(
                    'Operate API keys, quota, routing health, usage visibility, and creative studio workflows from the same account foundation.'
                  )}
                </p>
              </div>
              <div className='mt-6 flex flex-wrap gap-2'>
                <Button
                  className='h-9 rounded-xl bg-white text-black hover:bg-cyan-50'
                  render={<Link to='/keys' />}
                >
                  <KeyRound data-icon='inline-start' />
                  {t('Create API Key')}
                </Button>
                <Button
                  variant='outline'
                  className='h-9 rounded-xl border-white/15 bg-white/[0.035] text-white hover:bg-white/10'
                  render={<Link to='/playground/studio' />}
                >
                  <WandSparkles data-icon='inline-start' />
                  {t('Open YuCore Studio')}
                </Button>
              </div>
              <div className='yucore-dashboard-hero-signals mt-7 grid gap-2 sm:grid-cols-3'>
                {heroSignals.map((signal) => (
                  <DashboardSignalItem key={signal.label} signal={signal} />
                ))}
              </div>
            </div>
            <YucoreOpsPulse
              className='mt-7'
              requestCount={requestCount}
              quota={remainQuota}
              usedQuota={usedQuota}
              modelName={modelsQuery.data?.[0]}
            />
          </div>
        </CardStaggerItem>

        <CardStaggerItem>
          <YucoreStudioEntry className='h-full' compact />
        </CardStaggerItem>
      </CardStaggerContainer>

      {setupGuideExpanded ? (
        <CardStaggerContainer className='grid items-stretch gap-4 2xl:grid-cols-[minmax(0,1fr)_22rem]'>
          <CardStaggerItem className='yucore-panel h-full overflow-hidden rounded-lg text-white'>
            <div className='relative h-full overflow-hidden p-4 sm:p-5'>
              <SetupGuideBackdrop />
              <div className='relative grid gap-5 2xl:grid-cols-[minmax(0,1fr)_21rem]'>
                <div className='flex min-w-0 flex-col gap-5'>
                  <div className='flex flex-wrap items-start justify-between gap-3'>
                    <div className='flex max-w-2xl flex-col gap-1'>
                      <div className='flex items-center gap-2 text-xs font-medium tracking-[0.22em] text-cyan-100/55 uppercase'>
                        <ListChecks className='size-3.5' aria-hidden='true' />
                        {t('Launch sequence')}
                      </div>
                      <h3 className='text-xl font-semibold tracking-tight text-white sm:text-2xl'>
                        {t('Build on your API gateway in minutes')}
                      </h3>
                      <p className='max-w-xl text-sm leading-relaxed text-white/52'>
                        {t(
                          'A focused home for keys, balance, routing, and service health.'
                        )}
                      </p>
                    </div>
                    <div className='flex flex-wrap items-center gap-2'>
                      <Button
                        variant='outline'
                        size='sm'
                        className='border-white/15 bg-white/[0.035] text-white hover:bg-white/10'
                        onClick={handleSetupGuideToggle}
                      >
                        <ChevronUp data-icon='inline-start' />
                        {t('Hide setup guide')}
                      </Button>
                      <Button
                        size='sm'
                        className='bg-white text-black hover:bg-cyan-50'
                        render={<Link to='/keys' />}
                      >
                        <KeyRound data-icon='inline-start' />
                        {t('Create API Key')}
                      </Button>
                    </div>
                  </div>

                  <div className='grid gap-3 2xl:grid-cols-[minmax(0,1fr)_12rem]'>
                    <ol className='rounded-lg border border-white/10 bg-black/35 p-2 backdrop-blur'>
                      {startSteps.map((step, index) => (
                        <StartStepItem
                          key={step.title}
                          step={step}
                          index={index}
                          isLast={index === startSteps.length - 1}
                        />
                      ))}
                    </ol>
                    <ReadinessMeter
                      completed={completedStepCount}
                      total={startSteps.length}
                    />
                  </div>
                </div>

                <RequestPreview
                  example={requestExample}
                  signals={heroSignals}
                />
              </div>
            </div>
          </CardStaggerItem>

          <CardStaggerItem className='yucore-panel h-full rounded-lg p-4 text-white sm:p-5'>
            <div className='flex h-full flex-col gap-4'>
              <div className='flex flex-col gap-1'>
                <div className='flex items-center gap-2 text-xs font-medium tracking-[0.22em] text-cyan-100/55 uppercase'>
                  <Sparkles className='size-3.5' aria-hidden='true' />
                  {t('Recommended actions')}
                </div>
                <h3 className='text-lg font-semibold tracking-tight text-white'>
                  {t('Keep the platform ready')}
                </h3>
              </div>
              <div className='grid gap-2'>
                {visibleQuickActions.map((action) => (
                  <QuickActionItem key={action.title} action={action} />
                ))}
              </div>
            </div>
          </CardStaggerItem>
        </CardStaggerContainer>
      ) : (
        <CardStaggerContainer>
          <CardStaggerItem className='yucore-panel overflow-hidden rounded-lg text-white'>
            <div className='relative overflow-hidden px-4 py-3 sm:px-5'>
              <SetupGuideBackdrop compact />
              <div className='relative flex flex-wrap items-center justify-between gap-3'>
                <div className='flex min-w-0 items-center gap-3'>
                  <span className='flex size-9 shrink-0 items-center justify-center rounded-xl border border-emerald-200/25 bg-emerald-300/10 shadow-xs'>
                    <Check
                      className='size-4 text-emerald-200'
                      aria-hidden='true'
                    />
                  </span>
                  <div className='min-w-0'>
                    <div className='flex items-center gap-2'>
                      <h3 className='truncate text-sm font-semibold text-white'>
                        {setupComplete
                          ? t('Setup guide complete')
                          : t('Setup guide')}
                      </h3>
                      <span className='rounded-md border border-white/10 bg-white/[0.035] px-2 py-0.5 text-xs text-white/48'>
                        {t('Setup progress: {{completed}}/{{total}}', {
                          completed: completedStepCount,
                          total: startSteps.length,
                        })}
                      </span>
                    </div>
                    <p className='line-clamp-1 text-xs text-white/45'>
                      {setupComplete
                        ? t(
                            'Your setup guide is collapsed so usage stays in focus.'
                          )
                        : t('Setup guide is collapsed. Expand it anytime.')}
                    </p>
                  </div>
                </div>

                <div className='flex flex-wrap items-center gap-2'>
                  {visibleQuickActions.map((action) => (
                    <CompactQuickAction key={action.title} action={action} />
                  ))}
                  <Button
                    variant='outline'
                    size='sm'
                    className='h-8 min-w-28 rounded-xl border-white/10 bg-white/[0.035] text-white hover:border-cyan-200/25 hover:bg-cyan-300/10'
                    onClick={handleSetupGuideToggle}
                  >
                    <ChevronDown data-icon='inline-start' />
                    {t('Show setup guide')}
                  </Button>
                </div>
              </div>
            </div>
          </CardStaggerItem>
        </CardStaggerContainer>
      )}

      <Suspense fallback={<OverviewSecondaryPanelsFallback plan={panelPlan} />}>
        <LazyOverviewSecondaryPanels plan={panelPlan} />
      </Suspense>
    </div>
  )
}
