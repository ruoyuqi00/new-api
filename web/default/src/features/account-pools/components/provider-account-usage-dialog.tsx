import { Check, Copy, RefreshCw } from 'lucide-react'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'

export type ProviderAccountUsageDialogData = {
  success: boolean
  message?: string
  upstream_status?: number
  data?: Record<string, unknown>
}

type ProviderAccountUsageDialogProps = {
  open: boolean
  accountName: string
  accountId: number
  response: ProviderAccountUsageDialogData | null
  isRefreshing?: boolean
  onRefresh: () => void | Promise<void>
  onOpenChange: (open: boolean) => void
}

export function ProviderAccountUsageDialog(
  props: ProviderAccountUsageDialogProps
) {
  const { t } = useTranslation()
  const { copiedText, copyToClipboard } = useCopyToClipboard({ notify: false })
  const rawJson = useMemo(() => {
    if (!props.response) return ''
    return JSON.stringify(props.response, null, 2)
  }, [props.response])
  const data = props.response?.data ?? {}
  const rateLimit = (data.rate_limit ?? {}) as Record<string, unknown>
  const requestWindow = (rateLimit.requests ?? {}) as Record<string, unknown>
  const tokenWindow = (rateLimit.tokens ?? {}) as Record<string, unknown>
  const billing = (data.billing ?? {}) as Record<string, unknown>
  const plan = String(data.plan_type ?? billing.plan ?? '').trim()
  const status = props.response?.upstream_status ?? 0
  const healthy = Boolean(props.response?.success && status >= 200 && status < 300)

  return (
    <Dialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={`${t('Usage')} · ${props.accountName} (#${props.accountId})`}
      contentClassName='yucore-app-shell sm:max-w-[760px]'
      bodyClassName='flex flex-col gap-4'
      footer={
        <Button
          type='button'
          variant='outline'
          onClick={() => props.onOpenChange(false)}
        >
          {t('Close')}
        </Button>
      }
    >
      <div className='flex flex-col gap-4'>
        {props.response?.message && !healthy ? (
          <div className='rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-800 dark:bg-red-950/30 dark:text-red-400'>
            {props.response.message}
          </div>
        ) : null}
        <Card size='sm' className='gap-0 py-0'>
          <CardHeader className='flex-row items-center justify-between p-4 pb-2'>
            <CardTitle className='text-sm'>{t('Status')}</CardTitle>
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={props.onRefresh}
              disabled={Boolean(props.isRefreshing)}
            >
              <RefreshCw className={props.isRefreshing ? 'animate-spin' : undefined} />
              {t('Refresh')}
            </Button>
          </CardHeader>
          <CardContent className='grid grid-cols-2 gap-3 p-4 pt-2 sm:grid-cols-4'>
            <StatusBadge
              label={healthy ? t('Healthy') : t('Refresh Failed')}
              variant={healthy ? 'success' : 'danger'}
              copyable={false}
            />
            <StatusBadge
              label={`HTTP ${status || '-'}`}
              variant='neutral'
              copyable={false}
            />
            <StatusBadge
              label={plan || t('Unknown')}
              variant='blue'
              copyable={false}
            />
            <StatusBadge
              label={rateLimit.headers_observed ? t('Observed') : t('Not Checked')}
              variant={rateLimit.headers_observed ? 'success' : 'neutral'}
              copyable={false}
            />
          </CardContent>
        </Card>

        <div className='grid grid-cols-1 gap-3 sm:grid-cols-2'>
          <QuotaWindow title={t('Requests')} value={requestWindow} />
          <QuotaWindow title={t('Tokens')} value={tokenWindow} />
        </div>

        <div className='rounded-lg border'>
          <div className='flex items-center justify-between border-b px-3 py-2'>
            <span className='text-sm font-medium'>{t('Raw JSON')}</span>
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={() => copyToClipboard(rawJson)}
              disabled={!rawJson}
            >
              {copiedText === rawJson ? <Check /> : <Copy />}
              {t('Copy')}
            </Button>
          </div>
          <pre className='bg-muted/30 max-h-[38vh] overflow-auto p-3 text-xs break-words whitespace-pre-wrap'>
            {rawJson || '-'}
          </pre>
        </div>
      </div>
    </Dialog>
  )
}

function QuotaWindow({
  title,
  value,
}: {
  title: string
  value: Record<string, unknown>
}) {
  const { t } = useTranslation()
  const limit = Number(value.limit)
  const remaining = Number(value.remaining)
  const hasValues = Number.isFinite(limit) || Number.isFinite(remaining)
  return (
    <Card size='sm'>
      <CardHeader className='p-3 pb-1'>
        <CardTitle className='text-sm'>{title}</CardTitle>
      </CardHeader>
      <CardContent className='grid grid-cols-2 gap-3 p-3 pt-1 text-xs'>
        <div>
          <div className='text-muted-foreground'>{t('Limit')}</div>
          <div className='mt-1 font-mono'>{hasValues && Number.isFinite(limit) ? limit : '-'}</div>
        </div>
        <div>
          <div className='text-muted-foreground'>{t('Remaining')}</div>
          <div className='mt-1 font-mono'>{hasValues && Number.isFinite(remaining) ? remaining : '-'}</div>
        </div>
      </CardContent>
    </Card>
  )
}
