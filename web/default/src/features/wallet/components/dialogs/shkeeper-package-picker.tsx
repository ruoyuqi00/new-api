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
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription } from '@/components/ui/alert'
import { Label } from '@/components/ui/label'
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group'

import { sortSHKeeperPackages } from '../../lib/shkeeper-payment-model'
import type { SHKeeperNetwork, SHKeeperPackage } from '../../types'

interface SHKeeperPackagePickerProps {
  packages: SHKeeperPackage[]
  networks: SHKeeperNetwork[]
  amount: number | null
  crypto: SHKeeperNetwork | null
  onAmountChange: (amount: number) => void
  onCryptoChange: (crypto: SHKeeperNetwork) => void
  disabled: boolean
  expiryMinutes?: number
}

export function SHKeeperPackagePicker(props: SHKeeperPackagePickerProps) {
  const { t } = useTranslation()
  const networkLabels = {
    'BNB-USDT': 'BSC (BEP20)',
    USDT: 'TRON (TRC20)',
    'POLYGON-USDT': 'Polygon',
  }
  return (
    <div className='space-y-4 py-2'>
      <fieldset className='space-y-2.5' disabled={props.disabled}>
        <legend className='text-sm font-medium'>
          {t('Select a USDT package')}
        </legend>
        <RadioGroup
          aria-label={t('Select a USDT package')}
          value={props.amount}
          onValueChange={(value) => props.onAmountChange(Number(value))}
          disabled={props.disabled}
          className='grid-cols-2 gap-2 sm:grid-cols-4'
        >
          {sortSHKeeperPackages(props.packages).map((item) => (
            <Label
              key={item.usdt}
              htmlFor={`shkeeper-package-${item.usdt}`}
              className='border-input has-data-[checked]:border-primary has-data-[checked]:bg-primary/5 flex min-h-24 cursor-pointer flex-col items-start justify-start gap-2 rounded-lg border p-3 font-normal'
            >
              <span className='flex w-full items-center justify-between gap-2'>
                <span className='font-semibold tabular-nums'>
                  {item.usdt} USDT
                </span>
                <RadioGroupItem
                  id={`shkeeper-package-${item.usdt}`}
                  value={item.usdt}
                />
              </span>
              <span className='text-muted-foreground text-xs'>
                {t('Balance credit: {{amount}}', { amount: item.balance })}
              </span>
              {item.label && <span className='text-xs'>{item.label}</span>}
            </Label>
          ))}
        </RadioGroup>
      </fieldset>
      <Alert>
        <AlertDescription>
          {t(
            'Send USDT only on the selected network. Transfers on another network cannot be credited.'
          )}
        </AlertDescription>
      </Alert>
      <fieldset className='space-y-2.5' disabled={props.disabled}>
        <legend className='text-sm font-medium'>{t('Network')}</legend>
        <RadioGroup
          aria-label={t('Network')}
          value={props.crypto}
          onValueChange={(value) =>
            props.onCryptoChange(value as SHKeeperNetwork)
          }
          disabled={props.disabled}
          className='grid-cols-1 sm:grid-cols-3'
        >
          {props.networks.map((crypto) => (
            <Label
              key={crypto}
              htmlFor={`shkeeper-network-${crypto}`}
              className='border-input has-data-[checked]:border-primary has-data-[checked]:bg-primary/5 flex min-h-12 cursor-pointer gap-2 rounded-lg border p-3'
            >
              <RadioGroupItem
                id={`shkeeper-network-${crypto}`}
                value={crypto}
              />
              {networkLabels[crypto]}
            </Label>
          ))}
        </RadioGroup>
      </fieldset>
      <p className='text-muted-foreground text-xs'>
        {t(
          'Send the exact package amount. Partial payments receive no credit; excess USDT does not increase the fixed balance credit.'
        )}
      </p>
      {props.expiryMinutes && (
        <p className='text-muted-foreground text-xs'>
          {t('Pay within {{minutes}} minutes after the invoice is created.', {
            minutes: props.expiryMinutes,
          })}
        </p>
      )}
    </div>
  )
}
