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

  const constrainedCpu =
    capabilities.hardwareConcurrency !== undefined &&
    capabilities.hardwareConcurrency <= 4
  const constrainedMemory =
    capabilities.deviceMemory !== undefined && capabilities.deviceMemory <= 4
  const denseSmallScreen =
    capabilities.viewportWidth < 720 && capabilities.devicePixelRatio >= 2.5
  if (constrainedCpu || constrainedMemory || denseSmallScreen) return 'reduced'

  const capableDesktop =
    capabilities.viewportWidth >= 1024 &&
    capabilities.devicePixelRatio <= 2 &&
    capabilities.hardwareConcurrency !== undefined &&
    capabilities.hardwareConcurrency >= 8 &&
    capabilities.deviceMemory !== undefined &&
    capabilities.deviceMemory >= 8
  return capableDesktop ? 'full' : 'balanced'
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
