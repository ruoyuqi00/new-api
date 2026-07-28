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
        'yucore-app-shell bg-background text-foreground relative min-h-svh overflow-hidden',
        props.className
      )}
    >
      <YucoreBackground
        active
        coreMode='ambient'
        corePlacement='hero'
        intensity='calm'
        showEarthCore={false}
        className='yucore-public-background fixed opacity-95'
      />
      <YucorePersistentCore
        active
        animated={false}
        className='yucore-persistent-core-console yucore-persistent-core-public'
        webglActive={false}
      />
      <div
        aria-hidden='true'
        className='yucore-background-readability pointer-events-none fixed inset-0 z-0'
      />
      <div className='relative z-10 min-h-svh'>{props.children}</div>
    </div>
  )
}
