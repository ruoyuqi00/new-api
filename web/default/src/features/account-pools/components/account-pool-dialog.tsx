import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery } from '@tanstack/react-query'
import { Plus, Route, Trash2 } from 'lucide-react'
import { useEffect, useMemo } from 'react'
import { useFieldArray, useForm, useWatch } from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { MultiSelect } from '@/components/multi-select'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { ScrollArea } from '@/components/ui/scroll-area'
import { CHANNEL_TYPE_OPTIONS } from '@/features/channels/constants'
import { getChannelTypeLabel } from '@/features/channels/lib/channel-utils'
import { getGroups } from '@/features/users/api'

import { accountPoolFormSchema, type AccountPoolFormValues } from '../schema'
import type {
  AccountPoolChannel,
  AccountPoolDetail,
  AccountPoolPayload,
} from '../types'
import { AccountImportPanel } from './account-import-panel'

type AccountPoolDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  detail: AccountPoolDetail | null
  channels: AccountPoolChannel[]
  isSaving: boolean
  onSubmit: (payload: AccountPoolPayload) => void
}

const emptyValues: AccountPoolFormValues = {
  name: '',
  provider: '',
  adapter_type: 1,
  group: 'default',
  status: 1,
  priority: 0,
  weight: 100,
  remark: '',
  channel_ids: [],
  accounts: [],
}

export function AccountPoolDialog(props: AccountPoolDialogProps) {
  const { t } = useTranslation()
  const form = useForm<AccountPoolFormValues>({
    resolver: zodResolver(accountPoolFormSchema),
    defaultValues: emptyValues,
  })
  const accounts = useFieldArray({ control: form.control, name: 'accounts' })
  const groupValue = useWatch({ control: form.control, name: 'group' })
  const groupsQuery = useQuery({
    queryKey: ['groups'],
    queryFn: getGroups,
    staleTime: 60_000,
  })
  const selectedGroups = useMemo(
    () =>
      groupValue
        .split(',')
        .map((group) => group.trim())
        .filter(Boolean),
    [groupValue]
  )
  const groupOptions = useMemo(
    () =>
      [...new Set([...(groupsQuery.data?.data ?? []), ...selectedGroups])].map(
        (group) => ({ label: group, value: group })
      ),
    [groupsQuery.data?.data, selectedGroups]
  )
  const adapterTypes = useMemo(
    () =>
      [
        ...new Set([
          ...CHANNEL_TYPE_OPTIONS.map((option) => option.value),
          ...props.channels.map((channel) => channel.type),
        ]),
      ].sort((left, right) => left - right),
    [props.channels]
  )

  useEffect(() => {
    if (!props.open) return
    if (!props.detail) {
      form.reset(emptyValues)
      return
    }
    form.reset({
      name: props.detail.pool.name,
      provider: props.detail.pool.provider,
      adapter_type: props.detail.pool.adapter_type || 1,
      group: props.detail.pool.group,
      status: props.detail.pool.status,
      priority: props.detail.pool.priority,
      weight: props.detail.pool.weight,
      remark: props.detail.pool.remark,
      channel_ids: props.detail.channel_ids,
      accounts: props.detail.accounts.map((account) => ({
        id: account.id,
        name: account.name,
        type: account.type,
        credential: '',
        credential_set: account.credential_set,
        base_url: account.base_url,
        model_mapping: account.model_mapping,
        status: account.status,
        priority: account.priority,
        weight: account.weight,
        concurrency_limit: account.concurrency_limit,
        cooldown_seconds: account.cooldown_seconds,
        expires_at: account.expires_at,
        metadata: account.metadata,
      })),
    })
  }, [form, props.detail, props.open])

  const submit = form.handleSubmit((values) => {
    props.onSubmit({
      id: props.detail?.pool.id,
      name: values.name,
      provider: values.provider,
      adapter_type: values.adapter_type,
      group: values.group,
      status: values.status,
      priority: values.priority,
      weight: values.weight,
      remark: values.remark,
      channel_ids: values.channel_ids,
      accounts: values.accounts.map((account) => ({
        id: account.id,
        name: account.name,
        type: account.type,
        credential: account.credential || undefined,
        base_url: account.base_url,
        model_mapping: account.model_mapping,
        status: account.status,
        priority: account.priority,
        weight: account.weight,
        concurrency_limit: account.concurrency_limit,
        cooldown_seconds: account.cooldown_seconds,
        expires_at: account.expires_at,
        metadata: account.metadata,
      })),
    })
  })

  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent className='yucore-app-shell flex h-[min(92dvh,60rem)] max-h-[calc(100dvh-2rem)] flex-col gap-0 overflow-hidden p-0 sm:max-w-5xl'>
        <DialogHeader className='shrink-0 border-b px-5 py-4'>
          <DialogTitle>
            {props.detail ? t('Edit Account Pool') : t('Create Account Pool')}
          </DialogTitle>
          <DialogDescription className='sr-only'>
            {t('Account Pools')}
          </DialogDescription>
        </DialogHeader>

        <ScrollArea className='min-h-0 flex-1'>
          <form
            id='account-pool-form'
            onSubmit={submit}
            className='min-w-0 space-y-6 overflow-x-hidden p-5'
          >
            <section className='border-primary/20 grid gap-4 border-l-2 pl-4 md:grid-cols-2 lg:grid-cols-4'>
              <Field
                label={t('Name')}
                error={
                  form.formState.errors.name?.message
                    ? t(form.formState.errors.name.message)
                    : undefined
                }
              >
                <Input {...form.register('name')} />
              </Field>
              <Field label={t('Provider')}>
                <Input
                  {...form.register('provider')}
                  placeholder='OpenAI / Anthropic'
                />
              </Field>
              <Field label={t('Adapter Type')}>
                <NativeSelect
                  className='w-full'
                  value={String(form.watch('adapter_type'))}
                  onChange={(event) =>
                    form.setValue('adapter_type', Number(event.target.value), {
                      shouldDirty: true,
                    })
                  }
                >
                  {(adapterTypes.length > 0 ? adapterTypes : [1]).map(
                    (type) => (
                      <NativeSelectOption key={type} value={String(type)}>
                        {t(getChannelTypeLabel(type))}
                      </NativeSelectOption>
                    )
                  )}
                </NativeSelect>
              </Field>
              <Field
                label={t('Group')}
                error={
                  form.formState.errors.group?.message
                    ? t(form.formState.errors.group.message)
                    : undefined
                }
              >
                <MultiSelect
                  options={groupOptions}
                  selected={selectedGroups}
                  onChange={(groups) =>
                    form.setValue('group', groups.join(','), {
                      shouldDirty: true,
                      shouldValidate: true,
                    })
                  }
                  placeholder={t('Group')}
                  allowCreate
                  maxVisibleChips={3}
                />
              </Field>
              <Field label={t('Status')}>
                <NativeSelect
                  className='w-full'
                  value={String(form.watch('status'))}
                  onChange={(event) =>
                    form.setValue('status', Number(event.target.value))
                  }
                >
                  <NativeSelectOption value='1'>
                    {t('Enabled')}
                  </NativeSelectOption>
                  <NativeSelectOption value='2'>
                    {t('Disabled')}
                  </NativeSelectOption>
                </NativeSelect>
              </Field>
              <Field label={t('Pool Priority')}>
                <Input
                  type='number'
                  {...form.register('priority', { valueAsNumber: true })}
                />
              </Field>
              <Field label={t('Pool Weight')}>
                <Input
                  type='number'
                  min={0}
                  {...form.register('weight', { valueAsNumber: true })}
                />
              </Field>
              <Field label={t('Remark')} className='md:col-span-2'>
                <Input {...form.register('remark')} />
              </Field>
            </section>

            <details className='border-y py-3'>
              <summary className='flex cursor-pointer list-none items-center gap-2 font-medium'>
                <Route className='size-4' />
                {t('Advanced Channel Routing')}
                <span className='text-muted-foreground ml-auto text-sm font-normal'>
                  {t('Optional')}
                </span>
              </summary>
              <p className='text-muted-foreground mt-2 text-sm'>
                {t(
                  'Leave empty to route automatically by user group and adapter type.'
                )}
              </p>
              <div className='mt-3 grid max-h-40 gap-2 overflow-y-auto border p-3 sm:grid-cols-2 lg:grid-cols-3'>
                {props.channels
                  .filter(
                    (channel) => channel.type === form.watch('adapter_type')
                  )
                  .map((channel) => {
                    const checked = form
                      .watch('channel_ids')
                      .includes(channel.id)
                    return (
                      <Label
                        key={channel.id}
                        className='hover:bg-muted/50 flex min-w-0 items-center gap-2 rounded-md px-2 py-1.5'
                      >
                        <Checkbox
                          checked={checked}
                          onCheckedChange={(next) => {
                            const current = form.getValues('channel_ids')
                            form.setValue(
                              'channel_ids',
                              next
                                ? [...current, channel.id]
                                : current.filter((id) => id !== channel.id),
                              { shouldDirty: true }
                            )
                          }}
                        />
                        <span className='truncate'>{channel.name}</span>
                        <span className='text-muted-foreground ml-auto'>
                          #{channel.id}
                        </span>
                      </Label>
                    )
                  })}
              </div>
            </details>

            <section className='space-y-3'>
              <div className='flex items-center justify-between gap-3'>
                <div>
                  <h3 className='font-medium'>{t('Provider Accounts')}</h3>
                </div>
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  onClick={() =>
                    accounts.append({
                      name: '',
                      type: 'api_key',
                      credential: '',
                      credential_set: false,
                      base_url: '',
                      model_mapping: '',
                      status: 1,
                      priority: 0,
                      weight: 100,
                      concurrency_limit: 0,
                      cooldown_seconds: 20,
                      expires_at: 0,
                      metadata: '',
                    })
                  }
                >
                  <Plus />
                  {t('Add Account')}
                </Button>
              </div>

              <AccountImportPanel
                provider={form.watch('provider')}
                existingCount={accounts.fields.length}
                onImport={(imported) => {
                  const inferredAdapterTypes = [
                    ...new Set(
                      imported.flatMap((account) =>
                        account.adapter_type ? [account.adapter_type] : []
                      )
                    ),
                  ]
                  const inferredProviders = [
                    ...new Set(
                      imported.flatMap((account) =>
                        account.provider ? [account.provider] : []
                      )
                    ),
                  ]
                  if (inferredAdapterTypes.length === 1) {
                    form.setValue('adapter_type', inferredAdapterTypes[0], {
                      shouldDirty: true,
                    })
                  }
                  if (
                    inferredProviders.length === 1 &&
                    !form.getValues('provider').trim()
                  ) {
                    form.setValue('provider', inferredProviders[0], {
                      shouldDirty: true,
                    })
                  }
                  accounts.append(
                    imported.map((account) => {
                      const {
                        adapter_type: _adapterType,
                        provider: _provider,
                        ...values
                      } = account
                      return {
                        ...values,
                        credential_set: true,
                        status: account.status ?? 1,
                        expires_at: account.expires_at ?? 0,
                        metadata: account.metadata ?? '',
                      }
                    })
                  )
                }}
              />

              {form.formState.errors.accounts?.message ? (
                <p className='text-destructive text-sm'>
                  {t(form.formState.errors.accounts.message)}
                </p>
              ) : null}

              {accounts.fields.length === 0 ? (
                <div className='text-muted-foreground rounded-md border border-dashed px-4 py-8 text-center text-sm'>
                  {t('No provider accounts in this pool')}
                </div>
              ) : (
                <div className='space-y-3'>
                  {accounts.fields.map((account, index) => (
                    <div
                      key={account.id}
                      className='grid gap-3 rounded-md border p-3 md:grid-cols-4'
                    >
                      <Field
                        label={t('Account Name')}
                        error={
                          form.formState.errors.accounts?.[index]?.name?.message
                            ? t(
                                form.formState.errors.accounts[index]?.name
                                  ?.message ?? ''
                              )
                            : undefined
                        }
                      >
                        <Input {...form.register(`accounts.${index}.name`)} />
                      </Field>
                      <Field label={t('Credential Type')}>
                        <NativeSelect
                          className='w-full'
                          value={form.watch(`accounts.${index}.type`)}
                          onChange={(event) =>
                            form.setValue(
                              `accounts.${index}.type`,
                              event.target.value
                            )
                          }
                        >
                          <NativeSelectOption value='api_key'>
                            API Key
                          </NativeSelectOption>
                          <NativeSelectOption value='oauth_json'>
                            OAuth JSON
                          </NativeSelectOption>
                          <NativeSelectOption value='cookie'>
                            Cookie
                          </NativeSelectOption>
                          <NativeSelectOption value='custom'>
                            {t('Custom')}
                          </NativeSelectOption>
                        </NativeSelect>
                      </Field>
                      <Field
                        label={t('Credential')}
                        className='md:col-span-2'
                        error={
                          form.formState.errors.accounts?.[index]?.credential
                            ?.message
                            ? t(
                                form.formState.errors.accounts[index]
                                  ?.credential?.message ?? ''
                              )
                            : undefined
                        }
                      >
                        <Input
                          type='password'
                          autoComplete='new-password'
                          placeholder={
                            form.watch(`accounts.${index}.credential_set`)
                              ? t('Leave blank to keep the current credential')
                              : t('Enter credential')
                          }
                          {...form.register(`accounts.${index}.credential`)}
                        />
                      </Field>
                      <Field
                        label={t('Base URL')}
                        className='md:col-span-2'
                        error={
                          form.formState.errors.accounts?.[index]?.base_url
                            ?.message
                            ? t(
                                form.formState.errors.accounts[index]?.base_url
                                  ?.message ?? ''
                              )
                            : undefined
                        }
                      >
                        <Input
                          inputMode='url'
                          placeholder='https://api.example.com'
                          {...form.register(`accounts.${index}.base_url`)}
                        />
                      </Field>
                      <Field
                        label={t('Model Mapping')}
                        className='md:col-span-2'
                        error={
                          form.formState.errors.accounts?.[index]?.model_mapping
                            ?.message
                            ? t(
                                form.formState.errors.accounts[index]
                                  ?.model_mapping?.message ?? ''
                              )
                            : undefined
                        }
                      >
                        <Input
                          placeholder='{"gpt-4o":"upstream-model"}'
                          {...form.register(`accounts.${index}.model_mapping`)}
                        />
                      </Field>
                      <Field label={t('Account Priority')}>
                        <Input
                          type='number'
                          {...form.register(`accounts.${index}.priority`, {
                            valueAsNumber: true,
                          })}
                        />
                      </Field>
                      <Field label={t('Account Weight')}>
                        <Input
                          type='number'
                          min={0}
                          {...form.register(`accounts.${index}.weight`, {
                            valueAsNumber: true,
                          })}
                        />
                      </Field>
                      <Field label={t('Concurrency Limit')}>
                        <Input
                          type='number'
                          min={0}
                          {...form.register(
                            `accounts.${index}.concurrency_limit`,
                            { valueAsNumber: true }
                          )}
                        />
                      </Field>
                      <Field label={t('Cooldown Seconds')}>
                        <Input
                          type='number'
                          min={0}
                          {...form.register(
                            `accounts.${index}.cooldown_seconds`,
                            { valueAsNumber: true }
                          )}
                        />
                      </Field>
                      <div className='flex items-end gap-2 md:col-span-4'>
                        <NativeSelect
                          className='w-40'
                          value={String(form.watch(`accounts.${index}.status`))}
                          onChange={(event) =>
                            form.setValue(
                              `accounts.${index}.status`,
                              Number(event.target.value)
                            )
                          }
                        >
                          <NativeSelectOption value='1'>
                            {t('Enabled')}
                          </NativeSelectOption>
                          <NativeSelectOption value='2'>
                            {t('Disabled')}
                          </NativeSelectOption>
                        </NativeSelect>
                        <Button
                          type='button'
                          variant='ghost'
                          size='icon'
                          className='ml-auto'
                          aria-label={t('Remove Account')}
                          onClick={() => accounts.remove(index)}
                        >
                          <Trash2 />
                        </Button>
                      </div>
                    </div>
                  ))}
                </div>
              )}
            </section>
          </form>
        </ScrollArea>

        <DialogFooter className='m-0 shrink-0 rounded-none px-5 py-3'>
          <Button
            type='button'
            variant='outline'
            onClick={() => props.onOpenChange(false)}
          >
            {t('Cancel')}
          </Button>
          <Button
            type='submit'
            form='account-pool-form'
            disabled={props.isSaving}
          >
            {props.isSaving ? t('Saving...') : t('Save')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function Field(props: {
  label: string
  children: React.ReactNode
  error?: string
  className?: string
}) {
  return (
    <div className={props.className}>
      <Label className='mb-1.5'>{props.label}</Label>
      {props.children}
      {props.error ? (
        <p className='text-destructive mt-1 text-xs'>{props.error}</p>
      ) : null}
    </div>
  )
}
