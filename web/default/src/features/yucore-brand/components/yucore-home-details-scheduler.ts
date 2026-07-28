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
export type YucoreHomeSchedulerHost = {
  addEventListener: (
    type: string,
    listener: () => void,
    options?: AddEventListenerOptions
  ) => void
  removeEventListener: (type: string, listener: () => void) => void
  requestAnimationFrame: (callback: FrameRequestCallback) => number
  cancelAnimationFrame: (handle: number) => void
  requestIdleCallback?: (
    callback: () => void,
    options?: { timeout?: number }
  ) => number
  cancelIdleCallback?: (handle: number) => void
}

export function scheduleYucoreHomeDetails(
  host: YucoreHomeSchedulerHost,
  onReady: () => void
): () => void {
  let completed = false
  let disposed = false
  let frame: number | undefined
  let idle: number | undefined
  const events = ['pointerdown', 'keydown', 'scroll'] as const

  const cleanup = () => {
    if (frame !== undefined) host.cancelAnimationFrame(frame)
    if (idle !== undefined) host.cancelIdleCallback?.(idle)
    frame = undefined
    idle = undefined
    for (const event of events) host.removeEventListener(event, reveal)
  }

  function reveal() {
    if (disposed || completed) return
    completed = true
    cleanup()
    onReady()
  }

  host.addEventListener('keydown', reveal, { once: true })
  host.addEventListener('pointerdown', reveal, {
    once: true,
    passive: true,
  })
  host.addEventListener('scroll', reveal, { once: true, passive: true })

  if (host.requestIdleCallback && host.cancelIdleCallback) {
    idle = host.requestIdleCallback(reveal, { timeout: 1200 })
  } else {
    frame = host.requestAnimationFrame(reveal)
  }

  return () => {
    disposed = true
    cleanup()
  }
}

export function scheduleYucoreHomeSecondaryDetails(
  host: YucoreHomeSchedulerHost,
  onReady: () => void
): () => void {
  let completed = false
  let disposed = false
  let frame: number | undefined = host.requestAnimationFrame(() => {
    frame = undefined
    if (disposed || completed) return
    completed = true
    onReady()
  })

  return () => {
    disposed = true
    if (frame !== undefined) host.cancelAnimationFrame(frame)
    frame = undefined
  }
}
