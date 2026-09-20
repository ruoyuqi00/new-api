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
import { PencilLine, Search, SlidersHorizontal } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import {
  InputGroup,
  InputGroupAddon,
  InputGroupInput,
} from '@/components/ui/input-group'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { usePricingData } from '@/features/pricing/hooks'
import type {
  PricingModel,
  VideoBillingUnit,
  VideoTierPricingMetadata,
} from '@/features/pricing/types'

import {
  getVideoTierModelOverride,
  parseVideoTierOverrides,
  removeVideoTierOverride,
  saveVideoTierOverride,
  type VideoTierModelPrices,
} from './video-tier-pricing'
import {
  VideoTierPricingDrawer,
  type VideoTierPricingModel,
} from './video-tier-pricing-drawer'

type VideoTierPricingSettingsProps = {
  value: string
  onChange: (value: string) => Promise<void>
  isSaving: boolean
}

function unitLabelKey(unit: VideoBillingUnit): string {
  if (unit === 'per_second') return 'Per second'
  if (unit === 'per_successful_task') return 'Per successful task'
  return 'Per 1M video tokens'
}

function hasVideoTierPricing<T extends PricingModel>(
  model: T
): model is T & { video_tier_pricing: VideoTierPricingMetadata } {
  return model.video_tier_pricing !== undefined
}

export function VideoTierPricingSettings(props: VideoTierPricingSettingsProps) {
  const { t } = useTranslation()
  const { models, isLoading } = usePricingData()
  const [search, setSearch] = useState('')
  const [selectedModel, setSelectedModel] =
    useState<VideoTierPricingModel | null>(null)
  const [resetOpen, setResetOpen] = useState(false)
  const overrides = useMemo(
    () => parseVideoTierOverrides(props.value),
    [props.value]
  )
  const videoModels = useMemo(() => {
    const normalizedSearch = search.trim().toLowerCase()
    return models
      .filter(hasVideoTierPricing)
      .filter(
        (model) =>
          normalizedSearch === '' ||
          model.model_name.toLowerCase().includes(normalizedSearch)
      )
      .sort((left, right) => left.model_name.localeCompare(right.model_name))
  }, [models, search])

  const selectedMetadata = selectedModel?.video_tier_pricing
  const selectedOverride = selectedModel
    ? getVideoTierModelOverride(overrides, selectedModel.model_name)
    : undefined
  const initialPrices = selectedOverride ?? selectedMetadata?.prices

  async function saveModelPrices(prices: VideoTierModelPrices) {
    if (!selectedModel || !selectedMetadata) return
    const next = saveVideoTierOverride(
      overrides,
      selectedModel.model_name,
      prices,
      selectedMetadata
    )
    await props.onChange(JSON.stringify(next, null, 2))
    setSelectedModel(null)
  }

  async function resetModelPrices() {
    if (!selectedModel) return
    const next = removeVideoTierOverride(overrides, selectedModel.model_name)
    await props.onChange(JSON.stringify(next, null, 2))
    setResetOpen(false)
    setSelectedModel(null)
  }

  if (isLoading) {
    return (
      <div className='flex flex-col gap-3'>
        <Skeleton className='h-8 w-full' />
        <Skeleton className='h-48 w-full' />
      </div>
    )
  }

  return (
    <div className='flex flex-col gap-4'>
      <div className='flex flex-wrap items-end justify-between gap-3'>
        <div className='max-w-2xl'>
          <h3 className='text-sm font-semibold'>{t('Video tier prices')}</h3>
          <p className='text-muted-foreground mt-1 text-xs leading-5'>
            {t(
              'Configure exact prices by resolution. Models without an override continue to follow their base price.'
            )}
          </p>
        </div>
        <InputGroup className='w-full sm:w-72'>
          <InputGroupAddon>
            <Search aria-hidden='true' />
          </InputGroupAddon>
          <InputGroupInput
            value={search}
            onChange={(event) => setSearch(event.target.value)}
            placeholder={t('Search video models')}
            aria-label={t('Search video models')}
          />
        </InputGroup>
      </div>

      {videoModels.length === 0 ? (
        <Empty>
          <EmptyHeader>
            <EmptyMedia variant='icon'>
              <SlidersHorizontal aria-hidden='true' />
            </EmptyMedia>
            <EmptyTitle>{t('No video pricing models found')}</EmptyTitle>
            <EmptyDescription>
              {t('Clear the search or enable a supported video model first.')}
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      ) : (
        <div className='rounded-lg border'>
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Model')}</TableHead>
                <TableHead>{t('Billing unit')}</TableHead>
                <TableHead>{t('Price source')}</TableHead>
                <TableHead>{t('Resolution tiers')}</TableHead>
                <TableHead className='w-12'>
                  <span className='sr-only'>{t('Actions')}</span>
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {videoModels.map((model) => {
                const metadata = model.video_tier_pricing
                const explicit = Boolean(
                  getVideoTierModelOverride(overrides, model.model_name)
                )
                return (
                  <TableRow key={model.model_name}>
                    <TableCell className='max-w-64 truncate font-mono font-medium'>
                      {model.model_name}
                    </TableCell>
                    <TableCell>
                      <Badge variant='outline'>
                        {t(unitLabelKey(metadata.billing_unit))}
                      </Badge>
                    </TableCell>
                    <TableCell>
                      <Badge variant={explicit ? 'secondary' : 'outline'}>
                        {explicit ? t('Explicit') : t('Inherited')}
                      </Badge>
                    </TableCell>
                    <TableCell className='font-mono text-xs'>
                      {metadata.tiers
                        .map((tier) => tier.toUpperCase())
                        .join(' / ')}
                    </TableCell>
                    <TableCell>
                      <Button
                        type='button'
                        variant='ghost'
                        size='icon-sm'
                        onClick={() => setSelectedModel(model)}
                        aria-label={t('Edit video tier prices')}
                        title={t('Edit video tier prices')}
                      >
                        <PencilLine />
                      </Button>
                    </TableCell>
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
        </div>
      )}

      {selectedModel && selectedMetadata && initialPrices && (
        <VideoTierPricingDrawer
          key={`${selectedModel.model_name}:${props.value}`}
          open
          model={selectedModel}
          initialPrices={initialPrices}
          explicit={Boolean(selectedOverride)}
          isSaving={props.isSaving}
          onOpenChange={(open) => {
            if (!open) setSelectedModel(null)
          }}
          onSave={saveModelPrices}
          onRequestReset={() => setResetOpen(true)}
        />
      )}

      <ConfirmDialog
        open={resetOpen}
        onOpenChange={setResetOpen}
        title={t('Use inherited video prices?')}
        desc={t(
          'This removes the independent tier prices for this model and returns every resolution to base-price scaling.'
        )}
        confirmText={t('Use inherited prices')}
        isLoading={props.isSaving}
        handleConfirm={resetModelPrices}
      />
    </div>
  )
}
