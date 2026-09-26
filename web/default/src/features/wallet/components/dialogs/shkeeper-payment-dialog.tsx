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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '@/components/ui/dialog'
import { Separator } from '@/components/ui/separator'

import { useSHKeeperPayment } from '../../hooks/use-shkeeper-payment'
import { canRecoverSHKeeperTransaction } from '../../lib/shkeeper-payment-model'
import type { SHKeeperNetwork, TopupInfo } from '../../types'
import { SHKeeperInvoiceView } from './shkeeper-invoice-view'
import { SHKeeperPackagePicker } from './shkeeper-package-picker'
import { SHKeeperTransactionRecovery } from './shkeeper-transaction-recovery'

export function SHKeeperPaymentDialog(props: {
  topupInfo: TopupInfo
  onClose: () => void
  onCredited: () => void | Promise<void>
}) {
  const { t } = useTranslation()
  const [amount, setAmount] = useState<number | null>(null)
  const [crypto, setCrypto] = useState<SHKeeperNetwork | null>(null)
  const payment = useSHKeeperPayment(props.onCredited)
  const canCreate =
    amount !== null &&
    crypto !== null &&
    !payment.creating &&
    !payment.createError
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
            {payment.invoice
              ? t('Pay the exact amount using the network and address below.')
              : t('Choose a fixed package and payment network.')}
          </DialogDescription>
        </DialogHeader>
        {payment.invoice ? (
          <SHKeeperInvoiceView invoice={payment.invoice} />
        ) : (
          <SHKeeperPackagePicker
            packages={props.topupInfo.shkeeper_packages ?? []}
            networks={props.topupInfo.shkeeper_networks ?? []}
            amount={amount}
            crypto={crypto}
            onAmountChange={setAmount}
            onCryptoChange={setCrypto}
            disabled={payment.creating || Boolean(payment.createError)}
            expiryMinutes={props.topupInfo.shkeeper_invoice_expiry_minutes}
          />
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
        {payment.pollingError && payment.invoice && (
          <Alert variant='destructive'>
            <AlertDescription>
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
        {payment.invoice &&
          props.topupInfo.shkeeper_transaction_recovery_enabled &&
          canRecoverSHKeeperTransaction(payment.invoice) && (
            <>
              <Separator />
              <SHKeeperTransactionRecovery
                onSubmit={payment.submitTransaction}
                pending={payment.recovering}
              />
            </>
          )}
        <DialogFooter>
          <Button variant='outline' onClick={props.onClose}>
            {t('Close')}
          </Button>
          {!payment.invoice && (
            <Button
              disabled={!canCreate}
              onClick={() => {
                if (amount !== null && crypto) {
                  payment.createInvoice({ usdt_amount: amount, crypto })
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
