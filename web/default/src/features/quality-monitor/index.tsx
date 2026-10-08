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
  AlertCircleIcon,
  BrainIcon,
  CheckmarkCircle02Icon,
  RefreshIcon,
} from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Field, FieldLabel } from '@/components/ui/field'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import { getQualityMonitorOptions, getQualityMonitorResults } from './api'
import { PlanManager } from './components/plan-manager'
import { ResultsHistory } from './components/results-history'
import type { QualityMonitorFilters } from './types'

export function QualityMonitor() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const isAdmin = useAuthStore(
    (state) => (state.auth.user?.role ?? 0) >= ROLE.ADMIN
  )
  const [tab, setTab] = useState('results')
  const activeTab = isAdmin ? tab : 'results'
  const [filters, setFilters] = useState<QualityMonitorFilters>({
    group: '',
    model: '',
    page: 1,
    page_size: 20,
  })
  const options = useQuery({
    queryKey: ['quality-monitor', 'options'],
    queryFn: getQualityMonitorOptions,
    staleTime: 30_000,
  })
  const results = useQuery({
    queryKey: ['quality-monitor', 'results', filters],
    queryFn: () => getQualityMonitorResults(filters),
    staleTime: 15_000,
    enabled: activeTab === 'results',
  })
  const items = results.data?.items ?? []
  const passed = items.filter((result) => result.status === 'passed').length
  const wrong = items.filter((result) => result.status === 'failed').length
  const errors = items.filter((result) => result.status === 'error').length

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Quality monitor')}</SectionPageLayout.Title>
      <SectionPageLayout.Actions>
        <Button
          type='button'
          variant='outline'
          disabled={options.isFetching || results.isFetching}
          onClick={() => {
            void queryClient.invalidateQueries({
              queryKey: ['quality-monitor'],
            })
          }}
        >
          <HugeiconsIcon
            icon={RefreshIcon}
            data-icon='inline-start'
            aria-hidden='true'
          />
          {t('Refresh')}
        </Button>
      </SectionPageLayout.Actions>
      <SectionPageLayout.Content>
        <div className='mx-auto flex w-full max-w-7xl min-w-0 flex-col gap-5 pb-4'>
          <Card className='border-primary/20 from-primary/10 via-card to-card relative border bg-gradient-to-br'>
            <CardHeader className='gap-3 sm:pr-20'>
              <HugeiconsIcon
                icon={BrainIcon}
                className='text-primary size-7'
                aria-hidden='true'
              />
              <CardTitle className='text-xl tracking-tight sm:text-2xl'>
                {t('A closer look at model answers')}
              </CardTitle>
              <CardDescription className='max-w-3xl leading-relaxed'>
                {t(
                  'Compare real answers from gpt-6-astra and gpt-6.1-sol across your groups with repeatable questions.'
                )}
              </CardDescription>
            </CardHeader>
          </Card>
          {options.isError && (
            <Alert variant='destructive'>
              <AlertDescription>
                {t('Unable to load monitor options')}{' '}
                <Button
                  type='button'
                  variant='outline'
                  size='sm'
                  onClick={() => void options.refetch()}
                >
                  {t('Retry')}
                </Button>
              </AlertDescription>
            </Alert>
          )}
          <Tabs
            value={activeTab}
            onValueChange={(value) => setTab(String(value))}
            className='gap-4'
          >
            <TabsList>
              <TabsTrigger value='results'>{t('Probe results')}</TabsTrigger>
              {isAdmin && (
                <TabsTrigger value='plans'>{t('Monitoring plans')}</TabsTrigger>
              )}
            </TabsList>
            <TabsContent value='results' className='flex flex-col gap-4'>
              <div className='flex flex-col gap-3 sm:flex-row'>
                <Field className='sm:max-w-xs'>
                  <FieldLabel htmlFor='quality-group-filter'>
                    {t('Group')}
                  </FieldLabel>
                  <NativeSelect
                    className='w-full'
                    id='quality-group-filter'
                    value={filters.group}
                    onChange={(event) =>
                      setFilters((previous) => ({
                        ...previous,
                        group: event.target.value,
                        page: 1,
                      }))
                    }
                  >
                    <NativeSelectOption value=''>
                      {t('All groups')}
                    </NativeSelectOption>
                    {options.data?.groups.map((group) => (
                      <NativeSelectOption key={group.name} value={group.name}>
                        {group.name}
                      </NativeSelectOption>
                    ))}
                  </NativeSelect>
                </Field>
                <Field className='sm:max-w-xs'>
                  <FieldLabel htmlFor='quality-model-filter'>
                    {t('Model')}
                  </FieldLabel>
                  <NativeSelect
                    className='w-full'
                    id='quality-model-filter'
                    value={filters.model}
                    onChange={(event) =>
                      setFilters((previous) => ({
                        ...previous,
                        model: event.target.value,
                        page: 1,
                      }))
                    }
                  >
                    <NativeSelectOption value=''>
                      {t('All models')}
                    </NativeSelectOption>
                    <NativeSelectOption value='gpt-6-astra'>
                      gpt-6-astra
                    </NativeSelectOption>
                    <NativeSelectOption value='gpt-6.1-sol'>
                      gpt-6.1-sol
                    </NativeSelectOption>
                  </NativeSelect>
                </Field>
              </div>
              <div className='grid gap-3 sm:grid-cols-3'>
                <Card size='sm'>
                  <CardHeader>
                    <CardDescription className='flex items-center gap-2'>
                      <HugeiconsIcon
                        icon={CheckmarkCircle02Icon}
                        className='text-success size-4'
                        aria-hidden='true'
                      />
                      {t('Passed')}
                    </CardDescription>
                  </CardHeader>
                  <CardContent>
                    <p className='text-2xl font-semibold tabular-nums'>
                      {passed}
                    </p>
                    <p className='text-muted-foreground mt-1 text-xs'>
                      {t('On this page')}
                    </p>
                  </CardContent>
                </Card>
                <Card size='sm'>
                  <CardHeader>
                    <CardDescription className='flex items-center gap-2'>
                      <HugeiconsIcon
                        icon={AlertCircleIcon}
                        className='text-warning size-4'
                        aria-hidden='true'
                      />
                      {t('Wrong answer')}
                    </CardDescription>
                  </CardHeader>
                  <CardContent>
                    <p className='text-2xl font-semibold tabular-nums'>
                      {wrong}
                    </p>
                    <p className='text-muted-foreground mt-1 text-xs'>
                      {t('On this page')}
                    </p>
                  </CardContent>
                </Card>
                <Card size='sm'>
                  <CardHeader>
                    <CardDescription className='flex items-center gap-2'>
                      <HugeiconsIcon
                        icon={AlertCircleIcon}
                        className='text-destructive size-4'
                        aria-hidden='true'
                      />
                      {t('Call failed')}
                    </CardDescription>
                  </CardHeader>
                  <CardContent>
                    <p className='text-2xl font-semibold tabular-nums'>
                      {errors}
                    </p>
                    <p className='text-muted-foreground mt-1 text-xs'>
                      {t('On this page')}
                    </p>
                  </CardContent>
                </Card>
              </div>
              {results.isLoading && <Skeleton className='h-64 rounded-xl' />}
              {results.isError && (
                <Alert variant='destructive'>
                  <AlertDescription>
                    {t('Unable to load probe results')}{' '}
                    <Button
                      type='button'
                      variant='outline'
                      size='sm'
                      onClick={() => void results.refetch()}
                    >
                      {t('Retry')}
                    </Button>
                  </AlertDescription>
                </Alert>
              )}
              {!results.isLoading && !results.isError && (
                <ResultsHistory
                  items={items}
                  total={results.data?.total ?? 0}
                  page={filters.page}
                  pageSize={filters.page_size}
                  isAdmin={isAdmin}
                  isFetching={results.isFetching}
                  onPageChange={(page) =>
                    setFilters((previous) => ({ ...previous, page }))
                  }
                />
              )}
            </TabsContent>
            {isAdmin && (
              <TabsContent value='plans'>
                {options.data && <PlanManager options={options.data} />}
                {options.isLoading && <Skeleton className='h-64 rounded-xl' />}
              </TabsContent>
            )}
          </Tabs>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
