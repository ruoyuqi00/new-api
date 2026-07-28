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

interface YucoreBootCanvasProps {
  className?: string
  colorMode?: 'dark' | 'light'
  durationMs?: number
}

const DEFAULT_BOOT_DURATION_MS = 5200

export function YucoreBootCanvas(props: YucoreBootCanvasProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null)
  const durationMs =
    props.durationMs && props.durationMs > 0
      ? props.durationMs
      : DEFAULT_BOOT_DURATION_MS

  useEffect(() => {
    const canvas = canvasRef.current
    if (!canvas) return

    let disposed = false
    let startFrame = 0
    let stopFallback: (() => void) | undefined
    let worker: Worker | undefined
    let resizeObserver: ResizeObserver | undefined
    let handleVisibilityChange: (() => void) | undefined
    const startFallback = () => {
      void import('./yucore-boot-renderer').then((renderer) => {
        if (!disposed) {
          stopFallback = renderer.startBootCanvasRenderer(canvas, durationMs)
        }
      })
    }

    const startRenderer = () => {
      if (disposed) return

      const canRenderOffThread =
        typeof Worker !== 'undefined' && 'transferControlToOffscreen' in canvas
      if (!canRenderOffThread) {
        startFallback()
        return
      }

      try {
        worker = new Worker(
          new URL('./yucore-boot-canvas.worker.ts', import.meta.url),
          { type: 'module' }
        )
        const rect = canvas.getBoundingClientRect()
        const offscreen = canvas.transferControlToOffscreen()
        worker.postMessage(
          {
            canvas: offscreen,
            dpr: Math.min(window.devicePixelRatio || 1, 1),
            durationMs,
            height: Math.max(1, rect.height),
            reduceMotion: window.matchMedia('(prefers-reduced-motion: reduce)')
              .matches,
            type: 'init',
            width: Math.max(1, rect.width),
          },
          [offscreen]
        )

        resizeObserver = new ResizeObserver(() => {
          const nextRect = canvas.getBoundingClientRect()
          worker?.postMessage({
            dpr: Math.min(window.devicePixelRatio || 1, 1),
            height: Math.max(1, nextRect.height),
            type: 'resize',
            width: Math.max(1, nextRect.width),
          })
        })
        resizeObserver.observe(canvas)
        handleVisibilityChange = () => {
          worker?.postMessage({
            hidden: document.hidden,
            type: 'visibility',
          })
        }
        document.addEventListener('visibilitychange', handleVisibilityChange)
      } catch {
        worker?.terminate()
        worker = undefined
        startFallback()
      }
    }

    startFrame = window.requestAnimationFrame(startRenderer)

    return () => {
      disposed = true
      window.cancelAnimationFrame(startFrame)
      stopFallback?.()
      resizeObserver?.disconnect()
      if (handleVisibilityChange) {
        document.removeEventListener('visibilitychange', handleVisibilityChange)
      }
      worker?.postMessage({ type: 'dispose' })
      worker?.terminate()
    }
  }, [durationMs])

  return (
    <canvas
      ref={canvasRef}
      className={cn(
        'yucore-boot-canvas absolute inset-0 h-full w-full',
        props.className
      )}
      aria-hidden='true'
      data-theme={props.colorMode ?? 'dark'}
    />
  )
}
