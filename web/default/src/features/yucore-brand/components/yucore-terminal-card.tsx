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
import { TerminalSquare } from 'lucide-react'

import { cn } from '@/lib/utils'

import { yucoreHeroSignals, yucoreTerminalLines } from '../data/content'
import { useYucoreTranslation } from '../i18n/use-yucore-translation'

interface YucoreTerminalCardProps {
  className?: string
  title?: string
  lines?: string[]
}

export function YucoreTerminalCard(props: YucoreTerminalCardProps) {
  const { t } = useYucoreTranslation()
  const lines = props.lines ?? yucoreTerminalLines

  return (
    <div
      className={cn(
        'yucore-panel yucore-sweep relative overflow-hidden rounded-2xl p-4 shadow-2xl shadow-cyan-950/30',
        props.className
      )}
    >
      <div className='mb-4 flex items-center justify-between gap-3 border-b border-white/10 pb-3'>
        <div className='flex min-w-0 items-center gap-2'>
          <span className='flex size-9 shrink-0 items-center justify-center rounded-xl border border-cyan-300/20 bg-cyan-300/10 text-cyan-100'>
            <TerminalSquare className='size-4' aria-hidden='true' />
          </span>
          <div className='min-w-0'>
            <div className='truncate text-sm font-semibold text-white'>
              {props.title ?? t('YuCore gateway request')}
            </div>
            <div className='text-xs text-white/45'>
              {t('copy-ready API core')}
            </div>
          </div>
        </div>
        <div className='flex items-center gap-1.5' aria-hidden='true'>
          <span className='size-2.5 rounded-full bg-red-400/80' />
          <span className='size-2.5 rounded-full bg-amber-300/80' />
          <span className='size-2.5 rounded-full bg-emerald-300/80' />
        </div>
      </div>
      <pre className='overflow-hidden font-mono text-[12px] leading-6 text-cyan-50/78 sm:text-[13px]'>
        {lines.map((line, index) => (
          <code key={line} className='block truncate'>
            <span className='mr-3 text-white/25'>
              {String(index + 1).padStart(2, '0')}
            </span>
            {line}
          </code>
        ))}
      </pre>
      <div className='mt-5 grid gap-2 sm:grid-cols-3'>
        {yucoreHeroSignals.map((signal) => {
          const Icon = signal.icon

          return (
            <div
              key={signal.label}
              className='rounded-xl border border-white/10 bg-white/[0.035] px-3 py-2'
            >
              <div className='flex items-center gap-1.5 text-[11px] text-white/45'>
                <Icon className='size-3' aria-hidden='true' />
                {t(signal.label)}
              </div>
              <div className='mt-1 text-sm font-semibold text-white'>
                {t(signal.value)}
              </div>
            </div>
          )
        })}
      </div>
    </div>
  )
}
