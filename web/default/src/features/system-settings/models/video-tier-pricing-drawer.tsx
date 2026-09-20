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
import { zodResolver } from '@hookform/resolvers/zod'
import { Save, Undo2 } from 'lucide-react'
import { useMemo } from 'react'
import { type Path, useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

import {
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import {
  InputGroup,
  InputGroupAddon,
  InputGroupInput,
} from '@/components/ui/input-group'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import type {
  PricingModel,
  VideoBillingUnit,
  VideoTierPricingMetadata,
} from '@/features/pricing/types'

import type { VideoTierModelPrices } from './video-tier-pricing'

type VideoTierFormValues = {
  prices: VideoTierModelPrices
}

export type VideoTierPricingModel = PricingModel & {
  video_tier_pricing: VideoTierPricingMetadata
}

type VideoTierPricingDrawerProps = {
  open: boolean
  model: VideoTierPricingModel
  initialPrices: VideoTierModelPrices
  explicit: boolean
  isSaving: boolean
  onOpenChange: (open: boolean) => void
  onSave: (prices: VideoTierModelPrices) => Promise<void>
  onRequestReset: () => void
}

function unitLabelKey(unit: VideoBillingUnit): string {
  if (unit === 'per_second') return 'per second'
  if (unit === 'per_successful_task') return 'per successful task'
  return 'per 1M video tokens'
}

function createVideoTierSchema(
  metadata: VideoTierPricingMetadata,
  positiveMessage: string,
  requiredMessage: string
) {
  const positivePrice = z
    .number({ error: positiveMessage })
    .refine((value) => Number.isFinite(value) && value > 0, positiveMessage)

  return z
    .object({
      prices: z.record(
        z.string(),
        z.object({
          standard: positivePrice,
          with_reference_video: positivePrice.optional(),
        })
      ),
    })
    .superRefine((values, context) => {
      for (const tier of metadata.tiers) {
        const point = values.prices[tier]
        if (!point) {
          context.addIssue({
            code: 'custom',
            path: ['prices', tier, 'standard'],
            message: requiredMessage,
          })
          continue
        }
        if (
          metadata.prices[tier]?.with_reference_video !== undefined &&
          point.with_reference_video === undefined
        ) {
          context.addIssue({
            code: 'custom',
            path: ['prices', tier, 'with_reference_video'],
            message: requiredMessage,
          })
        }
      }
    })
}

export function VideoTierPricingDrawer(props: VideoTierPricingDrawerProps) {
  const { t } = useTranslation()
  const metadata = props.model.video_tier_pricing

  const schema = useMemo(
    () =>
      createVideoTierSchema(
        metadata,
        t('Must be greater than zero'),
        t('Required')
      ),
    [metadata, t]
  )
  const form = useForm<VideoTierFormValues>({
    resolver: zodResolver(schema),
    defaultValues: { prices: structuredClone(props.initialPrices) },
  })

  const handleSubmit = form.handleSubmit(async (values) => {
    await props.onSave(values.prices)
  })

  return (
    <Sheet open={props.open} onOpenChange={props.onOpenChange}>
      <SheetContent
        side='right'
        className={sideDrawerContentClassName('sm:max-w-2xl')}
      >
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <div className='flex min-w-0 items-start justify-between gap-3'>
            <div className='min-w-0'>
              <SheetTitle className='truncate font-mono text-base'>
                {props.model.model_name}
              </SheetTitle>
              <SheetDescription className='mt-1'>
                {t('Set an independent price for every supported video tier.')}
              </SheetDescription>
            </div>
            <Badge variant={props.explicit ? 'secondary' : 'outline'}>
              {props.explicit ? t('Explicit tier prices') : t('Inherited')}
            </Badge>
          </div>
        </SheetHeader>

        <Form {...form}>
          <form
            onSubmit={handleSubmit}
            className='flex min-h-0 flex-1 flex-col'
          >
            <div className={sideDrawerFormClassName()}>
              <div className='overflow-x-auto pb-1'>
                <div className='grid min-w-[34rem] grid-cols-[7rem_minmax(10rem,1fr)_minmax(10rem,1fr)] gap-x-3 gap-y-4'>
                  <div className='text-muted-foreground text-xs font-medium'>
                    {t('Resolution')}
                  </div>
                  <div className='text-muted-foreground text-xs font-medium'>
                    {t('Standard price')}
                  </div>
                  <div className='text-muted-foreground text-xs font-medium'>
                    {t('With reference video')}
                  </div>

                  {metadata.tiers.map((tier) => {
                    const standardName =
                      `prices.${tier}.standard` as Path<VideoTierFormValues>
                    const referenceName =
                      `prices.${tier}.with_reference_video` as Path<VideoTierFormValues>
                    const hasReferencePrice =
                      metadata.prices[tier]?.with_reference_video !== undefined

                    return (
                      <div key={tier} className='contents'>
                        <div className='flex h-8 items-center font-mono text-sm font-medium'>
                          {tier.toUpperCase()}
                        </div>
                        <FormField
                          control={form.control}
                          name={standardName}
                          render={({ field }) => (
                            <FormItem>
                              <FormLabel className='sr-only'>
                                {tier} {t('Standard price')}
                              </FormLabel>
                              <FormControl>
                                <InputGroup>
                                  <InputGroupInput
                                    type='number'
                                    min='0'
                                    step='any'
                                    inputMode='decimal'
                                    value={
                                      typeof field.value === 'number' &&
                                      Number.isFinite(field.value)
                                        ? field.value
                                        : ''
                                    }
                                    onChange={(event) =>
                                      field.onChange(
                                        event.target.value === ''
                                          ? Number.NaN
                                          : Number(event.target.value)
                                      )
                                    }
                                    aria-invalid={Boolean(
                                      form.getFieldState(standardName).error
                                    )}
                                  />
                                  <InputGroupAddon align='inline-end'>
                                    {t(unitLabelKey(metadata.billing_unit))}
                                  </InputGroupAddon>
                                </InputGroup>
                              </FormControl>
                              <FormMessage />
                            </FormItem>
                          )}
                        />
                        {hasReferencePrice ? (
                          <FormField
                            control={form.control}
                            name={referenceName}
                            render={({ field }) => (
                              <FormItem>
                                <FormLabel className='sr-only'>
                                  {tier} {t('With reference video')}
                                </FormLabel>
                                <FormControl>
                                  <InputGroup>
                                    <InputGroupInput
                                      type='number'
                                      min='0'
                                      step='any'
                                      inputMode='decimal'
                                      value={
                                        typeof field.value === 'number' &&
                                        Number.isFinite(field.value)
                                          ? field.value
                                          : ''
                                      }
                                      onChange={(event) =>
                                        field.onChange(
                                          event.target.value === ''
                                            ? undefined
                                            : Number(event.target.value)
                                        )
                                      }
                                      aria-invalid={Boolean(
                                        form.getFieldState(referenceName).error
                                      )}
                                    />
                                    <InputGroupAddon align='inline-end'>
                                      {t(unitLabelKey(metadata.billing_unit))}
                                    </InputGroupAddon>
                                  </InputGroup>
                                </FormControl>
                                <FormMessage />
                              </FormItem>
                            )}
                          />
                        ) : (
                          <div className='text-muted-foreground flex h-8 items-center text-sm'>
                            {t('Not applicable')}
                          </div>
                        )}
                      </div>
                    )
                  })}
                </div>
              </div>
            </div>

            <div className={sideDrawerFooterClassName()}>
              {props.explicit && (
                <Button
                  type='button'
                  variant='outline'
                  onClick={props.onRequestReset}
                  disabled={props.isSaving}
                >
                  <Undo2 data-icon='inline-start' />
                  {t('Use inherited prices')}
                </Button>
              )}
              <Button type='submit' disabled={props.isSaving}>
                <Save data-icon='inline-start' />
                {props.isSaving ? t('Saving...') : t('Save tier prices')}
              </Button>
            </div>
          </form>
        </Form>
      </SheetContent>
    </Sheet>
  )
}
