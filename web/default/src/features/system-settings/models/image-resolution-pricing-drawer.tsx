/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.
*/
import { zodResolver } from '@hookform/resolvers/zod'
import { Save } from 'lucide-react'
import { useId, useMemo } from 'react'
import { useForm } from 'react-hook-form'
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
  FormDescription,
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
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import type {
  ImageResolutionTier,
  PricingModel,
} from '@/features/pricing/types'

import {
  IMAGE_RESOLUTION_TIERS,
  normalizeImageResolutionModelName,
  type ImageResolutionPricePolicy,
} from './image-resolution-pricing'

type ImageResolutionFormValues = {
  model_name: string
  default_tier: ImageResolutionTier
  prices: Record<string, number>
}

type ImageResolutionPricingDrawerProps = {
  open: boolean
  model?: PricingModel
  availableModels: string[]
  configuredModels: string[]
  initialPolicy: ImageResolutionPricePolicy
  isSaving: boolean
  onOpenChange: (open: boolean) => void
  onSave: (
    modelName: string,
    policy: ImageResolutionPricePolicy
  ) => Promise<void>
}

function createImageResolutionSchema(
  isCreating: boolean,
  configuredModels: string[],
  positiveMessage: string,
  requiredMessage: string,
  monotonicMessage: string,
  duplicateMessage: string
) {
  const positivePrice = z
    .number({ error: positiveMessage })
    .refine((value) => Number.isFinite(value) && value > 0, positiveMessage)

  return z
    .object({
      model_name: z
        .string()
        .trim()
        .refine(
          (name) => normalizeImageResolutionModelName(name) !== '',
          requiredMessage
        )
        .refine(
          (name) =>
            !isCreating ||
            !configuredModels.includes(normalizeImageResolutionModelName(name)),
          duplicateMessage
        ),
      default_tier: z.enum(IMAGE_RESOLUTION_TIERS),
      prices: z.record(z.string(), positivePrice),
    })
    .superRefine((values, context) => {
      let previousPrice: number | undefined
      for (const tier of IMAGE_RESOLUTION_TIERS) {
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
  const suggestionsId = useId()
  const isCreating = !props.model
  const schema = useMemo(
    () =>
      createImageResolutionSchema(
        isCreating,
        props.configuredModels,
        t('Must be greater than zero'),
        t('Required'),
        t('Higher tiers cannot cost less than lower tiers'),
        t('Image resolution prices already configured')
      ),
    [isCreating, props.configuredModels, t]
  )
  const form = useForm<ImageResolutionFormValues>({
    resolver: zodResolver(schema),
    mode: 'onChange',
    defaultValues: {
      model_name: props.model?.model_name ?? '',
      default_tier: props.initialPolicy.default_tier as ImageResolutionTier,
      prices: structuredClone(props.initialPolicy.prices),
    },
  })
  const handleSubmit = form.handleSubmit(async (values) => {
    await props.onSave(values.model_name, {
      default_tier: values.default_tier,
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
                {props.model?.model_name ?? t('Add image model')}
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
                {isCreating && (
                  <FormField
                    control={form.control}
                    name='model_name'
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>{t('Model name')}</FormLabel>
                        <FormControl>
                          <InputGroup>
                            <InputGroupInput
                              {...field}
                              list={suggestionsId}
                              aria-label={t('Model name')}
                              placeholder={t(
                                'Select or enter an image model name'
                              )}
                              autoComplete='off'
                            />
                          </InputGroup>
                        </FormControl>
                        <datalist id={suggestionsId}>
                          {props.availableModels.map((name) => (
                            <option key={name} value={name} />
                          ))}
                        </datalist>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                )}
                <FormField
                  control={form.control}
                  name='default_tier'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Default tier')}</FormLabel>
                      <Select
                        value={field.value}
                        onValueChange={field.onChange}
                      >
                        <FormControl>
                          <SelectTrigger>
                            <SelectValue>
                              {(value) => String(value).toUpperCase()}
                            </SelectValue>
                          </SelectTrigger>
                        </FormControl>
                        <SelectContent>
                          <SelectGroup>
                            {IMAGE_RESOLUTION_TIERS.map((tier) => (
                              <SelectItem key={tier} value={tier}>
                                {tier.toUpperCase()}
                              </SelectItem>
                            ))}
                          </SelectGroup>
                        </SelectContent>
                      </Select>
                      <FormDescription>
                        {t(
                          'Default tier is used when size is omitted or auto.'
                        )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                {IMAGE_RESOLUTION_TIERS.map((tier) => {
                  const fieldName = `prices.${tier}` as const
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
                                  name={field.name}
                                  ref={field.ref}
                                  onBlur={field.onBlur}
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
                <p className='text-muted-foreground text-xs'>
                  {t('Prices apply per image before group ratios.')}
                </p>
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
