import { Upload } from 'lucide-react'
import { useMemo, useState } from 'react'
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
import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { getChannelTypeLabel } from '@/features/channels/lib/channel-utils'

import type { ImportedAccount } from '../import-parser'
import type { AccountPoolSummary } from '../types'
import { AccountImportPanel } from './account-import-panel'
import { GrokOAuthImport } from './grok-oauth-import'

type ProviderAccountImportDialogProps = {
  open: boolean
  pools: AccountPoolSummary[]
  isImporting: boolean
  onOpenChange: (open: boolean) => void
  onImport: (poolId: number, accounts: ImportedAccount[]) => void
}

export function ProviderAccountImportDialog(
  props: ProviderAccountImportDialogProps
) {
  const { t } = useTranslation()
  const [poolId, setPoolId] = useState(0)
  const [accounts, setAccounts] = useState<ImportedAccount[]>([])
  const selectedPool = useMemo(
    () => props.pools.find((pool) => pool.id === poolId),
    [poolId, props.pools]
  )
  const mismatchedCount = useMemo(() => {
    if (!selectedPool?.adapter_type) return 0
    return accounts.filter(
      (account) =>
        account.adapter_type &&
        account.adapter_type !== selectedPool.adapter_type
    ).length
  }, [accounts, selectedPool])

  const handleOpenChange = (open: boolean) => {
    if (!open) {
      setAccounts([])
      setPoolId(0)
    }
    props.onOpenChange(open)
  }

  return (
    <Dialog open={props.open} onOpenChange={handleOpenChange}>
      <DialogContent className='yucore-app-shell flex h-[min(90dvh,48rem)] max-h-[calc(100dvh-2rem)] flex-col overflow-hidden sm:max-w-4xl'>
        <DialogHeader className='shrink-0'>
          <DialogTitle>{t('Import Provider Accounts')}</DialogTitle>
          <DialogDescription>
            {t(
              'Import accounts independently, then route them through a pool that controls groups and channel bindings.'
            )}
          </DialogDescription>
        </DialogHeader>

        <div className='min-h-0 flex-1 space-y-4 overflow-y-auto overscroll-contain px-1 py-2'>
          <div className='grid gap-2'>
            <Label htmlFor='provider-account-target-pool'>
              {t('Target Account Pool')}
            </Label>
            <NativeSelect
              id='provider-account-target-pool'
              value={poolId || ''}
              onChange={(event) => {
                setPoolId(Number(event.target.value))
                setAccounts([])
              }}
            >
              <NativeSelectOption value=''>
                {t('Select an account pool')}
              </NativeSelectOption>
              {props.pools.map((pool) => (
                <NativeSelectOption key={pool.id} value={pool.id}>
                  {pool.name} · {pool.group} ·{' '}
                  {pool.adapter_type > 0
                    ? t(getChannelTypeLabel(pool.adapter_type))
                    : t('Bound channels')}
                </NativeSelectOption>
              ))}
            </NativeSelect>
          </div>

          {selectedPool ? (
            <div className='space-y-4'>
              {selectedPool.adapter_type === 48 ? (
                <GrokOAuthImport
                  key={`grok-${selectedPool.id}`}
                  onAccountReady={(account) =>
                    setAccounts((current) => [...current, account])
                  }
                />
              ) : null}
              <AccountImportPanel
                key={selectedPool.id}
                provider={selectedPool.provider}
                existingCount={selectedPool.account_count}
                defaultCredentialType={
                  selectedPool.adapter_type === 57 ? 'oauth_json' : 'api_key'
                }
                onImport={(nextAccounts) =>
                  setAccounts((current) => [...current, ...nextAccounts])
                }
              />
            </div>
          ) : null}

          {accounts.length > 0 ? (
            <div className='bg-muted/30 flex flex-wrap items-center justify-between gap-3 border px-4 py-3 text-sm'>
              <div>
                <div className='font-medium'>
                  {t('{{count}} accounts ready to import', {
                    count: accounts.length,
                  })}
                </div>
                <div className='text-muted-foreground mt-1'>
                  {selectedPool?.name} · {selectedPool?.group}
                </div>
              </div>
              {mismatchedCount > 0 ? (
                <div className='text-destructive'>
                  {t('{{count}} accounts do not match the pool adapter', {
                    count: mismatchedCount,
                  })}
                </div>
              ) : null}
            </div>
          ) : null}
        </div>

        <DialogFooter className='shrink-0'>
          <Button
            type='button'
            variant='outline'
            onClick={() => handleOpenChange(false)}
          >
            {t('Cancel')}
          </Button>
          <Button
            type='button'
            disabled={
              !poolId ||
              accounts.length === 0 ||
              mismatchedCount > 0 ||
              props.isImporting
            }
            onClick={() => props.onImport(poolId, accounts)}
          >
            <Upload />
            {props.isImporting
              ? t('Importing...')
              : t('Import {{count}} Accounts', { count: accounts.length })}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
