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
import { useRouterState } from '@tanstack/react-router'
import { useEffect, useState } from 'react'

import { cn } from '@/lib/utils'

import { getYucoreConsoleMotionMode } from './yucore-console-motion'

import './yucore-console-background.css'

type YucoreConsoleBackgroundProps = {
  active?: boolean
  className?: string
}

export function YucoreConsoleBackground(props: YucoreConsoleBackgroundProps) {
  const pathname = useRouterState({
    select: (state) => state.location.pathname,
  })
  const [documentVisible, setDocumentVisible] = useState(
    () => typeof document === 'undefined' || !document.hidden
  )

  useEffect(() => {
    const handleVisibilityChange = () => setDocumentVisible(!document.hidden)
    document.addEventListener('visibilitychange', handleVisibilityChange)
    return () =>
      document.removeEventListener('visibilitychange', handleVisibilityChange)
  }, [])

  const motionMode = getYucoreConsoleMotionMode(
    pathname,
    props.active !== false,
    documentVisible
  )

  return (
    <div
      aria-hidden='true'
      className={cn(
        'yucore-console-background pointer-events-none fixed inset-0 z-0 overflow-hidden',
        props.className
      )}
      data-motion={motionMode}
    >
      <div className='yucore-console-background-base absolute inset-0' />
      <svg
        className='yucore-console-background-signal absolute inset-0 h-full w-full'
        viewBox='0 0 1600 900'
        preserveAspectRatio='xMidYMid slice'
        focusable='false'
      >
        <defs>
          <clipPath id='yucore-console-globe-clip'>
            <circle cx='1230' cy='398' r='256' />
          </clipPath>
        </defs>

        <g className='yucore-console-brand-mark'>
          <path d='M88 122h38l21 36 21-36h38l-40 65v45h-38v-45z' />
          <text x='222' y='193'>
            YUCORE
          </text>
          <path
            className='yucore-console-brand-rule'
            d='M88 253H386'
            pathLength='1'
          />
        </g>

        <g className='yucore-console-routes'>
          <path
            d='M-80 666C190 650 258 506 484 514S742 674 936 612s242-240 520-248h224'
            pathLength='1'
          />
          <path
            d='M-60 294C204 308 284 404 510 386s346-196 542-138 304 212 626 198'
            pathLength='1'
          />
          <path
            d='M122 900C176 694 344 624 504 628s250 128 402 78 254-238 416-244'
            pathLength='1'
          />
          <path
            className='yucore-console-route-secondary'
            d='M360-40c42 184 132 256 286 290s272-28 402 56 204 250 486 274'
            pathLength='1'
          />
        </g>

        <g className='yucore-console-route-packets'>
          <path
            d='M-80 666C190 650 258 506 484 514S742 674 936 612s242-240 520-248h224'
            pathLength='1'
          />
          <path
            d='M-60 294C204 308 284 404 510 386s346-196 542-138 304 212 626 198'
            pathLength='1'
          />
        </g>

        <g className='yucore-console-globe'>
          <circle
            className='yucore-console-globe-halo'
            cx='1230'
            cy='398'
            r='286'
          />
          <circle
            className='yucore-console-globe-shell'
            cx='1230'
            cy='398'
            r='256'
          />
          <g clipPath='url(#yucore-console-globe-clip)'>
            <ellipse cx='1230' cy='398' rx='78' ry='256' />
            <ellipse cx='1230' cy='398' rx='164' ry='256' />
            <ellipse cx='1230' cy='398' rx='228' ry='256' />
            <ellipse cx='1230' cy='398' rx='256' ry='65' />
            <ellipse cx='1230' cy='398' rx='256' ry='136' />
            <ellipse cx='1230' cy='398' rx='256' ry='205' />
            <path d='M973 364c92 24 166-18 248 7s158 91 267 49' />
            <path d='M988 476c90-55 166-42 248-5s148 35 229-2' />
          </g>
          <g className='yucore-console-orbits'>
            <ellipse
              cx='1230'
              cy='398'
              rx='338'
              ry='136'
              transform='rotate(-18 1230 398)'
            />
            <ellipse
              cx='1230'
              cy='398'
              rx='324'
              ry='112'
              transform='rotate(58 1230 398)'
            />
          </g>
        </g>

        <g className='yucore-console-nodes'>
          <circle
            className='yucore-console-node yucore-console-node-cyan'
            cx='484'
            cy='514'
            r='5'
          />
          <circle
            className='yucore-console-node yucore-console-node-yellow'
            cx='936'
            cy='612'
            r='5'
          />
          <circle
            className='yucore-console-node yucore-console-node-coral'
            cx='510'
            cy='386'
            r='4'
          />
          <circle
            className='yucore-console-node yucore-console-node-cyan'
            cx='1052'
            cy='248'
            r='4'
          />
          <circle
            className='yucore-console-node yucore-console-node-yellow'
            cx='1456'
            cy='364'
            r='5'
          />
          <circle
            className='yucore-console-node yucore-console-node-coral'
            cx='1230'
            cy='142'
            r='4'
          />
        </g>
      </svg>
      <div className='yucore-console-background-grid absolute inset-0' />
      <div className='yucore-console-background-rails absolute inset-0' />
      <div className='yucore-console-background-vignette absolute inset-0' />
    </div>
  )
}
