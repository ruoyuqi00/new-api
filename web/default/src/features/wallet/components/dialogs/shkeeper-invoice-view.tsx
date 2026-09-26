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
import { QRCodeSVG } from 'qrcode.react'
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Label } from '@/components/ui/label'

import type { SHKeeperInvoice } from '../../types'

export function SHKeeperInvoiceView(props: { invoice: SHKeeperInvoice }) {
  const { t } = useTranslation()
  const invoice = props.invoice
  const statusLabels = {
    pending_provider: t('Preparing invoice'),
    unpaid: t('Awaiting payment'),
    partial: t('Partial payment'),
    confirming: t('Confirming payment'),
    paid: t('Paid'),
    overpaid: t('Overpaid'),
    late: t('Late payment'),
    failed: t('Payment failed'),
  }
  return (
    <div className='space-y-4 py-2'>
      <div className='flex flex-wrap items-start justify-between gap-3'>
        <div className='space-y-1'>
          <p className='text-muted-foreground text-sm'>{t('Send exactly')}</p>
          <p className='text-2xl font-semibold tabular-nums'>
            {invoice.usdt_amount} USDT
          </p>
          <p className='text-sm'>
            {t('Fixed balance credit: {{amount}}', {
              amount: invoice.balance_amount,
            })}
          </p>
        </div>
        <Badge
          variant={invoice.status === 'failed' ? 'destructive' : 'secondary'}
        >
          {statusLabels[invoice.status]}
        </Badge>
      </div>
      <Alert>
        <AlertDescription>
          {t(
            'Use {{network}} only. Sending on another network may result in lost funds.',
            { network: invoice.network }
          )}
        </AlertDescription>
      </Alert>
      {invoice.address ? (
        <div className='flex flex-col items-center gap-4 sm:flex-row sm:items-start'>
          <QRCodeSVG
            value={invoice.qr_payload || invoice.address}
            size={160}
            marginSize={2}
            title={t('Payment address QR code')}
            className='shrink-0'
          />
          <div className='w-full min-w-0 space-y-2'>
            <Label>{t('Payment address')}</Label>
            <div className='bg-muted/30 flex items-center gap-2 rounded-md border p-2'>
              <code className='min-w-0 flex-1 text-xs break-all'>
                {invoice.address}
              </code>
              <CopyButton
                value={invoice.address}
                aria-label={t('Copy payment address')}
              />
            </div>
            <p className='text-muted-foreground text-xs'>
              {t(
                'Send the exact package amount. Partial payments receive no credit; excess USDT does not increase the fixed balance credit.'
              )}
            </p>
          </div>
        </div>
      ) : (
        <Alert>
          <AlertDescription>
            {t(
              'The provider is preparing your payment address. Keep this invoice open; do not create another payment.'
            )}
          </AlertDescription>
        </Alert>
      )}
      <dl className='grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-2 text-sm'>
        <dt className='text-muted-foreground'>{t('Order number')}</dt>
        <dd className='text-right break-all'>{invoice.trade_no}</dd>
        <dt className='text-muted-foreground'>{t('Expires at')}</dt>
        <dd className='text-right'>
          {new Date(invoice.expires_at * 1000).toLocaleString()}
        </dd>
        <dt className='text-muted-foreground'>{t('Received')}</dt>
        <dd className='text-right tabular-nums'>
          {invoice.received_usdt} USDT
        </dd>
        <dt className='text-muted-foreground'>{t('Credited balance')}</dt>
        <dd className='text-right tabular-nums'>{invoice.credited_balance}</dd>
      </dl>
      {invoice.status === 'partial' && (
        <Alert>
          <AlertDescription>
            {t(
              'No balance has been credited. Complete the exact package payment before expiry.'
            )}
          </AlertDescription>
        </Alert>
      )}
      {invoice.status === 'late' && (
        <Alert>
          <AlertDescription>
            {t(
              'This payment arrived after expiry. Check the credited balance and contact support if needed. Excess USDT does not increase the fixed package credit.'
            )}
          </AlertDescription>
        </Alert>
      )}
      {(invoice.transaction_ids?.length ?? 0) > 0 && (
        <div className='space-y-2'>
          <Label>{t('Detected transactions')}</Label>
          <ul className='space-y-2'>
            {invoice.transaction_ids?.map((txid) => (
              <li key={txid} className='flex items-center gap-2'>
                <code className='min-w-0 flex-1 text-xs break-all'>{txid}</code>
                <CopyButton
                  value={txid}
                  aria-label={t('Copy transaction ID')}
                />
              </li>
            ))}
          </ul>
        </div>
      )}
    </div>
  )
}
