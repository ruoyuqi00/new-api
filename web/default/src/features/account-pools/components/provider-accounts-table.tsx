import { Gauge, Pencil, RefreshCw, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { Progress } from '@/components/ui/progress'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { getChannelTypeLabel } from '@/features/channels/lib/channel-utils'
import { formatTimestampToDate } from '@/lib/format'

import { getProviderAccountHealth } from '../provider-account-health'
import type { ProviderAccountSummary } from '../types'

type ProviderAccountsTableProps = {
  accounts: ProviderAccountSummary[]
  isLoading: boolean
  selectedIds: Set<number>
  onSelectedIdsChange: (ids: Set<number>) => void
  onUsage: (account: ProviderAccountSummary) => void
  onRefreshUsage: (account: ProviderAccountSummary) => void
  refreshingUsageIds: Set<number>
  onEdit: (account: ProviderAccountSummary) => void
  onDelete: (account: ProviderAccountSummary) => void
}

export function ProviderAccountsTable(props: ProviderAccountsTableProps) {
  const { t } = useTranslation()
  const allVisibleSelected =
    props.accounts.length > 0 &&
    props.accounts.every((account) => props.selectedIds.has(account.id))
  const someVisibleSelected = props.accounts.some((account) =>
    props.selectedIds.has(account.id)
  )

  return (
    <div className='min-h-0 min-w-0 flex-1 overflow-auto overscroll-contain rounded-md border [&_[data-slot=table-container]]:overflow-visible'>
      <Table className='min-w-[1320px]'>
        <TableHeader className='bg-background sticky top-0 z-10 shadow-[0_1px_0_var(--border)]'>
          <TableRow>
            <TableHead className='w-10'>
              <Checkbox
                checked={allVisibleSelected}
                indeterminate={!allVisibleSelected && someVisibleSelected}
                onCheckedChange={(checked) => {
                  props.onSelectedIdsChange(
                    checked
                      ? new Set(props.accounts.map((account) => account.id))
                      : new Set()
                  )
                }}
                aria-label={t('Select all')}
              />
            </TableHead>
            <TableHead>{t('Account')}</TableHead>
            <TableHead>{t('Provider / Type')}</TableHead>
            <TableHead>{t('Routing Pool')}</TableHead>
            <TableHead>{t('Status')}</TableHead>
            <TableHead>{t('Priority / Weight')}</TableHead>
            <TableHead>{t('Concurrency / Cooldown')}</TableHead>
            <TableHead>
              {t('Plan')} / {t('Usage')}
            </TableHead>
            <TableHead>{t('Expires / Last Used')}</TableHead>
            <TableHead>{t('Health / Last Error')}</TableHead>
            <TableHead className='w-24 text-right'>{t('Actions')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {props.accounts.map((account) => (
            <TableRow
              key={account.id}
              data-state={
                props.selectedIds.has(account.id) ? 'selected' : undefined
              }
            >
              <TableCell>
                <Checkbox
                  checked={props.selectedIds.has(account.id)}
                  onCheckedChange={(checked) => {
                    const next = new Set(props.selectedIds)
                    if (checked) next.add(account.id)
                    else next.delete(account.id)
                    props.onSelectedIdsChange(next)
                  }}
                  aria-label={t('Select account')}
                />
              </TableCell>
              <TableCell>
                <div className='max-w-48'>
                  <div className='truncate font-medium'>{account.name}</div>
                  <div className='text-muted-foreground mt-1 font-mono text-xs'>
                    {account.type === 'oauth_json' && account.credential_set
                      ? t('Configured')
                      : account.credential_preview || '****'}
                  </div>
                </div>
              </TableCell>
              <TableCell>
                <div>
                  {account.pool_adapter_type > 0
                    ? t(getChannelTypeLabel(account.pool_adapter_type))
                    : '-'}
                </div>
                <div className='text-muted-foreground mt-1 text-xs'>
                  {account.type}
                </div>
              </TableCell>
              <TableCell>
                <div className='max-w-40 truncate font-medium'>
                  {account.pool_name}
                </div>
                <div className='text-muted-foreground mt-1 text-xs'>
                  {account.pool_group} ·{' '}
                  {t('{{count}} channels', { count: account.channel_count })}
                </div>
              </TableCell>
              <TableCell>
                <Badge variant={account.status === 1 ? 'default' : 'secondary'}>
                  {account.status === 1 ? t('Enabled') : t('Disabled')}
                </Badge>
              </TableCell>
              <TableCell>
                {account.priority} / {account.weight}
              </TableCell>
              <TableCell>
                {account.concurrency_limit || t('Unlimited')} /{' '}
                {account.cooldown_seconds}s
              </TableCell>
              <TableCell>
                {(account.pool_adapter_type === 57 || account.pool_adapter_type === 48) &&
                account.type === 'oauth_json' ? (
                  <div className='min-w-36 space-y-1.5'>
                    <div className='flex items-center gap-2'>
                      <Badge variant='outline'>
                        {account.plan_type || t('Unknown')}
                      </Badge>
                      <Button
                        type='button'
                        variant='ghost'
                        size='icon-xs'
                        className='ml-auto'
                        onClick={() => props.onRefreshUsage(account)}
                        disabled={props.refreshingUsageIds.has(account.id)}
                        aria-label={t('Refresh')}
                      >
                        <RefreshCw
                          className={
                            props.refreshingUsageIds.has(account.id)
                              ? 'animate-spin'
                              : undefined
                          }
                        />
                      </Button>
                    </div>
                    {account.primary_usage_percent !== null ? (
                      <UsageLine
                        label={t('5-Hour Window')}
                        value={account.primary_usage_percent}
                      />
                    ) : null}
                    {account.secondary_usage_percent !== null ? (
                      <UsageLine
                        label={t('Weekly Window')}
                        value={account.secondary_usage_percent}
                      />
                    ) : null}
                    {account.primary_usage_percent === null &&
                    account.secondary_usage_percent === null ? (
                      <div className='text-muted-foreground text-xs'>
                        {t('No recent usage')}
                      </div>
                    ) : null}
                    {account.usage_updated_at > 0 ? (
                      <div className='text-muted-foreground text-[11px]'>
                        {t('Last updated:')}{' '}
                        {formatTimestampToDate(account.usage_updated_at)}
                      </div>
                    ) : null}
                  </div>
                ) : (
                  <span className='text-muted-foreground text-xs'>-</span>
                )}
              </TableCell>
              <TableCell className='text-xs whitespace-nowrap'>
                <div>{formatTimestampToDate(account.expires_at)}</div>
                <div className='text-muted-foreground mt-1'>
                  {formatTimestampToDate(account.last_used_at)}
                </div>
              </TableCell>
              <TableCell>
                <div className='max-w-48 space-y-1.5'>
                  <ProviderAccountHealthBadge account={account} />
                  <ProviderAccountError account={account} />
                  {account.usage_checked_at > 0 ? (
                    <div className='text-muted-foreground text-[11px]'>
                      {t('Checked:')}{' '}
                      {formatTimestampToDate(account.usage_checked_at)}
                    </div>
                  ) : null}
                </div>
              </TableCell>
              <TableCell>
                <div className='flex justify-end gap-1'>
                  <ActionButton
                    label={t('Edit')}
                    onClick={() => props.onEdit(account)}
                  >
                    <Pencil />
                  </ActionButton>
                  {(account.pool_adapter_type === 57 || account.pool_adapter_type === 48) &&
                  account.type === 'oauth_json' ? (
                    <ActionButton
                      label={t('Usage')}
                      onClick={() => props.onUsage(account)}
                    >
                      <Gauge />
                    </ActionButton>
                  ) : null}
                  <ActionButton
                    label={t('Delete')}
                    destructive
                    onClick={() => props.onDelete(account)}
                  >
                    <Trash2 />
                  </ActionButton>
                </div>
              </TableCell>
            </TableRow>
          ))}
          {!props.isLoading && props.accounts.length === 0 ? (
            <TableRow>
              <TableCell
                colSpan={11}
                className='text-muted-foreground h-32 text-center'
              >
                {t('No provider accounts found')}
              </TableCell>
            </TableRow>
          ) : null}
        </TableBody>
      </Table>
    </div>
  )
}

function ProviderAccountHealthBadge(props: {
  account: ProviderAccountSummary
}) {
  const { t } = useTranslation()
  const health = getProviderAccountHealth(props.account)
  if (health === 'healthy') {
    return (
      <Badge className='border-emerald-500/25 bg-emerald-500/10 text-emerald-600 dark:text-emerald-300'>
        {t('Healthy')}
      </Badge>
    )
  }
  if (health === 'rate_limited') {
    return (
      <Badge className='border-amber-500/25 bg-amber-500/10 text-amber-700 dark:text-amber-300'>
        HTTP 429 / {t('Rate Limited')}
      </Badge>
    )
  }
  if (health === 'auth_error') {
    return (
      <Badge variant='destructive'>
        HTTP {props.account.usage_upstream_status} /{' '}
        {t('Authentication Failed')}
      </Badge>
    )
  }
  if (health === 'error') {
    return (
      <Badge variant='destructive'>
        {props.account.usage_upstream_status > 0
          ? `HTTP ${props.account.usage_upstream_status}`
          : t('Refresh Failed')}
      </Badge>
    )
  }
  return <Badge variant='outline'>{t('Not Checked')}</Badge>
}

function ProviderAccountError(props: { account: ProviderAccountSummary }) {
  let error = props.account.last_error
  let displayError = error
  if (props.account.usage_last_error) {
    error = props.account.usage_last_error
    displayError = props.account.usage_error_code
      ? `${props.account.usage_error_code}: ${error}`
      : error
  }
  if (!error) return null
  return (
    <Tooltip>
      <TooltipTrigger render={<span className='block truncate text-xs' />}>
        {displayError}
      </TooltipTrigger>
      <TooltipContent className='max-w-sm'>{error}</TooltipContent>
    </Tooltip>
  )
}

function UsageLine(props: { label: string; value: number }) {
  const value = Math.max(0, Math.min(100, props.value))
  return (
    <div className='grid grid-cols-[3.25rem_1fr_2.5rem] items-center gap-1.5 text-[11px]'>
      <span className='text-muted-foreground'>{props.label}</span>
      <Progress value={value} className='h-1.5' />
      <span className='text-right tabular-nums'>{Math.round(value)}%</span>
    </div>
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
