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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
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

import type { ProviderAccountSummary } from '../types'

type ProviderAccountRoutingValues = {
  priority: number
  weight: number
  concurrency_limit: number
  cooldown_seconds: number
}

type ProviderAccountRoutingDialogProps = {
  account: ProviderAccountSummary
  isSaving: boolean
  onOpenChange: (open: boolean) => void
  onSave: (values: ProviderAccountRoutingValues) => void
}

export function ProviderAccountRoutingDialog(
  props: ProviderAccountRoutingDialogProps
) {
  const { t } = useTranslation()
  const [values, setValues] = useState<ProviderAccountRoutingValues>({
    priority: props.account.priority,
    weight: props.account.weight,
    concurrency_limit: props.account.concurrency_limit,
    cooldown_seconds: props.account.cooldown_seconds,
  })

  const setNumber = (
    field: keyof ProviderAccountRoutingValues,
    value: string
  ) => {
    const parsed = Number(value)
    setValues((current) => ({
      ...current,
      [field]: Number.isFinite(parsed) ? parsed : 0,
    }))
  }

  return (
    <Dialog open onOpenChange={props.onOpenChange}>
      <DialogContent className='yucore-app-shell sm:max-w-lg'>
        <DialogHeader>
          <DialogTitle>
            {t('Edit {{title}}', { title: props.account.name })}
          </DialogTitle>
          <DialogDescription>
            {t('Priority / Weight')} / {t('Concurrency / Cooldown')}
          </DialogDescription>
        </DialogHeader>

        <div className='grid gap-4 py-2 sm:grid-cols-2'>
          <NumberField
            id='provider-account-priority'
            label={t('Account Priority')}
            value={values.priority}
            onChange={(value) => setNumber('priority', value)}
          />
          <NumberField
            id='provider-account-weight'
            label={t('Account Weight')}
            min={0}
            value={values.weight}
            onChange={(value) => setNumber('weight', value)}
          />
          <NumberField
            id='provider-account-concurrency'
            label={t('Concurrency Limit')}
            min={0}
            value={values.concurrency_limit}
            description={t('0 means unlimited')}
            onChange={(value) => setNumber('concurrency_limit', value)}
          />
          <NumberField
            id='provider-account-cooldown'
            label={t('Cooldown Seconds')}
            min={0}
            value={values.cooldown_seconds}
            onChange={(value) => setNumber('cooldown_seconds', value)}
          />
        </div>

        <DialogFooter>
          <Button
            type='button'
            variant='outline'
            onClick={() => props.onOpenChange(false)}
          >
            {t('Cancel')}
          </Button>
          <Button
            type='button'
            disabled={props.isSaving}
            onClick={() =>
              props.onSave({
                priority: Math.trunc(values.priority),
                weight: Math.max(0, Math.trunc(values.weight)),
                concurrency_limit: Math.max(
                  0,
                  Math.trunc(values.concurrency_limit)
                ),
                cooldown_seconds: Math.max(
                  0,
                  Math.trunc(values.cooldown_seconds)
                ),
              })
            }
          >
            {t('Save Changes')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}

function NumberField(props: {
  id: string
  label: string
  value: number
  min?: number
  description?: string
  onChange: (value: string) => void
}) {
  return (
    <div className='grid gap-2'>
      <Label htmlFor={props.id}>{props.label}</Label>
      <Input
        id={props.id}
        type='number'
        min={props.min}
        value={props.value}
        onChange={(event) => props.onChange(event.target.value)}
      />
      {props.description ? (
        <p className='text-muted-foreground text-xs'>{props.description}</p>
      ) : null}
    </div>
  )
}
