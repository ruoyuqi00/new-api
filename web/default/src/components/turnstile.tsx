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

interface TurnstileApi {
  render: (element: HTMLElement, options: Record<string, unknown>) => string
  remove?: (widgetId: string) => void
}

declare global {
  interface Window {
    turnstile?: TurnstileApi
  }
}

const TURNSTILE_SCRIPT_ID = 'cf-turnstile'
const TURNSTILE_SCRIPT_SRC =
  'https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit'

let turnstileLoadPromise: Promise<TurnstileApi> | null = null

function loadTurnstile(): Promise<TurnstileApi> {
  if (window.turnstile) {
    return Promise.resolve(window.turnstile)
  }
  if (turnstileLoadPromise) {
    return turnstileLoadPromise
  }

  turnstileLoadPromise = new Promise<TurnstileApi>((resolve, reject) => {
    let script = document.querySelector<HTMLScriptElement>(
      `#${TURNSTILE_SCRIPT_ID}`
    )

    const handleLoad = () => {
      if (window.turnstile) {
        resolve(window.turnstile)
        return
      }
      reject(new Error('Turnstile loaded without exposing its API'))
    }
    const handleError = () => {
      script?.remove()
      reject(new Error('Failed to load Turnstile'))
    }

    if (!script) {
      script = document.createElement('script')
      script.id = TURNSTILE_SCRIPT_ID
      script.src = TURNSTILE_SCRIPT_SRC
      script.async = true
      script.defer = true
    }

    script.addEventListener('load', handleLoad, { once: true })
    script.addEventListener('error', handleError, { once: true })

    if (!script.isConnected) {
      document.head.appendChild(script)
    }
  }).catch((error: unknown) => {
    turnstileLoadPromise = null
    throw error
  })

  return turnstileLoadPromise
}

interface TurnstileProps {
  siteKey: string
  onVerify: (token: string) => void
  onExpire?: () => void
  className?: string
}

export function Turnstile({
  siteKey,
  onVerify,
  onExpire,
  className,
}: TurnstileProps) {
  const ref = useRef<HTMLDivElement | null>(null)
  const onVerifyRef = useRef(onVerify)
  const onExpireRef = useRef(onExpire)

  onVerifyRef.current = onVerify
  onExpireRef.current = onExpire

  useEffect(() => {
    let cancelled = false
    let widgetId: string | null = null

    void loadTurnstile()
      .then((turnstile) => {
        if (cancelled || !ref.current) return
        widgetId = turnstile.render(ref.current, {
          sitekey: siteKey,
          callback: (token: string) => onVerifyRef.current(token),
          'error-callback': () => onExpireRef.current?.(),
          'expired-callback': () => onExpireRef.current?.(),
          retry: 'auto',
          'retry-interval': 3000,
        })
      })
      .catch(() => {
        if (!cancelled) onExpireRef.current?.()
      })

    return () => {
      cancelled = true
      if (widgetId) window.turnstile?.remove?.(widgetId)
    }
  }, [siteKey])

  return <div ref={ref} className={className} />
}
