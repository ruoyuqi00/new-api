import { AlertTriangle, RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { ScrollArea } from '@/components/ui/scroll-area'
import { Spinner } from '@/components/ui/spinner'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'

import type { AccountPoolModelDiscovery, AccountPoolSummary } from '../types'

type PoolModelsDialogProps = {
  pool: AccountPoolSummary | null
  discovery?: AccountPoolModelDiscovery
  error?: string
  isLoading: boolean
  onOpenChange: (open: boolean) => void
  onRefresh: () => void
}

export function PoolModelsDialog(props: PoolModelsDialogProps) {
  const { t } = useTranslation()

  return (
    <Dialog open={props.pool !== null} onOpenChange={props.onOpenChange}>
      <DialogContent className='flex max-h-[min(90dvh,52rem)] flex-col overflow-hidden sm:max-w-4xl'>
        <DialogHeader>
          <DialogTitle>
            {t('Account pool model discovery')}
            {props.pool ? ` · ${props.pool.name}` : ''}
          </DialogTitle>
          <DialogDescription>
            {t(
              'Models are fetched with each enabled account. Only models supported by every successfully checked account are safe for automatic channel sync.'
            )}
          </DialogDescription>
        </DialogHeader>

        <ScrollArea className='min-h-0 flex-1 pr-3'>
          {props.isLoading ? (
            <div className='text-muted-foreground flex min-h-48 items-center justify-center gap-2 text-sm'>
              <Spinner />
              {t('Discovering account models...')}
            </div>
          ) : null}

          {!props.isLoading && props.error ? (
            <Alert variant='destructive'>
              <AlertTriangle />
              <AlertTitle>{t('Model discovery failed')}</AlertTitle>
              <AlertDescription>{props.error}</AlertDescription>
            </Alert>
          ) : null}

          {!props.isLoading && props.discovery ? (
            <div className='space-y-5'>
              <div className='flex flex-wrap items-center gap-2 text-sm'>
                <Badge variant='outline'>
                  {t('{{succeeded}}/{{total}} accounts checked', {
                    succeeded: props.discovery.succeeded_accounts,
                    total: props.discovery.total_accounts,
                  })}
                </Badge>
                <Badge variant='outline'>
                  {t('{{count}} common models', {
                    count: props.discovery.common_models.length,
                  })}
                </Badge>
                {props.discovery.channel_id > 0 ? (
                  <span className='text-muted-foreground'>
                    {t('Channel template: {{name}} (#{{id}})', {
                      name: props.discovery.channel_name,
                      id: props.discovery.channel_id,
                    })}
                  </span>
                ) : null}
              </div>

              {!props.discovery.complete ? (
                <Alert>
                  <AlertTriangle />
                  <AlertTitle>{t('Incomplete model coverage')}</AlertTitle>
                  <AlertDescription>
                    {t(
                      'At least one enabled account could not be checked. Review the account errors before adding detected models to a channel.'
                    )}
                  </AlertDescription>
                </Alert>
              ) : null}

              <section className='space-y-2'>
                <h3 className='text-sm font-medium'>{t('Common models')}</h3>
                {props.discovery.common_models.length > 0 ? (
                  <div className='flex flex-wrap gap-1.5'>
                    {props.discovery.common_models.map((model) => (
                      <Badge key={model} variant='secondary'>
                        {model}
                      </Badge>
                    ))}
                  </div>
                ) : (
                  <p className='text-muted-foreground text-sm'>
                    {t('No common models were detected.')}
                  </p>
                )}
              </section>

              <section className='space-y-2'>
                <h3 className='text-sm font-medium'>{t('Model coverage')}</h3>
                <div className='overflow-hidden rounded-md border'>
                  <Table>
                    <TableHeader>
                      <TableRow>
                        <TableHead>{t('Model')}</TableHead>
                        <TableHead className='w-36 text-right'>
                          {t('Supported accounts')}
                        </TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {props.discovery.coverage.map((item) => (
                        <TableRow key={item.model}>
                          <TableCell className='font-mono text-xs'>
                            {item.model}
                          </TableCell>
                          <TableCell className='text-right tabular-nums'>
                            {item.support_count} /{' '}
                            {props.discovery?.succeeded_accounts}
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                </div>
              </section>

              <section className='space-y-2'>
                <h3 className='text-sm font-medium'>{t('Account results')}</h3>
                <div className='grid gap-2 sm:grid-cols-2'>
                  {props.discovery.accounts.map((account) => (
                    <div
                      key={account.account_id}
                      className='rounded-md border p-3 text-sm'
                    >
                      <div className='flex items-center justify-between gap-3'>
                        <span className='truncate font-medium'>
                          {account.account_name}
                        </span>
                        <Badge
                          variant={account.success ? 'outline' : 'destructive'}
                        >
                          {account.success
                            ? t('{{count}} models', {
                                count: account.models.length,
                              })
                            : t('Failed')}
                        </Badge>
                      </div>
                      {!account.success && account.message ? (
                        <p className='text-destructive mt-2 text-xs break-words'>
                          {account.message}
                        </p>
                      ) : null}
                    </div>
                  ))}
                </div>
              </section>
            </div>
          ) : null}
        </ScrollArea>

        <DialogFooter>
          <Button
            type='button'
            variant='outline'
            onClick={props.onRefresh}
            disabled={props.isLoading || props.pool === null}
          >
            <RefreshCw
              className={props.isLoading ? 'animate-spin' : undefined}
            />
            {t('Discover again')}
          </Button>
          <Button type='button' onClick={() => props.onOpenChange(false)}>
            {t('Close')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
