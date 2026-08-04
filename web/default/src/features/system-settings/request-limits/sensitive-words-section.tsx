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
import { Trash2 } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
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
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'

import { startSensitiveInputCleanupTask } from '../api'
import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const sensitiveSchema = z.object({
  CheckSensitiveEnabled: z.boolean(),
  CheckSensitiveOnPromptEnabled: z.boolean(),
  SensitiveWords: z.string().optional(),
  SensitiveInputRetentionDays: z.number().int().min(1).max(365),
})

type SensitiveFormValues = z.infer<typeof sensitiveSchema>

type SensitiveWordsSectionProps = {
  defaultValues: SensitiveFormValues
}

export function SensitiveWordsSection({
  defaultValues,
}: SensitiveWordsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const [showCleanupDialog, setShowCleanupDialog] = useState(false)
  const [isStartingCleanup, setIsStartingCleanup] = useState(false)
  const form = useForm<SensitiveFormValues>({
    resolver: zodResolver(sensitiveSchema),
    defaultValues,
  })

  useEffect(() => {
    form.reset(defaultValues)
  }, [defaultValues, form])

  const onSubmit = async (values: SensitiveFormValues) => {
    const updates = Object.entries(values).filter(
      ([key, value]) =>
        value !== defaultValues[key as keyof SensitiveFormValues]
    )

    for (const [key, value] of updates) {
      await updateOption.mutateAsync({ key, value: value ?? '' })
    }
  }

  const handleCleanup = async () => {
    setIsStartingCleanup(true)
    try {
      const response = await startSensitiveInputCleanupTask()
      if (!response.success || !response.data) {
        throw new Error(
          response.message || t('Failed to start sensitive input cleanup.')
        )
      }
      toast.success(t('Sensitive input cleanup started.'))
      setShowCleanupDialog(false)
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to start sensitive input cleanup.')
      )
    } finally {
      setIsStartingCleanup(false)
    }
  }

  return (
    <SettingsSection title={t('Sensitive Words')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            saveLabel='Save sensitive words'
          />
          <div className='space-y-4'>
            <FormField
              control={form.control}
              name='CheckSensitiveEnabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Enable filtering')}</FormLabel>
                    <FormDescription>
                      {t(
                        'Blocks messages when sensitive keywords are detected.'
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
              name='CheckSensitiveOnPromptEnabled'
              render={({ field }) => (
                <SettingsSwitchItem>
                  <SettingsSwitchContent>
                    <FormLabel>{t('Inspect user prompts')}</FormLabel>
                    <FormDescription>
                      {t(
                        'When enabled, prompts are scanned before reaching upstream models.'
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
          </div>

          <FormField
            control={form.control}
            name='SensitiveWords'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Blocked keywords')}</FormLabel>
                <FormControl>
                  <Textarea
                    rows={12}
                    placeholder={t('Enter one keyword per line')}
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Each line represents one keyword. Leave blank to disable the list but keep the switch states.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='SensitiveInputRetentionDays'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Sensitive input retention days')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={1}
                    max={365}
                    className='w-32'
                    value={field.value}
                    onChange={(event) => field.onChange(event.target.valueAsNumber)}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Blocked input and matched words are removed after this many days. Billing audit records are kept.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <div className='flex flex-col gap-3 border-t pt-4 sm:flex-row sm:items-center sm:justify-between'>
            <div className='space-y-1'>
              <p className='text-sm font-medium'>
                {t('Audit evidence cleanup')}
              </p>
              <p className='text-muted-foreground text-xs'>
                {t(
                  'Run the retention cleanup now without deleting billing records.'
                )}
              </p>
            </div>
            <Button
              type='button'
              variant='outline'
              onClick={() => setShowCleanupDialog(true)}
              disabled={isStartingCleanup}
            >
              <Trash2 className='size-4' aria-hidden='true' />
              {t('Run cleanup now')}
            </Button>
          </div>
        </SettingsForm>
      </Form>

      <AlertDialog open={showCleanupDialog} onOpenChange={setShowCleanupDialog}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t('Run sensitive input cleanup now?')}
            </AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                'Only expired blocked input and matched words will be removed. Billing audit records will be kept.'
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel disabled={isStartingCleanup}>
              {t('Cancel')}
            </AlertDialogCancel>
            <AlertDialogAction
              variant='destructive'
              onClick={handleCleanup}
              disabled={isStartingCleanup}
            >
              {t('Run cleanup now')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </SettingsSection>
  )
}
