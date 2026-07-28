import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Edit3,
  Plus,
  RefreshCw,
  Route,
  Search,
  Trash2,
  Users,
} from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { SectionPageLayout } from '@/components/layout'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { getChannelTypeLabel } from '@/features/channels/lib/channel-utils'

import {
  createAccountPool,
  deleteAccountPool,
  getAccountPool,
  getAccountPoolChannels,
  getAccountPoolModels,
  getAccountPools,
  updateAccountPool,
} from './api'
import { AccountPoolDialog } from './components/account-pool-dialog'
import { PoolModelsDialog } from './components/pool-models-dialog'
import { ProviderAccountsView } from './components/provider-accounts-view'
import type {
  AccountPoolDetail,
  AccountPoolPayload,
  AccountPoolSummary,
} from './types'

export function AccountPools() {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [keyword, setKeyword] = useState('')
  const [activeTab, setActiveTab] = useState('accounts')
  const [search, setSearch] = useState('')
  const [dialogOpen, setDialogOpen] = useState(false)
  const [detail, setDetail] = useState<AccountPoolDetail | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<AccountPoolSummary | null>(
    null
  )
  const [modelsTarget, setModelsTarget] = useState<AccountPoolSummary | null>(
    null
  )

  const poolsQuery = useQuery({
    queryKey: ['account-pools', search],
    queryFn: () => getAccountPools(search),
  })
  const channelsQuery = useQuery({
    queryKey: ['account-pools', 'channels'],
    queryFn: getAccountPoolChannels,
    staleTime: 60_000,
  })

  const saveMutation = useMutation({
    mutationFn: async (payload: AccountPoolPayload) => {
      const result = payload.id
        ? await updateAccountPool(payload.id, payload)
        : await createAccountPool(payload)
      if (!result.success) {
        throw new Error(result.message || t('Failed to save account pool'))
      }
      return result
    },
    onSuccess: async () => {
      toast.success(t('Account pool saved'))
      setDialogOpen(false)
      setDetail(null)
      await queryClient.invalidateQueries({ queryKey: ['account-pools'] })
    },
    onError: (error) => toast.error(error.message),
  })

  const deleteMutation = useMutation({
    mutationFn: async (id: number) => {
      const result = await deleteAccountPool(id)
      if (!result.success) {
        throw new Error(result.message || t('Failed to delete account pool'))
      }
    },
    onSuccess: async () => {
      toast.success(t('Account pool deleted'))
      setDeleteTarget(null)
      await queryClient.invalidateQueries({ queryKey: ['account-pools'] })
    },
    onError: (error) => toast.error(error.message),
  })

  const modelsMutation = useMutation({
    mutationFn: async (id: number) => {
      const result = await getAccountPoolModels(id)
      if (!result.success || !result.data) {
        throw new Error(result.message || t('Model discovery failed'))
      }
      return result.data
    },
  })

  const openCreate = () => {
    setDetail(null)
    setDialogOpen(true)
  }

  const openEdit = async (pool: AccountPoolSummary) => {
    try {
      const response = await getAccountPool(pool.id)
      if (!response.success || !response.data) {
        throw new Error(response.message || t('Failed to load account pool'))
      }
      setDetail(response.data)
      setDialogOpen(true)
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to load account pool')
      )
    }
  }

  const openModels = (pool: AccountPoolSummary) => {
    modelsMutation.reset()
    setModelsTarget(pool)
    modelsMutation.mutate(pool.id)
  }

  const pools = poolsQuery.data?.data?.items ?? []
  const channels = channelsQuery.data?.data ?? []

  return (
    <>
      <SectionPageLayout fixedContent>
        <SectionPageLayout.Title>
          <span className='text-foreground'>{t('Account Management')}</span>
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          {activeTab === 'pools' ? (
            <>
              <Button
                variant='outline'
                size='icon'
                onClick={() => poolsQuery.refetch()}
                aria-label={t('Refresh')}
              >
                <RefreshCw />
              </Button>
              <Button onClick={openCreate}>
                <Plus />
                <span className='max-sm:hidden'>
                  {t('Create Account Pool')}
                </span>
              </Button>
            </>
          ) : null}
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <Tabs
            value={activeTab}
            onValueChange={setActiveTab}
            className='flex h-full min-h-0 flex-1 overflow-hidden'
          >
            <TabsList variant='line'>
              <TabsTrigger value='accounts'>
                <Users />
                {t('Provider Accounts')}
              </TabsTrigger>
              <TabsTrigger value='pools'>
                <Route />
                {t('Account Pools')}
              </TabsTrigger>
            </TabsList>
            <TabsContent
              value='accounts'
              className='flex min-h-0 overflow-hidden'
            >
              <ProviderAccountsView
                pools={pools}
                poolsLoading={poolsQuery.isLoading}
              />
            </TabsContent>
            <TabsContent value='pools' className='flex min-h-0 overflow-hidden'>
              <div className='text-foreground flex min-h-0 min-w-0 flex-1 flex-col gap-4'>
                <form
                  className='flex max-w-md gap-2'
                  onSubmit={(event) => {
                    event.preventDefault()
                    setSearch(keyword.trim())
                  }}
                >
                  <Input
                    value={keyword}
                    onChange={(event) => setKeyword(event.target.value)}
                    placeholder={t('Search account pools')}
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

                <div className='min-h-0 flex-1 overflow-auto rounded-md border'>
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{t('Name')}</TableHead>
                        <TableHead>{t('Provider')}</TableHead>
                        <TableHead>{t('Adapter Type')}</TableHead>
                        <TableHead>{t('Group')}</TableHead>
                        <TableHead>{t('Status')}</TableHead>
                        <TableHead>{t('Priority / Weight')}</TableHead>
                        <TableHead>{t('Accounts')}</TableHead>
                        <TableHead>{t('Routing')}</TableHead>
                        <TableHead className='w-24 text-right'>
                          {t('Actions')}
                        </TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {pools.map((pool) => (
                        <TableRow key={pool.id}>
                          <TableCell className='font-medium'>
                            {pool.name}
                          </TableCell>
                          <TableCell>{pool.provider || '-'}</TableCell>
                          <TableCell>
                            {pool.adapter_type > 0
                              ? t(getChannelTypeLabel(pool.adapter_type))
                              : '-'}
                          </TableCell>
                          <TableCell>{pool.group}</TableCell>
                          <TableCell>
                            <Badge
                              variant={
                                pool.status === 1 ? 'default' : 'secondary'
                              }
                            >
                              {pool.status === 1 ? t('Enabled') : t('Disabled')}
                            </Badge>
                          </TableCell>
                          <TableCell>
                            {pool.priority} / {pool.weight}
                          </TableCell>
                          <TableCell>
                            {pool.enabled_account_count} / {pool.account_count}
                          </TableCell>
                          <TableCell>
                            {pool.channel_count > 0
                              ? t('{{count}} bound channels', {
                                  count: pool.channel_count,
                                })
                              : t('Automatic by group')}
                          </TableCell>
                          <TableCell>
                            <div className='flex justify-end gap-1'>
                              <ActionButton
                                label={t('Discover models')}
                                onClick={() => openModels(pool)}
                              >
                                <Search />
                              </ActionButton>
                              <ActionButton
                                label={t('Edit')}
                                onClick={() => openEdit(pool)}
                              >
                                <Edit3 />
                              </ActionButton>
                              <ActionButton
                                label={t('Delete')}
                                onClick={() => setDeleteTarget(pool)}
                                destructive
                              >
                                <Trash2 />
                              </ActionButton>
                            </div>
                          </TableCell>
                        </TableRow>
                      ))}
                      {!poolsQuery.isLoading && pools.length === 0 ? (
                        <TableRow>
                          <TableCell
                            colSpan={9}
                            className='text-muted-foreground h-32 text-center'
                          >
                            {t('No account pools found')}
                          </TableCell>
                        </TableRow>
                      ) : null}
                    </TableBody>
                  </Table>
                </div>
              </div>
            </TabsContent>
          </Tabs>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <AccountPoolDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        detail={detail}
        channels={channels}
        isSaving={saveMutation.isPending}
        onSubmit={(payload) => saveMutation.mutate(payload)}
      />

      <PoolModelsDialog
        pool={modelsTarget}
        discovery={modelsMutation.data}
        error={modelsMutation.error?.message}
        isLoading={modelsMutation.isPending}
        onOpenChange={(open) => {
          if (!open) setModelsTarget(null)
        }}
        onRefresh={() => {
          if (modelsTarget) modelsMutation.mutate(modelsTarget.id)
        }}
      />

      <ConfirmDialog
        open={deleteTarget !== null}
        onOpenChange={(open) => !open && setDeleteTarget(null)}
        title={t('Delete Account Pool')}
        desc={t(
          'Deleting this pool also removes its provider accounts and channel bindings.'
        )}
        destructive
        isLoading={deleteMutation.isPending}
        handleConfirm={() =>
          deleteTarget && deleteMutation.mutate(deleteTarget.id)
        }
      />
    </>
  )
}

function ActionButton(props: {
  label: string
  onClick: () => void
  destructive?: boolean
  children: React.ReactNode
}) {
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button
            variant='ghost'
            size='icon-sm'
            className={props.destructive ? 'text-destructive' : undefined}
            onClick={props.onClick}
            aria-label={props.label}
          />
        }
      >
        {props.children}
      </TooltipTrigger>
      <TooltipContent>{props.label}</TooltipContent>
    </Tooltip>
  )
}
