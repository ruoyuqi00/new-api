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
type AnimationFrameScheduler = {
  cancelAnimationFrame: (id: number) => void
  requestAnimationFrame: (callback: FrameRequestCallback) => number
}

type YucoreRenderLoopOptions = {
  isActive: boolean
  isDocumentVisible?: boolean
  render: FrameRequestCallback
  scheduler?: AnimationFrameScheduler
}

export function createYucoreRenderLoop(options: YucoreRenderLoopOptions) {
  const scheduler = options.scheduler ?? window
  let animationFrame = 0
  let active = options.isActive
  let disposed = false
  let documentVisible =
    options.isDocumentVisible ??
    (typeof document === 'undefined' || !document.hidden)
  let viewportVisible = true

  const canRender = () => active && documentVisible && viewportVisible

  const schedule = () => {
    if (disposed || !canRender() || animationFrame !== 0) return
    animationFrame = scheduler.requestAnimationFrame((timestamp) => {
      animationFrame = 0
      if (!canRender()) return
      options.render(timestamp)
      schedule()
    })
  }

  const cancel = () => {
    if (animationFrame === 0) return
    scheduler.cancelAnimationFrame(animationFrame)
    animationFrame = 0
  }

  const sync = () => {
    if (canRender()) {
      schedule()
      return
    }
    cancel()
  }

  return {
    dispose() {
      disposed = true
      cancel()
    },
    setActive(nextActive: boolean) {
      active = nextActive
      sync()
    },
    setDocumentVisible(nextDocumentVisible: boolean) {
      documentVisible = nextDocumentVisible
      sync()
    },
    setViewportVisible(nextViewportVisible: boolean) {
      viewportVisible = nextViewportVisible
      sync()
    },
    start() {
      sync()
    },
  }
}
