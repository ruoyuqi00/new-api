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
import { YucoreBackground, YucorePersistentCore } from '@/features/yucore-brand'
import { cn } from '@/lib/utils'

type YucoreErrorShellProps = {
  children: React.ReactNode
  className?: string
}

export function YucoreErrorShell(props: YucoreErrorShellProps) {
  return (
    <div
      className={cn(
        'dark relative min-h-svh overflow-hidden bg-[#05070c] text-white',
        props.className
      )}
    >
      <YucoreBackground
        coreMode='ambient'
        intensity='calm'
        className='fixed yucore-public-background opacity-95'
      />
      <YucorePersistentCore
        active
        className='yucore-persistent-core-console yucore-persistent-core-public'
      />
      <div
        aria-hidden='true'
        className='pointer-events-none fixed inset-0 z-0 bg-[radial-gradient(circle_at_16%_0%,rgba(103,232,249,0.08),transparent_26%),linear-gradient(180deg,rgba(0,0,0,0.02),rgba(0,0,0,0.24))]'
      />
      <div className='relative z-10 min-h-svh'>{props.children}</div>
    </div>
  )
}
