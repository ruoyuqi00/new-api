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
export type YucoreMotionProfile = 'full' | 'balanced' | 'reduced'

export type YucoreMotionCapabilities = {
  deviceMemory?: number
  devicePixelRatio: number
  hardwareConcurrency?: number
  reducedMotion: boolean
  viewportWidth: number
}

export type YucoreGraphicsBackend =
  | 'hardware'
  | 'software'
  | 'unknown'
  | 'unavailable'

export type YucoreMotionBudget = Readonly<{
  bootParticleCount: number
  bootShardCount: number
  bootSpherePointCount: number
  bootTargetFps: number
  earthLoaderTargetFps: number
  earthPersistentTargetFps: number
  maxPixelRatio: number
  signalParticleCount: number
  signalRouteSegments: number
  signalTargetFps: number
}>

const motionBudgets: Record<YucoreMotionProfile, YucoreMotionBudget> = {
  reduced: Object.freeze({
    bootParticleCount: 520,
    bootShardCount: 120,
    bootSpherePointCount: 200,
    bootTargetFps: 20,
    earthLoaderTargetFps: 20,
    earthPersistentTargetFps: 16,
    maxPixelRatio: 1,
    signalParticleCount: 300,
    signalRouteSegments: 42,
    signalTargetFps: 24,
  }),
  balanced: Object.freeze({
    bootParticleCount: 820,
    bootShardCount: 180,
    bootSpherePointCount: 280,
    bootTargetFps: 30,
    earthLoaderTargetFps: 28,
    earthPersistentTargetFps: 24,
    maxPixelRatio: 1,
    signalParticleCount: 460,
    signalRouteSegments: 54,
    signalTargetFps: 36,
  }),
  full: Object.freeze({
    bootParticleCount: 1080,
    bootShardCount: 240,
    bootSpherePointCount: 360,
    bootTargetFps: 40,
    earthLoaderTargetFps: 32,
    earthPersistentTargetFps: 28,
    maxPixelRatio: 1.1,
    signalParticleCount: 620,
    signalRouteSegments: 64,
    signalTargetFps: 45,
  }),
}

export function resolveYucoreMotionProfile(
  capabilities: YucoreMotionCapabilities
): YucoreMotionProfile {
  if (capabilities.reducedMotion) return 'reduced'

  // Hardware capability is decided from the actual WebGL renderer. CPU and
  // memory hints alone cannot distinguish a real GPU from a software rasterizer.
  return 'full'
}

export function getYucoreMotionBudget(
  profile: YucoreMotionProfile
): YucoreMotionBudget {
  return motionBudgets[profile]
}

export function readYucoreMotionProfile(host: Window = window) {
  const navigatorWithMemory = host.navigator as Navigator & {
    deviceMemory?: number
  }

  return resolveYucoreMotionProfile({
    deviceMemory: navigatorWithMemory.deviceMemory,
    devicePixelRatio: host.devicePixelRatio || 1,
    hardwareConcurrency: host.navigator.hardwareConcurrency,
    reducedMotion: host.matchMedia('(prefers-reduced-motion: reduce)').matches,
    viewportWidth: host.innerWidth,
  })
}

const SOFTWARE_RENDERER_PATTERNS = [
  /swiftshader/i,
  /llvmpipe/i,
  /softpipe/i,
  /software\s+(?:renderer|rasterizer)/i,
  /microsoft\s+basic\s+render/i,
  /basic\s+render\s+driver/i,
]

export function classifyYucoreGraphicsBackend(
  webglAvailable: boolean,
  renderer?: string
): YucoreGraphicsBackend {
  if (!webglAvailable) return 'unavailable'

  const normalizedRenderer = renderer?.trim()
  if (!normalizedRenderer) return 'unknown'
  if (
    SOFTWARE_RENDERER_PATTERNS.some((pattern) =>
      pattern.test(normalizedRenderer)
    )
  ) {
    return 'software'
  }
  return 'hardware'
}

export function shouldUseYucoreDynamicGraphics(backend: YucoreGraphicsBackend) {
  return backend === 'hardware' || backend === 'unknown'
}

let cachedGraphicsBackend: YucoreGraphicsBackend | undefined

export function readYucoreGraphicsBackend(
  host?: Window
): YucoreGraphicsBackend {
  const target = host ?? (typeof window === 'undefined' ? undefined : window)
  if (!target) return 'unknown'
  if (!host && cachedGraphicsBackend) return cachedGraphicsBackend

  const canvas = target.document.createElement('canvas')
  let gl: WebGLRenderingContext | null = null
  try {
    gl = canvas.getContext('webgl', {
      alpha: true,
      failIfMajorPerformanceCaveat: false,
    })
  } catch {
    gl = null
  }
  if (!gl) {
    if (!host) cachedGraphicsBackend = 'unavailable'
    return 'unavailable'
  }

  const debugInfo = gl.getExtension('WEBGL_debug_renderer_info')
  const renderer = debugInfo
    ? gl.getParameter(debugInfo.UNMASKED_RENDERER_WEBGL)
    : gl.getParameter(gl.RENDERER)
  const backend = classifyYucoreGraphicsBackend(true, String(renderer ?? ''))
  if (!host) cachedGraphicsBackend = backend
  return backend
}
