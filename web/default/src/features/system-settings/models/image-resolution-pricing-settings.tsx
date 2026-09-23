/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { ImageIcon, PencilLine, Search } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

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
import type { ImageResolutionPricingMetadata } from '@/features/pricing/types'

import {
  buildImageResolutionPricingModels,
  getImageResolutionModelPolicy,
  parseImageResolutionPolicies,
  saveImageResolutionPolicy,
  type ImageResolutionPricePolicy,
} from './image-resolution-pricing'
import {
  ImageResolutionPricingDrawer,
  type ImageResolutionPricingModel,
} from './image-resolution-pricing-drawer'

type ImageResolutionPricingSettingsProps = {
  value: string
  onChange: (value: string) => Promise<void>
  isSaving: boolean
}

function metadataPolicy(
  metadata: ImageResolutionPricingMetadata
): ImageResolutionPricePolicy {
  return {
    default_tier: metadata.default_tier,
    prices: structuredClone(metadata.prices),
  }
}

export function ImageResolutionPricingSettings(
  props: ImageResolutionPricingSettingsProps
) {
  const { t } = useTranslation()
  const { models, isLoading } = usePricingData()
  const [search, setSearch] = useState('')
  const [selectedModel, setSelectedModel] =
    useState<ImageResolutionPricingModel | null>(null)
  const policies = useMemo(
    () => parseImageResolutionPolicies(props.value),
    [props.value]
  )
  const imageModels = useMemo(() => {
    const normalizedSearch = search.trim().toLowerCase()
    return buildImageResolutionPricingModels(models, policies).filter(
      (model) =>
        normalizedSearch === '' ||
        model.model_name.toLowerCase().includes(normalizedSearch)
    )
  }, [models, policies, search])

  const selectedMetadata = selectedModel?.image_resolution_pricing
  const selectedPolicy = selectedModel
    ? getImageResolutionModelPolicy(policies, selectedModel.model_name)
    : undefined
  const initialPolicy =
    selectedPolicy ??
    (selectedMetadata ? metadataPolicy(selectedMetadata) : undefined)

  async function saveModelPrices(policy: ImageResolutionPricePolicy) {
    if (!selectedModel || !selectedMetadata) return
    const next = saveImageResolutionPolicy(
      policies,
      selectedModel.model_name,
      policy,
      selectedMetadata
    )
    await props.onChange(JSON.stringify(next, null, 2))
    setSelectedModel(null)
  }

  if (isLoading) {
    return (
      <div className='flex flex-col gap-3'>
        <Skeleton className='h-8 w-full' />
        <Skeleton className='h-40 w-full' />
      </div>
    )
  }

  return (
    <div className='flex flex-col gap-4'>
      <div className='flex flex-wrap items-end justify-between gap-3'>
        <div className='max-w-2xl'>
          <h3 className='text-sm font-semibold'>
            {t('Image resolution prices')}
          </h3>
          <p className='text-muted-foreground mt-1 text-xs leading-5'>
            {t(
              'Configure 1K, 2K, and 4K prices under one official image model. The requested size selects the billing tier automatically.'
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
            placeholder={t('Search image models')}
            aria-label={t('Search image models')}
          />
        </InputGroup>
      </div>

      {imageModels.length === 0 ? (
        <Empty>
          <EmptyHeader>
            <EmptyMedia variant='icon'>
              <ImageIcon aria-hidden='true' />
            </EmptyMedia>
            <EmptyTitle>{t('No image pricing models found')}</EmptyTitle>
            <EmptyDescription>
              {t('Clear the search or enable a supported image model first.')}
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      ) : (
        <div className='overflow-x-auto rounded-lg border'>
          <Table className='min-w-[38rem]'>
            <TableHeader>
              <TableRow>
                <TableHead>{t('Model')}</TableHead>
                <TableHead>{t('Default tier')}</TableHead>
                <TableHead>{t('1K price')}</TableHead>
                <TableHead>{t('2K price')}</TableHead>
                <TableHead>{t('4K price')}</TableHead>
                <TableHead className='w-12'>
                  <span className='sr-only'>{t('Actions')}</span>
                </TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {imageModels.map((model) => {
                const metadata = model.image_resolution_pricing
                const policy =
                  getImageResolutionModelPolicy(policies, model.model_name) ??
                  metadataPolicy(metadata)
                return (
                  <TableRow key={model.model_name}>
                    <TableCell className='max-w-64 truncate font-mono font-medium'>
                      {model.model_name}
                    </TableCell>
                    <TableCell>
                      <Badge variant='outline'>
                        {policy.default_tier.toUpperCase()}
                      </Badge>
                    </TableCell>
                    {(['1k', '2k', '4k'] as const).map((tier) => (
                      <TableCell key={tier} className='font-mono text-xs'>
                        ${policy.prices[tier]}
                      </TableCell>
                    ))}
                    <TableCell>
                      <Button
                        type='button'
                        variant='ghost'
                        size='icon-sm'
                        onClick={() => setSelectedModel(model)}
                        aria-label={t('Edit image resolution prices')}
                        title={t('Edit image resolution prices')}
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

      {selectedModel && initialPolicy && (
        <ImageResolutionPricingDrawer
          key={`${selectedModel.model_name}:${props.value}`}
          open
          model={selectedModel}
          initialPolicy={initialPolicy}
          isSaving={props.isSaving}
          onOpenChange={(open) => {
            if (!open) setSelectedModel(null)
          }}
          onSave={saveModelPrices}
        />
      )}
    </div>
  )
}
