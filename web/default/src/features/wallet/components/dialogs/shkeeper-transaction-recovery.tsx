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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'

export function SHKeeperTransactionRecovery(props: {
  onSubmit: (txid: string) => Promise<void>
  pending: boolean
}) {
  const { t } = useTranslation()
  const [txid, setTxid] = useState('')
  return (
    <form
      className='space-y-2'
      onSubmit={async (event) => {
        event.preventDefault()
        if (!txid.trim() || props.pending) return
        try {
          await props.onSubmit(txid.trim())
          setTxid('')
          toast.success(
            t(
              'Transaction rescan requested. Credit is applied only after payment verification.'
            )
          )
        } catch {
          toast.error(
            t('Unable to request a transaction rescan. Please try again.')
          )
        }
      }}
    >
      <Label htmlFor='shkeeper-transaction-id'>
        {t('Already paid? Submit your transaction ID')}
      </Label>
      <div className='flex flex-col gap-2 sm:flex-row'>
        <Input
          id='shkeeper-transaction-id'
          value={txid}
          onChange={(event) => setTxid(event.target.value)}
          placeholder={t('Transaction hash')}
          autoComplete='off'
          className='min-w-0 flex-1'
          disabled={props.pending}
        />
        <Button
          type='submit'
          variant='outline'
          disabled={props.pending || !txid.trim()}
        >
          {props.pending ? t('Submitting...') : t('Request rescan')}
        </Button>
      </div>
      <p className='text-muted-foreground text-xs'>
        {t(
          'A rescan asks the payment provider to check your transaction. Submitting a hash does not credit your balance directly.'
        )}
      </p>
    </form>
  )
}
