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
import { lazy, Suspense } from 'react'

import { useTheme } from '@/context/theme-provider'
import { cn } from '@/lib/utils'

const LazyYucoreSignalFieldWebgl = lazy(() =>
  import('./yucore-signal-field-webgl').then((module) => ({
    default: module.YucoreSignalFieldWebgl,
  }))
)

const LazyYucoreWebglEarth = lazy(() =>
  import('./yucore-webgl-earth').then((module) => ({
    default: module.YucoreWebglEarth,
  }))
)

interface YucoreBackgroundProps {
  active?: boolean
  className?: string
  coreMode?: 'full' | 'ambient'
  corePlacement?: 'auth' | 'hero'
  intensity?: 'calm' | 'hero' | 'workbench'
  preparation?: YucoreBackgroundPreparation
  showEarthCore?: boolean
}

export type YucoreBackgroundPreparation = 'none' | 'signal' | 'all'

export function YucoreBackground(props: YucoreBackgroundProps) {
  const { resolvedTheme } = useTheme()
  const intensity = props.intensity ?? 'calm'
  const active = props.active !== false
  const preparation = props.preparation ?? (active ? 'all' : 'none')
  const signalPrepared = preparation !== 'none'
  const earthPrepared = preparation === 'all'

  return (
    <div
      aria-hidden='true'
      className={cn(
        'pointer-events-none absolute inset-0 z-0 overflow-hidden',
        signalPrepared
          ? 'yucore-background-webgl-active'
          : 'yucore-background-static',
        props.className
      )}
    >
      <div
        className={cn(
          'yucore-background-base absolute inset-0 z-0',
          intensity === 'calm' && 'opacity-85',
          intensity === 'hero' && 'opacity-100',
          intensity === 'workbench' && 'opacity-95'
        )}
      />
      {signalPrepared ? (
        <Suspense fallback={null}>
          <LazyYucoreSignalFieldWebgl
            active={props.active}
            colorMode={resolvedTheme}
            coreMode={props.coreMode}
            corePlacement={props.corePlacement}
            intensity={intensity}
            className={cn(
              'z-[2]',
              intensity === 'calm' && 'opacity-[0.7]',
              intensity === 'hero' && 'opacity-100',
              intensity === 'workbench' && 'opacity-[0.9]'
            )}
          />
        </Suspense>
      ) : (
        <div
          className={cn(
            'yucore-background-particle-mesh absolute inset-0 z-[2]',
            intensity === 'hero' && 'yucore-background-particle-mesh-hero',
            intensity === 'workbench' &&
              'yucore-background-particle-mesh-workbench',
            intensity === 'calm' && 'yucore-background-particle-mesh-calm'
          )}
        />
      )}
      {earthPrepared && props.showEarthCore !== false && (
        <div
          className={cn(
            'yucore-background-earth-core absolute z-[5]',
            intensity === 'calm' && 'yucore-background-earth-core-calm',
            intensity === 'hero' && 'yucore-background-earth-core-hero',
            intensity === 'workbench' &&
              'yucore-background-earth-core-workbench'
          )}
        >
          <Suspense fallback={null}>
            <LazyYucoreWebglEarth
              active={props.active}
              colorMode={resolvedTheme}
              density={intensity === 'hero' ? 'loader' : 'persistent'}
            />
          </Suspense>
        </div>
      )}
      <div
        className={cn(
          'yucore-energy-vortex absolute top-[18%] left-1/2 z-[1] h-[38rem] w-[38rem] -translate-x-1/2 rounded-full',
          intensity === 'hero' ? 'opacity-70' : 'opacity-40'
        )}
      />
      <div
        className={cn(
          'yucore-energy-thread yucore-energy-thread-a absolute top-[28%] left-[-12%] z-[4] h-px w-[124vw]',
          intensity === 'calm' && 'opacity-45'
        )}
      />
      <div
        className={cn(
          'yucore-energy-thread yucore-energy-thread-b absolute top-[58%] left-[-18%] z-[4] h-px w-[132vw]',
          intensity === 'workbench' ? 'opacity-70' : 'opacity-52'
        )}
      />
      <div
        className={cn(
          'yucore-background-power-field absolute inset-0 z-[3]',
          intensity === 'hero' && 'yucore-background-power-field-hero',
          intensity === 'workbench' &&
            'yucore-background-power-field-workbench',
          intensity === 'calm' && 'yucore-background-power-field-calm'
        )}
      />
      <div
        className={cn(
          'yucore-grid absolute inset-0 z-[1]',
          intensity === 'hero' ? 'opacity-[0.36]' : 'opacity-[0.28]'
        )}
      />
      <div className='yucore-scanlines absolute inset-0 z-[4] opacity-[0.18]' />
      <div className='yucore-background-vignette absolute inset-0 z-[4]' />
      <div className='yucore-background-horizon absolute inset-x-[-12%] bottom-[-18%] z-[4] h-[48%] rotate-[-2deg] opacity-70 blur-xl' />
      <div className='yucore-background-topline absolute top-[8%] left-1/2 z-[4] h-px w-[68rem] max-w-[95vw] -translate-x-1/2' />
    </div>
  )
}
