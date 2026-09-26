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
import { TestTubeIcon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import * as React from 'react'
import { useForm, type Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
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

import {
  getSHKeeperSettings,
  saveSHKeeperSettings,
  testSHKeeperConnection,
} from '../api'
import type {
  SHKeeperConnectionTest,
  SHKeeperNetwork,
  SHKeeperSettingsStatus,
} from '../types'
import { safeNumberFieldProps } from '../utils/numeric-field'
import { SHKeeperPackageEditor } from './shkeeper-package-editor'
import {
  buildSHKeeperFormDefaults,
  buildSHKeeperSettingsRequest,
  createSHKeeperSettingsSchema,
  type SHKeeperSettingsFormValues,
} from './shkeeper-settings-model'

const shkeeperNetworks: SHKeeperNetwork[] = ['BNB-USDT', 'USDT', 'POLYGON-USDT']

const emptyStatus: SHKeeperSettingsStatus = {
  enabled: false,
  base_url: '',
  api_key_configured: false,
  backend_key_configured: false,
  packages: null,
  enabled_networks: null,
  invoice_expiry_minutes: 30,
  reconcile_interval_seconds: 60,
  allow_private_url: false,
}

export function SHKeeperSettingsSection() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const schema = React.useMemo(() => createSHKeeperSettingsSchema(t), [t])
  const form = useForm<SHKeeperSettingsFormValues>({
    resolver: zodResolver(schema) as Resolver<SHKeeperSettingsFormValues>,
    mode: 'onChange',
    defaultValues: buildSHKeeperFormDefaults(emptyStatus),
  })
  const [testResult, setTestResult] =
    React.useState<SHKeeperConnectionTest | null>(null)
  const settingsQuery = useQuery({
    queryKey: ['shkeeper-settings'],
    queryFn: getSHKeeperSettings,
  })

  React.useEffect(() => {
    if (!settingsQuery.data?.success || !settingsQuery.data.data) return
    form.reset(buildSHKeeperFormDefaults(settingsQuery.data.data))
    setTestResult(null)
  }, [form, settingsQuery.data])

  const saveMutation = useMutation({
    mutationFn: saveSHKeeperSettings,
    onSuccess: (response) => {
      if (!response.success || !response.data) {
        toast.error(t('Failed to save SHKeeper settings'))
        return
      }
      form.reset(buildSHKeeperFormDefaults(response.data))
      setTestResult(null)
      queryClient.setQueryData(['shkeeper-settings'], response)
      toast.success(t('SHKeeper settings saved'))
    },
    onError: () => {
      toast.error(t('Failed to save SHKeeper settings'))
    },
  })

  const testMutation = useMutation({
    mutationFn: testSHKeeperConnection,
    onSuccess: (response) => {
      if (!response.success || !response.data) {
        setTestResult(null)
        toast.error(t('SHKeeper connection test failed'))
        return
      }
      setTestResult(response.data)
      if (response.data.ready) {
        toast.success(t('SHKeeper connection is ready'))
      } else {
        toast.error(t('SHKeeper connection needs attention'))
      }
    },
    onError: () => {
      setTestResult(null)
      toast.error(t('SHKeeper connection test failed'))
    },
  })

  const enabled = form.watch('enabled')
  const allowPrivateURL = form.watch('allow_private_url')
  const status = settingsQuery.data?.data
  const allNetworksReady =
    !!testResult?.networks.length &&
    testResult.networks.every(
      (network) =>
        network.available && network.quote_ok && network.amount_matches
    )

  const submitSettings = form.handleSubmit((values) => {
    saveMutation.mutate(buildSHKeeperSettingsRequest(values))
  })
  const testSettings = form.handleSubmit((values) => {
    testMutation.mutate(buildSHKeeperSettingsRequest(values))
  })

  if (settingsQuery.isLoading) {
    return (
      <div className='flex items-center gap-2 py-6 text-sm'>
        <Spinner />
        {t('Loading SHKeeper settings...')}
      </div>
    )
  }

  if (settingsQuery.isError || settingsQuery.data?.success === false) {
    return (
      <Alert variant='destructive'>
        <AlertTitle>{t('Unable to load SHKeeper settings')}</AlertTitle>
        <AlertDescription>{t('Please try again later.')}</AlertDescription>
      </Alert>
    )
  }

  return (
    <Form {...form}>
      <div className='flex flex-col gap-6 pt-4'>
        <div className='flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between'>
          <div className='min-w-0'>
            <h3 className='text-lg font-medium'>{t('SHKeeper Gateway')}</h3>
            <p className='text-muted-foreground text-sm'>
              {t('Accept fixed-package USDT payments through SHKeeper.')}
            </p>
          </div>
          <Badge variant={enabled ? 'default' : 'secondary'}>
            {enabled ? t('Enabled') : t('Disabled')}
          </Badge>
        </div>

        <Alert>
          <AlertTitle>{t('Write-only credentials')}</AlertTitle>
          <AlertDescription>
            {t(
              'Saved API and backend keys are never displayed. Leave either field blank to keep its current value.'
            )}
          </AlertDescription>
        </Alert>

        <FormField
          control={form.control}
          name='enabled'
          render={({ field }) => (
            <FormItem className='flex flex-row items-center justify-between gap-4 py-2.5'>
              <div className='min-w-0'>
                <FormLabel>{t('Enable SHKeeper')}</FormLabel>
                <FormDescription>
                  {t('Show SHKeeper fixed packages as a user top-up option.')}
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

        <FieldGroup>
          <div className='grid gap-6 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='base_url'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('SHKeeper base URL')}</FormLabel>
                  <FormControl>
                    <Input
                      type='url'
                      placeholder={t('https://payments.example.com')}
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Base URL of the SHKeeper API deployment.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='api_key'
              render={({ field }) => (
                <FormItem>
                  <div className='flex items-center gap-2'>
                    <FormLabel>{t('SHKeeper API key')}</FormLabel>
                    {status?.api_key_configured ? (
                      <Badge variant='secondary'>{t('Configured')}</Badge>
                    ) : null}
                  </div>
                  <FormControl>
                    <Input
                      type='password'
                      autoComplete='new-password'
                      placeholder={t('Enter a new key to update')}
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Used for server-to-server SHKeeper API requests.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='backend_key'
              render={({ field }) => (
                <FormItem>
                  <div className='flex items-center gap-2'>
                    <FormLabel>{t('SHKeeper backend key')}</FormLabel>
                    {status?.backend_key_configured ? (
                      <Badge variant='secondary'>{t('Configured')}</Badge>
                    ) : null}
                  </div>
                  <FormControl>
                    <Input
                      type='password'
                      autoComplete='new-password'
                      placeholder={t('Enter a new key to update')}
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Used to verify signed SHKeeper payment callbacks.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

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
                      <FieldGroup className='gap-3'>
                        {shkeeperNetworks.map((network) => {
                          const checked = field.value.includes(network)
                          return (
                            <Field key={network} orientation='horizontal'>
                              <Checkbox
                                id={`shkeeper-network-${network}`}
                                checked={checked}
                                onCheckedChange={(nextChecked) => {
                                  field.onChange(
                                    nextChecked
                                      ? [...field.value, network]
                                      : field.value.filter(
                                          (value) => value !== network
                                        )
                                  )
                                }}
                              />
                              <FieldContent>
                                <FieldLabel
                                  htmlFor={`shkeeper-network-${network}`}
                                >
                                  {network}
                                </FieldLabel>
                              </FieldContent>
                            </Field>
                          )
                        })}
                      </FieldGroup>
                    </FieldSet>
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <SHKeeperPackageEditor />

          <div className='grid gap-6 md:grid-cols-2'>
            <FormField
              control={form.control}
              name='invoice_expiry_minutes'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Invoice expiry (minutes)')}</FormLabel>
                  <FormControl>
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
              name='reconcile_interval_seconds'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>
                    {t('Reconciliation interval (seconds)')}
                  </FormLabel>
                  <FormControl>
                    <Input
                      type='number'
                      min={60}
                      step={1}
                      {...safeNumberFieldProps(field)}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </div>

          <FormField
            control={form.control}
            name='allow_private_url'
            render={({ field }) => (
              <FormItem className='flex flex-row items-center justify-between gap-4 py-2.5'>
                <div className='min-w-0'>
                  <FormLabel>{t('Allow private SHKeeper URL')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Permit localhost or private-network SHKeeper deployments.'
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

          {allowPrivateURL ? (
            <Alert variant='destructive'>
              <AlertTitle>{t('Private URL access is enabled')}</AlertTitle>
              <AlertDescription>
                {t(
                  'Only allow private addresses when the SHKeeper service is on a trusted network.'
                )}
              </AlertDescription>
            </Alert>
          ) : null}
        </FieldGroup>

        {testResult ? (
          <div className='flex flex-col gap-3'>
            <div className='flex flex-wrap items-center gap-2'>
              <h4 className='text-sm font-medium'>
                {t('Connection test results')}
              </h4>
              <Badge variant={allNetworksReady ? 'default' : 'destructive'}>
                {allNetworksReady ? t('Ready') : t('Needs attention')}
              </Badge>
            </div>
            <div className='flex flex-wrap gap-2'>
              {testResult.networks.map((network) => {
                const ready =
                  network.available &&
                  network.quote_ok &&
                  network.amount_matches
                return (
                  <Badge
                    key={network.crypto}
                    variant={ready ? 'outline' : 'destructive'}
                  >
                    {ready
                      ? t('{{network}} ready', { network: network.crypto })
                      : t('{{network}} needs attention', {
                          network: network.crypto,
                        })}
                  </Badge>
                )
              })}
            </div>
          </div>
        ) : null}

        <div className='flex flex-col-reverse gap-2 sm:flex-row sm:justify-end'>
          <Button
            type='button'
            variant='outline'
            disabled={testMutation.isPending || saveMutation.isPending}
            onClick={testSettings}
          >
            {testMutation.isPending ? (
              <Spinner data-icon='inline-start' />
            ) : (
              <HugeiconsIcon
                icon={TestTubeIcon}
                strokeWidth={2}
                data-icon='inline-start'
              />
            )}
            {t('Test connection')}
          </Button>
          <Button
            type='button'
            disabled={saveMutation.isPending || testMutation.isPending}
            onClick={submitSettings}
          >
            {saveMutation.isPending ? (
              <Spinner data-icon='inline-start' />
            ) : null}
            {t('Save SHKeeper settings')}
          </Button>
        </div>
      </div>
    </Form>
  )
}
