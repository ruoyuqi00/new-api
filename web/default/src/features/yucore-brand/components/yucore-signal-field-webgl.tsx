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
import { useEffect, useRef } from 'react'

import { cn } from '@/lib/utils'

import { createYucoreRenderLoop } from './yucore-render-loop'
import { getSignalFieldResourceKey } from './yucore-renderer-resource-key'

interface YucoreSignalFieldWebglProps {
  active?: boolean
  className?: string
  colorMode?: 'dark' | 'light'
  coreMode?: 'full' | 'ambient'
  corePlacement?: 'auth' | 'hero' | 'intro'
  intensity?: 'calm' | 'hero' | 'workbench'
  renderProfile?: 'default' | 'console' | 'entrance'
}

type SignalScene = {
  coreCount: number
  coreStart: number
  lineCount: number
  lineStart: number
  packetCount: number
  packetStart: number
  particleCount: number
  particleStart: number
  vertices: Float32Array
}

const FLOATS_PER_VERTEX = 8

const VERTEX_SHADER = `
attribute vec2 a_position;
attribute float a_size;
attribute float a_tone;
attribute float a_depth;
attribute float a_speed;
attribute float a_phase;
attribute float a_kind;

uniform vec2 u_resolution;
uniform float u_render_scale;
uniform float u_time;
uniform float u_intensity;
uniform float u_reveal;
uniform vec2 u_pointer;
uniform float u_pointer_active;
uniform float u_core_strength;
uniform vec2 u_core_position;

varying float v_alpha;
varying float v_kind;
varying float v_tone;

float routeY(float x, float route) {
  if (route < 0.5) {
    return 0.32 + sin(x * 7.2 + 0.4 + u_time * 0.16) * 0.048;
  }
  if (route < 1.5) {
    return 0.61 + sin(x * 5.4 + 2.1 + u_time * 0.13) * 0.068;
  }
  if (route < 2.5) {
    return 0.76 + sin(x * 9.6 + 4.6 + u_time * 0.11) * 0.036;
  }
  return 0.46 + sin(x * 6.4 + 3.3 + u_time * 0.14) * 0.055;
}

void main() {
  vec2 position = a_position;
  float alpha = 1.0;
  float pointSize = a_size;

  if (a_kind < 0.5) {
    position.x += sin(u_time * a_speed + a_phase) * (0.006 + a_depth * 0.012);
    position.y += cos(u_time * a_speed * 0.82 + a_phase) * (0.004 + a_depth * 0.009);
    if (u_pointer_active > 0.5) {
      vec2 pointerClip = vec2(u_pointer.x * 2.0 - 1.0, 1.0 - u_pointer.y * 2.0);
      vec2 delta = position - pointerClip;
      float distanceToPointer = max(0.08, length(delta));
      position += normalize(delta) * max(0.0, 0.22 - distanceToPointer) * 0.05;
    }
    alpha = (0.28 + a_depth * 0.62) * (0.64 + sin(u_time * (0.8 + a_speed) + a_phase) * 0.36);
    pointSize *= 0.72 + a_depth * 0.64;
  } else if (a_kind < 1.5) {
    alpha = 0.12 + 0.08 * sin(u_time * a_speed + a_phase);
  } else if (a_kind < 2.5) {
    float x = fract(u_time * a_speed + a_phase);
    float y = routeY(x, a_depth);
    position = vec2(x * 2.0 - 1.0, 1.0 - y * 2.0);
    alpha = 0.72 + 0.28 * sin(u_time * 2.0 + a_phase * 6.2831);
    pointSize *= 1.2;
  } else if (a_kind < 3.5) {
    float latitude = a_position.y;
    float longitude = a_phase + u_time * a_speed;
    float latitudeRadius = sqrt(max(0.0, 1.0 - latitude * latitude));
    float depth = sin(longitude) * latitudeRadius * 0.5 + 0.5;
    vec2 spherePoint = vec2(cos(longitude) * latitudeRadius, latitude);
    vec2 coreScale = vec2(u_resolution.y / max(1.0, u_resolution.x), 1.0) * 0.36;
    position = u_core_position + spherePoint * coreScale;
    alpha = u_core_strength * (0.14 + depth * 0.76);
    pointSize *= 0.7 + depth * 0.8;
  } else if (a_kind < 4.5) {
    float angle = a_phase + u_time * a_speed;
    float ringRadius = 0.22 + a_depth * 0.055;
    vec2 coreScale = vec2(u_resolution.y / max(1.0, u_resolution.x), 1.0);
    position = u_core_position + vec2(
      cos(angle) * ringRadius * coreScale.x,
      sin(angle) * ringRadius * (0.24 + a_depth * 0.08)
    );
    alpha = u_core_strength * (0.12 + a_depth * 0.08);
  } else if (a_kind < 5.5) {
    float latitude = a_depth;
    float longitude = a_phase + u_time * a_speed;
    float latitudeRadius = sqrt(max(0.0, 1.0 - latitude * latitude));
    float depth = sin(longitude) * latitudeRadius * 0.5 + 0.5;
    vec2 coreScale = vec2(u_resolution.y / max(1.0, u_resolution.x), 1.0) * 0.36;
    position = u_core_position + vec2(
      cos(longitude) * latitudeRadius,
      latitude
    ) * coreScale;
    alpha = u_core_strength * (0.08 + depth * 0.16);
  } else {
    float latitudeAngle = a_phase;
    float longitude = a_depth + u_time * a_speed;
    float latitudeRadius = cos(latitudeAngle);
    float depth = sin(longitude) * latitudeRadius * 0.5 + 0.5;
    vec2 coreScale = vec2(u_resolution.y / max(1.0, u_resolution.x), 1.0) * 0.36;
    position = u_core_position + vec2(
      cos(longitude) * latitudeRadius,
      sin(latitudeAngle)
    ) * coreScale;
    alpha = u_core_strength * (0.07 + depth * 0.14);
  }

  gl_Position = vec4(position, 0.0, 1.0);
  gl_PointSize = max(1.0, pointSize * u_intensity * u_render_scale);
  v_alpha = alpha * u_reveal * u_intensity;
  v_kind = a_kind;
  v_tone = a_tone;
}
`

const FRAGMENT_SHADER = `
precision mediump float;

varying float v_alpha;
varying float v_kind;
varying float v_tone;
uniform float u_light_mode;

void main() {
  vec3 color = mix(
    vec3(0.56, 0.96, 1.0),
    vec3(0.02, 0.42, 0.52),
    u_light_mode
  );
  if (v_tone > 1.5) {
    color = mix(
      vec3(1.0, 0.44, 0.52),
      vec3(0.7, 0.12, 0.27),
      u_light_mode
    );
  } else if (v_tone > 0.5) {
    color = mix(
      vec3(1.0, 0.79, 0.2),
      vec3(0.63, 0.4, 0.03),
      u_light_mode
    );
  }

  float alpha = v_alpha;
  if (v_kind < 0.5 || (v_kind > 1.5 && v_kind < 3.5)) {
    float distanceToCenter = length(gl_PointCoord - 0.5);
    float core = 1.0 - smoothstep(0.08, 0.5, distanceToCenter);
    float glow = 1.0 - smoothstep(0.2, 0.5, distanceToCenter);
    alpha *= core * 0.82 + glow * 0.28;
  }

  alpha *= mix(1.0, 0.88, u_light_mode);
  gl_FragColor = vec4(color * alpha, alpha);
}
`

function deterministicUnit(index: number, salt: number) {
  const value = Math.sin(index * 12.9898 + salt * 78.233) * 43758.5453

  return value - Math.floor(value)
}

function getParticleCount(intensity: YucoreSignalFieldWebglProps['intensity']) {
  if (intensity === 'hero') return 820
  if (intensity === 'workbench') return 420
  return 320
}

function getCorePointCount(
  intensity: YucoreSignalFieldWebglProps['intensity']
) {
  if (intensity === 'hero') return 320
  if (intensity === 'workbench') return 260
  return 220
}

function getTone(index: number) {
  if (index % 17 === 0) return 1
  if (index % 29 === 0) return 2
  return 0
}

function pushVertex(
  vertices: number[],
  x: number,
  y: number,
  size: number,
  tone: number,
  depth: number,
  speed: number,
  phase: number,
  kind: number
) {
  vertices.push(x, y, size, tone, depth, speed, phase, kind)
}

function createSignalScene(
  intensity: YucoreSignalFieldWebglProps['intensity'],
  coreMode: YucoreSignalFieldWebglProps['coreMode'],
  renderProfile: YucoreSignalFieldWebglProps['renderProfile']
): SignalScene {
  const vertices: number[] = []
  const particleStart = 0
  const particleCount =
    renderProfile === 'console' ? 180 : getParticleCount(intensity)

  for (let index = 0; index < particleCount; index += 1) {
    let x = deterministicUnit(index, 1) * 2 - 1
    let y = deterministicUnit(index, 2) * 2 - 1
    const centerDistance = Math.hypot(x * 0.92, y + 0.02)
    if (centerDistance < 0.34) {
      const angle = deterministicUnit(index, 3) * Math.PI * 2
      const radius = 0.38 + deterministicUnit(index, 4) * 0.2
      x = Math.cos(angle) * radius
      y = Math.sin(angle) * radius - 0.02
    }
    const depth = 0.18 + deterministicUnit(index, 5) * 0.82
    pushVertex(
      vertices,
      x,
      y,
      1.4 + deterministicUnit(index, 6) * 3.4,
      getTone(index),
      depth,
      0.18 + deterministicUnit(index, 7) * 0.62,
      deterministicUnit(index, 8) * Math.PI * 2,
      0
    )
  }

  const lineStart = vertices.length / FLOATS_PER_VERTEX
  const routeDefinitions = [
    { base: 0.32, amplitude: 0.048, frequency: 7.2, phase: 0.4 },
    { base: 0.61, amplitude: 0.068, frequency: 5.4, phase: 2.1 },
    { base: 0.76, amplitude: 0.036, frequency: 9.6, phase: 4.6 },
    { base: 0.46, amplitude: 0.055, frequency: 6.4, phase: 3.3 },
  ]
  const segmentsPerRoute = renderProfile === 'console' ? 42 : 72
  routeDefinitions.forEach((route, routeIndex) => {
    for (let segment = 0; segment < segmentsPerRoute; segment += 1) {
      const startX = segment / segmentsPerRoute
      const endX = (segment + 1) / segmentsPerRoute
      const startY =
        route.base +
        Math.sin(startX * route.frequency + route.phase) * route.amplitude
      const endY =
        route.base +
        Math.sin(endX * route.frequency + route.phase) * route.amplitude
      let tone = 0
      if (routeIndex === 1) {
        tone = 1
      } else if (routeIndex === 3) {
        tone = 2
      }
      pushVertex(
        vertices,
        startX * 2 - 1,
        1 - startY * 2,
        1,
        tone,
        routeIndex,
        0.42 + routeIndex * 0.08,
        segment * 0.12,
        1
      )
      pushVertex(
        vertices,
        endX * 2 - 1,
        1 - endY * 2,
        1,
        tone,
        routeIndex,
        0.42 + routeIndex * 0.08,
        segment * 0.12,
        1
      )
    }
  })

  if (coreMode === 'full') {
    const ringSegments = 64
    for (let ring = 0; ring < 3; ring += 1) {
      for (let segment = 0; segment < ringSegments; segment += 1) {
        const startAngle = (segment / ringSegments) * Math.PI * 2
        const endAngle = ((segment + 1) / ringSegments) * Math.PI * 2
        pushVertex(
          vertices,
          0,
          0,
          1,
          ring === 1 ? 1 : 0,
          ring,
          ring % 2 === 0 ? 0.12 : -0.1,
          startAngle,
          4
        )
        pushVertex(
          vertices,
          0,
          0,
          1,
          ring === 1 ? 1 : 0,
          ring,
          ring % 2 === 0 ? 0.12 : -0.1,
          endAngle,
          4
        )
      }
    }

    const sphereLineSegments = 48
    const latitudeBands = [-0.78, -0.52, -0.26, 0, 0.26, 0.52, 0.78]
    latitudeBands.forEach((latitude, latitudeIndex) => {
      for (let segment = 0; segment < sphereLineSegments; segment += 1) {
        const startAngle = (segment / sphereLineSegments) * Math.PI * 2
        const endAngle = ((segment + 1) / sphereLineSegments) * Math.PI * 2
        const tone = latitudeIndex === 3 ? 1 : 0
        pushVertex(vertices, 0, 0, 1, tone, latitude, 0.075, startAngle, 5)
        pushVertex(vertices, 0, 0, 1, tone, latitude, 0.075, endAngle, 5)
      }
    })

    const meridianCount = 8
    const meridianSegments = 36
    for (let meridian = 0; meridian < meridianCount; meridian += 1) {
      const longitude = (meridian / meridianCount) * Math.PI * 2
      for (let segment = 0; segment < meridianSegments; segment += 1) {
        const startLatitude =
          -Math.PI / 2 + (segment / meridianSegments) * Math.PI
        const endLatitude =
          -Math.PI / 2 + ((segment + 1) / meridianSegments) * Math.PI
        const tone = meridian % 4 === 1 ? 2 : 0
        pushVertex(vertices, 0, 0, 1, tone, longitude, 0.075, startLatitude, 6)
        pushVertex(vertices, 0, 0, 1, tone, longitude, 0.075, endLatitude, 6)
      }
    }
  }
  const lineCount = vertices.length / FLOATS_PER_VERTEX - lineStart

  const packetStart = vertices.length / FLOATS_PER_VERTEX
  const packetCount = renderProfile === 'console' ? 12 : 24
  for (let index = 0; index < packetCount; index += 1) {
    const route = index % 4
    pushVertex(
      vertices,
      0,
      0,
      3.2 + (index % 4) * 0.7,
      route === 1 ? 1 : 0,
      route,
      0.032 + (index % 5) * 0.006,
      deterministicUnit(index, 31),
      2
    )
  }

  const coreStart = vertices.length / FLOATS_PER_VERTEX
  const coreCount = coreMode === 'full' ? getCorePointCount(intensity) : 0
  const goldenAngle = Math.PI * (3 - Math.sqrt(5))
  for (let index = 0; index < coreCount; index += 1) {
    const latitude = 1 - (index / Math.max(1, coreCount - 1)) * 2
    pushVertex(
      vertices,
      0,
      latitude,
      1.3 + (index % 5) * 0.34,
      getTone(index + 5),
      0,
      0.1,
      index * goldenAngle,
      3
    )
  }

  return {
    coreCount,
    coreStart,
    lineCount,
    lineStart,
    packetCount,
    packetStart,
    particleCount,
    particleStart,
    vertices: new Float32Array(vertices),
  }
}

function createShader(gl: WebGLRenderingContext, type: number, source: string) {
  const shader = gl.createShader(type)
  if (!shader) return null

  gl.shaderSource(shader, source)
  gl.compileShader(shader)
  if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
    gl.deleteShader(shader)
    return null
  }

  return shader
}

function createProgram(gl: WebGLRenderingContext) {
  const vertexShader = createShader(gl, gl.VERTEX_SHADER, VERTEX_SHADER)
  const fragmentShader = createShader(gl, gl.FRAGMENT_SHADER, FRAGMENT_SHADER)
  if (!vertexShader || !fragmentShader) return null

  const program = gl.createProgram()
  if (!program) return null

  gl.attachShader(program, vertexShader)
  gl.attachShader(program, fragmentShader)
  gl.linkProgram(program)
  gl.deleteShader(vertexShader)
  gl.deleteShader(fragmentShader)

  if (!gl.getProgramParameter(program, gl.LINK_STATUS)) {
    gl.deleteProgram(program)
    return null
  }

  return program
}

function getIntensityValue(
  intensity: YucoreSignalFieldWebglProps['intensity']
) {
  if (intensity === 'hero') return 1
  if (intensity === 'workbench') return 0.82
  return 0.68
}

export function YucoreSignalFieldWebgl(props: YucoreSignalFieldWebglProps) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null)
  const activeRef = useRef(props.active !== false)
  const activationRef = useRef<((active: boolean) => void) | null>(null)
  const resourcePropsRef = useRef(props)
  resourcePropsRef.current = props
  const resourceKey = getSignalFieldResourceKey(props)

  useEffect(() => {
    const resourceProps = resourcePropsRef.current
    const canvas = canvasRef.current
    const gl = canvas?.getContext('webgl', {
      alpha: true,
      antialias: false,
      depth: false,
      powerPreference: 'high-performance',
      premultipliedAlpha: true,
      preserveDrawingBuffer: false,
    })
    if (!canvas || !gl) return

    const program = createProgram(gl)
    if (!program) return

    const buffer = gl.createBuffer()
    if (!buffer) {
      gl.deleteProgram(program)
      return
    }

    const scene = createSignalScene(
      resourceProps.intensity,
      resourceProps.coreMode,
      resourceProps.renderProfile
    )
    gl.bindBuffer(gl.ARRAY_BUFFER, buffer)
    gl.bufferData(gl.ARRAY_BUFFER, scene.vertices, gl.STATIC_DRAW)
    gl.enable(gl.BLEND)
    gl.blendFunc(gl.ONE, gl.ONE_MINUS_SRC_ALPHA)

    const stride = FLOATS_PER_VERTEX * Float32Array.BYTES_PER_ELEMENT
    const bindAttribute = (name: string, size: number, offset: number) => {
      const location = gl.getAttribLocation(program, name)
      if (location < 0) return
      gl.enableVertexAttribArray(location)
      gl.vertexAttribPointer(
        location,
        size,
        gl.FLOAT,
        false,
        stride,
        offset * Float32Array.BYTES_PER_ELEMENT
      )
    }

    gl.useProgram(program)
    bindAttribute('a_position', 2, 0)
    bindAttribute('a_size', 1, 2)
    bindAttribute('a_tone', 1, 3)
    bindAttribute('a_depth', 1, 4)
    bindAttribute('a_speed', 1, 5)
    bindAttribute('a_phase', 1, 6)
    bindAttribute('a_kind', 1, 7)

    const resolutionLocation = gl.getUniformLocation(program, 'u_resolution')
    const renderScaleLocation = gl.getUniformLocation(program, 'u_render_scale')
    const timeLocation = gl.getUniformLocation(program, 'u_time')
    const intensityLocation = gl.getUniformLocation(program, 'u_intensity')
    const lightModeLocation = gl.getUniformLocation(program, 'u_light_mode')
    const revealLocation = gl.getUniformLocation(program, 'u_reveal')
    const pointerLocation = gl.getUniformLocation(program, 'u_pointer')
    const pointerActiveLocation = gl.getUniformLocation(
      program,
      'u_pointer_active'
    )
    const coreStrengthLocation = gl.getUniformLocation(
      program,
      'u_core_strength'
    )
    const corePositionLocation = gl.getUniformLocation(
      program,
      'u_core_position'
    )
    let corePosition: [number, number] = [-0.08, 0.16]
    if (resourceProps.corePlacement === 'hero') {
      corePosition = [0, 0.58]
    } else if (resourceProps.corePlacement === 'intro') {
      corePosition = [0, 0.16]
    }
    gl.uniform1f(intensityLocation, getIntensityValue(resourceProps.intensity))
    gl.uniform1f(lightModeLocation, resourceProps.colorMode === 'light' ? 1 : 0)
    gl.uniform1f(
      coreStrengthLocation,
      resourceProps.coreMode === 'full' ? 1 : 0
    )
    gl.uniform2f(corePositionLocation, corePosition[0], corePosition[1])
    gl.uniform2f(pointerLocation, 0.5, 0.5)
    gl.uniform1f(pointerActiveLocation, 0)

    let width = 1
    let height = 1
    let renderScale = 1
    let qualityLevel = resourceProps.renderProfile === 'console' ? 1 : 0
    let lastRenderTime = Number.NEGATIVE_INFINITY
    let lastAnimationTime = window.performance.now()
    let activeStartedAt = lastAnimationTime
    let wasActive = activeRef.current
    let slowFrameStreak = 0
    let stableFrameStreak = 0
    const reduceMotion = window.matchMedia(
      '(prefers-reduced-motion: reduce)'
    ).matches
    const navigatorWithMemory = navigator as Navigator & {
      deviceMemory?: number
    }
    if (
      navigator.hardwareConcurrency <= 4 ||
      (navigatorWithMemory.deviceMemory !== undefined &&
        navigatorWithMemory.deviceMemory <= 4)
    ) {
      qualityLevel = 1
    }

    const resize = () => {
      width = Math.max(1, Math.floor(canvas.clientWidth))
      height = Math.max(1, Math.floor(canvas.clientHeight))
      let renderScales = width < 720 ? [0.72, 0.62, 0.52] : [0.56, 0.48, 0.4]
      if (resourceProps.renderProfile === 'console') {
        renderScales = width < 720 ? [0.5, 0.42, 0.34] : [0.42, 0.36, 0.3]
      }
      renderScale = renderScales[qualityLevel] ?? renderScales[2]
      const pixelWidth = Math.max(1, Math.floor(width * renderScale))
      const pixelHeight = Math.max(1, Math.floor(height * renderScale))
      if (canvas.width !== pixelWidth || canvas.height !== pixelHeight) {
        canvas.width = pixelWidth
        canvas.height = pixelHeight
      }
      gl.viewport(0, 0, canvas.width, canvas.height)
      gl.uniform2f(resolutionLocation, width, height)
      gl.uniform1f(renderScaleLocation, renderScale)
    }

    const handlePointerMove = (event: PointerEvent) => {
      const rect = canvas.getBoundingClientRect()
      const pointerX = (event.clientX - rect.left) / Math.max(1, rect.width)
      const pointerY = (event.clientY - rect.top) / Math.max(1, rect.height)
      gl.uniform2f(pointerLocation, pointerX, pointerY)
      gl.uniform1f(pointerActiveLocation, 1)
    }

    const handlePointerLeave = () => {
      gl.uniform1f(pointerActiveLocation, 0)
    }

    const render = (timestamp: number) => {
      const animate = activeRef.current
      const animationGap = timestamp - lastAnimationTime
      lastAnimationTime = timestamp
      if (animate && animationGap > 42) {
        slowFrameStreak += 1
        stableFrameStreak = 0
      } else if (animate && animationGap > 0 && animationGap < 26) {
        slowFrameStreak = 0
        stableFrameStreak += 1
      }

      if (slowFrameStreak >= 3 && qualityLevel < 2) {
        qualityLevel += 1
        slowFrameStreak = 0
        stableFrameStreak = 0
        resize()
      } else if (stableFrameStreak >= 360 && qualityLevel > 0) {
        qualityLevel -= 1
        slowFrameStreak = 0
        stableFrameStreak = 0
        resize()
      }

      let desktopFps = [40, 32, 24]
      let mobileFps = [30, 24, 18]
      if (resourceProps.renderProfile === 'console') {
        desktopFps = [20, 16, 12]
        mobileFps = [15, 12, 10]
      } else if (resourceProps.renderProfile === 'entrance') {
        desktopFps = [60, 50, 40]
        mobileFps = [60, 45, 36]
      }
      const targetFps =
        width < 720
          ? (mobileFps[qualityLevel] ?? 18)
          : (desktopFps[qualityLevel] ?? 24)
      const frameIntervalMs = reduceMotion ? 1000 / 12 : 1000 / targetFps
      if (animate && timestamp - lastRenderTime < frameIntervalMs - 0.75) {
        return
      }
      lastRenderTime = timestamp

      gl.clearColor(0, 0, 0, 0)
      gl.clear(gl.COLOR_BUFFER_BIT)
      gl.uniform1f(
        timeLocation,
        animate && !reduceMotion ? timestamp / 1000 : 0
      )
      const reveal =
        animate && !reduceMotion
          ? Math.min(1, (timestamp - activeStartedAt) / 360)
          : 1
      gl.uniform1f(revealLocation, 1 - Math.pow(1 - reveal, 3))

      gl.drawArrays(gl.POINTS, scene.particleStart, scene.particleCount)
      if (resourceProps.renderProfile !== 'console') {
        gl.drawArrays(gl.LINES, scene.lineStart, scene.lineCount)
      }
      gl.drawArrays(gl.POINTS, scene.packetStart, scene.packetCount)
      if (scene.coreCount > 0) {
        gl.drawArrays(gl.POINTS, scene.coreStart, scene.coreCount)
      }
    }

    const renderLoop = createYucoreRenderLoop({
      isActive: activeRef.current,
      render,
    })

    const setActive = (nextActive: boolean) => {
      if (nextActive && !wasActive) {
        activeStartedAt = window.performance.now()
        lastAnimationTime = activeStartedAt
        lastRenderTime = Number.NEGATIVE_INFINITY
      }
      wasActive = nextActive
      activeRef.current = nextActive
      renderLoop.setActive(nextActive)
    }
    activationRef.current = setActive

    const handleVisibilityChange = () => {
      if (!document.hidden) {
        lastAnimationTime = window.performance.now()
      }
      renderLoop.setDocumentVisible(!document.hidden)
    }

    resize()
    render(0)
    renderLoop.start()
    const resizeObserver = new ResizeObserver(resize)
    resizeObserver.observe(canvas)
    const intersectionObserver =
      'IntersectionObserver' in window
        ? new IntersectionObserver(([entry]) => {
            const viewportVisible = entry?.isIntersecting ?? true
            if (viewportVisible) {
              lastAnimationTime = window.performance.now()
            }
            renderLoop.setViewportVisible(viewportVisible)
          })
        : undefined
    intersectionObserver?.observe(canvas)
    document.addEventListener('visibilitychange', handleVisibilityChange)
    if (!reduceMotion && resourceProps.renderProfile !== 'console') {
      window.addEventListener('pointermove', handlePointerMove, {
        passive: true,
      })
      window.addEventListener('pointerleave', handlePointerLeave)
    }

    return () => {
      renderLoop.dispose()
      resizeObserver.disconnect()
      intersectionObserver?.disconnect()
      if (activationRef.current === setActive) activationRef.current = null
      document.removeEventListener('visibilitychange', handleVisibilityChange)
      if (!reduceMotion && resourceProps.renderProfile !== 'console') {
        window.removeEventListener('pointermove', handlePointerMove)
        window.removeEventListener('pointerleave', handlePointerLeave)
      }
      gl.deleteBuffer(buffer)
      gl.deleteProgram(program)
    }
  }, [resourceKey])

  useEffect(() => {
    activationRef.current?.(props.active !== false)
  }, [props.active])

  return (
    <canvas
      ref={canvasRef}
      aria-hidden='true'
      className={cn(
        'yucore-motion-canvas yucore-signal-field-webgl absolute inset-0 h-full w-full',
        props.className
      )}
      data-renderer='webgl-points'
      data-theme={props.colorMode ?? 'dark'}
    />
  )
}
