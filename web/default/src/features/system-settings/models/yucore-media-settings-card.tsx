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
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { RefreshCw } from 'lucide-react'
import { useEffect, useMemo, useRef } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

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
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import {
  getYucoreMediaHealth,
  listYucoreMediaModels,
  type YucoreMediaHealth,
  type YucoreMediaModel,
} from '@/features/yucore-brand/api/studio'

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'
import { safeNumberFieldProps } from '../utils/numeric-field'

const adapterOptions = [
  'mock',
  'openai-compatible',
  'yuapi-channel',
  'uag-proxy',
] as const

const schema = z.object({
  yucore_media: z.object({
    adapter: z.enum(adapterOptions),
    base_url: z.string(),
    api_key: z.string(),
    timeout_seconds: z.coerce.number().int().min(1).max(600),
    require_real_assets: z.boolean(),
    model_capabilities: z.string(),
    managed_token_group: z.string(),
    uag_model_map: z.string(),
    uag_allowed_providers: z.string(),
    uag_allowed_models: z.string(),
    upstream_verified: z.boolean(),
  }),
})

type YucoreMediaFormInput = z.input<typeof schema>
type YucoreMediaFormValues = z.output<typeof schema>

type FlatYucoreMediaSettings = {
  'yucore_media.adapter': (typeof adapterOptions)[number]
  'yucore_media.base_url': string
  'yucore_media.api_key': string
  'yucore_media.timeout_seconds': number
  'yucore_media.require_real_assets': boolean
  'yucore_media.model_capabilities': string
  'yucore_media.managed_token_group': string
  'yucore_media.uag_model_map': string
  'yucore_media.uag_allowed_providers': string
  'yucore_media.uag_allowed_models': string
  'yucore_media.upstream_verified': boolean
}

type YucoreMediaSettingsDefaults = Omit<
  FlatYucoreMediaSettings,
  'yucore_media.api_key'
> & {
  'yucore_media.api_key'?: string
}

type YucoreMediaSettingsCardProps = {
  defaultValues: YucoreMediaSettingsDefaults
}

function normalizeAdapter(value: string): (typeof adapterOptions)[number] {
  if (adapterOptions.includes(value as (typeof adapterOptions)[number])) {
    return value as (typeof adapterOptions)[number]
  }
  return 'mock'
}

function buildFormDefaults(
  defaults: YucoreMediaSettingsDefaults
): YucoreMediaFormInput {
  return {
    yucore_media: {
      adapter: normalizeAdapter(defaults['yucore_media.adapter']),
      base_url: defaults['yucore_media.base_url'] ?? '',
      api_key: defaults['yucore_media.api_key'] ?? '',
      timeout_seconds: defaults['yucore_media.timeout_seconds'] ?? 90,
      require_real_assets:
        defaults['yucore_media.require_real_assets'] ?? false,
      model_capabilities: defaults['yucore_media.model_capabilities'] ?? '',
      managed_token_group: defaults['yucore_media.managed_token_group'] ?? '',
      uag_model_map: defaults['yucore_media.uag_model_map'] ?? '',
      uag_allowed_providers:
        defaults['yucore_media.uag_allowed_providers'] ?? '',
      uag_allowed_models: defaults['yucore_media.uag_allowed_models'] ?? '',
      upstream_verified: defaults['yucore_media.upstream_verified'] ?? false,
    },
  }
}

function flattenValues(values: YucoreMediaFormValues): FlatYucoreMediaSettings {
  return {
    'yucore_media.adapter': values.yucore_media.adapter,
    'yucore_media.base_url': values.yucore_media.base_url.trim(),
    'yucore_media.api_key': values.yucore_media.api_key.trim(),
    'yucore_media.timeout_seconds': values.yucore_media.timeout_seconds,
    'yucore_media.require_real_assets': values.yucore_media.require_real_assets,
    'yucore_media.model_capabilities':
      values.yucore_media.model_capabilities.trim(),
    'yucore_media.managed_token_group':
      values.yucore_media.managed_token_group.trim(),
    'yucore_media.uag_model_map': values.yucore_media.uag_model_map.trim(),
    'yucore_media.uag_allowed_providers':
      values.yucore_media.uag_allowed_providers.trim(),
    'yucore_media.uag_allowed_models':
      values.yucore_media.uag_allowed_models.trim(),
    'yucore_media.upstream_verified': values.yucore_media.upstream_verified,
  }
}

function buildComparableDefaults(
  defaults: YucoreMediaSettingsDefaults
): FlatYucoreMediaSettings {
  return flattenValues(schema.parse(buildFormDefaults(defaults)))
}

export function YucoreMediaSettingsCard({
  defaultValues,
}: YucoreMediaSettingsCardProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const updateOption = useUpdateOption()

  const formDefaults = useMemo(
    () => buildFormDefaults(defaultValues),
    [defaultValues]
  )
  const comparableDefaults = useMemo(
    () => buildComparableDefaults(defaultValues),
    [defaultValues]
  )

  const form = useForm<YucoreMediaFormInput, unknown, YucoreMediaFormValues>({
    resolver: zodResolver(schema),
    defaultValues: formDefaults,
  })

  const baselineRef = useRef<FlatYucoreMediaSettings>(comparableDefaults)
  const baselineSerializedRef = useRef<string>(
    JSON.stringify(comparableDefaults)
  )

  useEffect(() => {
    const serialized = JSON.stringify(comparableDefaults)
    if (serialized === baselineSerializedRef.current) return
    baselineRef.current = comparableDefaults
    baselineSerializedRef.current = serialized
    form.reset(formDefaults)
  }, [comparableDefaults, form, formDefaults])

  const healthQuery = useQuery<YucoreMediaHealth>({
    queryKey: ['yucore-media-health', 'admin'],
    queryFn: getYucoreMediaHealth,
  })

  const modelsQuery = useQuery<YucoreMediaModel[]>({
    queryKey: ['yucore-media-models', 'admin'],
    queryFn: listYucoreMediaModels,
  })

  const adapter = form.watch('yucore_media.adapter')
  const isOpenAICompatible = adapter === 'openai-compatible'
  const isYuAPIChannel = adapter === 'yuapi-channel'
  const isUagProxy = adapter === 'uag-proxy'

  const refreshRuntimeState = () => {
    queryClient.invalidateQueries({ queryKey: ['yucore-media-health'] })
    queryClient.invalidateQueries({ queryKey: ['yucore-media-models'] })
  }

  const onSubmit = async (values: YucoreMediaFormValues) => {
    const normalized = flattenValues(values)
    const changedKeys = (
      Object.keys(normalized) as Array<keyof FlatYucoreMediaSettings>
    ).filter((key) => {
      if (key === 'yucore_media.api_key' && normalized[key] === '') {
        return false
      }
      return normalized[key] !== baselineRef.current[key]
    })

    if (changedKeys.length === 0) {
      toast.info(t('No changes to save'))
      return
    }

    for (const key of changedKeys) {
      await updateOption.mutateAsync({
        key,
        value: normalized[key],
      })
    }

    const nextBaseline = { ...normalized, 'yucore_media.api_key': '' }
    baselineRef.current = nextBaseline
    baselineSerializedRef.current = JSON.stringify(nextBaseline)
    form.reset(buildFormDefaults(nextBaseline))
    refreshRuntimeState()
  }

  const health = healthQuery.data
  const models = modelsQuery.data ?? []
  const visibleModels = models.slice(0, 8)

  return (
    <SettingsSection title={t('YuCore Studio Media')}>
      <div className='grid gap-3 lg:grid-cols-3'>
        <div className='bg-muted/20 rounded-lg border p-3'>
          <div className='text-muted-foreground text-xs'>{t('Gateway')}</div>
          <div className='mt-2 flex flex-wrap gap-2'>
            <Badge variant={health?.configured ? 'default' : 'destructive'}>
              {health?.configured ? t('Configured') : t('Not configured')}
            </Badge>
            <Badge variant='outline'>{health?.adapter ?? adapter}</Badge>
          </div>
        </div>
        <div className='bg-muted/20 rounded-lg border p-3'>
          <div className='text-muted-foreground text-xs'>
            {t('Upstream verification')}
          </div>
          <div className='mt-2 flex flex-wrap gap-2'>
            <Badge
              variant={health?.upstream_verified ? 'default' : 'secondary'}
            >
              {health?.upstream_verification_status ?? t('Unknown')}
            </Badge>
            {health?.require_real_assets ? (
              <Badge variant='outline'>{t('Real assets required')}</Badge>
            ) : null}
            <Badge
              variant={health?.real_workflow_ready ? 'default' : 'destructive'}
            >
              {health?.real_workflow_ready
                ? t('Real workflow ready')
                : t('Real workflow blocked')}
            </Badge>
          </div>
        </div>
        <div className='bg-muted/20 rounded-lg border p-3'>
          <div className='text-muted-foreground text-xs'>
            {t('Visible user models')}
          </div>
          <div className='mt-2 text-sm font-medium'>
            {modelsQuery.isLoading ? t('Loading...') : models.length}
          </div>
        </div>
      </div>

      {health?.upstream_verification_message ? (
        <p className='text-muted-foreground text-sm'>
          {health.upstream_verification_message}
        </p>
      ) : null}
      {health?.verification_blockers?.length ? (
        <div className='border-destructive/25 bg-destructive/5 rounded-lg border p-3 text-sm'>
          <div className='font-medium'>{t('Real workflow blockers')}</div>
          <ul className='text-muted-foreground mt-2 list-inside list-disc space-y-1'>
            {health.verification_blockers.map((blocker) => (
              <li key={blocker}>{blocker}</li>
            ))}
          </ul>
        </div>
      ) : null}

      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
          />

          <FormField
            control={form.control}
            name='yucore_media.adapter'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Media adapter')}</FormLabel>
                <Select value={field.value} onValueChange={field.onChange}>
                  <FormControl>
                    <SelectTrigger>
                      <SelectValue />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent>
                    <SelectItem value='mock'>{t('Mock')}</SelectItem>
                    <SelectItem value='openai-compatible'>
                      {t('OpenAI compatible')}
                    </SelectItem>
                    <SelectItem value='yuapi-channel'>
                      {t('YuAPI channel')}
                    </SelectItem>
                    <SelectItem value='uag-proxy'>{t('UAG proxy')}</SelectItem>
                  </SelectContent>
                </Select>
                <FormDescription>
                  {t(
                    'User Studio stays in the playground; this controls which backend media gateway powers it.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='yucore_media.timeout_seconds'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Timeout seconds')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={1}
                    max={600}
                    {...safeNumberFieldProps(field)}
                  />
                </FormControl>
                <FormDescription>
                  {t('Maximum wait time for upstream media gateway calls.')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='yucore_media.base_url'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Gateway base URL')}</FormLabel>
                <FormControl>
                  <Input placeholder='http://127.0.0.1:17080' {...field} />
                </FormControl>
                <FormDescription>
                  {t('UAG proxy or OpenAI-compatible media endpoint base URL.')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='yucore_media.api_key'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Gateway API key')}</FormLabel>
                <FormControl>
                  <Input
                    type='password'
                    autoComplete='new-password'
                    disabled={isYuAPIChannel}
                    placeholder={
                      health?.api_key_configured
                        ? t('Configured; leave blank to keep current key')
                        : t('Paste a gateway token')
                    }
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t('Secrets are saved but not returned to this page.')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='yucore_media.require_real_assets'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Require real assets')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Hide local mock task results and fail when no real media gateway is configured.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <FormField
            control={form.control}
            name='yucore_media.upstream_verified'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Mark upstream verified')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Enable only after an ordinary user completes a real provider Studio workflow end to end.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <FormField
            control={form.control}
            name='yucore_media.managed_token_group'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Managed token group')}</FormLabel>
                <FormControl>
                  <Input
                    placeholder={t('Group')}
                    disabled={!isYuAPIChannel}
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Studio requests use a per-user managed token in this group and enter normal YuAPI channel scheduling.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='yucore_media.model_capabilities'
            render={({ field }) => (
              <FormItem>
                <FormLabel>
                  {t('OpenAI-compatible model capabilities')}
                </FormLabel>
                <FormControl>
                  <Textarea
                    rows={8}
                    placeholder='{"video-model":{"transport":"async-task","duration_policy":"seconds"}}'
                    disabled={!isOpenAICompatible && !isYuAPIChannel}
                    className='font-mono text-xs'
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'JSON keyed by model id; configures transport, paths, duration policy, and polling.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='yucore_media.uag_allowed_providers'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Allowed UAG providers')}</FormLabel>
                <FormControl>
                  <Textarea
                    rows={3}
                    placeholder='gpt, grok, flow'
                    disabled={!isUagProxy}
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t('Comma, semicolon, or newline separated provider ids.')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='yucore_media.uag_allowed_models'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Allowed Studio models')}</FormLabel>
                <FormControl>
                  <Textarea
                    rows={3}
                    placeholder='img-v3, gpt-image-2'
                    disabled={!isUagProxy}
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t('Only these UAG models are exposed in the user Studio.')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='yucore_media.uag_model_map'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('UAG model mapping')}</FormLabel>
                <FormControl>
                  <Textarea
                    rows={3}
                    placeholder='gpt-image-2=img-v3'
                    disabled={!isUagProxy}
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t('Map user-facing model ids to UAG model codes.')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <div className='bg-muted/20 space-y-3 rounded-lg border p-3'>
            <div className='flex items-center justify-between gap-3'>
              <div>
                <div className='text-sm font-medium'>
                  {t('Current user-side model exposure')}
                </div>
                <p className='text-muted-foreground text-xs'>
                  {t(
                    'This is the model list ordinary users receive from the Studio API.'
                  )}
                </p>
              </div>
              <Button
                type='button'
                variant='outline'
                size='icon'
                onClick={refreshRuntimeState}
                aria-label={t('Refresh')}
              >
                <RefreshCw className='size-4' />
              </Button>
            </div>
            <div className='flex flex-wrap gap-2'>
              {visibleModels.length > 0 ? (
                visibleModels.map((model) => (
                  <Badge key={model.id} variant='outline'>
                    {model.id}
                  </Badge>
                ))
              ) : (
                <span className='text-muted-foreground text-sm'>
                  {modelsQuery.isLoading
                    ? t('Loading...')
                    : t('No models exposed')}
                </span>
              )}
              {models.length > visibleModels.length ? (
                <Badge variant='secondary'>
                  {t('{{count}} more', {
                    count: models.length - visibleModels.length,
                  })}
                </Badge>
              ) : null}
            </div>
          </div>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
