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
import { useTranslation } from 'react-i18next'

import { StaticDataTable } from '@/components/data-table'
import { GroupBadge } from '@/components/group-badge'
import { StatusBadge } from '@/components/status-badge'

import {
  buildVideoTierRows,
  formatVideoTierPrice,
  getVideoBillingUnitLabelKey,
} from '../lib/video-tier-price'
import type { VideoTierPricingMetadata } from '../types'

type VideoTierPricingBreakdownProps = {
  metadata: VideoTierPricingMetadata
  groups: string[]
  groupRatios: Record<string, number>
  showRechargePrice: boolean
  priceRate: number
  usdExchangeRate: number
}

const BASE_GROUP_KEY = '__base'

export function VideoTierPricingBreakdown(
  props: VideoTierPricingBreakdownProps
) {
  const { t } = useTranslation()
  const displayGroups =
    props.groups.length > 0
      ? props.groups.map((group) => ({
          key: group,
          label: group,
          ratio: props.groupRatios[group] ?? 1,
        }))
      : [{ key: BASE_GROUP_KEY, label: t('Base Price'), ratio: 1 }]
  const displayRatios = Object.fromEntries(
    displayGroups.map((group) => [group.key, group.ratio])
  )
  const rows = buildVideoTierRows(props.metadata, displayRatios)
  const hasReferencePrices = rows.some(
    (row) => row.withReferenceVideo !== undefined
  )
  const formatPrice = (price: number | undefined) =>
    price === undefined
      ? '-'
      : formatVideoTierPrice(price, {
          showRechargePrice: props.showRechargePrice,
          priceRate: props.priceRate,
          usdExchangeRate: props.usdExchangeRate,
        })
  const headerClassName =
    'text-muted-foreground py-2 text-[10px] font-medium tracking-wider uppercase'

  return (
    <div className='space-y-3'>
      <div className='flex flex-wrap items-center gap-2'>
        <StatusBadge
          label={t(getVideoBillingUnitLabelKey(props.metadata.billing_unit))}
          variant='info'
          copyable={false}
          size='sm'
        />
        <StatusBadge
          label={t(props.metadata.inherited ? 'Inherited' : 'Explicit')}
          variant={props.metadata.inherited ? 'neutral' : 'success'}
          copyable={false}
          size='sm'
        />
      </div>

      {displayGroups.map((group) => (
        <div key={group.key} className='overflow-hidden rounded-lg border'>
          <div className='bg-muted/20 flex items-center border-b px-3 py-2'>
            {group.key === BASE_GROUP_KEY ? (
              <span className='text-muted-foreground text-xs font-medium'>
                {group.label}
              </span>
            ) : (
              <GroupBadge group={group.label} ratio={group.ratio} size='sm' />
            )}
          </div>
          <StaticDataTable
            className='rounded-none border-0'
            tableClassName={hasReferencePrices ? 'min-w-[34rem]' : 'min-w-80'}
            headerRowClassName='hover:bg-transparent'
            data={rows}
            getRowKey={(row) => `${group.key}-${row.tier}`}
            columns={[
              {
                id: 'resolution',
                header: t('Resolution'),
                className: headerClassName,
                cellClassName: 'text-muted-foreground py-2.5 font-medium',
                cell: (row) => row.tier,
              },
              {
                id: 'standard',
                header: t('Standard'),
                className: `${headerClassName} text-right`,
                cellClassName: 'py-2.5 text-right font-mono',
                cell: (row) => formatPrice(row.standard[group.key]),
              },
              ...(hasReferencePrices
                ? [
                    {
                      id: 'with-reference-video',
                      header: t('With reference video'),
                      className: `${headerClassName} text-right`,
                      cellClassName: 'py-2.5 text-right font-mono',
                      cell: (row: (typeof rows)[number]) =>
                        formatPrice(row.withReferenceVideo?.[group.key]),
                    },
                  ]
                : []),
            ]}
          />
        </div>
      ))}
    </div>
  )
}
