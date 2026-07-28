import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  ChevronDown,
  Gauge,
  Power,
  PowerOff,
  RefreshCw,
  Search,
  Timer,
  Trash2,
  Upload,
} from 'lucide-react'
import { lazy, Suspense, useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Button } from '@/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuCheckboxItem,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu'
import { Input } from '@/components/ui/input'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import type { CodexUsageDialogData } from '@/features/channels/components/dialogs/codex-usage-dialog'
import {
  assignProviderAccounts,
  deleteProviderAccount,
  deleteProviderAccounts,
  getProviderAccounts,
  getProviderAccountUsage,
  importProviderAccounts,
  refreshProviderAccountUsages,
  updateProviderAccountRouting,
  updateProviderAccountStatuses,
} from '../api'
import type { ImportedAccount } from '../import-parser'
import {
  getProviderAccountHealth,
  type ProviderAccountHealth,
} from '../provider-account-health'
import type {
  AccountPoolSummary,
  ProviderAccountSummary,
  ProviderAccountUsageBatchResponse,
} from '../types'
import { ProviderAccountImportDialog } from './provider-account-import-dialog'
import { ProviderAccountRoutingDialog } from './provider-account-routing-dialog'
import { ProviderAccountsTable } from './provider-accounts-table'
import {
  ProviderAccountUsageDialog,
  type ProviderAccountUsageDialogData,
} from './provider-account-usage-dialog'

const CodexUsageDialog = lazy(() =>
  import('@/features/channels/components/dialogs/codex-usage-dialog').then(
    (module) => ({ default: module.CodexUsageDialog })
  )
)

type ProviderAccountsViewProps = {
  pools: AccountPoolSummary[]
  poolsLoading: boolean
}

type HealthFilter = 'all' | ProviderAccountHealth

type AutoRefreshSettings = {
  enabled: boolean
  intervalMinutes: number
}

const AUTO_REFRESH_STORAGE_KEY = 'provider-account-quota-auto-refresh'
const AUTO_REFRESH_INTERVALS = [5, 15, 30, 60]

export function ProviderAccountsView(props: ProviderAccountsViewProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [keyword, setKeyword] = useState('')
  const [search, setSearch] = useState('')
  const [poolFilter, setPoolFilter] = useState(0)
  const [statusFilter, setStatusFilter] = useState(0)
  const [healthFilter, setHealthFilter] = useState<HealthFilter>('all')
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set())
  const [targetPoolId, setTargetPoolId] = useState(0)
  const [importOpen, setImportOpen] = useState(false)
  const [deleteTarget, setDeleteTarget] =
    useState<ProviderAccountSummary | null>(null)
  const [batchDeleteOpen, setBatchDeleteOpen] = useState(false)
  const [routingTarget, setRoutingTarget] =
    useState<ProviderAccountSummary | null>(null)
  const [usageTarget, setUsageTarget] = useState<ProviderAccountSummary | null>(
    null
  )
  const [usageResponse, setUsageResponse] =
    useState<CodexUsageDialogData | ProviderAccountUsageDialogData | null>(null)
  const [lastBatchResult, setLastBatchResult] = useState<{
    data: NonNullable<ProviderAccountUsageBatchResponse['data']>
  } | null>(null)
  const [autoRefresh, setAutoRefresh] = useState<AutoRefreshSettings>(() => {
    if (typeof window === 'undefined') {
      return { enabled: false, intervalMinutes: 15 }
    }
    try {
      const stored = window.localStorage.getItem(AUTO_REFRESH_STORAGE_KEY)
      if (!stored) return { enabled: false, intervalMinutes: 15 }
      const parsed = JSON.parse(stored) as Partial<AutoRefreshSettings>
      const intervalMinutes = AUTO_REFRESH_INTERVALS.includes(
        Number(parsed.intervalMinutes)
      )
        ? Number(parsed.intervalMinutes)
        : 15
      return { enabled: parsed.enabled === true, intervalMinutes }
    } catch {
      return { enabled: false, intervalMinutes: 15 }
    }
  })
  const [autoRefreshCountdown, setAutoRefreshCountdown] = useState(
    autoRefresh.intervalMinutes * 60
  )
  const nextAutoRefreshAt = useRef(
    Date.now() + autoRefresh.intervalMinutes * 60_000
  )
  const autoRefreshRunRef = useRef<() => boolean>(() => false)

  const accountsQuery = useQuery({
    queryKey: ['provider-accounts', search, poolFilter, statusFilter],
    queryFn: () =>
      getProviderAccounts({
        keyword: search,
        poolId: poolFilter,
        status: statusFilter,
      }),
  })
  const allAccounts = useMemo(
    () => accountsQuery.data?.data?.items ?? [],
    [accountsQuery.data?.data?.items]
  )
  const accounts = useMemo(
    () =>
      healthFilter === 'all'
        ? allAccounts
        : allAccounts.filter(
            (account) => getProviderAccountHealth(account) === healthFilter
          ),
    [allAccounts, healthFilter]
  )

  const refreshData = async () => {
    setSelectedIds(new Set())
    await queryClient.invalidateQueries({ queryKey: ['provider-accounts'] })
    await queryClient.invalidateQueries({ queryKey: ['account-pools'] })
  }

  const assignMutation = useMutation({
    mutationFn: () =>
      assignProviderAccounts([...selectedIds], targetPoolId).then((result) => {
        if (!result.success) {
          throw new Error(result.message || t('Failed to assign accounts'))
        }
      }),
    onSuccess: async () => {
      toast.success(t('Accounts assigned'))
      setTargetPoolId(0)
      await refreshData()
    },
    onError: (error) => toast.error(error.message),
  })

  const statusMutation = useMutation({
    mutationFn: (status: number) =>
      updateProviderAccountStatuses([...selectedIds], status).then((result) => {
        if (!result.success) {
          throw new Error(result.message || t('Failed to update accounts'))
        }
      }),
    onSuccess: async () => {
      toast.success(t('Account status updated'))
      await refreshData()
    },
    onError: (error) => toast.error(error.message),
  })

  const deleteMutation = useMutation({
    mutationFn: (id: number) =>
      deleteProviderAccount(id).then((result) => {
        if (!result.success) {
          throw new Error(result.message || t('Failed to delete account'))
        }
      }),
    onSuccess: async () => {
      toast.success(t('Account deleted'))
      setDeleteTarget(null)
      await refreshData()
    },
    onError: (error) => toast.error(error.message),
  })

  const batchDeleteMutation = useMutation({
    mutationFn: (accountIds: number[]) =>
      deleteProviderAccounts(accountIds).then((result) => {
        if (!result.success) {
          throw new Error(result.message || t('Failed to delete account'))
        }
        return result.data?.count ?? 0
      }),
    onSuccess: async (count) => {
      toast.success(t('{{count}} provider accounts deleted', { count }))
      setBatchDeleteOpen(false)
      await refreshData()
    },
    onError: (error) => toast.error(error.message),
  })

  const routingMutation = useMutation({
    mutationFn: ({
      id,
      values,
    }: {
      id: number
      values: {
        priority: number
        weight: number
        concurrency_limit: number
        cooldown_seconds: number
      }
    }) =>
      updateProviderAccountRouting(id, values).then((result) => {
        if (!result.success) {
          throw new Error(result.message || t('Failed to update accounts'))
        }
      }),
    onSuccess: async () => {
      toast.success(t('Saved successfully'))
      setRoutingTarget(null)
      await refreshData()
    },
    onError: (error) => toast.error(error.message),
  })

  const importMutation = useMutation({
    mutationFn: ({
      poolId,
      imported,
    }: {
      poolId: number
      imported: ImportedAccount[]
    }) =>
      importProviderAccounts(poolId, imported).then((result) => {
        if (!result.success) {
          throw new Error(result.message || t('Failed to import accounts'))
        }
        return result
      }),
    onSuccess: async (result) => {
      toast.success(
        t('{{count}} accounts imported', { count: result.data?.count ?? 0 })
      )
      setImportOpen(false)
      await refreshData()
    },
    onError: (error) => toast.error(error.message),
  })

  const usageMutation = useMutation({
    mutationFn: getProviderAccountUsage,
    onSuccess: async (result) => {
      setUsageResponse(result)
      if (!result.success) {
        toast.error(result.message || t('Failed to fetch usage'))
      }
      await queryClient.invalidateQueries({ queryKey: ['provider-accounts'] })
    },
    onError: (error) => toast.error(error.message),
  })

  const usageBatchMutation = useMutation({
    mutationFn: async (variables: {
      accountIds: number[]
      automatic?: boolean
    }) => {
      const result = await refreshProviderAccountUsages(variables.accountIds)
      if (!result.success || !result.data) {
        throw new Error(result.message || t('Failed to refresh account quotas'))
      }
      return result.data
    },
    onSuccess: async (data, variables) => {
      setLastBatchResult({ data })
      await queryClient.invalidateQueries({ queryKey: ['provider-accounts'] })
      if (!variables.automatic) {
        toast.success(
          t(
            'Quota refresh finished: {{succeeded}} succeeded, {{failed}} failed',
            {
              succeeded: data.succeeded,
              failed: data.failed,
            }
          )
        )
      } else if (data.failed > 0) {
        toast.warning(
          t('Automatic quota refresh found {{count}} failed accounts', {
            count: data.failed,
          })
        )
      }
    },
    onError: (error, variables) => {
      if (!variables.automatic) toast.error(error.message)
    },
  })

  const selectedCount = selectedIds.size
  const selectedAdapterTypes = useMemo(
    () =>
      new Set(
        accounts
          .filter((account) => selectedIds.has(account.id))
          .map((account) => account.pool_adapter_type)
          .filter((adapterType) => adapterType > 0)
      ),
    [accounts, selectedIds]
  )
  const assignablePools = useMemo(
    () =>
      props.pools.filter(
        (pool) =>
          selectedAdapterTypes.size !== 1 ||
          pool.adapter_type === 0 ||
          selectedAdapterTypes.has(pool.adapter_type)
      ),
    [props.pools, selectedAdapterTypes]
  )
  const selectedLabel = useMemo(
    () => t('{{count}} selected', { count: selectedCount }),
    [selectedCount, t]
  )
  const targetPoolCompatible = assignablePools.some(
    (pool) => pool.id === targetPoolId
  )
  const batchRefreshIds = useMemo(
    () =>
      selectedCount > 0
        ? [...selectedIds]
        : accounts.map((account) => account.id),
    [accounts, selectedCount, selectedIds]
  )
  const refreshingUsageIds = useMemo(() => {
    const ids = new Set<number>()
    if (usageMutation.isPending && usageMutation.variables) {
      ids.add(usageMutation.variables)
    }
    if (usageBatchMutation.isPending) {
      for (const id of usageBatchMutation.variables.accountIds) ids.add(id)
    }
    return ids
  }, [
    usageBatchMutation.isPending,
    usageBatchMutation.variables,
    usageMutation.isPending,
    usageMutation.variables,
  ])

  autoRefreshRunRef.current = () => {
    if (
      document.hidden ||
      usageBatchMutation.isPending ||
      accounts.length === 0
    ) {
      return false
    }
    usageBatchMutation.mutate({
      accountIds: accounts.map((account) => account.id),
      automatic: true,
    })
    return true
  }

  useEffect(() => {
    window.localStorage.setItem(
      AUTO_REFRESH_STORAGE_KEY,
      JSON.stringify(autoRefresh)
    )
    nextAutoRefreshAt.current =
      Date.now() + autoRefresh.intervalMinutes * 60_000
    setAutoRefreshCountdown(autoRefresh.intervalMinutes * 60)
  }, [autoRefresh])

  useEffect(() => {
    if (!autoRefresh.enabled) return
    const timer = window.setInterval(() => {
      const remainingSeconds = Math.max(
        0,
        Math.ceil((nextAutoRefreshAt.current - Date.now()) / 1000)
      )
      setAutoRefreshCountdown(remainingSeconds)
      if (remainingSeconds > 0) return
      if (!autoRefreshRunRef.current()) return
      nextAutoRefreshAt.current =
        Date.now() + autoRefresh.intervalMinutes * 60_000
      setAutoRefreshCountdown(autoRefresh.intervalMinutes * 60)
    }, 1000)
    return () => window.clearInterval(timer)
  }, [autoRefresh.enabled, autoRefresh.intervalMinutes])

  const openUsage = (account: ProviderAccountSummary) => {
    setUsageTarget(account)
    setUsageResponse(null)
    usageMutation.mutate(account.id)
  }

  const refreshUsage = (account: ProviderAccountSummary) => {
    setUsageTarget(null)
    setUsageResponse(null)
    usageMutation.mutate(account.id)
  }

  return (
    <div className='text-foreground flex h-full min-h-0 min-w-0 flex-1 flex-col gap-4 overflow-hidden'>
      <div className='flex flex-wrap items-center gap-2'>
        <form
          className='flex min-w-64 flex-1 gap-2 sm:max-w-md'
          onSubmit={(event) => {
            event.preventDefault()
            setSearch(keyword.trim())
            setSelectedIds(new Set())
          }}
        >
          <Input
            value={keyword}
            onChange={(event) => setKeyword(event.target.value)}
            placeholder={t('Search provider accounts')}
          />
          <Button
            type='submit'
            variant='outline'
            size='icon'
            aria-label={t('Search')}
          >
            <Search />
          </Button>
        </form>
        <NativeSelect
          className='w-full sm:w-44'
          value={poolFilter}
          onChange={(event) => {
            setPoolFilter(Number(event.target.value))
            setSelectedIds(new Set())
          }}
          aria-label={t('Account Pool')}
        >
          <NativeSelectOption value={0}>
            {t('All Account Pools')}
          </NativeSelectOption>
          {props.pools.map((pool) => (
            <NativeSelectOption key={pool.id} value={pool.id}>
              {pool.name}
            </NativeSelectOption>
          ))}
        </NativeSelect>
        <NativeSelect
          className='w-full sm:w-32'
          value={statusFilter}
          onChange={(event) => {
            setStatusFilter(Number(event.target.value))
            setSelectedIds(new Set())
          }}
          aria-label={t('Status')}
        >
          <NativeSelectOption value={0}>{t('All Status')}</NativeSelectOption>
          <NativeSelectOption value={1}>{t('Enabled')}</NativeSelectOption>
          <NativeSelectOption value={2}>{t('Disabled')}</NativeSelectOption>
        </NativeSelect>
        <NativeSelect
          className='w-full sm:w-40'
          value={healthFilter}
          onChange={(event) => {
            setHealthFilter(event.target.value as HealthFilter)
            setSelectedIds(new Set())
          }}
          aria-label={t('Account Health')}
        >
          <NativeSelectOption value='all'>{t('All Health')}</NativeSelectOption>
          <NativeSelectOption value='healthy'>
            {t('Healthy')}
          </NativeSelectOption>
          <NativeSelectOption value='rate_limited'>
            {t('Rate Limited')}
          </NativeSelectOption>
          <NativeSelectOption value='auth_error'>
            {t('Authentication Failed')}
          </NativeSelectOption>
          <NativeSelectOption value='error'>
            {t('Other Errors')}
          </NativeSelectOption>
          <NativeSelectOption value='never'>
            {t('Not Checked')}
          </NativeSelectOption>
        </NativeSelect>
        <Button
          variant='outline'
          size='icon'
          onClick={() => void accountsQuery.refetch()}
          aria-label={t('Refresh Account List')}
        >
          <RefreshCw />
        </Button>
        <Button
          variant='outline'
          onClick={() =>
            usageBatchMutation.mutate({ accountIds: batchRefreshIds })
          }
          disabled={
            batchRefreshIds.length === 0 || usageBatchMutation.isPending
          }
        >
          <Gauge
            className={
              usageBatchMutation.isPending ? 'animate-pulse' : undefined
            }
          />
          {selectedCount > 0
            ? t('Refresh Selected Quotas')
            : t('Refresh Visible Quotas')}
        </Button>
        <DropdownMenu>
          <DropdownMenuTrigger render={<Button variant='outline' />}>
            <Timer />
            <span className='max-sm:hidden'>
              {autoRefresh.enabled
                ? t('Auto refresh: {{time}}', {
                    time: formatCountdown(autoRefreshCountdown),
                  })
                : t('Auto Refresh')}
            </span>
            <ChevronDown />
          </DropdownMenuTrigger>
          <DropdownMenuContent align='end' className='w-56'>
            <DropdownMenuCheckboxItem
              checked={autoRefresh.enabled}
              onCheckedChange={(checked) =>
                setAutoRefresh((current) => ({
                  ...current,
                  enabled: checked === true,
                }))
              }
            >
              {t('Enable Automatic Quota Refresh')}
            </DropdownMenuCheckboxItem>
            <DropdownMenuSeparator />
            <DropdownMenuRadioGroup
              value={String(autoRefresh.intervalMinutes)}
              onValueChange={(value) =>
                setAutoRefresh((current) => ({
                  ...current,
                  intervalMinutes: Number(value),
                }))
              }
            >
              <DropdownMenuLabel>{t('Refresh Interval')}</DropdownMenuLabel>
              {AUTO_REFRESH_INTERVALS.map((minutes) => (
                <DropdownMenuRadioItem key={minutes} value={String(minutes)}>
                  {t('{{count}} minutes', { count: minutes })}
                </DropdownMenuRadioItem>
              ))}
            </DropdownMenuRadioGroup>
          </DropdownMenuContent>
        </DropdownMenu>
        <Button
          onClick={() => setImportOpen(true)}
          disabled={props.poolsLoading}
        >
          <Upload />
          {t('Import Accounts')}
        </Button>
      </div>

      <div className='bg-muted/20 flex min-h-10 min-w-0 flex-wrap items-center gap-2 border px-3 py-2'>
        <span className='text-muted-foreground mr-1 w-full text-sm sm:w-auto'>
          {selectedLabel}
        </span>
        <NativeSelect
          className='w-full sm:w-48'
          value={targetPoolId}
          onChange={(event) => setTargetPoolId(Number(event.target.value))}
          disabled={selectedCount === 0}
          aria-label={t('Target Account Pool')}
        >
          <NativeSelectOption value={0}>
            {t('Move to account pool')}
          </NativeSelectOption>
          {assignablePools.map((pool) => (
            <NativeSelectOption key={pool.id} value={pool.id}>
              {pool.name} · {pool.group}
            </NativeSelectOption>
          ))}
        </NativeSelect>
        <Button
          size='sm'
          variant='outline'
          disabled={
            selectedCount === 0 ||
            targetPoolId === 0 ||
            !targetPoolCompatible ||
            assignMutation.isPending
          }
          onClick={() => assignMutation.mutate()}
        >
          {t('Assign')}
        </Button>
        <Button
          size='sm'
          variant='outline'
          disabled={selectedCount === 0 || statusMutation.isPending}
          onClick={() => statusMutation.mutate(1)}
        >
          <Power />
          {t('Enable')}
        </Button>
        <Button
          size='sm'
          variant='outline'
          disabled={selectedCount === 0 || statusMutation.isPending}
          onClick={() => statusMutation.mutate(2)}
        >
          <PowerOff />
          {t('Disable')}
        </Button>
        <Button
          size='sm'
          variant='destructive'
          disabled={selectedCount === 0 || batchDeleteMutation.isPending}
          onClick={() => setBatchDeleteOpen(true)}
        >
          <Trash2 />
          {t('Delete Selected')}
        </Button>
        {lastBatchResult ? (
          <span className='text-muted-foreground ml-auto text-xs'>
            {t(
              'Last quota refresh: {{succeeded}} succeeded, {{failed}} failed',
              {
                succeeded: lastBatchResult.data.succeeded,
                failed: lastBatchResult.data.failed,
              }
            )}
          </span>
        ) : null}
      </div>

      <ProviderAccountsTable
        accounts={accounts}
        isLoading={accountsQuery.isLoading}
        selectedIds={selectedIds}
        onSelectedIdsChange={setSelectedIds}
        onUsage={openUsage}
        onRefreshUsage={refreshUsage}
        onEdit={setRoutingTarget}
        refreshingUsageIds={refreshingUsageIds}
        onDelete={setDeleteTarget}
      />
      <ProviderAccountImportDialog
        open={importOpen}
        pools={props.pools}
        isImporting={importMutation.isPending}
        onOpenChange={setImportOpen}
        onImport={(poolId, imported) =>
          importMutation.mutate({ poolId, imported })
        }
      />

      {routingTarget ? (
        <ProviderAccountRoutingDialog
          account={routingTarget}
          isSaving={routingMutation.isPending}
          onOpenChange={(open) => !open && setRoutingTarget(null)}
          onSave={(values) =>
            routingMutation.mutate({ id: routingTarget.id, values })
          }
        />
      ) : null}

      <ConfirmDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title={t('Delete Provider Account')}
        desc={t('This account will be removed from its routing pool.')}
        destructive
        isLoading={deleteMutation.isPending}
        handleConfirm={() =>
          deleteTarget && deleteMutation.mutate(deleteTarget.id)
        }
      />

      <ConfirmDialog
        open={batchDeleteOpen}
        onOpenChange={setBatchDeleteOpen}
        title={t('Delete Selected Provider Accounts')}
        desc={t(
          'Permanently delete {{count}} selected provider accounts? This action cannot be undone.',
          { count: selectedCount }
        )}
        destructive
        isLoading={batchDeleteMutation.isPending}
        handleConfirm={() => batchDeleteMutation.mutate([...selectedIds])}
      />

      {usageTarget && usageTarget.pool_adapter_type === 48 ? (
        <Suspense fallback={null}>
          <ProviderAccountUsageDialog
            open
            onOpenChange={(open) => {
              if (!open) {
                setUsageTarget(null)
                setUsageResponse(null)
              }
            }}
            accountName={usageTarget.name}
            accountId={usageTarget.id}
            response={usageResponse}
            onRefresh={() => usageMutation.mutate(usageTarget.id)}
            isRefreshing={usageMutation.isPending}
          />
        </Suspense>
      ) : null}
      {usageTarget && usageTarget.pool_adapter_type === 57 ? (
        <Suspense fallback={null}>
          <CodexUsageDialog
            open
            onOpenChange={(open) => {
              if (!open) {
                setUsageTarget(null)
                setUsageResponse(null)
              }
            }}
            channelDisplayName={usageTarget.name}
            channelDisplayId={String(usageTarget.id)}
            subjectLabel={t('Account')}
            response={usageResponse as CodexUsageDialogData | null}
            onRefresh={() => usageMutation.mutate(usageTarget.id)}
            isRefreshing={usageMutation.isPending}
          />
        </Suspense>
      ) : null}
    </div>
  )
}

function formatCountdown(totalSeconds: number): string {
  const minutes = Math.floor(totalSeconds / 60)
  const seconds = totalSeconds % 60
  return `${minutes}:${String(seconds).padStart(2, '0')}`
}
