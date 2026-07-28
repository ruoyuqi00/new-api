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
import { createBootScene, renderBootScene } from './yucore-boot-renderer'

type BootWorkerMessage =
  | {
      canvas: OffscreenCanvas
      dpr: number
      durationMs: number
      height: number
      reduceMotion: boolean
      type: 'init'
      width: number
    }
  | { dpr: number; height: number; type: 'resize'; width: number }
  | { hidden: boolean; type: 'visibility' }
  | { type: 'dispose' }

let canvas: OffscreenCanvas | null = null
let ctx: OffscreenCanvasRenderingContext2D | null = null
let width = 1
let height = 1
let dpr = 1
let durationMs = 1
let reduceMotion = false
let hidden = false
let disposed = false
let animationFrame = 0
let lastRenderTime = Number.NEGATIVE_INFINITY
let startedAt = 0
const scene = createBootScene()
const requestFrame =
  typeof self.requestAnimationFrame === 'function'
    ? self.requestAnimationFrame.bind(self)
    : (callback: FrameRequestCallback) =>
        self.setTimeout(() => callback(performance.now()), 1000 / 60)
const cancelFrame =
  typeof self.cancelAnimationFrame === 'function'
    ? self.cancelAnimationFrame.bind(self)
    : self.clearTimeout.bind(self)

function resize() {
  if (!canvas || !ctx) return

  canvas.width = Math.max(1, Math.floor(width * dpr))
  canvas.height = Math.max(1, Math.floor(height * dpr))
  ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
}

function render(now: number) {
  animationFrame = 0
  if (!ctx || disposed || hidden) return

  const frameIntervalMs = 1000 / 60
  if (!reduceMotion && now - lastRenderTime < frameIntervalMs - 0.75) {
    animationFrame = requestFrame(render)
    return
  }
  lastRenderTime = now
  const elapsed = reduceMotion
    ? durationMs
    : Math.min(now - startedAt, durationMs)
  renderBootScene(
    ctx as unknown as CanvasRenderingContext2D,
    width,
    height,
    elapsed,
    durationMs,
    scene
  )

  if (!reduceMotion && elapsed < durationMs + 120) {
    animationFrame = requestFrame(render)
  }
}

function scheduleRender() {
  if (!disposed && !hidden && animationFrame === 0) {
    animationFrame = requestFrame(render)
  }
}

self.addEventListener('message', (event: MessageEvent<BootWorkerMessage>) => {
  const message = event.data

  if (message.type === 'init') {
    canvas = message.canvas
    ctx = canvas.getContext('2d', {
      alpha: false,
      desynchronized: true,
    })
    width = message.width
    height = message.height
    dpr = message.dpr
    durationMs = message.durationMs
    reduceMotion = message.reduceMotion
    startedAt = performance.now()
    resize()
    scheduleRender()
    return
  }

  if (message.type === 'resize') {
    width = message.width
    height = message.height
    dpr = message.dpr
    resize()
    return
  }

  if (message.type === 'visibility') {
    hidden = message.hidden
    if (hidden && animationFrame !== 0) {
      cancelFrame(animationFrame)
      animationFrame = 0
    } else {
      scheduleRender()
    }
    return
  }

  disposed = true
  if (animationFrame !== 0) {
    cancelFrame(animationFrame)
  }
  self.close()
})
