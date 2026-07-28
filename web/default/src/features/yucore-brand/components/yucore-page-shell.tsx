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
import { cn } from '@/lib/utils'

import { YucoreBackground } from './yucore-background'
import { YucoreEntranceLoader } from './yucore-entrance-loader'
import { YucorePersistentCore } from './yucore-persistent-core'

interface YucorePageShellProps {
  children: React.ReactNode
  className?: string
  contentClassName?: string
  intensity?: 'calm' | 'hero' | 'workbench'
  paddedTop?: boolean
  persistentCoreClassName?: string
  showBackground?: boolean
  showEntranceLoader?: boolean
  showPersistentCore?: boolean
}

export function YucorePageShell(props: YucorePageShellProps) {
  return (
    <div
      className={cn(
        'yucore-app-shell text-foreground relative min-h-svh overflow-hidden',
        props.showBackground === false ? 'bg-transparent' : 'bg-background',
        props.className
      )}
    >
      {props.showEntranceLoader && <YucoreEntranceLoader />}
      {props.showBackground !== false && (
        <YucoreBackground
          active
          coreMode='ambient'
          corePlacement='hero'
          intensity={props.intensity ?? 'calm'}
          showEarthCore={!props.showPersistentCore}
        />
      )}
      {props.showPersistentCore && (
        <YucorePersistentCore
          active
          animated={false}
          className={cn(
            'yucore-persistent-core-console',
            props.persistentCoreClassName
          )}
          webglActive={false}
        />
      )}
      <div
        className={cn(
          'relative mx-auto w-full max-w-[1800px] px-3 pb-8 sm:px-6 sm:pb-10 xl:px-8',
          props.paddedTop !== false && 'pt-20 sm:pt-24',
          props.contentClassName
        )}
      >
        {props.children}
      </div>
    </div>
  )
}
