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
import {
  getYucoreMotionBudget,
  readYucoreMotionProfile,
  type YucoreMotionBudget,
} from './yucore-motion-performance'

type BootShard = {
  angle: number
  depth: number
  jitter: number
  lane: number
  size: number
  spin: number
  tone: number
}

type SpherePoint = {
  phase: number
  size: number
  tone: number
  x: number
  y: number
  z: number
}

type BootParticle = {
  angle: number
  depthSeed: number
  fieldSeedX: number
  fieldSeedY: number
  fieldSide: number
  farField: boolean
  glow: boolean
  index: number
  lane: number
  lowerGrid: boolean
  noisePhase: number
  shellWeight: number
  sideField: boolean
  spin: number
  tone: number
}

const DEFAULT_BOOT_DURATION_MS = 5200
const BOOT_SPHERE_CENTER_Y_RATIO = 0.4
const BOOT_SPHERE_RADIUS_START = 0.2
const BOOT_SPHERE_RADIUS_END = 0.25

function clamp(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, value))
}

function smoothstep(edge0: number, edge1: number, value: number) {
  const next = clamp((value - edge0) / (edge1 - edge0), 0, 1)
  return next * next * (3 - 2 * next)
}

function easeOutCubic(value: number) {
  return 1 - Math.pow(1 - clamp(value, 0, 1), 3)
}

function areFiniteNumbers(...values: number[]) {
  return values.every((value) => Number.isFinite(value))
}

function getBootSphereGeometry(width: number, height: number, build = 1) {
  return {
    centerX: width * 0.5,
    centerY: height * BOOT_SPHERE_CENTER_Y_RATIO,
    radius:
      Math.min(width, height) *
      (BOOT_SPHERE_RADIUS_START +
        (BOOT_SPHERE_RADIUS_END - BOOT_SPHERE_RADIUS_START) * build),
  }
}

function getBootTone(
  index: number,
  primaryModulo: number,
  secondaryModulo: number
) {
  if (index % primaryModulo === 0) return 2
  if (index % secondaryModulo === 0) return 1
  return 0
}

function getBootShellWeight(
  index: number,
  seed: number,
  farField: boolean,
  sideField: boolean,
  lowerGrid: boolean
) {
  if (farField) return 0.13 + (index % 3) * 0.028
  if (sideField) return 0.22 + seed * 0.1
  if (lowerGrid) return 0.1 + seed * 0.06
  if (index % 9 === 0) return 0.38
  if (index % 4 === 0) return 0.48
  return 0.56
}

function getBootDensityStride(progress: number) {
  if (progress < 0.22) return 5
  if (progress < 0.38) return 3
  if (progress < 0.58) return 2
  return 1
}

function getBootBaseRadius(
  farField: boolean,
  lowerGrid: boolean,
  lane: number,
  seed: number,
  time: number
) {
  if (farField) return 0.32 + lane * 0.52 + seed * 0.16
  if (lowerGrid) return 0.36 + seed * 0.48
  return 0.22 + lane * 0.45 + Math.sin(seed * 12 + time) * 0.016
}

function getBootFieldX(
  width: number,
  radialX: number,
  farField: boolean,
  lowerGrid: boolean,
  fieldSide: number,
  seed: number,
  fieldSeedX: number
) {
  if (farField) return width * (0.5 + fieldSide * (0.34 + seed * 0.18))
  if (lowerGrid) return width * (-0.08 + fieldSeedX * 1.16)
  return radialX
}

function getBootFieldY(
  height: number,
  radialY: number,
  farField: boolean,
  lowerGrid: boolean,
  fieldSeedY: number
) {
  if (farField) return height * (0.14 + fieldSeedY * 0.76)
  if (lowerGrid) return height * (0.64 + fieldSeedY * 0.28)
  return radialY
}

function getBootFieldBlend(
  farField: boolean,
  lowerGrid: boolean,
  sideField: boolean,
  corePull: number
) {
  let base = 0.1
  if (farField) {
    base = 0.7 + (1 - corePull) * 0.18
  } else if (lowerGrid) {
    base = 0.74 + (1 - corePull) * 0.14
  } else if (sideField) {
    base = 0.34 + (1 - corePull) * 0.18
  }

  let corePullWeight = 0.72
  if (farField) {
    corePullWeight = 0.58
  } else if (lowerGrid) {
    corePullWeight = 0.34
  }

  return base * (1 - corePull * corePullWeight)
}

function getBootOrderedLift(
  lateOrder: number,
  farField: boolean,
  lowerGrid: boolean,
  sideField: boolean
) {
  if (farField) return lateOrder * 0.26
  if (lowerGrid) return lateOrder * 0.2
  if (sideField) return lateOrder * 0.08
  return 0
}

function getBootParticleDepthAlphaScale(farField: boolean, lowerGrid: boolean) {
  if (farField) return 0.27
  if (lowerGrid) return 0.24
  return 0.16
}

function getBootParticleSizeScale(farField: boolean, lowerGrid: boolean) {
  if (farField) return 0.92
  if (lowerGrid) return 0.72
  return 0.92
}

function createBootShards(count: number) {
  return Array.from({ length: count }, (_, index): BootShard => {
    const lane = (index % 9) / 8
    const seed = ((index * 47) % 113) / 113

    return {
      angle: ((index * 137.508 + seed * 36) * Math.PI) / 180,
      depth: ((index * 29) % 101) / 101,
      jitter: seed * Math.PI * 2,
      lane,
      size: 3.2 + ((index * 17) % 11),
      spin: index % 2 === 0 ? 1 : -1,
      tone: getBootTone(index, 11, 5),
    }
  })
}

function createSpherePoints(count: number) {
  const goldenAngle = Math.PI * (3 - Math.sqrt(5))

  return Array.from({ length: count }, (_, index): SpherePoint => {
    const y = 1 - (index / Math.max(1, count - 1)) * 2
    const radius = Math.sqrt(1 - y * y)
    const theta = index * goldenAngle

    return {
      phase: index * 0.071,
      size: 0.9 + ((index * 13) % 7) * 0.18,
      tone: getBootTone(index, 13, 5),
      x: Math.cos(theta) * radius,
      y,
      z: Math.sin(theta) * radius,
    }
  })
}

function createBootParticles(count: number) {
  return Array.from({ length: count }, (_, index): BootParticle => {
    const seed = ((index * 37) % 997) / 997
    const lane = (index % 13) / 12
    const farField = index % 13 < 8
    const sideField = index % 13 === 8 || index % 13 === 9 || index % 13 === 10
    const lowerGrid = index % 13 >= 10
    const shellWeight = getBootShellWeight(
      index,
      seed,
      farField,
      sideField,
      lowerGrid
    )

    return {
      angle: (index * 137.508 * Math.PI) / 180 + Math.sin(index * 0.17) * 0.12,
      depthSeed: seed,
      fieldSeedX: ((index * 79) % 113) / 113,
      fieldSeedY: ((index * 61) % 107) / 107,
      fieldSide: index % 2 === 0 ? -1 : 1,
      farField,
      glow: index % 5 === 0 || index % 17 === 0,
      index,
      lane,
      lowerGrid,
      noisePhase: seed * 16 + index * 0.031,
      shellWeight,
      sideField,
      spin: index % 2 === 0 ? 1 : -1,
      tone: getBootTone(index, 17, 7),
    }
  })
}

function toneColor(tone: number, alpha: number) {
  if (tone === 2) return `rgba(251, 191, 36, ${alpha})`
  if (tone === 1) return `rgba(244, 114, 182, ${alpha})`
  return `rgba(191, 246, 255, ${alpha})`
}

function drawBackdrop(
  ctx: CanvasRenderingContext2D,
  width: number,
  height: number,
  progress: number,
  time: number
) {
  const glow = 0.28 + smoothstep(0.34, 0.68, progress) * 0.46
  const maxSide = Math.max(width, height)

  ctx.save()
  ctx.fillStyle = '#010203'
  ctx.fillRect(0, 0, width, height)

  const sweep = ctx.createLinearGradient(0, 0, width, height)
  sweep.addColorStop(0, 'rgba(0, 7, 12, 0.96)')
  sweep.addColorStop(0.42, 'rgba(2, 4, 7, 1)')
  sweep.addColorStop(0.72, 'rgba(12, 11, 6, 0.98)')
  sweep.addColorStop(1, 'rgba(4, 2, 7, 1)')
  ctx.fillStyle = sweep
  ctx.fillRect(0, 0, width, height)

  const field = ctx.createRadialGradient(
    width * 0.5,
    height * 0.42,
    0,
    width * 0.5,
    height * 0.44,
    maxSide * 0.62
  )
  field.addColorStop(0, `rgba(103, 232, 249, ${0.08 + glow * 0.15})`)
  field.addColorStop(0.24, `rgba(147, 51, 234, ${0.04 + glow * 0.07})`)
  field.addColorStop(0.46, `rgba(250, 204, 21, ${0.025 + glow * 0.04})`)
  field.addColorStop(1, 'rgba(0, 0, 0, 0)')
  ctx.fillStyle = field
  ctx.fillRect(0, 0, width, height)

  const aperture = ctx.createRadialGradient(
    width * 0.5,
    height * 0.48,
    maxSide * 0.14,
    width * 0.5,
    height * 0.48,
    maxSide * 0.72
  )
  aperture.addColorStop(0, 'rgba(0, 0, 0, 0)')
  aperture.addColorStop(0.56, 'rgba(0, 0, 0, 0.18)')
  aperture.addColorStop(1, 'rgba(0, 0, 0, 0.82)')
  ctx.fillStyle = aperture
  ctx.fillRect(0, 0, width, height)

  ctx.globalCompositeOperation = 'screen'
  ctx.globalAlpha = 0.024 + Math.sin(time * 2.2) * 0.006
  ctx.fillStyle = 'rgba(255, 255, 255, 0.65)'
  for (let i = 0; i < 22; i += 1) {
    const x = ((i * 97 + Math.floor(time * 22) * 13) % 997) / 997
    const y = ((i * 53 + Math.floor(time * 24) * 17) % 983) / 983
    const w = 1 + ((i * 19) % 2)
    ctx.fillRect(x * width, y * height, w, 1)
  }
  ctx.restore()
}

function drawBrandParticleField(
  ctx: CanvasRenderingContext2D,
  width: number,
  height: number,
  progress: number,
  time: number,
  particles: BootParticle[],
  performanceStride: number
) {
  const visible =
    smoothstep(0.16, 0.22, progress) * (1 - smoothstep(0.66, 0.8, progress))

  if (visible < 0.01) return

  const sphere = getBootSphereGeometry(width, height)
  const centerX = sphere.centerX
  const centerY = sphere.centerY
  const globeRadius = sphere.radius
  const maxSide = Math.max(width, height)
  const corePull = smoothstep(0.46, 0.76, progress)
  const scatter =
    smoothstep(0.22, 0.52, progress) * (1 - smoothstep(0.62, 0.78, progress))
  const densityStride = Math.max(
    getBootDensityStride(progress),
    performanceStride
  )
  const lateOrder = smoothstep(0.54, 0.78, progress)

  ctx.save()
  ctx.globalCompositeOperation = 'screen'

  for (
    let particleIndex = 0;
    particleIndex < particles.length;
    particleIndex += densityStride
  ) {
    const particle = particles[particleIndex]
    const seed = particle.depthSeed
    const lane = particle.lane
    const tone = particle.tone
    const spin = particle.spin
    const farField = particle.farField
    const sideField = particle.sideField
    const lowerGrid = particle.lowerGrid
    const index = particle.index
    const angle =
      particle.angle +
      time * spin * (0.16 + lane * 0.16) +
      Math.sin(time * 0.6 + particle.noisePhase) * (0.16 - lateOrder * 0.08)
    const baseRadius =
      maxSide * getBootBaseRadius(farField, lowerGrid, lane, seed, time)
    const burstRadius = baseRadius * (1 + scatter * (0.45 + seed * 0.72))
    const shellWeight = particle.shellWeight
    const shellPull = corePull * shellWeight
    const sphereParticleRadius =
      Math.min(width, height) *
      (0.2 + seed * 0.24 + (index % 11 === 0 ? 0.12 : 0))
    const radius =
      burstRadius * (1 - shellPull) + sphereParticleRadius * shellPull
    const ellipse = 0.54 + lane * 0.16
    const orbitalNoise =
      Math.sin(time * 2.4 + particle.noisePhase) *
      maxSide *
      (0.006 - lateOrder * 0.002)
    const radialX =
      centerX +
      Math.cos(angle) * radius +
      Math.cos(angle * 3 + seed * 5) * orbitalNoise
    const radialY =
      centerY +
      Math.sin(angle) * radius * ellipse +
      Math.sin(angle * 2.2 + seed * 4) * orbitalNoise
    const fieldSide = particle.fieldSide
    const fieldX = getBootFieldX(
      width,
      radialX,
      farField,
      lowerGrid,
      fieldSide,
      seed,
      particle.fieldSeedX
    )
    const fieldY = getBootFieldY(
      height,
      radialY,
      farField,
      lowerGrid,
      particle.fieldSeedY
    )
    const fieldBlend = getBootFieldBlend(
      farField,
      lowerGrid,
      sideField,
      corePull
    )
    const x = radialX * (1 - fieldBlend) + fieldX * fieldBlend
    const y = radialY * (1 - fieldBlend) + fieldY * fieldBlend
    const depth = 0.32 + Math.sin(angle + seed * 3) * 0.24 + lane * 0.42
    const distanceToCore = Math.hypot(x - centerX, y - centerY)
    const centerRelief = smoothstep(0.54, 0.88, progress)
    const coreClear = smoothstep(
      Math.min(width, height) * (0.2 + centerRelief * 0.06),
      Math.min(width, height) * (0.38 + centerRelief * 0.12),
      distanceToCore
    )
    const coreAlphaFloor = 0.18 - smoothstep(0.5, 0.82, progress) * 0.1
    const globeRelief =
      1 -
      lateOrder *
        (1 - smoothstep(globeRadius * 0.76, globeRadius * 1.34, distanceToCore))
    const orderedLift = getBootOrderedLift(
      lateOrder,
      farField,
      lowerGrid,
      sideField
    )
    const alpha =
      visible *
      (0.062 +
        depth * getBootParticleDepthAlphaScale(farField, lowerGrid) +
        shellPull * 0.02 +
        orderedLift) *
      (coreAlphaFloor + coreClear * (1 - coreAlphaFloor)) *
      (0.36 + globeRelief * 0.64)
    const size =
      (0.34 +
        depth * getBootParticleSizeScale(farField, lowerGrid) +
        shellPull * 0.18) *
      1.28

    if (!areFiniteNumbers(x, y, alpha, size)) continue

    if (index % 31 === 0 || (farField && index % 23 === 0)) {
      if (!areFiniteNumbers(centerX, centerY, x, y)) continue
      const tailRatio = 0.16 + seed * 0.18
      const tailX = farField
        ? x - Math.cos(angle + fieldSide * 0.8) * maxSide * (0.08 + seed * 0.08)
        : centerX + (x - centerX) * tailRatio
      const tailY = farField
        ? y -
          Math.sin(angle + fieldSide * 0.8) * maxSide * (0.035 + seed * 0.04)
        : centerY + (y - centerY) * tailRatio
      const trail = ctx.createLinearGradient(tailX, tailY, x, y)
      trail.addColorStop(0, 'rgba(103, 232, 249, 0)')
      trail.addColorStop(0.72, toneColor(tone, alpha * 0.22))
      trail.addColorStop(1, toneColor(tone, alpha * 0.48))
      ctx.strokeStyle = trail
      ctx.lineWidth = 0.45 + depth * 0.76
      ctx.beginPath()
      ctx.moveTo(tailX, tailY)
      ctx.lineTo(x, y)
      ctx.stroke()
    }

    ctx.fillStyle = toneColor(tone, Math.min(0.92, alpha * 1.4))
    ctx.fillRect(x, y, size, size)
    if (particle.glow) {
      ctx.fillStyle = `rgba(255, 255, 255, ${Math.min(0.8, alpha * 0.7)})`
      ctx.fillRect(
        x + size * 0.18,
        y + size * 0.18,
        Math.max(0.72, size * 0.38),
        Math.max(0.72, size * 0.38)
      )
    }
  }

  const coreGlow = ctx.createRadialGradient(
    centerX,
    centerY,
    0,
    centerX,
    centerY,
    Math.min(width, height) * (0.18 + corePull * 0.08)
  )
  coreGlow.addColorStop(
    0,
    `rgba(255, 255, 255, ${visible * (0.045 + corePull * 0.1)})`
  )
  coreGlow.addColorStop(
    0.32,
    `rgba(103, 232, 249, ${visible * (0.04 + corePull * 0.08)})`
  )
  coreGlow.addColorStop(1, 'rgba(103, 232, 249, 0)')
  ctx.shadowBlur = 0
  ctx.fillStyle = coreGlow
  ctx.fillRect(0, 0, width, height)

  ctx.restore()
}

function drawSequencedPowerLanes(
  ctx: CanvasRenderingContext2D,
  width: number,
  height: number,
  progress: number,
  time: number
) {
  const enter = smoothstep(0.72, 0.82, progress)
  const hold = 1 - smoothstep(1.02, 1.12, progress)
  const alpha = enter * hold

  if (alpha < 0.01) return

  const { centerX, centerY, radius } = getBootSphereGeometry(
    width,
    height,
    smoothstep(0.44, 0.64, progress)
  )
  const minSide = Math.min(width, height)
  const lowerAlpha = alpha * smoothstep(0.62, 0.88, progress)

  ctx.save()
  ctx.globalCompositeOperation = 'screen'
  ctx.lineCap = 'round'

  for (let lane = 0; lane < 8; lane += 1) {
    const direction = lane % 2 === 0 ? 1 : -1
    const laneRadius = radius * (1.04 + lane * 0.13)
    const laneRatio = 0.26 + (lane % 4) * 0.038
    const rotation =
      -0.5 + lane * 0.2 + time * direction * (0.045 + lane * 0.006)
    const laneAlpha = alpha * (0.12 + lane * 0.022)

    ctx.save()
    ctx.translate(centerX, centerY)
    ctx.rotate(rotation)
    ctx.setLineDash(
      lane % 2 === 0
        ? [laneRadius * 0.16, laneRadius * 0.09]
        : [laneRadius * 0.07, laneRadius * 0.13]
    )
    ctx.lineDashOffset = -time * direction * (18 + lane * 2)
    ctx.strokeStyle =
      lane % 3 === 0
        ? `rgba(250, 204, 21, ${laneAlpha * 0.78})`
        : `rgba(125, 249, 255, ${laneAlpha})`
    ctx.lineWidth = lane % 3 === 0 ? 1.02 : 0.82
    ctx.beginPath()
    ctx.ellipse(
      0,
      0,
      laneRadius * (1.22 + lane * 0.015),
      laneRadius * laneRatio,
      0,
      0,
      Math.PI * 2
    )
    ctx.stroke()
    ctx.setLineDash([])

    for (let packet = 0; packet < 4; packet += 1) {
      const packetAngle =
        time * direction * (0.58 + lane * 0.045) +
        packet * (Math.PI / 2) +
        lane * 0.4
      const packetDepth = 0.5 + Math.sin(packetAngle) * 0.5
      const px = Math.cos(packetAngle) * laneRadius * (1.22 + lane * 0.015)
      const py = Math.sin(packetAngle) * laneRadius * laneRatio
      ctx.fillStyle =
        lane % 3 === 0
          ? `rgba(255, 241, 180, ${laneAlpha * (1.3 + packetDepth)})`
          : `rgba(225, 252, 255, ${laneAlpha * (1.1 + packetDepth)})`
      ctx.beginPath()
      ctx.arc(px, py, 0.9 + packetDepth * 1.7, 0, Math.PI * 2)
      ctx.fill()
    }
    ctx.restore()
  }

  for (let lane = 0; lane < 7; lane += 1) {
    const y = height * (0.58 + lane * 0.046)
    const slope = (lane - 3) * minSide * 0.018
    const offset = ((time * (34 + lane * 5)) % (width * 0.34)) - width * 0.17
    const startX = width * -0.04 + lane * minSide * 0.018 + offset * 0.12
    const endX = width * 1.04 - lane * minSide * 0.014 + offset * 0.18
    const lineAlpha = lowerAlpha * (0.06 + lane * 0.012)
    const gradient = ctx.createLinearGradient(startX, y, endX, y + slope)
    gradient.addColorStop(0, 'rgba(103, 232, 249, 0)')
    gradient.addColorStop(0.24, `rgba(103, 232, 249, ${lineAlpha})`)
    gradient.addColorStop(0.68, `rgba(250, 204, 21, ${lineAlpha * 0.7})`)
    gradient.addColorStop(1, 'rgba(103, 232, 249, 0)')
    ctx.strokeStyle = gradient
    ctx.lineWidth = lane % 3 === 0 ? 1.1 : 0.72
    ctx.beginPath()
    ctx.moveTo(startX, y)
    ctx.bezierCurveTo(
      width * 0.28,
      y - minSide * 0.06,
      width * 0.62,
      y + slope + minSide * 0.05,
      endX,
      y + slope
    )
    ctx.stroke()

    for (let packet = 0; packet < 3; packet += 1) {
      const t =
        (((time * (0.055 + lane * 0.004) + packet / 3 + lane * 0.07) % 1) + 1) %
        1
      const px = startX + (endX - startX) * t
      const py = y + slope * t + Math.sin(t * Math.PI) * minSide * 0.025
      ctx.fillStyle =
        packet === 1
          ? `rgba(255, 241, 180, ${lowerAlpha * 0.38})`
          : `rgba(225, 252, 255, ${lowerAlpha * 0.34})`
      ctx.beginPath()
      ctx.arc(px, py, 1.1 + lane * 0.08, 0, Math.PI * 2)
      ctx.fill()
    }
  }

  ctx.restore()
}

function drawWireFrame(
  ctx: CanvasRenderingContext2D,
  width: number,
  height: number,
  progress: number,
  time: number
) {
  const enter = smoothstep(0.02, 0.12, progress)
  const exit = 1 - smoothstep(0.3, 0.42, progress)
  const alpha = enter * exit

  if (alpha < 0.01) return

  const size = Math.min(width, height) * (0.16 + Math.sin(time * 2.5) * 0.012)
  const { centerX, centerY } = getBootSphereGeometry(width, height)

  ctx.save()
  ctx.translate(centerX, centerY)
  ctx.rotate(-0.34 + time * 0.62)
  ctx.globalCompositeOperation = 'screen'
  ctx.lineWidth = 1.2

  const drawFrame = () => {
    for (let i = 0; i < 4; i += 1) {
      const grow = i * size * 0.09
      ctx.strokeRect(
        -size / 2 - grow,
        -size / 2 - grow,
        size + grow * 2,
        size + grow * 2
      )
    }
    ctx.beginPath()
    ctx.moveTo(-size * 0.64, 0)
    ctx.lineTo(size * 0.64, 0)
    ctx.moveTo(0, -size * 0.64)
    ctx.lineTo(0, size * 0.64)
    ctx.stroke()
  }

  ctx.save()
  ctx.translate(-2.8, 0)
  ctx.strokeStyle = `rgba(34, 211, 238, ${alpha * 0.56})`
  drawFrame()
  ctx.restore()

  ctx.save()
  ctx.translate(2.8, 0)
  ctx.strokeStyle = `rgba(251, 113, 133, ${alpha * 0.42})`
  drawFrame()
  ctx.restore()

  ctx.strokeStyle = `rgba(255, 255, 255, ${alpha * 0.76})`
  ctx.setLineDash([size * 0.18, size * 0.08])
  drawFrame()
  ctx.restore()
}

function drawTriangle(ctx: CanvasRenderingContext2D, size: number) {
  ctx.beginPath()
  ctx.moveTo(0, -size * 0.58)
  ctx.lineTo(size * 0.5, size * 0.5)
  ctx.lineTo(-size * 0.46, size * 0.38)
  ctx.closePath()
}

function drawShardTunnel(
  ctx: CanvasRenderingContext2D,
  width: number,
  height: number,
  progress: number,
  time: number,
  shards: BootShard[],
  performanceStride: number
) {
  const enter = smoothstep(0.24, 0.34, progress)
  const exit = 1 - smoothstep(0.64, 0.78, progress)
  const burst = enter * exit

  if (burst < 0.01) return

  const { centerX, centerY } = getBootSphereGeometry(width, height)
  const maxSide = Math.max(width, height)
  const minSide = Math.min(width, height)
  const fly = easeOutCubic(smoothstep(0.24, 0.7, progress))

  ctx.save()
  ctx.globalCompositeOperation = 'screen'

  for (let index = 0; index < shards.length; index += performanceStride) {
    const shard = shards[index]
    const depth = (shard.depth - fly * 1.35 + 1) % 1
    const perspective = Math.pow(1 - depth, 1.82)
    const angle =
      shard.angle +
      time * 0.34 * shard.spin +
      burst * shard.spin * (0.4 + shard.lane * 0.66)
    const radius =
      maxSide *
      (0.055 + Math.pow(depth, 0.78) * (0.22 + shard.lane * 0.3) + fly * 0.06)
    const vibration =
      Math.sin(time * 3.1 + shard.jitter) * maxSide * 0.012 * burst
    const x =
      centerX + Math.cos(angle) * radius + Math.cos(angle * 2.6) * vibration
    const y =
      centerY +
      Math.sin(angle) * radius * (0.55 + shard.lane * 0.14) +
      Math.sin(angle * 3.2) * vibration
    const size = shard.size * (0.36 + perspective * 1.42 + burst * 0.24)
    const distanceToCore = Math.hypot(x - centerX, y - centerY)
    const coreClear = smoothstep(minSide * 0.26, minSide * 0.5, distanceToCore)
    const alpha = burst * (0.1 + perspective * 0.68) * (0.36 + coreClear * 0.64)

    if (!areFiniteNumbers(x, y, size, alpha)) continue

    if (index % 9 === 0) {
      const lineAlpha = alpha * 0.16
      if (!areFiniteNumbers(centerX, centerY, x, y, lineAlpha)) continue
      const startRatio = 0.18 + shard.depth * 0.16
      const startX = centerX + (x - centerX) * startRatio
      const startY = centerY + (y - centerY) * startRatio
      const gradient = ctx.createLinearGradient(startX, startY, x, y)
      gradient.addColorStop(0, 'rgba(103, 232, 249, 0)')
      gradient.addColorStop(0.68, `rgba(103, 232, 249, ${lineAlpha})`)
      gradient.addColorStop(1, `rgba(255, 255, 255, ${lineAlpha * 0.8})`)
      ctx.strokeStyle = gradient
      ctx.lineWidth = 0.8 + perspective * 1.6
      ctx.beginPath()
      ctx.moveTo(startX, startY)
      ctx.lineTo(x, y)
      ctx.stroke()
    }

    ctx.save()
    ctx.translate(x, y)
    ctx.rotate(angle + time * shard.spin * 1.7)

    ctx.translate(-2.6, 0)
    drawTriangle(ctx, size)
    ctx.fillStyle = `rgba(34, 211, 238, ${alpha * 0.42})`
    ctx.fill()

    ctx.translate(5.2, 0)
    drawTriangle(ctx, size)
    ctx.fillStyle = `rgba(251, 113, 133, ${alpha * 0.32})`
    ctx.fill()

    ctx.translate(-2.6, 0)
    drawTriangle(ctx, size * 0.86)
    ctx.fillStyle = toneColor(shard.tone, alpha * 0.74)
    ctx.fill()
    ctx.restore()
  }

  const vortexAlpha = burst * (0.16 + fly * 0.22)
  ctx.lineCap = 'round'
  for (let lane = 0; lane < 6; lane += 1) {
    const direction = lane % 2 === 0 ? 1 : -1
    const laneRadius = minSide * (0.09 + lane * 0.034 + fly * 0.028)
    const laneRatio = 0.28 + lane * 0.032
    const laneRotation =
      -0.5 + lane * 0.24 + time * direction * (0.18 + lane * 0.025)
    const laneAlpha = vortexAlpha * (0.42 + lane * 0.08)

    ctx.save()
    ctx.translate(centerX, centerY)
    ctx.rotate(laneRotation)
    ctx.setLineDash(
      lane % 2 === 0
        ? [laneRadius * 0.18, laneRadius * 0.09]
        : [laneRadius * 0.08, laneRadius * 0.12]
    )
    ctx.lineDashOffset = time * direction * 22
    ctx.strokeStyle =
      lane % 3 === 0
        ? `rgba(250, 204, 21, ${laneAlpha * 0.68})`
        : `rgba(191, 246, 255, ${laneAlpha})`
    ctx.lineWidth = lane % 3 === 0 ? 1.05 : 0.78
    ctx.beginPath()
    ctx.ellipse(
      0,
      0,
      laneRadius * 1.34,
      laneRadius * laneRatio,
      0,
      0,
      Math.PI * 2
    )
    ctx.stroke()
    ctx.setLineDash([])

    for (let packet = 0; packet < 3; packet += 1) {
      const packetAngle =
        time * direction * (0.66 + lane * 0.04) + packet * 2.09 + lane
      const depth = 0.5 + Math.sin(packetAngle) * 0.5
      const px = Math.cos(packetAngle) * laneRadius * 1.34
      const py = Math.sin(packetAngle) * laneRadius * laneRatio
      ctx.fillStyle =
        lane % 3 === 0
          ? `rgba(255, 236, 168, ${laneAlpha * (0.8 + depth)})`
          : `rgba(236, 254, 255, ${laneAlpha * (0.78 + depth)})`
      ctx.beginPath()
      ctx.arc(px, py, 0.9 + depth * 1.8, 0, Math.PI * 2)
      ctx.fill()
    }
    ctx.restore()
  }

  ctx.restore()
}

function drawSphere(
  ctx: CanvasRenderingContext2D,
  width: number,
  height: number,
  progress: number,
  time: number,
  spherePoints: SpherePoint[],
  performanceStride: number
) {
  const build = smoothstep(0.64, 0.78, progress)
  const fade = 1 - smoothstep(0.985, 1.08, progress) * 0.18
  const alpha = build * fade

  if (alpha < 0.01) return

  const { centerX, centerY, radius } = getBootSphereGeometry(
    width,
    height,
    build
  )
  const rotateY = time * 0.42 + build * 0.58
  const rotateX = -0.28 + Math.sin(time * 0.56) * 0.08
  const cosY = Math.cos(rotateY)
  const sinY = Math.sin(rotateY)
  const cosX = Math.cos(rotateX)
  const sinX = Math.sin(rotateX)

  ctx.save()

  ctx.globalCompositeOperation = 'screen'
  const atmosphere = ctx.createRadialGradient(
    centerX - radius * 0.22,
    centerY - radius * 0.28,
    radius * 0.08,
    centerX,
    centerY,
    radius * 1.22
  )
  atmosphere.addColorStop(0, `rgba(255, 255, 255, ${alpha * 0.05})`)
  atmosphere.addColorStop(0.28, `rgba(103, 232, 249, ${alpha * 0.04})`)
  atmosphere.addColorStop(0.72, `rgba(250, 204, 21, ${alpha * 0.018})`)
  atmosphere.addColorStop(1, 'rgba(103, 232, 249, 0)')
  ctx.fillStyle = atmosphere
  ctx.fillRect(
    centerX - radius * 1.3,
    centerY - radius * 1.3,
    radius * 2.6,
    radius * 2.6
  )

  const ringAlpha = alpha * 0.32
  ctx.lineWidth = 1
  ctx.strokeStyle = `rgba(103, 232, 249, ${ringAlpha})`
  ctx.beginPath()
  ctx.ellipse(
    centerX,
    centerY,
    radius * 1.14,
    radius * 0.48,
    time * 0.22,
    0,
    Math.PI * 2
  )
  ctx.stroke()
  ctx.strokeStyle = `rgba(250, 204, 21, ${ringAlpha * 0.7})`
  ctx.beginPath()
  ctx.ellipse(
    centerX,
    centerY,
    radius * 0.94,
    radius * 0.58,
    -time * 0.18,
    0,
    Math.PI * 2
  )
  ctx.stroke()

  for (let lane = 0; lane < 5; lane += 1) {
    const direction = lane % 2 === 0 ? 1 : -1
    const laneAlpha = alpha * (0.18 + lane * 0.028)
    const laneRadius = radius * (0.78 + lane * 0.11)
    const laneRatio = 0.26 + lane * 0.045
    const rotation = -0.42 + lane * 0.3 + time * direction * 0.08
    ctx.save()
    ctx.translate(centerX, centerY)
    ctx.rotate(rotation)
    ctx.setLineDash(
      lane % 2 === 0
        ? [laneRadius * 0.18, laneRadius * 0.08]
        : [laneRadius * 0.08, laneRadius * 0.12]
    )
    ctx.lineDashOffset = time * direction * 24
    ctx.strokeStyle =
      lane % 3 === 0
        ? `rgba(250, 204, 21, ${laneAlpha})`
        : `rgba(191, 246, 255, ${laneAlpha})`
    ctx.lineWidth = lane % 3 === 0 ? 1.18 : 0.82
    ctx.beginPath()
    ctx.ellipse(
      0,
      0,
      laneRadius * 1.24,
      laneRadius * laneRatio,
      0,
      0,
      Math.PI * 2
    )
    ctx.stroke()
    ctx.setLineDash([])
    for (let packet = 0; packet < 3; packet += 1) {
      const packetAngle =
        time * direction * (0.64 + lane * 0.06) + packet * 2.09 + lane * 0.5
      const px = Math.cos(packetAngle) * laneRadius * 1.24
      const py = Math.sin(packetAngle) * laneRadius * laneRatio
      const depth = 0.5 + Math.sin(packetAngle) * 0.5
      ctx.fillStyle =
        lane % 3 === 0
          ? `rgba(255, 236, 168, ${laneAlpha * (1.2 + depth)})`
          : `rgba(236, 254, 255, ${laneAlpha * (1.2 + depth)})`
      ctx.beginPath()
      ctx.arc(px, py, 1.2 + depth * 1.8, 0, Math.PI * 2)
      ctx.fill()
    }
    ctx.restore()
  }

  for (
    let pointIndex = 0;
    pointIndex < spherePoints.length;
    pointIndex += performanceStride
  ) {
    const point = spherePoints[pointIndex]
    const rotatedX = point.x * cosY + point.z * sinY
    const zAfterY = -point.x * sinY + point.z * cosY
    const rotatedY = point.y * cosX - zAfterY * sinX
    const rotatedZ = point.y * sinX + zAfterY * cosX
    const depth = (rotatedZ + 1) / 2
    const surfaceNoise =
      1 + Math.sin(time * 2.1 + point.phase + depth * 3) * 0.035
    const px =
      centerX + rotatedX * radius * surfaceNoise * (0.92 + depth * 0.18)
    const py = centerY + rotatedY * radius * surfaceNoise
    const dotAlpha = alpha * (0.18 + depth * 0.46)
    const dotSize = point.size * (0.48 + depth * 0.82) * (0.52 + build * 0.38)

    ctx.fillStyle = toneColor(point.tone, dotAlpha * 1.35)
    const renderedSize = Math.max(0.7, dotSize * 0.72)
    ctx.fillRect(
      px - renderedSize * 0.5,
      py - renderedSize * 0.5,
      renderedSize,
      renderedSize
    )
  }

  const core = ctx.createRadialGradient(
    centerX,
    centerY,
    0,
    centerX,
    centerY,
    radius * 1.1
  )
  core.addColorStop(0, `rgba(255, 255, 255, ${alpha * 0.12})`)
  core.addColorStop(0.42, `rgba(103, 232, 249, ${alpha * 0.05})`)
  core.addColorStop(1, 'rgba(103, 232, 249, 0)')
  ctx.fillStyle = core
  ctx.fillRect(
    centerX - radius * 1.18,
    centerY - radius * 1.18,
    radius * 2.36,
    radius * 2.36
  )
  ctx.restore()
}

function drawBootCore(
  ctx: CanvasRenderingContext2D,
  width: number,
  height: number,
  progress: number,
  time: number
) {
  const intro = 1 - smoothstep(0.3, 0.44, progress)

  if (intro < 0.01) return

  const sphere = getBootSphereGeometry(width, height)
  const centerX = sphere.centerX
  const lift = smoothstep(0.12, 0.3, progress)
  const centerY = height * 0.47 * (1 - lift) + sphere.centerY * lift
  const pulse = 0.72 + Math.sin(time * 5.4) * 0.28

  ctx.save()
  ctx.globalCompositeOperation = 'screen'
  ctx.strokeStyle = `rgba(103, 232, 249, ${intro * 0.48})`
  ctx.lineWidth = 1.3
  ctx.beginPath()
  ctx.arc(
    centerX,
    centerY,
    34 + pulse * 8,
    time * 1.6,
    time * 1.6 + Math.PI * 1.38
  )
  ctx.stroke()
  ctx.strokeStyle = `rgba(250, 204, 21, ${intro * 0.26})`
  ctx.beginPath()
  ctx.arc(centerX, centerY, 54 + pulse * 9, -time * 1.1, -time * 1.1 + Math.PI)
  ctx.stroke()

  const core = ctx.createRadialGradient(
    centerX,
    centerY,
    0,
    centerX,
    centerY,
    72
  )
  core.addColorStop(0, `rgba(255, 255, 255, ${intro * 0.72})`)
  core.addColorStop(0.16, `rgba(103, 232, 249, ${intro * 0.46})`)
  core.addColorStop(0.58, `rgba(167, 139, 250, ${intro * 0.12})`)
  core.addColorStop(1, 'rgba(103, 232, 249, 0)')
  ctx.fillStyle = core
  ctx.fillRect(centerX - 78, centerY - 78, 156, 156)
  ctx.restore()
}

function drawBootFrame(
  ctx: CanvasRenderingContext2D,
  width: number,
  height: number,
  progress: number,
  time: number,
  particles: BootParticle[],
  shards: BootShard[],
  spherePoints: SpherePoint[],
  performanceStride: number
) {
  if (
    !areFiniteNumbers(width, height, progress, time) ||
    width <= 0 ||
    height <= 0
  ) {
    return
  }

  drawBackdrop(ctx, width, height, progress, time)
  drawBrandParticleField(
    ctx,
    width,
    height,
    progress,
    time,
    particles,
    performanceStride
  )
  drawSequencedPowerLanes(ctx, width, height, progress, time)
  drawWireFrame(ctx, width, height, progress, time)
  drawShardTunnel(ctx, width, height, progress, time, shards, performanceStride)
  drawSphere(
    ctx,
    width,
    height,
    progress,
    time,
    spherePoints,
    performanceStride
  )
  drawBootCore(ctx, width, height, progress, time)
}

export type BootScene = {
  particles: BootParticle[]
  shards: BootShard[]
  spherePoints: SpherePoint[]
}

export function createBootScene(budget: YucoreMotionBudget): BootScene {
  return {
    particles: createBootParticles(budget.bootParticleCount),
    shards: createBootShards(budget.bootShardCount),
    spherePoints: createSpherePoints(budget.bootSpherePointCount),
  }
}

export function renderBootScene(
  ctx: CanvasRenderingContext2D,
  width: number,
  height: number,
  elapsed: number,
  durationMs: number,
  scene: BootScene
) {
  drawBootFrame(
    ctx,
    width,
    height,
    clamp(elapsed / durationMs, 0, 1),
    elapsed / 1000,
    scene.particles,
    scene.shards,
    scene.spherePoints,
    8
  )
}

export function startBootCanvasRenderer(
  canvas: HTMLCanvasElement,
  durationMs = DEFAULT_BOOT_DURATION_MS,
  budget = getYucoreMotionBudget(readYucoreMotionProfile())
) {
  const ctx = canvas.getContext('2d', {
    alpha: false,
    desynchronized: true,
  })
  if (!ctx) return

  const scene = createBootScene(budget)
  let width = 1
  let height = 1
  let animationFrame = 0
  let lastRenderTime = Number.NEGATIVE_INFINITY
  const reduceMotion = window.matchMedia(
    '(prefers-reduced-motion: reduce)'
  ).matches
  const startedAt = performance.now()

  const resize = () => {
    const rect = canvas.getBoundingClientRect()
    const dpr = Math.min(window.devicePixelRatio || 1, budget.maxPixelRatio)
    width = Math.max(1, Number.isFinite(rect.width) ? rect.width : 1)
    height = Math.max(1, Number.isFinite(rect.height) ? rect.height : 1)
    canvas.width = Math.floor(width * dpr)
    canvas.height = Math.floor(height * dpr)
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
  }

  const render = (now: number) => {
    animationFrame = 0
    if (document.hidden) return

    const frameIntervalMs = 1000 / budget.bootTargetFps
    if (!reduceMotion && now - lastRenderTime < frameIntervalMs - 0.75) {
      animationFrame = window.requestAnimationFrame(render)
      return
    }
    lastRenderTime = now
    const elapsed = reduceMotion
      ? durationMs
      : Math.min(now - startedAt, durationMs)
    renderBootScene(ctx, width, height, elapsed, durationMs, scene)

    if (!reduceMotion && elapsed < durationMs + 120) {
      animationFrame = window.requestAnimationFrame(render)
    }
  }

  const handleVisibilityChange = () => {
    if (!document.hidden && animationFrame === 0) {
      animationFrame = window.requestAnimationFrame(render)
    }
  }

  resize()
  const resizeObserver = new ResizeObserver(resize)
  resizeObserver.observe(canvas)
  document.addEventListener('visibilitychange', handleVisibilityChange)
  animationFrame = window.requestAnimationFrame(render)

  return () => {
    window.cancelAnimationFrame(animationFrame)
    resizeObserver.disconnect()
    document.removeEventListener('visibilitychange', handleVisibilityChange)
  }
}
