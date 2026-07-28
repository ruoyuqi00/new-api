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
  Braces,
  Clapperboard,
  ImagePlus,
  Layers3,
  RadioTower,
  Route,
  ShieldCheck,
  type LucideIcon,
  Zap,
} from 'lucide-react'
import { useEffect, useState } from 'react'

import { cn } from '@/lib/utils'

import { useYucoreTranslation } from '../i18n/use-yucore-translation'

type CommandMode = {
  title: string
  eyebrow: string
  icon: LucideIcon
  accent: 'cyan' | 'amber' | 'rose'
  command: string[]
  metrics: Array<{ label: string; value: string; icon: LucideIcon }>
  pipeline: string[]
}

const commandModes: CommandMode[] = [
  {
    title: 'API Core',
    eyebrow: 'OpenAI-compatible route',
    icon: Route,
    accent: 'cyan',
    command: [
      'POST /v1/responses',
      'model: gpt-5.5',
      'fallback: claude + grok',
      'quota: synced',
    ],
    metrics: [
      { label: 'Latency', value: '312 ms', icon: Zap },
      { label: 'Failover', value: 'Armed', icon: Route },
      { label: 'Auth', value: 'Scoped', icon: ShieldCheck },
    ],
    pipeline: ['key verified', 'pool selected', 'usage recorded'],
  },
  {
    title: 'Image Lab',
    eyebrow: 'Canvas and generation bridge',
    icon: ImagePlus,
    accent: 'amber',
    command: [
      'canvas.compose()',
      'refs: 4 images',
      'size: 1536 x 1024',
      'format: b64_json',
    ],
    metrics: [
      { label: 'Assets', value: '28', icon: Layers3 },
      { label: 'Prompt', value: 'Clean', icon: Braces },
      { label: 'Sync', value: 'Ready', icon: RadioTower },
    ],
    pipeline: ['references locked', 'prompt weighted', 'result archived'],
  },
  {
    title: 'Video Render',
    eyebrow: 'Motion queue and media billing',
    icon: Clapperboard,
    accent: 'rose',
    command: [
      'render.video()',
      'duration: 8 sec',
      'motion: camera drift',
      'billing: tracked',
    ],
    metrics: [
      { label: 'Queue', value: '02', icon: Activity },
      { label: 'Route', value: 'Media', icon: Route },
      { label: 'Codec', value: 'H.264', icon: Braces },
    ],
    pipeline: ['storyboard parsed', 'frames queued', 'cost reserved'],
  },
]

const accentClassName = {
  cyan: 'border-cyan-300/30 bg-cyan-300/10 text-cyan-100',
  amber: 'border-amber-300/30 bg-amber-300/10 text-amber-100',
  rose: 'border-rose-300/30 bg-rose-300/10 text-rose-100',
} as const

const telemetry = ['auth', 'route', 'bill', 'cache', 'media', 'logs']

interface YucoreCommandCenterProps {
  className?: string
}

export function YucoreCommandCenter(props: YucoreCommandCenterProps) {
  const { t } = useYucoreTranslation()
  const [activeIndex, setActiveIndex] = useState(0)
  const activeMode = commandModes[activeIndex]
  const ActiveIcon = activeMode.icon

  useEffect(() => {
    const interval = window.setInterval(() => {
      setActiveIndex((index) => (index + 1) % commandModes.length)
    }, 2600)

    return () => window.clearInterval(interval)
  }, [])

  return (
    <div className={cn('yucore-command-deck relative', props.className)}>
      <div className='yucore-command-glow absolute inset-4 rounded-[2rem]' />
      <div className='yucore-command-orbit absolute top-1/2 left-1/2 hidden size-[34rem] -translate-x-1/2 -translate-y-1/2 rounded-full min-[920px]:block' />
      <div className='yucore-command-node yucore-command-node-a absolute hidden size-2 rounded-full bg-cyan-200 min-[920px]:block' />
      <div className='yucore-command-node yucore-command-node-b absolute hidden size-2 rounded-full bg-amber-200 min-[920px]:block' />
      <div className='yucore-command-node yucore-command-node-c absolute hidden size-2 rounded-full bg-rose-200 min-[920px]:block' />
      <div className='yucore-command-frame relative overflow-hidden rounded-[1.7rem] border border-white/10 bg-[#05070c]/80 p-3 shadow-[0_36px_120px_rgba(0,0,0,0.46)] backdrop-blur-2xl'>
        <div className='absolute inset-0 bg-[radial-gradient(circle_at_68%_20%,rgba(250,204,21,0.12),transparent_28%),radial-gradient(circle_at_18%_14%,rgba(34,211,238,0.16),transparent_28%)]' />
        <div className='yucore-command-scan absolute inset-0' />
        <div className='relative overflow-hidden rounded-[1.25rem] border border-white/10 bg-black/35'>
          <div className='flex items-center justify-between gap-3 border-b border-white/10 px-3 py-3 sm:px-4'>
            <div className='flex min-w-0 items-center gap-2'>
              <span
                className={cn(
                  'flex size-9 shrink-0 items-center justify-center rounded-md border',
                  accentClassName[activeMode.accent]
                )}
              >
                <ActiveIcon className='size-4' aria-hidden='true' />
              </span>
              <div className='min-w-0'>
                <div className='truncate text-sm font-semibold text-white'>
                  {t(activeMode.title)}
                </div>
                <div className='truncate text-xs text-white/45'>
                  {t(activeMode.eyebrow)}
                </div>
              </div>
            </div>
            <div className='hidden items-center gap-1.5 rounded-full border border-emerald-300/20 bg-emerald-300/10 px-2.5 py-1 text-[11px] font-medium text-emerald-100 sm:flex'>
              <span className='relative flex size-1.5'>
                <span className='absolute inline-flex h-full w-full animate-ping rounded-full bg-emerald-300 opacity-70' />
                <span className='relative inline-flex size-1.5 rounded-full bg-emerald-200' />
              </span>
              {t('Live')}
            </div>
          </div>

          <div className='grid gap-3 p-3 sm:p-4'>
            <div className='yucore-holo-stage relative min-h-[13.5rem] overflow-hidden rounded-[1.1rem] border border-white/10 bg-[#030406]'>
              <div className='yucore-holo-grid absolute inset-0' />
              <div className='yucore-holo-radar absolute top-1/2 left-1/2 size-52 -translate-x-1/2 -translate-y-1/2 rounded-full' />
              <div className='yucore-holo-beam absolute top-1/2 left-1/2 h-px w-[72%] -translate-x-1/2 -translate-y-1/2' />
              <div className='yucore-holo-beam yucore-holo-beam-vertical absolute top-1/2 left-1/2 h-px w-[58%] -translate-x-1/2 -translate-y-1/2' />
              <div
                className={cn(
                  'yucore-route-core absolute top-1/2 left-1/2 h-24 w-36 -translate-x-1/2 -translate-y-1/2 rounded-md border bg-black/55',
                  accentClassName[activeMode.accent]
                )}
                aria-hidden='true'
              >
                <span className='yucore-route-core-grid absolute inset-0' />
                <span className='yucore-route-core-rail yucore-route-core-rail-horizontal absolute' />
                <span className='yucore-route-core-rail yucore-route-core-rail-vertical absolute' />
                <span className='yucore-route-core-node yucore-route-core-node-left absolute' />
                <span className='yucore-route-core-node yucore-route-core-node-top absolute' />
                <span className='yucore-route-core-node yucore-route-core-node-right absolute' />
                <span className='yucore-route-core-kernel absolute' />
              </div>
              <div className='absolute inset-x-4 bottom-12 grid grid-cols-6 gap-1.5'>
                {telemetry.map((item, index) => (
                  <span
                    key={item}
                    className='yucore-telemetry-bar h-1 rounded-full bg-cyan-100/30'
                    style={{ animationDelay: `${index * 120}ms` }}
                    aria-hidden='true'
                  />
                ))}
              </div>
              <div className='absolute top-4 left-4 rounded-full border border-cyan-300/20 bg-cyan-300/10 px-2.5 py-1 text-[11px] text-cyan-100'>
                {t('model mesh')}
              </div>
              <div className='absolute top-8 right-4 rounded-full border border-amber-300/20 bg-amber-300/10 px-2.5 py-1 text-[11px] text-amber-100'>
                {t('quota lock')}
              </div>
              <div className='absolute bottom-4 left-1/2 -translate-x-1/2 rounded-full border border-white/10 bg-black/45 px-3 py-1 text-[11px] text-white/68'>
                {t('command core synchronized')}
              </div>
            </div>

            <div className='grid gap-3 min-[520px]:grid-cols-[1fr_0.86fr]'>
              <div className='overflow-hidden rounded-2xl border border-white/10 bg-white/[0.035] p-3'>
                <div className='mb-2 flex items-center justify-between gap-2 text-[11px] text-white/38'>
                  <span>{t('request')}</span>
                  <span>{t('copy-ready')}</span>
                </div>
                <pre className='font-mono text-[12px] leading-6 text-cyan-50/78'>
                  {activeMode.command.map((line, index) => (
                    <code key={line} className='block truncate'>
                      <span className='mr-2 text-white/22'>
                        {String(index + 1).padStart(2, '0')}
                      </span>
                      {line}
                    </code>
                  ))}
                </pre>
              </div>

              <div className='grid gap-2'>
                {activeMode.metrics.map((metric) => {
                  const Icon = metric.icon

                  return (
                    <div
                      key={metric.label}
                      className='flex items-center justify-between gap-3 rounded-2xl border border-white/10 bg-white/[0.035] px-3 py-2'
                    >
                      <span className='flex items-center gap-1.5 text-[11px] text-white/45'>
                        <Icon className='size-3' aria-hidden='true' />
                        {t(metric.label)}
                      </span>
                      <span className='text-xs font-semibold text-white'>
                        {t(metric.value)}
                      </span>
                    </div>
                  )
                })}
              </div>
            </div>

            <div className='grid gap-2 sm:grid-cols-3'>
              {activeMode.pipeline.map((step, index) => (
                <div
                  key={step}
                  className='yucore-route-step relative overflow-hidden rounded-full border border-white/10 bg-white/[0.035] px-3 py-2 text-center text-[11px] text-white/58'
                  style={{ animationDelay: `${index * 180}ms` }}
                >
                  {t(step)}
                </div>
              ))}
            </div>
          </div>
        </div>

        <div className='relative mt-3 grid grid-cols-3 gap-2'>
          {commandModes.map((mode, index) => {
            const Icon = mode.icon
            const active = index === activeIndex

            return (
              <button
                key={mode.title}
                type='button'
                aria-pressed={active}
                onClick={() => setActiveIndex(index)}
                className={cn(
                  'flex min-h-12 items-center justify-center gap-1.5 rounded-2xl border px-2 text-xs font-medium transition',
                  active
                    ? accentClassName[mode.accent]
                    : 'border-white/10 bg-white/[0.035] text-white/48 hover:bg-white/[0.07] hover:text-white/72'
                )}
              >
                <Icon className='size-3.5 shrink-0' aria-hidden='true' />
                <span className='truncate'>{t(mode.title)}</span>
              </button>
            )
          })}
        </div>
      </div>
    </div>
  )
}
