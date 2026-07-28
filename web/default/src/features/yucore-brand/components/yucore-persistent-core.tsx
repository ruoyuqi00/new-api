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
import type { CSSProperties } from 'react'

import { useTheme } from '@/context/theme-provider'
import { cn } from '@/lib/utils'

import { YucoreWebglEarth } from './yucore-webgl-earth'

interface YucorePersistentCoreProps {
  active?: boolean
  animated?: boolean
  className?: string
  webglActive?: boolean
  webglTimeOffsetSeconds?: number
}

const PERSISTENT_CORE_DOT_COUNT = 144

type PersistentCoreDotKind = 'surface' | 'halo' | 'field'
type PersistentCoreDotTone = 'amber' | 'rose' | 'cyan'

function deterministicUnit(index: number, salt: number) {
  const value = Math.sin(index * 12.9898 + salt * 78.233) * 43758.5453

  return value - Math.floor(value)
}

function getPersistentCoreDotKind(lane: number): PersistentCoreDotKind {
  if (lane < 8) return 'surface'
  if (lane < 36) return 'halo'
  return 'field'
}

function getPersistentFieldYOffset(index: number, fieldBand: number) {
  let bandOffset = 0
  if (fieldBand === 0) {
    bandOffset = -38
  } else if (fieldBand === 3) {
    bandOffset = 34
  } else if (fieldBand >= 6) {
    bandOffset = 52
  }

  return (deterministicUnit(index, 11) - 0.5) * 46 + bandOffset
}

function getPersistentFieldXSkew(index: number, fieldBand: number) {
  let bandOffset = 0
  if (fieldBand === 1) {
    bandOffset = -32
  } else if (fieldBand === 4) {
    bandOffset = 34
  }

  return (deterministicUnit(index, 13) - 0.5) * 52 + bandOffset
}

function getPersistentCoreDotAlpha(kind: PersistentCoreDotKind, depth: number) {
  if (kind === 'surface') return 0.1 + depth * 0.18
  if (kind === 'halo') return 0.072 + depth * 0.17
  return 0.085 + depth * 0.26
}

function getPersistentCoreHomeOpacity(kind: PersistentCoreDotKind) {
  if (kind === 'surface') return 0.08
  if (kind === 'halo') return 0.44
  return 0.92
}

function getPersistentCoreDotSize(
  index: number,
  kind: PersistentCoreDotKind,
  depth: number
) {
  const sizeStep = (index * 11) % 6

  if (kind === 'surface') return 0.72 + sizeStep * 0.12 + depth * 0.58
  if (kind === 'halo') return 0.56 + sizeStep * 0.11 + depth * 0.42
  return 0.44 + sizeStep * 0.1 + depth * 0.34
}

function getPersistentCoreDotTone(index: number): PersistentCoreDotTone {
  if (index % 13 === 0) return 'amber'
  if (index % 7 === 0) return 'rose'
  return 'cyan'
}

function getPersistentCoreDotColor(tone: PersistentCoreDotTone) {
  if (tone === 'amber') return 'rgb(252 211 77)'
  if (tone === 'rose') return 'rgb(251 113 133)'
  return 'rgb(190 242 255)'
}

const persistentCoreDots = Array.from(
  { length: PERSISTENT_CORE_DOT_COUNT },
  (_, index) => {
    const goldenAngle = Math.PI * (3 - Math.sqrt(5))
    const y = 1 - (index / (PERSISTENT_CORE_DOT_COUNT - 1)) * 2
    const radius = Math.sqrt(1 - y * y)
    const theta = index * goldenAngle
    const x = Math.cos(theta) * radius
    const z = Math.sin(theta) * radius
    const depth = (z + 1) / 2
    const lane = index % 100
    const kind = getPersistentCoreDotKind(lane)
    const haloAngle = theta + deterministicUnit(index, 3) * 0.38
    const haloRadius =
      kind === 'field'
        ? 132 + deterministicUnit(index, 5) * 96 + (index % 23 === 0 ? 34 : 0)
        : 72 + deterministicUnit(index, 5) * 44 + (index % 19 === 0 ? 12 : 0)
    const fieldBand = index % 8
    const fieldYOffset =
      kind === 'field' ? getPersistentFieldYOffset(index, fieldBand) : 0
    const fieldXSkew =
      kind === 'field' ? getPersistentFieldXSkew(index, fieldBand) : 0

    return {
      alpha: getPersistentCoreDotAlpha(kind, depth),
      delay: (index % 89) * 41,
      depth,
      homeOpacity: getPersistentCoreHomeOpacity(kind),
      id: `persistent-core-dot-${index}`,
      kind,
      size: getPersistentCoreDotSize(index, kind, depth),
      tone: getPersistentCoreDotTone(index),
      x:
        kind === 'surface'
          ? 50 + x * (34 + depth * 5)
          : 50 + Math.cos(haloAngle) * haloRadius + fieldXSkew,
      y:
        kind === 'surface'
          ? 50 + y * 33
          : 50 +
            Math.sin(haloAngle) *
              haloRadius *
              (kind === 'field'
                ? 0.58 + deterministicUnit(index, 7) * 0.46
                : 0.5 + deterministicUnit(index, 7) * 0.24) +
            fieldYOffset,
    }
  }
)

const persistentCoreSurfaceDots = persistentCoreDots.filter(
  (dot) => dot.kind !== 'field'
)

const persistentCoreFieldDots = persistentCoreDots.filter(
  (dot) => dot.kind === 'field'
)

export function YucorePersistentCore(props: YucorePersistentCoreProps) {
  const { resolvedTheme } = useTheme()
  if (!props.active) return null

  const animated = props.animated !== false

  return (
    <div
      aria-hidden='true'
      className={cn(
        'yucore-persistent-core pointer-events-none fixed left-[68vw] top-[36svh] z-[2]',
        !animated && 'yucore-persistent-core-static',
        props.className
      )}
      data-active={props.active ? 'true' : 'false'}
    >
      <div className='yucore-persistent-core-aura absolute inset-[-20%]' />
      <div className='yucore-persistent-core-planet absolute rounded-full'>
        <span className='yucore-persistent-core-ocean absolute inset-0 rounded-full' />
        <span className='yucore-persistent-core-land yucore-persistent-core-land-a absolute inset-[-3%]' />
        <span className='yucore-persistent-core-land yucore-persistent-core-land-b absolute inset-[-3%]' />
        <YucoreWebglEarth
          active={props.webglActive ?? animated}
          className='yucore-persistent-core-webgl'
          colorMode={resolvedTheme}
          density='persistent'
          timeOffsetSeconds={props.webglTimeOffsetSeconds}
        />
        <span className='yucore-persistent-core-clouds absolute inset-[-2%] rounded-full' />
        <span className='yucore-persistent-core-grid absolute inset-0 rounded-full' />
        <span className='yucore-persistent-core-night absolute inset-0 rounded-full' />
        <span className='yucore-persistent-core-rim absolute inset-0 rounded-full' />
      </div>
      <div className='yucore-persistent-core-terminator absolute rounded-full' />
      <div className='yucore-persistent-core-orbit yucore-persistent-core-orbit-a absolute rounded-full' />
      <div className='yucore-persistent-core-orbit yucore-persistent-core-orbit-b absolute rounded-full' />
      <div className='yucore-persistent-core-equator absolute rounded-full' />
      <div className='yucore-persistent-core-route-layer absolute inset-[-6%]'>
        <span className='yucore-persistent-core-route yucore-persistent-core-route-a absolute rounded-full' />
        <span className='yucore-persistent-core-route yucore-persistent-core-route-b absolute rounded-full' />
        <span className='yucore-persistent-core-route yucore-persistent-core-route-c absolute rounded-full' />
      </div>
      <div className='yucore-persistent-core-field absolute inset-[-34%]'>
        {persistentCoreFieldDots.map((dot) => (
          <span
            key={dot.id}
            className='yucore-persistent-core-dot yucore-persistent-core-dot-field absolute rounded-full'
            style={
              {
                '--dot-alpha': dot.alpha,
                '--dot-depth': dot.depth,
                '--dot-delay': `${dot.delay}ms`,
                '--dot-home-opacity': dot.homeOpacity,
                '--dot-size': `${dot.size}px`,
                '--dot-x': `${dot.x}%`,
                '--dot-y': `${dot.y}%`,
                color: getPersistentCoreDotColor(dot.tone),
              } as CSSProperties
            }
          />
        ))}
      </div>
      <div className='yucore-persistent-core-shell absolute inset-0'>
        {persistentCoreSurfaceDots.map((dot) => (
          <span
            key={dot.id}
            className={cn(
              'yucore-persistent-core-dot absolute rounded-full',
              dot.kind === 'surface' && 'yucore-persistent-core-dot-surface',
              dot.kind === 'halo' && 'yucore-persistent-core-dot-halo',
              dot.kind === 'field' && 'yucore-persistent-core-dot-field'
            )}
            style={
              {
                '--dot-alpha': dot.alpha,
                '--dot-depth': dot.depth,
                '--dot-delay': `${dot.delay}ms`,
                '--dot-home-opacity': dot.homeOpacity,
                '--dot-size': `${dot.size}px`,
                '--dot-x': `${dot.x}%`,
                '--dot-y': `${dot.y}%`,
                color: getPersistentCoreDotColor(dot.tone),
              } as CSSProperties
            }
          />
        ))}
      </div>
      <div className='yucore-persistent-core-mark absolute'>
        <span className='yucore-persistent-core-mark-inner' />
      </div>
      <div className='yucore-persistent-core-sweep absolute inset-0' />
    </div>
  )
}
