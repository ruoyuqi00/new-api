/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { zodResolver } from '@hookform/resolvers/zod'
import { Save } from 'lucide-react'
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
  ImageResolutionPricingMetadata,
  PricingModel,
} from '@/features/pricing/types'

import type { ImageResolutionPricePolicy } from './image-resolution-pricing'

type ImageResolutionFormValues = {
  prices: Record<string, number>
}

export type ImageResolutionPricingModel = PricingModel & {
  image_resolution_pricing: ImageResolutionPricingMetadata
}

type ImageResolutionPricingDrawerProps = {
  open: boolean
  model: ImageResolutionPricingModel
  initialPolicy: ImageResolutionPricePolicy
  isSaving: boolean
  onOpenChange: (open: boolean) => void
  onSave: (policy: ImageResolutionPricePolicy) => Promise<void>
}

function createImageResolutionSchema(
  metadata: ImageResolutionPricingMetadata,
  positiveMessage: string,
  requiredMessage: string,
  monotonicMessage: string
) {
  const positivePrice = z
    .number({ error: positiveMessage })
    .refine((value) => Number.isFinite(value) && value > 0, positiveMessage)

  return z
    .object({ prices: z.record(z.string(), positivePrice) })
    .superRefine((values, context) => {
      let previousPrice: number | undefined
      for (const tier of Object.keys(metadata.prices)) {
        const price = values.prices[tier]
        if (price === undefined) {
          context.addIssue({
            code: 'custom',
            path: ['prices', tier],
            message: requiredMessage,
          })
          continue
        }
        if (previousPrice !== undefined && price < previousPrice) {
          context.addIssue({
            code: 'custom',
            path: ['prices', tier],
            message: monotonicMessage,
          })
        }
        previousPrice = price
      }
    })
}

export function ImageResolutionPricingDrawer(
  props: ImageResolutionPricingDrawerProps
) {
  const { t } = useTranslation()
  const metadata = props.model.image_resolution_pricing
  const schema = useMemo(
    () =>
      createImageResolutionSchema(
        metadata,
        t('Must be greater than zero'),
        t('Required'),
        t('Higher tiers cannot cost less than lower tiers')
      ),
    [metadata, t]
  )
  const form = useForm<ImageResolutionFormValues>({
    resolver: zodResolver(schema),
    mode: 'onChange',
    defaultValues: { prices: structuredClone(props.initialPolicy.prices) },
  })
  const handleSubmit = form.handleSubmit(async (values) => {
    await props.onSave({
      default_tier: metadata.default_tier,
      prices: values.prices,
    })
  })

  return (
    <Sheet open={props.open} onOpenChange={props.onOpenChange}>
      <SheetContent
        side='right'
        className={sideDrawerContentClassName('sm:max-w-xl')}
      >
        <SheetHeader className={sideDrawerHeaderClassName('pr-14 sm:pr-14')}>
          <div className='flex min-w-0 items-start justify-between gap-3'>
            <div className='min-w-0'>
              <SheetTitle className='truncate font-mono text-base'>
                {props.model.model_name}
              </SheetTitle>
              <SheetDescription className='mt-1'>
                {t('Set the base per-image price for each resolution tier.')}
              </SheetDescription>
            </div>
            <Badge variant='secondary'>{t('Per image')}</Badge>
          </div>
        </SheetHeader>

        <Form {...form}>
          <form
            onSubmit={handleSubmit}
            className='flex min-h-0 flex-1 flex-col'
          >
            <div className={sideDrawerFormClassName()}>
              <div className='grid gap-4'>
                {Object.keys(metadata.prices).map((tier) => {
                  const fieldName =
                    `prices.${tier}` as Path<ImageResolutionFormValues>
                  return (
                    <FormField
                      key={tier}
                      control={form.control}
                      name={fieldName}
                      render={({ field, fieldState }) => (
                        <FormItem className='grid grid-cols-[4rem_minmax(0,1fr)] items-start gap-x-3'>
                          <FormLabel className='flex h-8 items-center font-mono text-sm font-semibold'>
                            {tier.toUpperCase()}
                          </FormLabel>
                          <div className='min-w-0'>
                            <FormControl>
                              <InputGroup>
                                <InputGroupAddon>$</InputGroupAddon>
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
                                  aria-label={`${tier} ${t('Base price')}`}
                                  aria-invalid={Boolean(fieldState.error)}
                                />
                                <InputGroupAddon align='inline-end'>
                                  {t('per image')}
                                </InputGroupAddon>
                              </InputGroup>
                            </FormControl>
                            <FormMessage />
                          </div>
                        </FormItem>
                      )}
                    />
                  )
                })}
              </div>
            </div>

            <div className={sideDrawerFooterClassName()}>
              <Button type='submit' disabled={props.isSaving}>
                <Save data-icon='inline-start' />
                {props.isSaving ? t('Saving...') : t('Save resolution prices')}
              </Button>
            </div>
          </form>
        </Form>
      </SheetContent>
    </Sheet>
  )
}
