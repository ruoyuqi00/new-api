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
import { Add01Icon, Delete02Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useFieldArray, useFormContext, useWatch } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Field, FieldError, FieldGroup } from '@/components/ui/field'
import {
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'

import { safeNumberFieldProps } from '../utils/numeric-field'
import {
  getSHKeeperArrayErrorMessage,
  type SHKeeperSettingsFormValues,
} from './shkeeper-settings-model'

export function SHKeeperPackageEditor(props: { errorId?: string }) {
  const { t } = useTranslation()
  const form = useFormContext<SHKeeperSettingsFormValues>()
  const packages = useWatch({ control: form.control, name: 'packages' })
  const packageFields = useFieldArray({
    control: form.control,
    name: 'packages',
  })
  const arrayError = getSHKeeperArrayErrorMessage(
    form.formState.errors.packages
  )

  return (
    <FieldGroup className='gap-3'>
      <div className='flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between'>
        <div className='min-w-0'>
          <h4 className='text-sm font-medium'>{t('Fixed top-up packages')}</h4>
          <p className='text-muted-foreground text-sm'>
            {t('Map each exact USDT payment amount to the balance credited.')}
          </p>
        </div>
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={() =>
            packageFields.append({ usdt: 10, balance: '', label: '' })
          }
        >
          <HugeiconsIcon
            icon={Add01Icon}
            strokeWidth={2}
            data-icon='inline-start'
          />
          {t('Add package')}
        </Button>
      </div>

      {packageFields.fields.length === 0 ? (
        <p className='text-muted-foreground text-sm'>
          {t('No fixed packages configured.')}
        </p>
      ) : null}

      {packageFields.fields.map((packageField, index) => {
        const preview = packages[index]
        const summary = t('{{usdt}} USDT credits {{balance}} balance', {
          usdt: preview?.usdt || '-',
          balance: preview?.balance || '-',
        })

        return (
          <Field
            key={packageField.id}
            data-invalid={!!form.formState.errors.packages?.[index]}
            className='rounded-lg border p-3'
          >
            <div className='grid min-w-0 gap-3 sm:grid-cols-[minmax(7rem,0.7fr)_minmax(9rem,1fr)_minmax(10rem,1.4fr)_auto] sm:items-start'>
              <FormField
                control={form.control}
                name={`packages.${index}.usdt`}
                render={({ field, fieldState }) => (
                  <FormItem>
                    <FormLabel>{t('USDT amount')}</FormLabel>
                    <FormControl
                      aria-invalid={!!fieldState.error || !!arrayError}
                      aria-errormessage={
                        arrayError
                          ? (props.errorId ?? 'shkeeper-packages-error')
                          : undefined
                      }
                    >
                      <Input
                        type='number'
                        min={1}
                        step={1}
                        {...safeNumberFieldProps(field)}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name={`packages.${index}.balance`}
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Credited balance')}</FormLabel>
                    <FormControl>
                      <Input
                        inputMode='decimal'
                        placeholder={t('Balance amount')}
                        {...field}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name={`packages.${index}.label`}
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Package label (optional)')}</FormLabel>
                    <FormControl>
                      <Input placeholder={t('Starter package')} {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <Tooltip>
                <TooltipTrigger
                  render={
                    <Button
                      type='button'
                      variant='ghost'
                      size='icon'
                      className='text-destructive sm:mt-6'
                      onClick={() => packageFields.remove(index)}
                      aria-label={t('Remove package {{number}}', {
                        number: index + 1,
                      })}
                    />
                  }
                >
                  <HugeiconsIcon icon={Delete02Icon} strokeWidth={2} />
                </TooltipTrigger>
                <TooltipContent>{t('Remove package')}</TooltipContent>
              </Tooltip>
            </div>

            <div className='flex min-w-0 flex-wrap items-center gap-2'>
              <Badge variant='outline'>{t('Preview')}</Badge>
              <p className='text-muted-foreground min-w-0 text-sm break-words'>
                {preview?.label
                  ? t('Package preview: {{summary}} - {{label}}', {
                      summary,
                      label: preview.label,
                    })
                  : t('Package preview: {{summary}}', { summary })}
              </p>
            </div>
          </Field>
        )
      })}

      <FieldError id={props.errorId ?? 'shkeeper-packages-error'}>
        {arrayError}
      </FieldError>
    </FieldGroup>
  )
}
