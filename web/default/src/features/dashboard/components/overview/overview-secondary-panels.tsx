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
  CardStaggerContainer,
  CardStaggerItem,
} from '@/components/page-transition'
import { cn } from '@/lib/utils'

import { AnnouncementsPanel } from './announcements-panel'
import { ApiInfoPanel } from './api-info-panel'
import { FAQPanel } from './faq-panel'
import type { OverviewPanelPlan } from './overview-panel-plan'
import { PerformanceHealthPanel } from './performance-health-panel'
import { SummaryCards } from './summary-cards'
import { UptimePanel } from './uptime-panel'

export function OverviewSecondaryPanels(props: { plan: OverviewPanelPlan }) {
  const hasLeftPanels = props.plan.left.length > 0
  const showContentPanels = hasLeftPanels || props.plan.uptime

  return (
    <>
      <SummaryCards />
      {showContentPanels && (
        <CardStaggerContainer
          className={cn(
            'grid grid-cols-1 gap-4',
            hasLeftPanels &&
              props.plan.uptime &&
              'xl:grid-cols-[minmax(0,1fr)_22rem]'
          )}
        >
          {hasLeftPanels && (
            <div
              className={cn(
                'grid min-w-0 grid-cols-1 gap-4',
                props.plan.left.some((panel) => panel !== 'performance') &&
                  'lg:grid-cols-2'
              )}
            >
              {props.plan.left.includes('performance') && (
                <CardStaggerItem className='lg:col-span-2'>
                  <PerformanceHealthPanel />
                </CardStaggerItem>
              )}
              {props.plan.left.includes('api-info') && (
                <CardStaggerItem>
                  <ApiInfoPanel />
                </CardStaggerItem>
              )}
              {props.plan.left.includes('announcements') && (
                <CardStaggerItem>
                  <AnnouncementsPanel />
                </CardStaggerItem>
              )}
              {props.plan.left.includes('faq') && (
                <CardStaggerItem>
                  <FAQPanel />
                </CardStaggerItem>
              )}
            </div>
          )}
          {props.plan.uptime && (
            <CardStaggerItem>
              <UptimePanel />
            </CardStaggerItem>
          )}
        </CardStaggerContainer>
      )}
    </>
  )
}
