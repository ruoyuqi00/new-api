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
import { Sparkles } from 'lucide-react'

import { cn } from '@/lib/utils'

import { YUCORE_BRAND_NAME } from '../data/content'

interface YucoreBrandMarkProps {
  className?: string
  compact?: boolean
}

export function YucoreBrandMark(props: YucoreBrandMarkProps) {
  return (
    <span className={cn('inline-flex items-center gap-2', props.className)}>
      <span className='relative flex size-8 shrink-0 items-center justify-center rounded-full border border-cyan-300/35 bg-black shadow-[0_0_34px_rgba(34,211,238,0.28)]'>
        <span className='absolute inset-1 rounded-full bg-[conic-gradient(from_160deg,#22d3ee,#a78bfa,#34d399,#22d3ee)] opacity-80 blur-[1px]' />
        <span className='relative flex size-5 items-center justify-center rounded-full bg-[#05070c]'>
          <Sparkles className='size-3.5 text-cyan-200' aria-hidden='true' />
        </span>
      </span>
      {!props.compact && (
        <span className='flex min-w-0 flex-col leading-none'>
          <span className='text-foreground text-sm font-semibold tracking-tight'>
            {YUCORE_BRAND_NAME}
          </span>
          <span className='text-primary/55 mt-1 text-[10px] font-medium tracking-[0.24em] uppercase'>
            模型核心
          </span>
        </span>
      )}
    </span>
  )
}
