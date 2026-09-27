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
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import * as React from 'react'
import { useForm, type Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Field,
  FieldContent,
  FieldGroup,
  FieldLabel,
  FieldLegend,
  FieldSet,
} from '@/components/ui/field'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Spinner } from '@/components/ui/spinner'
import { Switch } from '@/components/ui/switch'

import { getTokenPaySettings, saveTokenPaySettings } from '../api'
import type { TokenPayNetwork } from '../types'
import { SHKeeperPackageEditor } from './shkeeper-package-editor'
import {
  buildTokenPaySettingsRequest,
  createTokenPaySettingsSchema,
  tokenPayFormDefaults,
  tokenPayNetworks,
  type TokenPaySettingsFormValues,
} from './tokenpay-settings-model'

export type TokenPaySettingsHandle = { save: () => Promise<void> }

function TokenPaySettingsSectionComponent(
  _props: Record<never, never>,
  ref: React.ForwardedRef<TokenPaySettingsHandle>
) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const settingsQuery = useQuery({
    queryKey: ['tokenpay-settings'],
    queryFn: ({ signal }) => getTokenPaySettings(signal),
    refetchOnWindowFocus: false,
  })
  const configured = Boolean(settingsQuery.data?.data?.api_token_configured)
  const networkLabels: Record<TokenPayNetwork, string> = {
    USDT_TRC20: 'TRON (TRC20)',
    EVM_BSC_USDT_BEP20: 'BSC (BEP20)',
    EVM_Polygon_USDT_ERC20: 'Polygon',
  }
  const schema = React.useMemo(
    () => createTokenPaySettingsSchema(configured),
    [configured]
  )
  const form = useForm<TokenPaySettingsFormValues>({
    resolver: zodResolver(schema) as Resolver<TokenPaySettingsFormValues>,
    mode: 'onChange',
    defaultValues: tokenPayFormDefaults(),
  })

  React.useEffect(() => {
    if (
      settingsQuery.data?.success &&
      settingsQuery.data.data &&
      !form.formState.isDirty
    ) {
      form.reset(tokenPayFormDefaults(settingsQuery.data.data))
    }
  }, [settingsQuery.data, form, form.formState.isDirty])

  const saveMutation = useMutation({ mutationFn: saveTokenPaySettings })
  const saveSettings = React.useCallback(async () => {
    await form.handleSubmit(
      async (values) => {
        if (!settingsQuery.data?.success || !settingsQuery.data.data) {
          toast.error(t('Unable to load TokenPay settings'))
          throw new Error('TokenPay settings are not loaded')
        }
        try {
          const response = await saveMutation.mutateAsync(
            buildTokenPaySettingsRequest(values)
          )
          if (!response.success || !response.data) {
            throw new Error(
              response.message || 'Unable to save TokenPay settings'
            )
          }
          form.reset(tokenPayFormDefaults(response.data))
          queryClient.setQueryData(['tokenpay-settings'], response)
          toast.success(t('TokenPay settings saved'))
        } catch (error) {
          toast.error(t('Failed to save TokenPay settings'))
          throw error
        }
      },
      () => {
        toast.error(t('Fix validation errors before saving'))
        throw new Error('Invalid TokenPay settings')
      }
    )()
  }, [form, queryClient, saveMutation, settingsQuery.data, t])
  React.useImperativeHandle(ref, () => ({ save: saveSettings }), [saveSettings])

  if (settingsQuery.isLoading) {
    return (
      <div className='flex items-center gap-2 py-6 text-sm'>
        <Spinner />
        {t('Loading settings...')}
      </div>
    )
  }
  if (settingsQuery.isError || settingsQuery.data?.success === false) {
    return (
      <Alert variant='destructive'>
        <AlertTitle>{t('Unable to load TokenPay settings')}</AlertTitle>
      </Alert>
    )
  }

  return (
    <Form {...form}>
      <div className='flex flex-col gap-6 pt-4'>
        <div className='flex flex-wrap items-start justify-between gap-3'>
          <h3 className='text-lg font-medium'>TokenPay</h3>
          <Badge variant={form.watch('enabled') ? 'default' : 'secondary'}>
            {form.watch('enabled') ? t('Enabled') : t('Disabled')}
          </Badge>
        </div>
        <Alert>
          <AlertTitle>{t('Write-only credentials')}</AlertTitle>
          <AlertDescription>
            {t('Saved token is never shown. Leave the field blank to keep it.')}
          </AlertDescription>
        </Alert>

        <FormField
          control={form.control}
          name='enabled'
          render={({ field }) => (
            <FormItem className='flex flex-row items-center justify-between gap-4 py-2.5'>
              <FormLabel>{t('Enable TokenPay')}</FormLabel>
              <FormControl>
                <Switch
                  checked={field.value}
                  onCheckedChange={field.onChange}
                />
              </FormControl>
            </FormItem>
          )}
        />

        <FieldGroup>
          <div className='grid gap-6 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='base_url'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Base URL')}</FormLabel>
                  <FormControl>
                    <Input
                      type='url'
                      placeholder='https://pay.example.com'
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
            <FormField
              control={form.control}
              name='api_token'
              render={({ field }) => (
                <FormItem>
                  <div className='flex items-center gap-2'>
                    <FormLabel>{t('API token')}</FormLabel>
                    {configured && (
                      <Badge variant='secondary'>{t('Configured')}</Badge>
                    )}
                  </div>
                  <FormControl>
                    <Input
                      type='password'
                      autoComplete='new-password'
                      placeholder={t('Enter a new key to update')}
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <FormField
            control={form.control}
            name='enabled_networks'
            render={({ field }) => (
              <FormItem>
                <FormControl>
                  <FieldSet>
                    <FieldLegend variant='label'>
                      {t('Enabled networks')}
                    </FieldLegend>
                    <FieldGroup className='grid gap-3 sm:grid-cols-3'>
                      {tokenPayNetworks.map((network) => (
                        <Field key={network} orientation='horizontal'>
                          <Checkbox
                            id={`tokenpay-network-${network}`}
                            checked={field.value.includes(network)}
                            onCheckedChange={(checked) => {
                              field.onChange(
                                checked
                                  ? [...field.value, network]
                                  : field.value.filter(
                                      (item) => item !== network
                                    )
                              )
                            }}
                          />
                          <FieldContent>
                            <FieldLabel htmlFor={`tokenpay-network-${network}`}>
                              {networkLabels[network]}
                            </FieldLabel>
                          </FieldContent>
                        </Field>
                      ))}
                    </FieldGroup>
                  </FieldSet>
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />

          <SHKeeperPackageEditor errorId='tokenpay-packages-error' />
        </FieldGroup>

        <FormField
          control={form.control}
          name='allow_private_url'
          render={({ field }) => (
            <FormItem className='flex flex-row items-center justify-between gap-4 py-2.5'>
              <div className='min-w-0'>
                <FormLabel>{t('Allow private TokenPay URL')}</FormLabel>
                <FormDescription>
                  {t(
                    'Permit a TokenPay service on localhost or a private network.'
                  )}
                </FormDescription>
              </div>
              <FormControl>
                <Switch
                  checked={field.value}
                  onCheckedChange={field.onChange}
                />
              </FormControl>
            </FormItem>
          )}
        />
      </div>
    </Form>
  )
}

export const TokenPaySettingsSection = React.forwardRef(
  TokenPaySettingsSectionComponent
)
