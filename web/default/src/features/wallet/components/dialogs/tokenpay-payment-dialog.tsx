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
import { Wallet01Icon } from '@hugeicons/core-free-icons'
import { HugeiconsIcon } from '@hugeicons/react'
import { ExternalLink } from 'lucide-react'
import { QRCodeSVG } from 'qrcode.react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { CopyButton } from '@/components/copy-button'
import { Alert, AlertDescription } from '@/components/ui/alert'
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
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'
import { Separator } from '@/components/ui/separator'

import { useTokenPayPayment } from '../../hooks/use-tokenpay-payment'
import {
  sortTokenPayPackages,
  tokenPayNetworkLabel,
} from '../../lib/tokenpay-payment-model'
import type { TokenPayNetwork, TopupInfo } from '../../types'

export function TokenPayPaymentDialog(props: {
  topupInfo: TopupInfo
  onClose: () => void
  onCredited: () => void | Promise<void>
}) {
  const { t } = useTranslation()
  const [amount, setAmount] = useState<number | null>(null)
  const [network, setNetwork] = useState<TokenPayNetwork | null>(null)
  const [transactionID, setTransactionID] = useState('')
  const payment = useTokenPayPayment(props.onCredited)
  const invoice = payment.invoice
  const statusLabels = {
    pending_provider: t('Preparing invoice'),
    unpaid: t('Awaiting payment'),
    paid: t('Paid'),
    failed: t('Payment failed'),
  }

  const submitTransaction = async () => {
    try {
      await payment.submitTransaction(transactionID.trim())
      toast.success(t('Submitted for manual review'))
      setTransactionID('')
    } catch {
      toast.error(t('Unable to submit transaction for review'))
    }
  }

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) props.onClose()
      }}
    >
      <DialogContent
        className='max-h-[85dvh] overflow-y-auto sm:max-w-2xl'
        showCloseButton={false}
      >
        <DialogHeader>
          <DialogTitle className='flex items-center gap-2'>
            <HugeiconsIcon icon={Wallet01Icon} size={20} aria-hidden='true' />
            {t('USDT top-up')}
          </DialogTitle>
          <DialogDescription>
            {invoice
              ? t('Pay the exact amount using the network and address below.')
              : t('Choose a fixed package and payment network.')}
          </DialogDescription>
        </DialogHeader>

        {invoice ? (
          <div className='space-y-4 py-2'>
            <div className='flex flex-wrap items-start justify-between gap-3'>
              <div className='space-y-1'>
                <p className='text-muted-foreground text-sm'>
                  {t('Send exactly')}
                </p>
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
                variant={
                  invoice.status === 'failed' ? 'destructive' : 'secondary'
                }
              >
                {statusLabels[invoice.status]}
              </Badge>
            </div>
            <Alert>
              <AlertDescription>
                {t(
                  'Use {{network}} only. Sending on another network may result in lost funds.',
                  { network: tokenPayNetworkLabel(invoice.network) }
                )}
              </AlertDescription>
            </Alert>
            <div className='flex flex-col items-center gap-4 sm:flex-row sm:items-start'>
              <QRCodeSVG
                value={invoice.address}
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
                <Button
                  type='button'
                  variant='outline'
                  className='w-full sm:w-auto'
                  onClick={() =>
                    window.open(
                      invoice.payment_url,
                      '_blank',
                      'noopener,noreferrer'
                    )
                  }
                >
                  <ExternalLink className='size-4' aria-hidden='true' />
                  {t('Open payment page')}
                </Button>
              </div>
            </div>
            <dl className='grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-2 text-sm'>
              <dt className='text-muted-foreground'>{t('Order number')}</dt>
              <dd className='text-right break-all'>{invoice.trade_no}</dd>
              <dt className='text-muted-foreground'>{t('Expires at')}</dt>
              <dd className='text-right'>
                {new Date(invoice.expires_at * 1000).toLocaleString()}
              </dd>
            </dl>
            {invoice.status === 'unpaid' && (
              <>
                <Separator />
                <div className='space-y-2'>
                  <Label htmlFor='tokenpay-transaction-hash'>
                    {t('Transaction hash')}
                  </Label>
                  <div className='flex flex-col gap-2 sm:flex-row'>
                    <Input
                      id='tokenpay-transaction-hash'
                      value={transactionID}
                      onChange={(event) => setTransactionID(event.target.value)}
                      className='min-w-0 flex-1'
                    />
                    <Button
                      type='button'
                      variant='outline'
                      disabled={payment.recovering || !transactionID.trim()}
                      onClick={() => void submitTransaction()}
                    >
                      {t('Submit for review')}
                    </Button>
                  </div>
                  {invoice.recovery_submitted && (
                    <p className='text-muted-foreground text-xs'>
                      {t('Submitted for manual review')}
                    </p>
                  )}
                </div>
              </>
            )}
          </div>
        ) : (
          <div className='space-y-4 py-2'>
            <fieldset className='space-y-2.5' disabled={payment.creating}>
              <legend className='text-sm font-medium'>
                {t('Select a USDT package')}
              </legend>
              <RadioGroup
                aria-label={t('Select a USDT package')}
                value={amount}
                onValueChange={(value) => setAmount(Number(value))}
                disabled={payment.creating}
                className='grid-cols-2 gap-2 sm:grid-cols-4'
              >
                {sortTokenPayPackages(
                  props.topupInfo.tokenpay_packages ?? []
                ).map((item) => (
                  <Label
                    key={item.usdt}
                    htmlFor={`tokenpay-package-${item.usdt}`}
                    className='border-input has-data-[checked]:border-primary has-data-[checked]:bg-primary/5 flex min-h-24 cursor-pointer flex-col items-start justify-start gap-2 rounded-md border p-3 font-normal'
                  >
                    <span className='flex w-full items-center justify-between gap-2'>
                      <span className='font-semibold tabular-nums'>
                        {item.usdt} USDT
                      </span>
                      <RadioGroupItem
                        id={`tokenpay-package-${item.usdt}`}
                        value={item.usdt}
                      />
                    </span>
                    <span className='text-muted-foreground text-xs'>
                      {t('Balance credit: {{amount}}', {
                        amount: item.balance,
                      })}
                    </span>
                    {item.label && (
                      <span className='text-xs'>{item.label}</span>
                    )}
                  </Label>
                ))}
              </RadioGroup>
            </fieldset>
            <fieldset className='space-y-2.5' disabled={payment.creating}>
              <legend className='text-sm font-medium'>{t('Network')}</legend>
              <RadioGroup
                aria-label={t('Network')}
                value={network}
                onValueChange={(value) => setNetwork(value as TokenPayNetwork)}
                disabled={payment.creating}
                className='grid-cols-1 sm:grid-cols-3'
              >
                {(props.topupInfo.tokenpay_networks ?? []).map((item) => (
                  <Label
                    key={item}
                    htmlFor={`tokenpay-network-${item}`}
                    className='border-input has-data-[checked]:border-primary has-data-[checked]:bg-primary/5 flex min-h-12 cursor-pointer gap-2 rounded-md border p-3'
                  >
                    <RadioGroupItem
                      id={`tokenpay-network-${item}`}
                      value={item}
                    />
                    {tokenPayNetworkLabel(item)}
                  </Label>
                ))}
              </RadioGroup>
            </fieldset>
          </div>
        )}

        {payment.createError && (
          <Alert variant='destructive'>
            <AlertDescription>
              {t(
                'Unable to create a payment invoice. Check your order history before starting a new payment.'
              )}
            </AlertDescription>
          </Alert>
        )}
        {payment.pollingError && invoice && (
          <Alert variant='destructive'>
            <AlertDescription className='space-y-2'>
              <p>
                {t(
                  'Unable to refresh payment status. Your last invoice is still shown.'
                )}
              </p>
              <Button
                variant='outline'
                size='sm'
                disabled={payment.fetching}
                onClick={() => void payment.retryOrder()}
              >
                {t('Retry')}
              </Button>
            </AlertDescription>
          </Alert>
        )}
        <DialogFooter>
          <Button variant='outline' onClick={props.onClose}>
            {t('Close')}
          </Button>
          {!invoice && (
            <Button
              disabled={
                amount === null ||
                network === null ||
                payment.creating ||
                Boolean(payment.createError)
              }
              onClick={() => {
                if (amount !== null && network) {
                  payment.createInvoice({ usdt_amount: amount, network })
                }
              }}
            >
              {payment.creating
                ? t('Creating invoice...')
                : t('Create payment invoice')}
            </Button>
          )}
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
