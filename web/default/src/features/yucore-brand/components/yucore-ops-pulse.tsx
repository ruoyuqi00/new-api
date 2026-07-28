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
import { Activity, CreditCard, RadioTower, Timer } from 'lucide-react'

import { cn } from '@/lib/utils'

import { useYucoreTranslation } from '../i18n/use-yucore-translation'

interface YucoreOpsPulseProps {
  className?: string
  modelName?: string
  quota: number
  requestCount: number
  usedQuota: number
}

const signalBars = [0, 1, 2, 3, 4, 5, 6] as const

export function YucoreOpsPulse(props: YucoreOpsPulseProps) {
  const { t } = useYucoreTranslation()
  const stats = [
    {
      label: 'Requests',
      value: props.requestCount.toLocaleString(),
      icon: Activity,
    },
    {
      label: 'Quota',
      value: props.quota.toLocaleString(),
      icon: CreditCard,
    },
    {
      label: 'Used',
      value: props.usedQuota.toLocaleString(),
      icon: Timer,
    },
    {
      label: 'Model',
      value: props.modelName || 'ready',
      icon: RadioTower,
    },
  ]

  return (
    <section
      className={cn('yucore-ops-pulse', props.className)}
      aria-label={t('routing field')}
    >
      <span className='yucore-ops-ledger-scan' aria-hidden='true' />
      <div className='yucore-ops-ledger-lead'>
        <header className='yucore-ops-header'>
          <div className='min-w-0'>
            <div className='yucore-ops-eyebrow'>{t('Live operations')}</div>
            <h3 className='yucore-ops-title'>{t('routing field')}</h3>
          </div>
          <span className='yucore-ops-status'>{t('stable')}</span>
        </header>
        <div className='yucore-ops-signal' aria-hidden='true'>
          {signalBars.map((bar) => (
            <span className='yucore-ops-signal-bar' key={bar} />
          ))}
        </div>
      </div>

      <div className='yucore-ops-metrics'>
        {stats.map((stat) => {
          const Icon = stat.icon
          const displayValue = t(stat.value)

          return (
            <div key={stat.label} className='yucore-ops-metric min-w-0'>
              <div className='yucore-ops-metric-label'>
                <Icon className='yucore-ops-metric-icon' aria-hidden='true' />
                <span className='truncate'>{t(stat.label)}</span>
              </div>
              <div
                className='yucore-ops-metric-value truncate tabular-nums'
                title={displayValue}
              >
                {displayValue}
              </div>
            </div>
          )
        })}
      </div>
    </section>
  )
}
