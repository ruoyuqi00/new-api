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
import { Add01Icon, Delete02Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import {
  Controller,
  FormProvider,
  useFieldArray,
  useForm,
  useFormContext,
} from 'react-hook-form'
import { useTranslation } from 'react-i18next'

import { MultiSelect } from '@/components/multi-select'
import { Button } from '@/components/ui/button'
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
  FieldLegend,
  FieldSet,
} from '@/components/ui/field'
import { Input } from '@/components/ui/input'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Separator } from '@/components/ui/separator'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'

import {
  qualityMonitorPlanSchema,
  type QualityMonitorPlanInput,
} from '../lib/plan-schema'
import type { QualityMonitorOptions, QualityMonitorPlan } from '../types'

type PlanFormProps = {
  options: QualityMonitorOptions
  plan?: QualityMonitorPlan
  isSaving: boolean
  onSave: (plan: QualityMonitorPlanInput) => void
  onCancel: () => void
}

function QuestionFields(props: {
  index: number
  onRemove: () => void
  canRemove: boolean
}) {
  const { t } = useTranslation()
  const form = useFormContext<QualityMonitorPlanInput>()
  const errors = form.formState.errors.questions?.[props.index]
  return (
    <FieldSet className='bg-muted/20 rounded-xl border p-4'>
      <FieldLegend className='mb-0 flex w-full items-center justify-between gap-2 text-sm'>
        <span>{t('Question {{number}}', { number: props.index + 1 })}</span>
        <Button
          type='button'
          variant='ghost'
          size='icon-sm'
          disabled={!props.canRemove}
          onClick={props.onRemove}
          aria-label={t('Remove question {{number}}', {
            number: props.index + 1,
          })}
        >
          <HugeiconsIcon icon={Delete02Icon} aria-hidden='true' />
        </Button>
      </FieldLegend>
      <FieldGroup>
        <Field data-invalid={Boolean(errors?.prompt)}>
          <FieldLabel htmlFor={`prompt-${props.index}`}>
            {t('Question')}
          </FieldLabel>
          <Textarea
            id={`prompt-${props.index}`}
            {...form.register(`questions.${props.index}.prompt`)}
            rows={3}
            aria-invalid={Boolean(errors?.prompt)}
          />
          {errors?.prompt?.message && (
            <FieldError>{t(errors.prompt.message)}</FieldError>
          )}
        </Field>
        <Field>
          <FieldLabel htmlFor={`match-${props.index}`}>
            {t('Answer matching')}
          </FieldLabel>
          <NativeSelect
            id={`match-${props.index}`}
            {...form.register(`questions.${props.index}.match_type`)}
            className='w-full'
          >
            <NativeSelectOption value='manual'>
              {t('Manual review')}
            </NativeSelectOption>
            <NativeSelectOption value='exact'>
              {t('Exact match')}
            </NativeSelectOption>
            <NativeSelectOption value='contains'>
              {t('Contains (case insensitive)')}
            </NativeSelectOption>
          </NativeSelect>
        </Field>
        <Field data-invalid={Boolean(errors?.expected_answer)}>
          <FieldLabel htmlFor={`expected-${props.index}`}>
            {t('Reference answer')}
          </FieldLabel>
          <Textarea
            id={`expected-${props.index}`}
            {...form.register(`questions.${props.index}.expected_answer`)}
            rows={2}
            aria-invalid={Boolean(errors?.expected_answer)}
          />
          {errors?.expected_answer?.message && (
            <FieldError>{t(errors.expected_answer.message)}</FieldError>
          )}
        </Field>
      </FieldGroup>
    </FieldSet>
  )
}

export function PlanForm(props: PlanFormProps) {
  const { t } = useTranslation()
  const form = useForm<QualityMonitorPlanInput>({
    resolver: zodResolver(qualityMonitorPlanSchema),
    defaultValues: props.plan ?? {
      name: '',
      enabled: false,
      published: false,
      groups: [],
      models: [],
      interval_minutes: 60,
      reasoning_effort: 'high',
      max_output_tokens: 2048,
      timeout_seconds: 120,
      questions: [
        {
          id: crypto.randomUUID(),
          prompt: '',
          expected_answer: '',
          match_type: 'manual',
        },
      ],
    },
  })
  const questions = useFieldArray({
    control: form.control,
    name: 'questions',
    keyName: 'fieldKey',
  })
  const groups = form.watch('groups')
  const models = form.watch('models')
  const interval = form.watch('interval_minutes')
  const errors = form.formState.errors
  const groupOptions = props.options.groups.map((group) => ({
    label: group.name,
    value: group.name,
  }))
  // Preserve previously configured groups in the editor when capabilities change.
  for (const group of props.plan?.groups ?? []) {
    if (!groupOptions.some((option) => option.value === group)) {
      groupOptions.push({ label: group, value: group })
    }
  }
  const unsupportedPairs = groups.flatMap((group) =>
    models
      .filter(
        (model) =>
          !props.options.groups
            .find((option) => option.name === group)
            ?.models.includes(model)
      )
      .map((model) => `${group} / ${model}`)
  )
  const calls = groups.length * models.length * questions.fields.length
  const presets = [5, 15, 30, 60, 360, 1440]

  return (
    <FormProvider {...form}>
      <form
        onSubmit={form.handleSubmit(props.onSave)}
        className='flex min-h-0 flex-col gap-5'
      >
        <div className='min-h-0 overflow-y-auto overscroll-contain px-1 pb-2'>
          <FieldGroup>
            <Field data-invalid={Boolean(errors.name)}>
              <FieldLabel htmlFor='monitor-plan-name'>
                {t('Plan name')}
              </FieldLabel>
              <Input
                id='monitor-plan-name'
                {...form.register('name')}
                maxLength={200}
                aria-invalid={Boolean(errors.name)}
              />
              {errors.name?.message && (
                <FieldError>{t(errors.name.message)}</FieldError>
              )}
            </Field>
            <Field data-invalid={Boolean(errors.groups)}>
              <FieldLabel htmlFor='monitor-plan-groups'>
                {t('Groups')}
              </FieldLabel>
              <Controller
                control={form.control}
                name='groups'
                render={({ field }) => (
                  <MultiSelect
                    id='monitor-plan-groups'
                    options={groupOptions}
                    selected={field.value}
                    onChange={field.onChange}
                    placeholder={t('Select groups')}
                    maxVisibleChips={4}
                  />
                )}
              />
              <FieldDescription>
                {t(
                  'Select up to 20 specific groups. Unsupported combinations are skipped.'
                )}
              </FieldDescription>
              {errors.groups?.message && (
                <FieldError>{t(errors.groups.message)}</FieldError>
              )}
            </Field>
            <Field data-invalid={Boolean(errors.models)}>
              <FieldLabel id='monitor-models-label'>{t('Models')}</FieldLabel>
              <Controller
                control={form.control}
                name='models'
                render={({ field }) => (
                  <ToggleGroup
                    multiple
                    value={field.value}
                    onValueChange={field.onChange}
                    spacing={2}
                    variant='outline'
                    aria-labelledby='monitor-models-label'
                    className='flex-wrap justify-start'
                  >
                    <ToggleGroupItem value='gpt-6-astra'>
                      gpt-6-astra
                    </ToggleGroupItem>
                    <ToggleGroupItem value='gpt-6.1-sol'>
                      gpt-6.1-sol
                    </ToggleGroupItem>
                  </ToggleGroup>
                )}
              />
              {errors.models?.message && (
                <FieldError>{t(errors.models.message)}</FieldError>
              )}
              {unsupportedPairs.length > 0 && (
                <FieldDescription className='text-warning'>
                  {t('Skipped combinations')}: {unsupportedPairs.join(', ')}
                </FieldDescription>
              )}
            </Field>
            <Separator />
            <Field>
              <FieldLabel id='monitor-presets-label'>
                {t('Interval presets')}
              </FieldLabel>
              <ToggleGroup
                value={presets.includes(interval) ? [String(interval)] : []}
                onValueChange={(values) => {
                  if (values[0]) {
                    form.setValue('interval_minutes', Number(values[0]), {
                      shouldValidate: true,
                    })
                  }
                }}
                spacing={2}
                variant='outline'
                aria-labelledby='monitor-presets-label'
                className='flex-wrap justify-start'
              >
                {presets.map((minutes) => (
                  <ToggleGroupItem key={minutes} value={String(minutes)}>
                    {t('{{count}} min', { count: minutes })}
                  </ToggleGroupItem>
                ))}
              </ToggleGroup>
            </Field>
            <Field data-invalid={Boolean(errors.interval_minutes)}>
              <FieldLabel htmlFor='monitor-interval'>
                {t('Custom interval (minutes)')}
              </FieldLabel>
              <Input
                {...form.register('interval_minutes', { valueAsNumber: true })}
                id='monitor-interval'
                type='number'
                min={1}
                max={10080}
                step={1}
                aria-invalid={Boolean(errors.interval_minutes)}
              />
              {errors.interval_minutes?.message && (
                <FieldError>{t(errors.interval_minutes.message)}</FieldError>
              )}
            </Field>
            <Field>
              <FieldLabel id='monitor-reasoning-label'>
                {t('Reasoning effort')}
              </FieldLabel>
              <Controller
                control={form.control}
                name='reasoning_effort'
                render={({ field }) => (
                  <ToggleGroup
                    value={[field.value]}
                    onValueChange={(values) => {
                      if (values[0]) field.onChange(values[0])
                    }}
                    spacing={2}
                    variant='outline'
                    aria-labelledby='monitor-reasoning-label'
                    className='flex-wrap justify-start'
                  >
                    <ToggleGroupItem value='low'>{t('Low')}</ToggleGroupItem>
                    <ToggleGroupItem value='medium'>
                      {t('Medium')}
                    </ToggleGroupItem>
                    <ToggleGroupItem value='high'>{t('High')}</ToggleGroupItem>
                    <ToggleGroupItem value='max'>
                      {t('Maximum')}
                    </ToggleGroupItem>
                  </ToggleGroup>
                )}
              />
            </Field>
            <div className='grid gap-4 sm:grid-cols-2'>
              <Field data-invalid={Boolean(errors.max_output_tokens)}>
                <FieldLabel htmlFor='monitor-output'>
                  {t('Output token limit')}
                </FieldLabel>
                <Input
                  id='monitor-output'
                  {...form.register('max_output_tokens', {
                    valueAsNumber: true,
                  })}
                  type='number'
                  min={128}
                  max={8192}
                  step={1}
                  aria-invalid={Boolean(errors.max_output_tokens)}
                />
                {errors.max_output_tokens?.message && (
                  <FieldError>{t(errors.max_output_tokens.message)}</FieldError>
                )}
              </Field>
              <Field data-invalid={Boolean(errors.timeout_seconds)}>
                <FieldLabel htmlFor='monitor-timeout'>
                  {t('Timeout (seconds)')}
                </FieldLabel>
                <Input
                  id='monitor-timeout'
                  {...form.register('timeout_seconds', { valueAsNumber: true })}
                  type='number'
                  min={10}
                  max={180}
                  step={1}
                  aria-invalid={Boolean(errors.timeout_seconds)}
                />
                {errors.timeout_seconds?.message && (
                  <FieldError>{t(errors.timeout_seconds.message)}</FieldError>
                )}
              </Field>
            </div>
            <Separator />
            {questions.fields.map((question, index) => (
              <QuestionFields
                key={question.fieldKey}
                index={index}
                canRemove={questions.fields.length > 1}
                onRemove={() => questions.remove(index)}
              />
            ))}
            <Button
              type='button'
              variant='outline'
              disabled={questions.fields.length >= 5}
              onClick={() =>
                questions.append({
                  id: crypto.randomUUID(),
                  prompt: '',
                  expected_answer: '',
                  match_type: 'manual',
                })
              }
            >
              <HugeiconsIcon
                icon={Add01Icon}
                data-icon='inline-start'
                aria-hidden='true'
              />
              {t('Add question')}
            </Button>
            {errors.questions?.message && (
              <FieldError>{t(errors.questions.message)}</FieldError>
            )}
            <p className='text-muted-foreground text-xs'>
              {t('{{count}} calls per round (maximum 100)', { count: calls })}
            </p>
            {!props.plan && (
              <FieldDescription>
                {t(
                  'Save this plan first, then edit it to enable scheduled probes or publish results.'
                )}
              </FieldDescription>
            )}
            <Field orientation='horizontal' data-disabled={!props.plan}>
              <FieldLabel htmlFor='monitor-enabled'>
                {t('Enable scheduled probes')}
              </FieldLabel>
              <Controller
                control={form.control}
                name='enabled'
                render={({ field }) => (
                  <Switch
                    id='monitor-enabled'
                    checked={field.value}
                    onCheckedChange={field.onChange}
                    disabled={!props.plan}
                  />
                )}
              />
            </Field>
            <Field orientation='horizontal' data-disabled={!props.plan}>
              <FieldLabel htmlFor='monitor-published'>
                {t('Publish results to group members')}
              </FieldLabel>
              <Controller
                control={form.control}
                name='published'
                render={({ field }) => (
                  <Switch
                    id='monitor-published'
                    checked={field.value}
                    onCheckedChange={field.onChange}
                    disabled={!props.plan}
                  />
                )}
              />
            </Field>
            <FieldDescription>
              {t(
                'Published results include your questions, reference answers and model answers.'
              )}
            </FieldDescription>
          </FieldGroup>
        </div>
        <div className='flex shrink-0 justify-end gap-2 border-t pt-4'>
          <Button
            type='button'
            variant='outline'
            onClick={props.onCancel}
            disabled={props.isSaving}
          >
            {t('Cancel')}
          </Button>
          <Button type='submit' disabled={props.isSaving}>
            {props.isSaving ? t('Saving...') : t('Save plan')}
          </Button>
        </div>
      </form>
    </FormProvider>
  )
}
