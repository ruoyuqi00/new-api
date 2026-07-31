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
import { AlertTriangle, Gauge, Server } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { CopyButton } from '@/components/copy-button'
import { Badge } from '@/components/ui/badge'

const STANDARD_API_BASE_URL = 'https://api.yuaiapi.com/v1'
const VIP_API_BASE_URL = 'https://vip.yuaiapi.com/v1'

export function ApiEndpointNotice() {
  const { t } = useTranslation()

  return (
    <section
      aria-labelledby='api-base-url-title'
      className='shrink-0 overflow-hidden rounded-lg border border-cyan-400/30 bg-cyan-400/5 shadow-sm'
    >
      <div className='flex flex-wrap items-center justify-between gap-2 border-b border-cyan-400/20 px-3 py-2.5 sm:px-4'>
        <div className='flex min-w-0 items-center gap-2'>
          <Server className='size-4 shrink-0 text-cyan-500' />
          <h3 id='api-base-url-title' className='text-base font-semibold'>
            {t('API Base URL')}
          </h3>
          <Badge className='bg-cyan-500 text-cyan-950 hover:bg-cyan-500'>
            {t('Recommended')}
          </Badge>
        </div>
        <p className='flex items-center gap-1.5 text-xs font-medium text-amber-600 dark:text-amber-400'>
          <AlertTriangle className='size-3.5 shrink-0' />
          <span>{t('Do not use global.yuaiapi.com as an API Base URL.')}</span>
        </p>
      </div>

      <div className='grid divide-y divide-cyan-400/15 md:grid-cols-2 md:divide-x md:divide-y-0'>
        <div className='flex min-w-0 items-center gap-3 px-3 py-3 sm:px-4'>
          <Server className='size-4 shrink-0 text-cyan-600 dark:text-cyan-400' />
          <div className='min-w-0 flex-1'>
            <div className='text-muted-foreground mb-0.5 text-xs font-medium'>
              {t('For standard API requests')}
            </div>
            <code className='text-foreground block text-base font-bold break-all'>
              {STANDARD_API_BASE_URL}
            </code>
          </div>
          <CopyButton
            value={STANDARD_API_BASE_URL}
            variant='outline'
            size='icon'
            className='bg-background/70 size-8'
            tooltip={t('Copy API Base URL')}
          />
        </div>

        <div className='flex min-w-0 items-center gap-3 px-3 py-3 sm:px-4'>
          <Gauge className='size-4 shrink-0 text-rose-500' />
          <div className='min-w-0 flex-1'>
            <div className='text-muted-foreground mb-0.5 text-xs font-medium'>
              {t('For high concurrency and long-running requests')}
            </div>
            <code className='text-foreground block text-base font-bold break-all'>
              {VIP_API_BASE_URL}
            </code>
          </div>
          <CopyButton
            value={VIP_API_BASE_URL}
            variant='outline'
            size='icon'
            className='bg-background/70 size-8'
            tooltip={t('Copy API Base URL')}
          />
        </div>
      </div>
    </section>
  )
}
