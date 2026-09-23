/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { useTranslation } from 'react-i18next'

import { StaticDataTable } from '@/components/data-table'
import { GroupBadge } from '@/components/group-badge'
import { StatusBadge } from '@/components/status-badge'

import {
  buildImageResolutionRows,
  formatImageResolutionPrice,
} from '../lib/image-resolution-price'
import type { ImageResolutionPricingMetadata } from '../types'

type ImageResolutionPricingBreakdownProps = {
  metadata: ImageResolutionPricingMetadata
  groups: string[]
  groupRatios: Record<string, number>
  showRechargePrice: boolean
  priceRate: number
  usdExchangeRate: number
}

const BASE_GROUP_KEY = '__base'

export function ImageResolutionPricingBreakdown(
  props: ImageResolutionPricingBreakdownProps
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
  const rows = buildImageResolutionRows(
    props.metadata,
    Object.fromEntries(displayGroups.map((group) => [group.key, group.ratio]))
  )
  const formatPrice = (price: number) =>
    formatImageResolutionPrice(price, {
      showRechargePrice: props.showRechargePrice,
      priceRate: props.priceRate,
      usdExchangeRate: props.usdExchangeRate,
    })

  return (
    <div className='space-y-3'>
      <div className='flex flex-wrap items-center gap-2'>
        <StatusBadge
          label={t('Per image')}
          variant='info'
          copyable={false}
          size='sm'
        />
        <StatusBadge
          label={t('Automatic resolution billing')}
          variant='success'
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
            tableClassName='min-w-80'
            headerRowClassName='hover:bg-transparent'
            data={rows}
            getRowKey={(row) => `${group.key}-${row.tier}`}
            columns={[
              {
                id: 'resolution',
                header: t('Resolution'),
                className:
                  'text-muted-foreground py-2 text-[10px] font-medium tracking-wider uppercase',
                cellClassName:
                  'text-muted-foreground py-2.5 font-mono font-medium',
                cell: (row) => row.tier.toUpperCase(),
              },
              {
                id: 'price',
                header: t('Price per image'),
                className:
                  'text-muted-foreground py-2 text-right text-[10px] font-medium tracking-wider uppercase',
                cellClassName: 'py-2.5 text-right font-mono',
                cell: (row) => formatPrice(row.prices[group.key]),
              },
            ]}
          />
        </div>
      ))}
    </div>
  )
}
