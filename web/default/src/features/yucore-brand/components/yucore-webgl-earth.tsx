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
import { getEarthResourceKey } from './yucore-renderer-resource-key'

interface YucoreWebglEarthProps {
  active?: boolean
  className?: string
  colorMode?: 'dark' | 'light'
  density?: 'loader' | 'persistent'
  timeOffsetSeconds?: number
}

const vertexShaderSource = `
attribute vec2 a_position;
varying vec2 v_uv;

void main() {
  v_uv = a_position * 0.5 + 0.5;
  gl_Position = vec4(a_position, 0.0, 1.0);
}
`

const fragmentShaderSource = `
precision highp float;

varying vec2 v_uv;
uniform vec2 u_resolution;
uniform float u_time;
uniform float u_loader;
uniform float u_light_mode;
uniform float u_reveal;
uniform sampler2D u_earth_texture;
uniform float u_texture_ready;

float hash(vec2 p) {
  return fract(sin(dot(p, vec2(127.1, 311.7))) * 43758.5453123);
}

float lonDelta(float a, float b) {
  return atan(sin(a - b), cos(a - b));
}

float lineMask(float value, float width) {
  return 1.0 - smoothstep(0.0, width, abs(value));
}

float routeLine(float lon, float lat, float phase, float bias, float amp, float width) {
  float path =
    bias +
    sin(lon * 1.55 + phase) * amp +
    sin(lon * 3.15 - phase * 0.72) * amp * 0.34;
  return lineMask(lat - path, width);
}

mat3 rotateY(float angle) {
  float c = cos(angle);
  float s = sin(angle);
  return mat3(c, 0.0, -s, 0.0, 1.0, 0.0, s, 0.0, c);
}

mat3 rotateX(float angle) {
  float c = cos(angle);
  float s = sin(angle);
  return mat3(1.0, 0.0, 0.0, 0.0, c, -s, 0.0, s, c);
}

void main() {
  vec2 uv = v_uv * 2.0 - 1.0;
  uv.x *= u_resolution.x / max(u_resolution.y, 1.0);
  float reveal = smoothstep(0.0, 1.0, u_reveal);
  float signalBand = floor((uv.y + 1.0) * 42.0);
  float signalJitter = hash(vec2(signalBand, floor(u_time * 7.0)));
  uv.x += (signalJitter - 0.5) * (1.0 - reveal) * 0.13;
  float r2 = dot(uv, uv);
  if (r2 > 1.0) {
    discard;
  }

  float z = sqrt(1.0 - r2);
  vec3 normal = normalize(vec3(uv.x, uv.y, z));
  vec3 world = rotateY(u_time * mix(0.16, 0.24, u_loader)) * rotateX(-0.18) * normal;
  vec3 light = normalize(vec3(-0.42, 0.52, 0.74));
  float rawLight = dot(normal, light);
  float diffuse = clamp(rawLight * 0.58 + 0.42, 0.0, 1.0);
  float rim = pow(1.0 - z, 2.15);

  float lon = atan(world.z, world.x);
  float lat = asin(clamp(world.y, -1.0, 1.0));
  vec2 mapUv = vec2(lon / 6.2831853 + 0.5, lat / 3.1415926 + 0.5);
  vec3 photo = texture2D(u_earth_texture, mapUv).rgb;
  float photoLuma = dot(photo, vec3(0.2126, 0.7152, 0.0722));
  float fallbackLand = smoothstep(0.15, 0.68, sin(lon * 2.3 + sin(lat * 3.8)) * 0.5 + 0.5);
  float landSignal = photo.r * 0.9 + photo.g * 0.72 - photo.b * 1.08;
  float textureLand = smoothstep(-0.08, 0.2, landSignal);
  float land = mix(fallbackLand, textureLand, u_texture_ready);
  float cloud = smoothstep(0.56, 0.9, photoLuma) * smoothstep(0.08, 0.38, min(photo.r, photo.g));
  land *= 1.0 - cloud * 0.42;
  float neighbourLand = smoothstep(
    -0.08,
    0.2,
    dot(texture2D(u_earth_texture, mapUv + vec2(0.0018, 0.0)).rgb, vec3(0.9, 0.72, -1.08))
  );
  float coast = clamp(abs(textureLand - neighbourLand) * 7.0, 0.0, 1.0) * u_texture_ready;
  float ice = smoothstep(1.08, 1.42, abs(lat) + photoLuma * 0.14);
  float gridLat = 1.0 - smoothstep(0.0, 0.014, abs(fract((mapUv.y + 0.005) * 10.0) - 0.5));
  float gridLon = 1.0 - smoothstep(0.0, 0.012, abs(fract(mapUv.x * 16.0) - 0.5));
  float fineGridLon = 1.0 - smoothstep(0.0, 0.008, abs(fract(mapUv.x * 32.0 + u_time * 0.018) - 0.5));
  float grid = max(max(gridLat, gridLon), fineGridLon * 0.55) * 0.32;
  float equator = lineMask(lat + sin(lon * 2.0 + u_time * 0.42) * 0.012, 0.012);
  float meridianSweep =
    lineMask(lonDelta(lon, u_time * 0.34), 0.012) +
    lineMask(lonDelta(lon, -u_time * 0.24 + 2.18), 0.01);
  float routeA = routeLine(lon, lat, u_time * 0.42, -0.2, 0.12, 0.014);
  float routeB = routeLine(lon, lat, -u_time * 0.34 + 1.7, 0.08, 0.1, 0.012);
  float routeC = routeLine(lon, lat, u_time * 0.26 + 3.1, 0.34, 0.08, 0.011);
  float routeD = routeLine(lon, lat, -u_time * 0.5 + 4.2, -0.42, 0.06, 0.01);
  float route = max(max(routeA, routeB), max(routeC, routeD));
  float routePulse =
    smoothstep(0.82, 1.0, 0.5 + 0.5 * sin(lon * 5.2 - u_time * 3.4)) * routeA +
    smoothstep(0.86, 1.0, 0.5 + 0.5 * sin(lon * 4.4 + u_time * 3.0 + 1.4)) * routeB +
    smoothstep(0.88, 1.0, 0.5 + 0.5 * sin(lon * 6.0 - u_time * 2.7 + 2.8)) * routeC +
    smoothstep(0.86, 1.0, 0.5 + 0.5 * sin(lon * 7.2 + u_time * 3.8 + 0.8)) * routeD;
  float routeVisibility = smoothstep(-0.12, 0.58, z);
  float signalCell = hash(
    floor(mapUv * vec2(144.0, 72.0)) +
    vec2(0.0, floor(u_time * 2.0))
  );
  float signalScan = 1.0 - smoothstep(
    0.0,
    0.022,
    abs(fract((mapUv.y + u_time * 0.022) * 24.0) - 0.5)
  );

  vec3 photoColor = pow(max(photo, vec3(0.002)), vec3(0.96));
  photoColor = mix(vec3(photoLuma), photoColor, 0.8);
  vec3 ocean = mix(
    vec3(0.003, 0.018, 0.052),
    vec3(0.08, 0.36, 0.5),
    u_light_mode
  );
  vec3 landColor = mix(
    vec3(0.035, 0.2, 0.13),
    vec3(0.18, 0.5, 0.31),
    u_light_mode
  );
  vec3 fallbackSurface = mix(ocean, landColor, land);
  vec3 lightPhoto = mix(vec3(photoLuma), photoColor, 0.58);
  vec3 realSurface = mix(
    photoColor * vec3(0.84, 0.96, 1.0),
    lightPhoto * vec3(0.68, 0.88, 0.82),
    u_light_mode
  );
  vec3 color = mix(fallbackSurface, realSurface, u_texture_ready * 0.96);
  color *= 0.48 + diffuse * 0.78;
  vec3 iceColor = mix(vec3(0.76, 0.92, 0.96), vec3(0.68, 0.84, 0.82), u_light_mode);
  vec3 cyan = mix(vec3(0.52, 0.9, 1.0), vec3(0.02, 0.42, 0.54), u_light_mode);
  vec3 mint = mix(vec3(0.58, 1.0, 0.96), vec3(0.04, 0.48, 0.36), u_light_mode);
  vec3 gold = mix(vec3(1.0, 0.78, 0.28), vec3(0.66, 0.4, 0.03), u_light_mode);
  color += mix(vec3(0.92, 0.78, 0.34), vec3(0.64, 0.42, 0.08), u_light_mode) * coast * 0.32;
  color = mix(color, iceColor, ice * 0.42);
  color += mix(vec3(0.7, 0.96, 1.0), vec3(0.38, 0.64, 0.67), u_light_mode) * cloud * 0.18 * u_texture_ready;
  color += cyan * grid * (0.38 + routeVisibility * 0.3);
  color += mint * route * routeVisibility * (0.38 + diffuse * 0.32);
  color += gold * routePulse * routeVisibility * 0.82;
  color += cyan * meridianSweep * routeVisibility * 0.45;
  color += gold * equator * routeVisibility * 0.44;
  color += cyan * rim * 0.72;
  color += gold * pow(max(0.0, dot(normal, vec3(0.2, -0.1, 0.98))), 8.0) * 0.28;
  color += cyan * signalScan * (0.025 + routeVisibility * 0.035);

  float terminator = smoothstep(-0.18, 0.62, rawLight);
  float cityCell = hash(floor(mapUv * vec2(260.0, 130.0)));
  float nightLights =
    smoothstep(0.974, 0.998, cityCell) *
    land *
    (1.0 - terminator) *
    (0.3 + coast * 0.7);
  color += gold * nightLights * 1.4;
  color *= mix(0.34 + terminator * 0.84, 0.62 + terminator * 0.54, u_light_mode);
  float oceanSpecular = pow(max(0.0, dot(reflect(-light, normal), vec3(0.0, 0.0, 1.0))), 24.0) * (1.0 - land);
  color += mix(vec3(0.68, 0.94, 1.0), vec3(0.28, 0.58, 0.65), u_light_mode) * oceanSpecular * 0.36;
  color = pow(max(color, vec3(0.0)), vec3(0.9));

  float revealEdge = 0.82 - reveal * 2.05;
  float revealSweep = smoothstep(revealEdge, revealEdge + 0.24, uv.y);
  float packetReveal = smoothstep(
    0.05,
    0.68,
    revealSweep + reveal * 0.58 - signalCell * (1.0 - reveal) * 0.46
  );
  float lockLine = 1.0 - smoothstep(
    0.0,
    0.05,
    abs(uv.y - (revealEdge + 0.08))
  );
  float edgeSignal = mix(
    1.0,
    0.72 + signalCell * 0.28,
    smoothstep(0.66, 0.98, r2)
  );
  color += cyan * lockLine * (1.0 - reveal) * 0.72;
  float alpha =
    (1.0 - smoothstep(0.88, 1.0, r2)) *
    (0.96 + rim * 0.24) *
    packetReveal *
    edgeSignal;
  gl_FragColor = vec4(color, alpha);
}
`

function prefersReducedMotion() {
  return window.matchMedia('(prefers-reduced-motion: reduce)').matches
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
  const vertexShader = createShader(gl, gl.VERTEX_SHADER, vertexShaderSource)
  const fragmentShader = createShader(
    gl,
    gl.FRAGMENT_SHADER,
    fragmentShaderSource
  )
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

export function YucoreWebglEarth(props: YucoreWebglEarthProps) {
  const canvasRef = useRef<HTMLCanvasElement | null>(null)
  const activeRef = useRef(props.active !== false)
  const activationRef = useRef<((active: boolean) => void) | null>(null)
  const resourcePropsRef = useRef(props)
  resourcePropsRef.current = props
  const resourceKey = getEarthResourceKey(props)

  useEffect(() => {
    const resourceProps = resourcePropsRef.current
    const canvas = canvasRef.current
    const gl = canvas?.getContext('webgl', {
      alpha: true,
      antialias: false,
      depth: false,
      premultipliedAlpha: true,
      powerPreference: 'high-performance',
    })
    if (!canvas || !gl) return

    const program = createProgram(gl)
    if (!program) return

    const positionLocation = gl.getAttribLocation(program, 'a_position')
    const resolutionLocation = gl.getUniformLocation(program, 'u_resolution')
    const timeLocation = gl.getUniformLocation(program, 'u_time')
    const loaderLocation = gl.getUniformLocation(program, 'u_loader')
    const lightModeLocation = gl.getUniformLocation(program, 'u_light_mode')
    const revealLocation = gl.getUniformLocation(program, 'u_reveal')
    const textureLocation = gl.getUniformLocation(program, 'u_earth_texture')
    const textureReadyLocation = gl.getUniformLocation(
      program,
      'u_texture_ready'
    )
    const buffer = gl.createBuffer()
    const earthTexture = gl.createTexture()
    if (!buffer || !earthTexture) return

    gl.bindBuffer(gl.ARRAY_BUFFER, buffer)
    gl.bufferData(
      gl.ARRAY_BUFFER,
      new Float32Array([-1, -1, 1, -1, -1, 1, -1, 1, 1, -1, 1, 1]),
      gl.STATIC_DRAW
    )
    gl.useProgram(program)
    gl.enableVertexAttribArray(positionLocation)
    gl.vertexAttribPointer(positionLocation, 2, gl.FLOAT, false, 0, 0)
    gl.enable(gl.BLEND)
    gl.blendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
    gl.activeTexture(gl.TEXTURE0)
    gl.bindTexture(gl.TEXTURE_2D, earthTexture)
    gl.texImage2D(
      gl.TEXTURE_2D,
      0,
      gl.RGBA,
      1,
      1,
      0,
      gl.RGBA,
      gl.UNSIGNED_BYTE,
      resourceProps.colorMode === 'light'
        ? new Uint8Array([178, 218, 218, 255])
        : new Uint8Array([3, 14, 28, 255])
    )
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.REPEAT)
    gl.texParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
    gl.uniform1f(loaderLocation, resourceProps.density === 'loader' ? 1 : 0)
    gl.uniform1f(lightModeLocation, resourceProps.colorMode === 'light' ? 1 : 0)
    gl.uniform1i(textureLocation, 0)
    gl.uniform1f(textureReadyLocation, 0)

    let width = 0
    let height = 0
    let lastRenderTime = Number.NEGATIVE_INFINITY
    let disposed = false
    const reduceMotion = prefersReducedMotion()
    const motionScale = reduceMotion ? 0.2 : 1
    const targetFps = resourceProps.density === 'loader' ? 36 : 30
    const frameIntervalMs = reduceMotion ? 1000 / 12 : 1000 / targetFps
    const timeOffsetSeconds = resourceProps.timeOffsetSeconds ?? 0
    let sceneStartedAt = window.performance.now()
    let wasActive = activeRef.current

    const resize = () => {
      const maxPixelRatio = resourceProps.density === 'loader' ? 1.25 : 1.1
      const pixelRatio = Math.min(window.devicePixelRatio || 1, maxPixelRatio)
      // Layout dimensions stay stable while the parent entrance transform scales.
      width = Math.max(1, Math.floor(canvas.clientWidth))
      height = Math.max(1, Math.floor(canvas.clientHeight))
      const pixelWidth = Math.max(1, Math.floor(width * pixelRatio))
      const pixelHeight = Math.max(1, Math.floor(height * pixelRatio))
      if (canvas.width !== pixelWidth || canvas.height !== pixelHeight) {
        canvas.width = pixelWidth
        canvas.height = pixelHeight
      }
      gl.viewport(0, 0, canvas.width, canvas.height)
      gl.uniform2f(resolutionLocation, width, height)
    }

    const render = (timestamp: number) => {
      const animate = activeRef.current
      if (animate && timestamp - lastRenderTime < frameIntervalMs) {
        return
      }
      lastRenderTime = timestamp

      gl.clearColor(0, 0, 0, 0)
      gl.clear(gl.COLOR_BUFFER_BIT)
      gl.uniform1f(
        timeLocation,
        (timestamp / 1000 + timeOffsetSeconds) * motionScale
      )
      const revealProgress =
        !animate || reduceMotion || resourceProps.density !== 'loader'
          ? 1
          : Math.max(0, Math.min(1, (timestamp - sceneStartedAt) / 1200))
      gl.uniform1f(revealLocation, 1 - Math.pow(1 - revealProgress, 3))
      gl.drawArrays(gl.TRIANGLES, 0, 6)
    }

    const renderLoop = createYucoreRenderLoop({
      isActive: activeRef.current,
      render,
    })

    const setActive = (nextActive: boolean) => {
      if (nextActive && !wasActive) {
        sceneStartedAt = window.performance.now()
        lastRenderTime = Number.NEGATIVE_INFINITY
      }
      wasActive = nextActive
      activeRef.current = nextActive
      renderLoop.setActive(nextActive)
    }
    activationRef.current = setActive

    const handleVisibilityChange = () => {
      renderLoop.setDocumentVisible(!document.hidden)
    }

    const earthImage = new Image()
    earthImage.decoding = 'async'
    earthImage.addEventListener('load', () => {
      if (disposed) return
      gl.activeTexture(gl.TEXTURE0)
      gl.bindTexture(gl.TEXTURE_2D, earthTexture)
      gl.pixelStorei(gl.UNPACK_FLIP_Y_WEBGL, 1)
      gl.texImage2D(
        gl.TEXTURE_2D,
        0,
        gl.RGB,
        gl.RGB,
        gl.UNSIGNED_BYTE,
        earthImage
      )
      gl.generateMipmap(gl.TEXTURE_2D)
      gl.texParameteri(
        gl.TEXTURE_2D,
        gl.TEXTURE_MIN_FILTER,
        gl.LINEAR_MIPMAP_LINEAR
      )
      gl.uniform1f(textureReadyLocation, 1)
      if (!activeRef.current) render(window.performance.now())
    })
    earthImage.src = '/yucore-earth-blue-marble.webp'

    resize()
    render(0)
    renderLoop.start()
    const resizeObserver = new ResizeObserver(() => {
      resize()
      if (!activeRef.current) render(window.performance.now())
    })
    resizeObserver.observe(canvas)
    const intersectionObserver =
      'IntersectionObserver' in window
        ? new IntersectionObserver(([entry]) => {
            renderLoop.setViewportVisible(entry?.isIntersecting ?? true)
          })
        : undefined
    intersectionObserver?.observe(canvas)
    document.addEventListener('visibilitychange', handleVisibilityChange)

    return () => {
      disposed = true
      renderLoop.dispose()
      resizeObserver.disconnect()
      intersectionObserver?.disconnect()
      if (activationRef.current === setActive) activationRef.current = null
      document.removeEventListener('visibilitychange', handleVisibilityChange)
      gl.deleteBuffer(buffer)
      gl.deleteTexture(earthTexture)
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
        'yucore-webgl-earth absolute inset-0 h-full w-full rounded-full',
        props.className
      )}
      data-theme={props.colorMode ?? 'dark'}
    />
  )
}
