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
import { type CSSProperties, useEffect, useRef, useState } from 'react'

import { useTheme } from '@/context/theme-provider'
import { cn } from '@/lib/utils'

import { useYucoreTranslation } from '../i18n/use-yucore-translation'
import { YucoreBootCanvas } from './yucore-boot-canvas'
import { YucoreSignalFieldWebgl } from './yucore-signal-field-webgl'
import { YucoreWebglEarth } from './yucore-webgl-earth'

interface YucoreEntranceLoaderProps {
  className?: string
  durationMs?: number
  onComplete?: () => void
}

export const YUCORE_BOOT_LOADER_DURATION_MS = 8200

const LOADER_PARTICLE_PREWARM_RATIO = 0.13
const LOADER_PARTICLE_ENTER_RATIO = 0.2
const LOADER_PARTICLE_PREPARE_RATIO = 0.08
const LOADER_SPHERE_PREWARM_RATIO = 0.6
const LOADER_SPHERE_PREPARE_RATIO = 0.52
const LOADER_HANDOFF_RATIO = 0.66
const LOADER_PARTICLE_RELEASE_RATIO = 0.79
const LOADER_LAYER_TRANSITION_MS = 1050

const LOADER_SPHERE_RINGS = [
  { count: 8, y: -0.96 },
  { count: 12, y: -0.88 },
  { count: 18, y: -0.76 },
  { count: 24, y: -0.62 },
  { count: 30, y: -0.46 },
  { count: 36, y: -0.28 },
  { count: 42, y: -0.1 },
  { count: 46, y: 0.1 },
  { count: 42, y: 0.28 },
  { count: 36, y: 0.46 },
  { count: 30, y: 0.62 },
  { count: 24, y: 0.76 },
  { count: 18, y: 0.88 },
  { count: 12, y: 0.96 },
] as const

const LOADER_SHARD_COUNT = 520

function deterministicUnit(index: number, salt: number) {
  const value = Math.sin(index * 12.9898 + salt * 78.233) * 43758.5453

  return value - Math.floor(value)
}

type LoaderAccentTone = 'amber' | 'rose' | 'cyan'
type LoaderShardTone = 'amber' | 'cyan' | 'white'
type LoaderPowerOrbitalKind = 'orbital' | 'edge' | 'field'

function getLoaderGridTone(index: number, ringIndex: number): LoaderAccentTone {
  if (ringIndex === 4 && index % 4 === 0) {
    return 'amber'
  }

  if ((index + ringIndex) % 7 === 0) {
    return 'rose'
  }

  return 'cyan'
}

function getLoaderAccentTone(
  index: number,
  amberModulo: number,
  roseModulo: number
): LoaderAccentTone {
  if (index % amberModulo === 0) {
    return 'amber'
  }

  if (index % roseModulo === 0) {
    return 'rose'
  }

  return 'cyan'
}

function getLoaderShardTone(index: number): LoaderShardTone {
  if (index % 83 === 0) {
    return 'amber'
  }

  if (index % 13 === 0) {
    return 'cyan'
  }

  return 'white'
}

function getLoaderAccentColor(
  tone: LoaderAccentTone,
  colorMode: 'dark' | 'light'
) {
  if (colorMode === 'light') {
    if (tone === 'amber') {
      return 'rgb(161 98 7)'
    }

    if (tone === 'rose') {
      return 'rgb(190 24 93)'
    }

    return 'rgb(8 112 134)'
  }

  if (tone === 'amber') {
    return 'rgb(252 211 77)'
  }

  if (tone === 'rose') {
    return 'rgb(251 113 133)'
  }

  return 'rgb(190 242 255)'
}

function getLoaderShardColor(
  tone: LoaderShardTone,
  colorMode: 'dark' | 'light'
) {
  if (colorMode === 'light') {
    if (tone === 'amber') {
      return 'rgb(146 89 6)'
    }

    if (tone === 'cyan') {
      return 'rgb(8 112 134)'
    }

    return 'rgb(35 72 79)'
  }

  if (tone === 'amber') {
    return 'rgb(255 236 168)'
  }

  if (tone === 'cyan') {
    return 'rgb(204 251 255)'
  }

  return 'rgb(255 255 255)'
}

function getLoaderHaloSideBias(lane: number) {
  if (lane === 7) {
    return -22
  }

  if (lane === 8) {
    return 22
  }

  return 0
}

function getLoaderHaloUpperBias(lane: number) {
  if (lane === 9) {
    return -18
  }

  if (lane === 10) {
    return 20
  }

  return 0
}

function getLoaderFarFieldX(
  seedA: number,
  seedB: number,
  side: number,
  lowerGrid: boolean,
  edge: boolean,
  highWake: boolean
) {
  if (lowerGrid) {
    return -6 + seedA * 112
  }

  if (edge) {
    return 50 + side * (38 + seedB * 52)
  }

  if (highWake) {
    return 10 + seedA * 80
  }

  return 4 + seedA * 92
}

function getLoaderFarFieldY(
  seedA: number,
  seedB: number,
  lowerGrid: boolean,
  edge: boolean,
  highWake: boolean
) {
  if (lowerGrid) {
    return 60 + seedB * 34 + Math.sin(seedA * Math.PI * 2) * 4
  }

  if (edge) {
    return 12 + seedA * 76
  }

  if (highWake) {
    return 6 + seedB * 22
  }

  return 8 + seedB * 78
}

function getLoaderPowerOrbitalX(
  seedA: number,
  angle: number,
  radius: number,
  side: number,
  orbital: boolean,
  sideField: boolean
) {
  if (orbital) {
    return 50 + Math.cos(angle) * radius
  }

  if (sideField) {
    return 50 + side * (38 + seedA * 50)
  }

  return 6 + seedA * 88
}

function getLoaderPowerOrbitalY(
  seedB: number,
  seedE: number,
  angle: number,
  radius: number,
  orbital: boolean,
  sideField: boolean,
  upperField: boolean,
  lowerField: boolean
) {
  if (orbital) {
    return 36 + Math.sin(angle) * radius * (0.42 + seedE * 0.22)
  }

  if (sideField) {
    return 12 + seedB * 76
  }

  if (upperField) {
    return 7 + seedB * 20
  }

  if (lowerField) {
    return 62 + seedB * 31
  }

  return 16 + seedB * 68
}

function getLoaderPowerOrbitalKind(
  orbital: boolean,
  sideField: boolean
): LoaderPowerOrbitalKind {
  if (orbital) {
    return 'orbital'
  }

  if (sideField) {
    return 'edge'
  }

  return 'field'
}

function getLoaderPowerX(
  seedA: number,
  side: number,
  diagonal: boolean,
  edge: boolean
) {
  if (diagonal) {
    return -8 + seedA * 116
  }

  if (edge) {
    return 50 + side * (32 + seedA * 26)
  }

  return 8 + seedA * 86
}

function getLoaderPowerY(
  seedB: number,
  x: number,
  diagonal: boolean,
  edge: boolean
) {
  if (diagonal) {
    return 18 + seedB * 66 + (x - 50) * 0.08
  }

  if (edge) {
    return 13 + seedB * 78
  }

  return 56 + seedB * 34
}

function getLoaderPowerDriftX(
  diagonal: boolean,
  edge: boolean,
  side: number,
  seedC: number
) {
  let driftBase = 7

  if (diagonal) {
    driftBase = 16
  } else if (edge) {
    driftBase = 10
  }

  return driftBase * side + (seedC - 0.5) * 16
}

function getLoaderPowerDriftY(
  diagonal: boolean,
  lowerNet: boolean,
  seedD: number
) {
  if (diagonal) {
    return -5 + seedD * 16
  }

  if (lowerNet) {
    return 5 + seedD * 10
  }

  return (seedD - 0.5) * 12
}

function getLoaderShardBaseX(
  seedA: number,
  seedC: number,
  angle: number,
  shell: number,
  sphereBias: boolean,
  bandBias: boolean,
  tailBias: boolean,
  fieldBias: boolean
) {
  if (sphereBias) {
    return 50 + Math.cos(angle) * (20 + shell * 34) + (seedC - 0.5) * 13
  }

  if (bandBias) {
    return 50 + Math.cos(angle) * (34 + shell * 44) + (seedC - 0.5) * 24
  }

  if (tailBias) {
    return 24 + seedA * 52
  }

  if (fieldBias) {
    return 50 + Math.cos(angle) * (46 + shell * 42) + (seedC - 0.5) * 28
  }

  return 4 + seedA * 96
}

function getLoaderShardBaseY(
  seedB: number,
  seedD: number,
  angle: number,
  shell: number,
  sphereBias: boolean,
  bandBias: boolean,
  tailBias: boolean,
  fieldBias: boolean
) {
  if (sphereBias) {
    return 42 + Math.sin(angle * 1.08) * (16 + shell * 24) + (seedD - 0.5) * 12
  }

  if (bandBias) {
    return 40 + Math.sin(angle * 1.08) * (24 + shell * 30) + (seedD - 0.5) * 18
  }

  if (tailBias) {
    return 58 + seedB * 34
  }

  if (fieldBias) {
    return 42 + Math.sin(angle * 1.08) * (32 + shell * 28) + (seedD - 0.5) * 22
  }

  return 4 + seedB * 92
}

function getLoaderShardSize(
  giant: boolean,
  large: boolean,
  fieldBias: boolean,
  seedC: number
) {
  if (giant) {
    return 10 + seedC * 8
  }

  if (large) {
    return 4 + seedC * 6
  }

  if (fieldBias) {
    return 1.4 + seedC * 3.8
  }

  return 1.8 + seedC * 4.4
}

function getLoaderShardDrift(
  sphereBias: boolean,
  bandBias: boolean,
  fieldBias: boolean,
  shell: number,
  seedD: number
) {
  if (sphereBias) {
    return 6 + shell * 14
  }

  if (bandBias) {
    return 12 + seedD * 20
  }

  if (fieldBias) {
    return 14 + seedD * 24
  }

  return 10 + seedD * 18
}

function getLoaderShardAlpha(
  giant: boolean,
  large: boolean,
  fieldBias: boolean,
  seedD: number
) {
  if (giant) {
    return 0.44
  }

  if (large) {
    return 0.36
  }

  if (fieldBias) {
    return 0.2 + seedD * 0.24
  }

  return 0.24 + seedD * 0.28
}

function createLoaderSphereGridDots() {
  return LOADER_SPHERE_RINGS.flatMap((ring, ringIndex) => {
    const ringRadius = Math.sqrt(1 - ring.y * ring.y)
    const phase = ((ringIndex % 4) * Math.PI) / (ring.count * 2)

    return Array.from({ length: ring.count }, (_, index) => {
      const theta = (index / ring.count) * Math.PI * 2 + phase
      const x = Math.cos(theta) * ringRadius
      const z = Math.sin(theta) * ringRadius
      const depth = (z + 1) / 2
      const spin = ringIndex % 2 === 0 ? 1 : -1
      const tangent = theta + (Math.PI / 2) * spin
      const drift = 0.18 + depth * 0.32
      const jitter = 0.55 + depth * 1.8
      const tone = getLoaderGridTone(index, ringIndex)

      return {
        alpha: 0.12 + depth * 0.22,
        delay: Math.round((ringIndex * 37 + index * 19) % 2650),
        depth,
        endX: `${Math.cos(tangent) * drift * 0.42}vmin`,
        endY: `${Math.sin(tangent) * drift * 0.26}vmin`,
        id: `grid-${ringIndex}-${index}`,
        kind: 'grid' as const,
        midX: `${Math.cos(tangent) * drift}vmin`,
        midY: `${Math.sin(tangent) * drift * 0.52}vmin`,
        size: 0.72 + depth * 0.58 + (Math.abs(ring.y) < 0.12 ? 0.08 : 0),
        spin,
        startX: `${Math.cos(tangent) * drift * -0.62}vmin`,
        startY: `${Math.sin(tangent) * drift * -0.34}vmin`,
        tone,
        x:
          50 +
          x * (36 + depth * 4) +
          (deterministicUnit(index, ringIndex + 12) - 0.5) * jitter,
        y:
          50 +
          ring.y * 37 +
          (deterministicUnit(index, ringIndex + 19) - 0.5) * jitter * 0.72,
      }
    })
  })
}

function createLoaderSphereHaloDots() {
  return Array.from({ length: 240 }, (_, index) => {
    const seedA = Math.abs(deterministicUnit(index, 61))
    const seedB = Math.abs(deterministicUnit(index, 62))
    const seedC = Math.abs(deterministicUnit(index, 63))
    const seedD = Math.abs(deterministicUnit(index, 64))
    const seedE = Math.abs(deterministicUnit(index, 65))
    const lane = index % 10
    const far = lane >= 8
    const angle =
      seedA * Math.PI * 2 + (lane % 3) * 0.18 + Math.sin(index * 0.43) * 0.28
    const radius = far ? 88 + seedB * 78 : 64 + seedB * 46
    const verticalRatio = far ? 0.62 + seedD * 0.24 : 0.48 + seedD * 0.22
    const sideBias = getLoaderHaloSideBias(lane)
    const upperBias = getLoaderHaloUpperBias(lane)
    let x =
      50 + Math.cos(angle) * radius + (seedC - 0.5) * (far ? 20 : 12) + sideBias
    let y =
      47 +
      Math.sin(angle) * radius * verticalRatio +
      (seedE - 0.5) * (far ? 24 : 14) +
      upperBias
    const coreDx = x - 50
    const coreDy = (y - 42) * 1.18
    const coreDistance = Math.hypot(coreDx, coreDy)

    if (coreDistance < (far ? 34 : 24)) {
      const pushAngle = Math.atan2(
        coreDy || Math.sin(angle),
        coreDx || Math.cos(angle)
      )
      const push = (far ? 52 : 36) + seedE * (far ? 32 : 22)
      x = 50 + Math.cos(pushAngle) * push
      y = 42 + (Math.sin(pushAngle) * push) / 1.18
    }

    const drift = far ? 1.25 + seedC * 1.45 : 0.66 + seedC * 1.1
    const tangent = angle + (index % 2 === 0 ? 1 : -1) * (0.78 + seedD * 0.58)

    return {
      alpha: far ? 0.1 + seedC * 0.22 : 0.08 + seedC * 0.18,
      delay: Math.round(seedD * 2350),
      depth: 0.18 + seedB * 0.76,
      endX: `${Math.cos(tangent) * drift * 1.55}vmin`,
      endY: `${Math.sin(tangent) * drift * 0.92}vmin`,
      id: `halo-${index}`,
      kind: far ? ('far' as const) : ('halo' as const),
      midX: `${Math.cos(tangent) * drift * 0.8}vmin`,
      midY: `${Math.sin(tangent) * drift * 0.48}vmin`,
      size: far
        ? 0.52 + seedA * 0.76 + seedB * 0.36
        : 0.5 + seedA * 0.72 + seedB * 0.32,
      spin: index % 2 === 0 ? 1 : -1,
      startX: `${Math.cos(tangent) * drift * -0.72}vmin`,
      startY: `${Math.sin(tangent) * drift * -0.42}vmin`,
      tone: getLoaderAccentTone(index, 29, 17),
      x: Math.max(-18, Math.min(118, x)),
      y: Math.max(-14, Math.min(116, y)),
    }
  })
}

function createLoaderFarFieldDots() {
  return Array.from({ length: 360 }, (_, index) => {
    const seedA = Math.abs(deterministicUnit(index, 71))
    const seedB = Math.abs(deterministicUnit(index, 72))
    const seedC = Math.abs(deterministicUnit(index, 73))
    const seedD = Math.abs(deterministicUnit(index, 74))
    const lane = index % 16
    const side = index % 2 === 0 ? -1 : 1
    const lowerGrid = lane >= 10
    const edge = lane >= 4 && lane < 10
    const highWake = lane === 2 || lane === 3
    const angle = seedA * Math.PI * 2 + lane * 0.18
    const x = getLoaderFarFieldX(seedA, seedB, side, lowerGrid, edge, highWake)
    const y = getLoaderFarFieldY(seedA, seedB, lowerGrid, edge, highWake)
    const drift = lowerGrid ? 1.2 + seedC * 2.3 : 1.8 + seedC * 3.8
    const tangent = angle + side * (0.7 + seedD * 0.46)

    return {
      alpha: lowerGrid ? 0.12 + seedC * 0.22 : 0.1 + seedC * 0.24,
      delay: Math.round(seedD * 2600),
      depth: 0.16 + seedB * 0.78,
      endX: `${Math.cos(tangent) * drift * 2.2}vmin`,
      endY: `${Math.sin(tangent) * drift * (lowerGrid ? 1.25 : 1.55)}vmin`,
      id: `far-${index}`,
      midX: `${Math.cos(tangent) * drift}vmin`,
      midY: `${Math.sin(tangent) * drift * (lowerGrid ? 0.7 : 0.9)}vmin`,
      size: lowerGrid ? 0.42 + seedA * 0.64 : 0.46 + seedA * 0.78,
      spin: side,
      startX: `${Math.cos(tangent) * drift * -1.6}vmin`,
      startY: `${Math.sin(tangent) * drift * -1.1}vmin`,
      tone: getLoaderAccentTone(index, 31, 19),
      x: Math.max(-8, Math.min(108, x)),
      y: Math.max(-8, Math.min(108, y)),
    }
  })
}

function createLoaderPowerOrbitalDots() {
  return Array.from({ length: 260 }, (_, index) => {
    const seedA = Math.abs(deterministicUnit(index, 1))
    const seedB = Math.abs(deterministicUnit(index, 2))
    const seedC = Math.abs(deterministicUnit(index, 3))
    const seedD = Math.abs(deterministicUnit(index, 7))
    const seedE = Math.abs(deterministicUnit(index, 8))
    const angle = seedA * Math.PI * 2
    const zone = index % 12
    const orbital = zone < 3
    const sideField = zone >= 3 && zone < 8
    const upperField = zone === 8 || zone === 9
    const lowerField = zone >= 10
    const radius = 64 + seedB * 82 + (index % 9 === 0 ? 24 : 0)
    const depth = 0.14 + seedC * 0.72
    const driftX =
      (seedD - 0.5) * (orbital ? 7.4 + depth * 6.2 : 14 + depth * 10)
    const driftY =
      (seedE - 0.5) * (orbital ? 4.8 + depth * 4.6 : 10 + depth * 7)
    const side = index % 2 === 0 ? -1 : 1
    let x = getLoaderPowerOrbitalX(
      seedA,
      angle,
      radius,
      side,
      orbital,
      sideField
    )
    let y = getLoaderPowerOrbitalY(
      seedB,
      seedE,
      angle,
      radius,
      orbital,
      sideField,
      upperField,
      lowerField
    )
    const coreDx = x - 50
    const coreDy = (y - 31) * 1.36
    const coreDistance = Math.hypot(coreDx, coreDy)

    if (coreDistance < (orbital ? 20 : 34)) {
      const pushAngle = Math.atan2(
        coreDy || Math.sin(angle),
        coreDx || Math.cos(angle)
      )
      const push = (orbital ? 24 : 42) + seedC * (orbital ? 28 : 38)
      x = 50 + Math.cos(pushAngle) * push
      y = 31 + (Math.sin(pushAngle) * push) / 1.36
    }

    return {
      alpha: (orbital ? 0.14 : 0.18) + depth * (orbital ? 0.3 : 0.36),
      delay: Math.round(Math.abs(deterministicUnit(index, 4)) * 1800),
      depth,
      id: `orbital-${index}`,
      kind: getLoaderPowerOrbitalKind(orbital, sideField),
      midX: `${driftX}vmin`,
      midY: `${driftY}vmin`,
      size:
        0.72 +
        Math.abs(deterministicUnit(index, 5)) * (orbital ? 1.42 : 1.86) +
        depth * 0.46,
      spin: index % 2 === 0 ? 1 : -1,
      startX: `${driftX * -0.54}vmin`,
      startY: `${driftY * -0.46}vmin`,
      tone: getLoaderAccentTone(index, 17, 11),
      x,
      y,
    }
  })
}

function createLoaderPowerDots() {
  return Array.from({ length: 380 }, (_, index) => {
    const seedA = Math.abs(deterministicUnit(index, 41))
    const seedB = Math.abs(deterministicUnit(index, 42))
    const seedC = Math.abs(deterministicUnit(index, 43))
    const seedD = Math.abs(deterministicUnit(index, 44))
    const lane = index % 10
    const side = index % 2 === 0 ? -1 : 1
    const diagonal = lane < 4
    const edge = lane >= 4 && lane < 7
    const lowerNet = lane >= 7
    let x = getLoaderPowerX(seedA, side, diagonal, edge)
    let y = getLoaderPowerY(seedB, x, diagonal, edge)
    const centerDx = x - 50
    const centerDy = (y - 38) * 1.18
    const centerDistance = Math.hypot(centerDx, centerDy)

    if (!lowerNet && centerDistance < 28) {
      const pushAngle = Math.atan2(
        centerDy || seedB - 0.5,
        centerDx || seedA - 0.5
      )
      const push = 34 + seedC * 32
      x = 50 + Math.cos(pushAngle) * push
      y = 38 + (Math.sin(pushAngle) * push) / 1.18
    }
    const driftX = getLoaderPowerDriftX(diagonal, edge, side, seedC)
    const driftY = getLoaderPowerDriftY(diagonal, lowerNet, seedD)

    return {
      alpha: (lowerNet ? 0.18 : 0.22) + seedC * 0.34,
      delay: Math.round(seedD * 2200),
      driftX: `${driftX}vmin`,
      driftY: `${driftY}vmin`,
      id: `power-${index}`,
      size: 0.72 + seedC * (lowerNet ? 1.35 : 1.9),
      tone: getLoaderAccentTone(index, 31, 17),
      x,
      y,
    }
  })
}

function createLoaderSphereShards() {
  return Array.from({ length: LOADER_SHARD_COUNT }, (_, index) => {
    const seedA = deterministicUnit(index, 21)
    const seedB = deterministicUnit(index, 22)
    const seedC = deterministicUnit(index, 23)
    const seedD = deterministicUnit(index, 24)
    const group = index % 100
    const angle = seedA * Math.PI * 2 + Math.sin(index * 0.7) * 0.34
    const shell = 0.2 + Math.pow(seedB, 0.58) * 0.82
    const sphereBias = group < 18
    const bandBias = group >= 18 && group < 48
    const tailBias = group >= 48 && group < 72
    const fieldBias = group >= 72
    const giant = index % 137 === 0
    const large = giant || index % 31 === 0
    let baseX = getLoaderShardBaseX(
      seedA,
      seedC,
      angle,
      shell,
      sphereBias,
      bandBias,
      tailBias,
      fieldBias
    )
    let baseY = getLoaderShardBaseY(
      seedB,
      seedD,
      angle,
      shell,
      sphereBias,
      bandBias,
      tailBias,
      fieldBias
    )
    const centerDx = baseX - 50
    const centerDy = (baseY - 42) * 1.22
    const centerDistance = Math.hypot(centerDx, centerDy)

    if (!sphereBias && centerDistance < (bandBias ? 24 : 30)) {
      const pushAngle = Math.atan2(
        centerDy || Math.sin(angle),
        centerDx || Math.cos(angle)
      )
      const push = (bandBias ? 30 : 36) + seedD * (bandBias ? 18 : 26)
      baseX = 50 + Math.cos(pushAngle) * push
      baseY = 42 + (Math.sin(pushAngle) * push) / 1.22
    }

    const size = getLoaderShardSize(giant, large, fieldBias, seedC)
    const drift = getLoaderShardDrift(
      sphereBias,
      bandBias,
      fieldBias,
      shell,
      seedD
    )
    const driftAngle = angle + (index % 2 === 0 ? 1 : -1) * (0.82 + seedD * 0.9)

    return {
      alpha: getLoaderShardAlpha(giant, large, fieldBias, seedD),
      delay: Math.round(seedD * 2100),
      driftX: `${Math.cos(driftAngle) * drift}px`,
      driftY: `${Math.sin(driftAngle) * drift * 0.64}px`,
      id: `shard-${index}`,
      midDriftX: `${Math.cos(driftAngle) * drift * 0.28}px`,
      midDriftY: `${Math.sin(driftAngle) * drift * 0.14}px`,
      large,
      midRotate: `${Math.round(seedA * 360 + 18 * (index % 2 === 0 ? 1 : -1))}deg`,
      rotate: `${Math.round(seedA * 360)}deg`,
      size,
      spin: index % 2 === 0 ? 1 : -1,
      tailRotate: `${Math.round(seedA * 360 + 44 * (index % 2 === 0 ? 1 : -1))}deg`,
      tone: getLoaderShardTone(index),
      x: Math.max(-4, Math.min(104, baseX)),
      y: Math.max(-4, Math.min(106, baseY)),
    }
  })
}

function createLoaderParticleVisuals() {
  return {
    farFieldDots: createLoaderFarFieldDots().filter(
      (_, index) => index % 14 === 0
    ),
    powerDots: createLoaderPowerDots().filter((_, index) => index % 16 === 0),
    powerOrbitalDots: createLoaderPowerOrbitalDots().filter(
      (_, index) => index % 16 === 0
    ),
  }
}

function createLoaderSphereVisuals() {
  const sphereDots = [
    ...createLoaderSphereGridDots().filter((_, index) => index % 4 === 0),
    ...createLoaderSphereHaloDots(),
  ]

  return {
    sphereDots: sphereDots.filter((_, index) => index % 10 === 0),
    sphereShards: createLoaderSphereShards().filter(
      (_, index) => index % 16 === 0
    ),
  }
}

export function YucoreEntranceLoader(props: YucoreEntranceLoaderProps) {
  const { t } = useYucoreTranslation()
  const { resolvedTheme } = useTheme()
  const [mounted, setMounted] = useState(true)
  const [sequenceStage, setSequenceStage] = useState(0)
  const [particleVisuals, setParticleVisuals] = useState<ReturnType<
    typeof createLoaderParticleVisuals
  > | null>(null)
  const [sphereVisuals, setSphereVisuals] = useState<ReturnType<
    typeof createLoaderSphereVisuals
  > | null>(null)
  const particleVisualsRef = useRef<ReturnType<
    typeof createLoaderParticleVisuals
  > | null>(null)
  const sphereVisualsRef = useRef<ReturnType<
    typeof createLoaderSphereVisuals
  > | null>(null)
  const onCompleteRef = useRef(props.onComplete)
  const durationMs =
    props.durationMs && props.durationMs > 0
      ? props.durationMs
      : YUCORE_BOOT_LOADER_DURATION_MS

  onCompleteRef.current = props.onComplete

  useEffect(() => {
    setMounted(true)
    setSequenceStage(0)
    setParticleVisuals(null)
    setSphereVisuals(null)
    particleVisualsRef.current = null
    sphereVisualsRef.current = null

    const timers = [
      window.setTimeout(() => {
        particleVisualsRef.current = createLoaderParticleVisuals()
      }, durationMs * LOADER_PARTICLE_PREPARE_RATIO),
      window.setTimeout(() => {
        setParticleVisuals(
          particleVisualsRef.current ?? createLoaderParticleVisuals()
        )
        setSequenceStage(1)
      }, durationMs * LOADER_PARTICLE_PREWARM_RATIO),
      window.setTimeout(() => {
        setSequenceStage(2)
      }, durationMs * LOADER_PARTICLE_ENTER_RATIO),
      window.setTimeout(() => {
        sphereVisualsRef.current = createLoaderSphereVisuals()
      }, durationMs * LOADER_SPHERE_PREPARE_RATIO),
      window.setTimeout(() => {
        setSphereVisuals(
          sphereVisualsRef.current ?? createLoaderSphereVisuals()
        )
        setSequenceStage(3)
      }, durationMs * LOADER_SPHERE_PREWARM_RATIO),
      window.setTimeout(() => {
        setSequenceStage(4)
      }, durationMs * LOADER_HANDOFF_RATIO),
      window.setTimeout(() => {
        setSequenceStage(5)
        setParticleVisuals(null)
        particleVisualsRef.current = null
      }, durationMs * LOADER_PARTICLE_RELEASE_RATIO),
      window.setTimeout(() => {
        setMounted(false)
        onCompleteRef.current?.()
      }, durationMs + 180),
    ]

    return () => {
      timers.forEach((timer) => window.clearTimeout(timer))
      particleVisualsRef.current = null
      sphereVisualsRef.current = null
    }
  }, [durationMs])

  if (!mounted) return null

  const particleEffectsMounted = particleVisuals && sequenceStage < 5
  const sphereEffectsMounted = sphereVisuals && sequenceStage >= 3
  const particleStageDurationMs =
    durationMs * (LOADER_PARTICLE_RELEASE_RATIO - LOADER_PARTICLE_PREWARM_RATIO)
  const sphereStageDurationMs = durationMs * (1 - LOADER_SPHERE_PREWARM_RATIO)
  const particleEnterDelayMs =
    durationMs * (LOADER_PARTICLE_ENTER_RATIO - LOADER_PARTICLE_PREWARM_RATIO)
  const particleExitDelayMs =
    durationMs * (LOADER_HANDOFF_RATIO - LOADER_PARTICLE_PREWARM_RATIO)
  const sphereEnterDelayMs =
    durationMs * (LOADER_HANDOFF_RATIO - LOADER_SPHERE_PREWARM_RATIO)

  return (
    <div
      className={cn(
        'yucore-entrance-loader yucore-entrance-loader-lite fixed inset-0 z-[100] overflow-hidden',
        resolvedTheme === 'light'
          ? 'bg-[#f8fcfb] text-[#183b44]'
          : 'bg-[#010203] text-white',
        props.className
      )}
      style={
        {
          '--yucore-loader-duration': `${durationMs}ms`,
        } as CSSProperties
      }
      aria-live='polite'
      aria-label={t('YuCore AI is connecting to the model core')}
      data-theme={resolvedTheme}
      data-sequence-stage={sequenceStage}
    >
      <YucoreBootCanvas colorMode={resolvedTheme} durationMs={durationMs} />
      {particleEffectsMounted ? (
        <div
          className='yucore-loader-sequence-particle-layer absolute inset-0 z-[1]'
          style={
            {
              '--yucore-loader-duration': `${particleStageDurationMs}ms`,
              '--yucore-loader-layer-enter-delay': `${particleEnterDelayMs}ms`,
              '--yucore-loader-layer-exit-delay': `${particleExitDelayMs}ms`,
              '--yucore-loader-layer-transition': `${LOADER_LAYER_TRANSITION_MS}ms`,
            } as CSSProperties
          }
          data-yucore-loader-layer='particles-webgl'
          aria-hidden='true'
        >
          <YucoreSignalFieldWebgl
            active
            coreMode='ambient'
            corePlacement='hero'
            colorMode={resolvedTheme}
            intensity='hero'
            renderProfile='entrance'
            className={cn(
              'opacity-[0.52]',
              resolvedTheme === 'light'
                ? 'mix-blend-multiply'
                : 'mix-blend-screen'
            )}
          />
        </div>
      ) : null}
      <div className='yucore-loader-atmosphere absolute inset-0' />
      <div className='yucore-loader-noise absolute inset-0' />
      <div className='yucore-loader-vignette absolute inset-0' />
      <div className='yucore-loader-scan absolute inset-0' />
      <div
        className='yucore-loader-shutter absolute inset-0'
        aria-hidden='true'
      />
      {particleEffectsMounted ? (
        <div
          className='yucore-loader-sequence-particle-layer absolute inset-0 z-[5]'
          style={
            {
              '--yucore-loader-duration': `${particleStageDurationMs}ms`,
              '--yucore-loader-layer-enter-delay': `${particleEnterDelayMs}ms`,
              '--yucore-loader-layer-exit-delay': `${particleExitDelayMs}ms`,
              '--yucore-loader-layer-transition': `${LOADER_LAYER_TRANSITION_MS}ms`,
            } as CSSProperties
          }
          data-yucore-loader-layer='particles-dom'
          aria-hidden='true'
        >
          <div className='yucore-loader-power-field absolute inset-0'>
            {particleVisuals.powerDots.map((dot) => (
              <span
                key={dot.id}
                className='yucore-loader-power-dot absolute rounded-full'
                style={
                  {
                    '--power-alpha': dot.alpha,
                    '--power-delay': `${dot.delay}ms`,
                    '--power-drift-x': dot.driftX,
                    '--power-drift-y': dot.driftY,
                    '--power-size': `${dot.size}px`,
                    '--power-x': `${dot.x}%`,
                    '--power-y': `${dot.y}%`,
                    color: getLoaderAccentColor(dot.tone, resolvedTheme),
                  } as CSSProperties
                }
              />
            ))}
            {particleVisuals.powerOrbitalDots.map((dot) => (
              <span
                key={dot.id}
                className={cn(
                  'yucore-loader-power-dot yucore-loader-power-dot-orbital absolute rounded-full',
                  dot.kind === 'edge' && 'yucore-loader-power-dot-edge',
                  dot.kind === 'field' && 'yucore-loader-power-dot-field'
                )}
                style={
                  {
                    '--power-alpha': dot.alpha,
                    '--power-delay': `${dot.delay}ms`,
                    '--power-drift-x': dot.midX,
                    '--power-drift-y': dot.midY,
                    '--power-size': `${dot.size}px`,
                    '--power-x': `${dot.x}%`,
                    '--power-y': `${dot.y}%`,
                    color: getLoaderAccentColor(dot.tone, resolvedTheme),
                  } as CSSProperties
                }
              />
            ))}
          </div>
          <div className='yucore-loader-far-particle-field absolute inset-0'>
            {particleVisuals.farFieldDots.map((dot) => (
              <span
                key={dot.id}
                className='yucore-loader-far-dot absolute rounded-full'
                style={
                  {
                    '--dot-alpha': dot.alpha,
                    '--dot-depth': dot.depth,
                    '--dot-delay': `${dot.delay}ms`,
                    '--dot-end-x': dot.endX,
                    '--dot-end-y': dot.endY,
                    '--dot-mid-x': dot.midX,
                    '--dot-mid-y': dot.midY,
                    '--dot-size': `${dot.size}px`,
                    '--dot-spin': dot.spin,
                    '--dot-start-x': dot.startX,
                    '--dot-start-y': dot.startY,
                    '--dot-x': `${dot.x}%`,
                    '--dot-y': `${dot.y}%`,
                    color: getLoaderAccentColor(dot.tone, resolvedTheme),
                    backgroundColor: getLoaderAccentColor(
                      dot.tone,
                      resolvedTheme
                    ),
                  } as CSSProperties
                }
              />
            ))}
          </div>
        </div>
      ) : null}
      <div className='yucore-loader-system-code absolute inset-x-0 top-[9%] z-[7] flex justify-center px-5 text-center'>
        <div className='inline-flex max-w-[min(88vw,38rem)] items-center gap-3 overflow-hidden rounded-full border border-cyan-100/10 bg-black/26 px-4 py-2 text-[0.66rem] font-semibold tracking-[0.28em] text-cyan-50/52 uppercase backdrop-blur-md'>
          <span className='size-1.5 shrink-0 rounded-full bg-cyan-200 shadow-[0_0_16px_rgba(103,232,249,0.78)]' />
          <span className='truncate'>
            {t('YUCORE BOOT / MODEL ROUTE FABRIC / CORE WAKE')}
          </span>
        </div>
      </div>

      <div className='yucore-loader-init absolute top-[40%] left-1/2 flex flex-col items-center px-6 text-center'>
        <div className='yucore-loader-orbit relative mb-6 flex size-24 items-center justify-center rounded-full border border-cyan-200/25'>
          <span className='absolute size-40 rounded-full border border-amber-300/10' />
          <span className='absolute size-28 rounded-full border border-cyan-200/10' />
          <span className='absolute size-16 rounded-full border border-rose-300/10' />
          <span className='yucore-loader-core size-7 rounded-full bg-cyan-200 shadow-[0_0_34px_rgba(103,232,249,0.88)]' />
        </div>
        <div className='text-sm font-semibold tracking-tight text-cyan-50/80 sm:text-base'>
          {t('YuCore AI is connecting to the model power grid')}
        </div>
        <div className='mt-2 text-xs tracking-[0.18em] text-white/38 uppercase'>
          {t('Booting model routing fabric')}
        </div>
      </div>

      {sphereEffectsMounted ? (
        <div
          className='yucore-loader-sequence-sphere-layer absolute inset-0 z-[6]'
          style={
            {
              '--yucore-loader-duration': `${sphereStageDurationMs}ms`,
              '--yucore-loader-layer-enter-delay': `${sphereEnterDelayMs}ms`,
              '--yucore-loader-layer-transition': `${LOADER_LAYER_TRANSITION_MS}ms`,
            } as CSSProperties
          }
          data-yucore-loader-layer='sphere'
          aria-hidden='true'
        >
          <div className='yucore-loader-sphere absolute left-1/2'>
            <span className='yucore-loader-sphere-ring yucore-loader-sphere-ring-a absolute rounded-full' />
            <span className='yucore-loader-sphere-ring yucore-loader-sphere-ring-b absolute rounded-full' />
            <span className='yucore-loader-sphere-core absolute'>
              <span className='yucore-loader-earth-ocean absolute inset-0 rounded-full' />
              <span className='yucore-loader-earth-land yucore-loader-earth-land-a absolute inset-[-4%]' />
              <span className='yucore-loader-earth-land yucore-loader-earth-land-b absolute inset-[-4%]' />
              <YucoreWebglEarth
                active
                className='yucore-loader-earth-webgl'
                colorMode={resolvedTheme}
                density='loader'
              />
              <span className='yucore-loader-earth-clouds absolute inset-[-3%] rounded-full' />
              <span className='yucore-loader-earth-grid absolute inset-0 rounded-full' />
              <span className='yucore-loader-earth-night absolute inset-0 rounded-full' />
              <span className='yucore-loader-earth-rim absolute inset-0 rounded-full' />
            </span>
            <span className='yucore-loader-sphere-shard-field absolute inset-[-18%]'>
              {sphereVisuals.sphereShards.map((shard) => (
                <span
                  key={shard.id}
                  className={cn(
                    'yucore-loader-sphere-shard absolute',
                    shard.large && 'yucore-loader-sphere-shard-large'
                  )}
                  style={
                    {
                      '--shard-alpha': shard.alpha,
                      '--shard-delay': `${shard.delay}ms`,
                      '--shard-drift-x': shard.driftX,
                      '--shard-drift-y': shard.driftY,
                      '--shard-mid-drift-x': shard.midDriftX,
                      '--shard-mid-drift-y': shard.midDriftY,
                      '--shard-mid-rotate': shard.midRotate,
                      '--shard-rotate': shard.rotate,
                      '--shard-size': `${shard.size}px`,
                      '--shard-spin': shard.spin,
                      '--shard-tail-rotate': shard.tailRotate,
                      '--shard-x': `${shard.x}%`,
                      '--shard-y': `${shard.y}%`,
                      color: getLoaderShardColor(shard.tone, resolvedTheme),
                    } as CSSProperties
                  }
                />
              ))}
            </span>
            <span className='yucore-loader-sphere-wordmark absolute'>
              YuCore
            </span>
            <span className='yucore-loader-sphere-particle-field absolute inset-0'>
              {sphereVisuals.sphereDots.map((dot) => (
                <span
                  key={dot.id}
                  className={cn(
                    'yucore-loader-sphere-dot absolute rounded-full',
                    dot.kind === 'halo' && 'yucore-loader-sphere-dot-halo',
                    dot.kind === 'far' && 'yucore-loader-sphere-dot-far'
                  )}
                  style={
                    {
                      '--dot-alpha': dot.alpha,
                      '--dot-depth': dot.depth,
                      '--dot-delay': `${dot.delay}ms`,
                      '--dot-delay-negative': `${-dot.delay}ms`,
                      '--dot-end-x': dot.endX,
                      '--dot-end-y': dot.endY,
                      '--dot-mid-x': dot.midX,
                      '--dot-mid-y': dot.midY,
                      '--dot-size': `${dot.size}px`,
                      '--dot-spin': dot.spin,
                      '--dot-start-x': dot.startX,
                      '--dot-start-y': dot.startY,
                      '--dot-x': `${dot.x}%`,
                      '--dot-y': `${dot.y}%`,
                      color: getLoaderAccentColor(dot.tone, resolvedTheme),
                      backgroundColor: getLoaderAccentColor(
                        dot.tone,
                        resolvedTheme
                      ),
                    } as CSSProperties
                  }
                />
              ))}
            </span>
          </div>
        </div>
      ) : null}

      <div className='yucore-loader-cinematic-copy absolute inset-x-0 top-[70%] flex flex-col items-center px-5 text-center'>
        <div className='yucore-loader-stage-label mb-4 text-[0.68rem] font-semibold tracking-[0.34em] text-cyan-100/64 uppercase'>
          {t('Neural gateway handshake')}
        </div>
        <div
          className='yucore-loader-title yucore-loader-title-cinematic text-[clamp(2.75rem,8vw,7.2rem)] leading-none font-semibold tracking-tight'
          data-text={t('YuCore AI Core')}
        >
          {t('YuCore AI Core')}
        </div>
        <div className='yucore-loader-resolve-copy mt-5 max-w-2xl text-sm leading-6 font-semibold text-cyan-50/72 sm:text-base'>
          {t(
            'Gateway, media studio, billing, and quota intelligence are synchronizing.'
          )}
        </div>
        <div className='yucore-loader-resolve-copy mt-2 text-xs tracking-[0.22em] text-amber-100/44 uppercase'>
          {t('Routes online / creative engines armed / quota core synced')}
        </div>
        <div className='yucore-loader-line mt-7 h-px w-72 max-w-[72vw] overflow-hidden rounded-full bg-white/10'>
          <span className='block h-full w-1/2 rounded-full bg-gradient-to-r from-transparent via-cyan-200 to-amber-200' />
        </div>
      </div>
    </div>
  )
}
