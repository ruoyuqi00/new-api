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
import {
  Add01Icon,
  Clock01Icon,
  Delete02Icon,
  FlashIcon,
  PauseIcon,
  PencilEdit01Icon,
  PlayIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Alert, AlertDescription } from '@/components/ui/alert'
import {
  AlertDialog,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyTitle,
} from '@/components/ui/empty'
import { Skeleton } from '@/components/ui/skeleton'

import {
  deleteQualityMonitorPlan,
  getQualityMonitorPlans,
  runQualityMonitorPlan,
  saveQualityMonitorPlan,
} from '../api'
import { qualityMonitorPlanSchema } from '../lib/plan-schema'
import type { QualityMonitorOptions, QualityMonitorPlan } from '../types'
import { PlanForm } from './plan-form'

export function PlanManager(props: { options: QualityMonitorOptions }) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [editorOpen, setEditorOpen] = useState(false)
  const [editingPlan, setEditingPlan] = useState<QualityMonitorPlan>()
  const [deletingPlan, setDeletingPlan] = useState<QualityMonitorPlan>()
  const plans = useQuery({
    queryKey: ['quality-monitor', 'plans'],
    queryFn: getQualityMonitorPlans,
  })
  const save = useMutation({
    mutationFn: saveQualityMonitorPlan,
    onSuccess: () => {
      setEditorOpen(false)
      toast.success(t('Monitor plan saved'))
      void queryClient.invalidateQueries({ queryKey: ['quality-monitor'] })
    },
  })
  const remove = useMutation({
    mutationFn: deleteQualityMonitorPlan,
    onSuccess: () => {
      setDeletingPlan(undefined)
      toast.success(t('Monitor plan deleted'))
      void queryClient.invalidateQueries({ queryKey: ['quality-monitor'] })
    },
  })
  const run = useMutation({
    mutationFn: runQualityMonitorPlan,
    onSuccess: () => {
      toast.success(t('Probe queued. Refresh results after completion.'))
      void queryClient.invalidateQueries({ queryKey: ['quality-monitor'] })
    },
  })

  return (
    <div className='flex flex-col gap-4'>
      <div className='flex flex-wrap items-center justify-between gap-3'>
        <p className='text-muted-foreground max-w-2xl text-sm'>
          {t(
            'New plans start paused with private results. Run a probe whenever you are ready.'
          )}
        </p>
        <Button
          type='button'
          onClick={() => {
            setEditingPlan(undefined)
            setEditorOpen(true)
          }}
        >
          <HugeiconsIcon
            icon={Add01Icon}
            data-icon='inline-start'
            aria-hidden='true'
          />
          {t('Create plan')}
        </Button>
      </div>
      {plans.isLoading && <Skeleton className='h-52 rounded-xl' />}
      {plans.isError && (
        <Alert variant='destructive'>
          <AlertDescription>
            {t('Unable to load monitor plans')}{' '}
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={() => void plans.refetch()}
            >
              {t('Retry')}
            </Button>
          </AlertDescription>
        </Alert>
      )}
      {plans.data?.length === 0 && (
        <Empty className='rounded-xl border'>
          <EmptyHeader>
            <EmptyTitle>{t('No monitoring plans yet')}</EmptyTitle>
            <EmptyDescription>
              {t(
                'Choose your groups and questions to start observing model answers.'
              )}
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      )}
      <div className='grid items-start gap-4 xl:grid-cols-2'>
        {plans.data?.map((plan) => (
          <Card key={plan.id}>
            <CardHeader className='gap-3'>
              <div className='flex flex-wrap items-start justify-between gap-2'>
                <CardTitle className='min-w-0 break-words'>
                  {plan.name}
                </CardTitle>
                <div className='flex gap-1.5'>
                  <Badge variant={plan.enabled ? 'default' : 'secondary'}>
                    {plan.enabled ? t('Enabled') : t('Paused')}
                  </Badge>
                  <Badge variant='outline'>
                    {plan.published ? t('Published') : t('Private')}
                  </Badge>
                </div>
              </div>
              <CardDescription className='flex flex-wrap items-center gap-x-3 gap-y-1'>
                <span className='inline-flex items-center gap-1'>
                  <HugeiconsIcon
                    icon={Clock01Icon}
                    className='size-3.5'
                    aria-hidden='true'
                  />
                  {t('Every {{count}} minutes', {
                    count: plan.interval_minutes,
                  })}
                </span>
                <span>
                  {t('{{count}} questions', { count: plan.questions.length })}
                </span>
              </CardDescription>
            </CardHeader>
            <CardContent className='flex flex-col gap-3'>
              <div className='flex flex-wrap gap-1.5'>
                {plan.groups.map((group) => (
                  <Badge
                    variant='secondary'
                    key={group}
                    className='max-w-full break-all whitespace-normal'
                  >
                    {group}
                  </Badge>
                ))}
              </div>
              <div className='flex flex-wrap gap-2 text-xs'>
                {plan.models.map((model) => (
                  <span key={model} className='font-mono'>
                    {model}
                  </span>
                ))}
              </div>
              <dl className='text-muted-foreground grid gap-1 text-xs'>
                <div className='flex flex-wrap justify-between gap-x-2'>
                  <dt>{t('Last probe')}</dt>
                  <dd>
                    {plan.last_run_at
                      ? new Date(plan.last_run_at * 1000).toLocaleString()
                      : t('Never')}
                  </dd>
                </div>
                <div className='flex flex-wrap justify-between gap-x-2'>
                  <dt>{t('Next probe')}</dt>
                  <dd>
                    {plan.enabled && plan.next_run_at
                      ? new Date(plan.next_run_at * 1000).toLocaleString()
                      : '—'}
                  </dd>
                </div>
              </dl>
            </CardContent>
            <CardFooter className='flex flex-wrap gap-2'>
              <Button
                type='button'
                variant='outline'
                size='sm'
                disabled={save.isPending}
                onClick={() =>
                  save.mutate({
                    id: plan.id,
                    plan: qualityMonitorPlanSchema.parse({
                      ...plan,
                      enabled: !plan.enabled,
                    }),
                  })
                }
              >
                <HugeiconsIcon
                  icon={plan.enabled ? PauseIcon : PlayIcon}
                  data-icon='inline-start'
                  aria-hidden='true'
                />
                {plan.enabled ? t('Pause') : t('Enable')}
              </Button>
              <Button
                type='button'
                variant='outline'
                size='sm'
                disabled={run.isPending}
                onClick={() => run.mutate(plan.id)}
              >
                <HugeiconsIcon
                  icon={FlashIcon}
                  data-icon='inline-start'
                  aria-hidden='true'
                />
                {t('Run now')}
              </Button>
              <Button
                type='button'
                variant='ghost'
                size='sm'
                onClick={() => {
                  setEditingPlan(plan)
                  setEditorOpen(true)
                }}
              >
                <HugeiconsIcon
                  icon={PencilEdit01Icon}
                  data-icon='inline-start'
                  aria-hidden='true'
                />
                {t('Edit')}
              </Button>
              <Button
                type='button'
                variant='ghost'
                size='icon-sm'
                className='text-destructive ml-auto'
                aria-label={t('Delete plan')}
                onClick={() => setDeletingPlan(plan)}
              >
                <HugeiconsIcon icon={Delete02Icon} aria-hidden='true' />
              </Button>
            </CardFooter>
          </Card>
        ))}
      </div>
      <Dialog
        open={editorOpen}
        onOpenChange={(open) => {
          if (!save.isPending) setEditorOpen(open)
        }}
      >
        <DialogContent className='flex max-h-[92svh] flex-col sm:max-w-3xl'>
          <DialogHeader className='shrink-0 pr-8'>
            <DialogTitle>
              {editingPlan ? t('Edit monitor plan') : t('Create monitor plan')}
            </DialogTitle>
            <DialogDescription>
              {t('Configure group probes, question matching and publishing.')}
            </DialogDescription>
          </DialogHeader>
          <PlanForm
            key={editingPlan?.id ?? 'new'}
            options={props.options}
            plan={editingPlan}
            isSaving={save.isPending}
            onSave={(plan) => save.mutate({ id: editingPlan?.id, plan })}
            onCancel={() => setEditorOpen(false)}
          />
        </DialogContent>
      </Dialog>
      <AlertDialog
        open={Boolean(deletingPlan)}
        onOpenChange={(open) => {
          if (!open && !remove.isPending) setDeletingPlan(undefined)
        }}
      >
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('Delete monitor plan?')}</AlertDialogTitle>
            <AlertDialogDescription>
              {t(
                'The plan will stop running. Existing results will be retained.'
              )}
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <Button
              type='button'
              variant='outline'
              disabled={remove.isPending}
              onClick={() => setDeletingPlan(undefined)}
            >
              {t('Cancel')}
            </Button>
            <Button
              type='button'
              variant='destructive'
              disabled={remove.isPending}
              onClick={() => {
                if (deletingPlan) remove.mutate(deletingPlan.id)
              }}
            >
              {t('Delete')}
            </Button>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </div>
  )
}
