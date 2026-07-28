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
import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { BookOpen, CircleDollarSign, FileText } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { PublicLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { Markdown } from '@/components/ui/markdown'
import { Skeleton } from '@/components/ui/skeleton'

const apiDocsPath = '/developer-docs/yucore-api.md'

async function fetchApiDocs(): Promise<string> {
  const response = await fetch(apiDocsPath)
  if (!response.ok) {
    throw new Error(`HTTP ${response.status}`)
  }
  return response.text()
}

export function ApiDocs() {
  const { t } = useTranslation()
  const docsQuery = useQuery({
    queryKey: ['public-api-docs'],
    queryFn: fetchApiDocs,
    staleTime: Number.POSITIVE_INFINITY,
  })

  return (
    <PublicLayout showMainContainer={false} showYuCoreBackground>
      <div className='mx-auto grid min-h-svh w-full max-w-[90rem] grid-cols-1 pt-20 lg:grid-cols-[15rem_minmax(0,1fr)]'>
        <aside className='border-white/10 px-5 py-5 lg:sticky lg:top-20 lg:h-[calc(100svh-5rem)] lg:border-r lg:px-6 lg:py-8'>
          <div className='flex items-center gap-3'>
            <span className='grid size-9 place-items-center rounded-md border border-cyan-200/18 bg-cyan-300/8 text-cyan-100'>
              <BookOpen className='size-4' aria-hidden='true' />
            </span>
            <div className='min-w-0'>
              <div className='text-sm font-semibold text-white'>YuAPI</div>
              <div className='text-xs text-white/48'>{t('Docs')}</div>
            </div>
          </div>

          <div className='mt-5 flex gap-2 lg:grid'>
            <Button
              size='sm'
              variant='outline'
              className='justify-start border-white/10 bg-white/[0.035] text-white hover:bg-white/[0.08]'
              render={<a href={apiDocsPath} target='_blank' rel='noreferrer' />}
            >
              <FileText data-icon='inline-start' />
              Markdown
            </Button>
            <Button
              size='sm'
              variant='outline'
              className='justify-start border-white/10 bg-white/[0.035] text-white hover:bg-white/[0.08]'
              render={<Link to='/pricing' />}
            >
              <CircleDollarSign data-icon='inline-start' />
              {t('Pricing')}
            </Button>
          </div>
        </aside>

        <main className='min-w-0 border-t border-white/10 bg-black/20 px-5 py-8 sm:px-8 lg:border-t-0 lg:px-12 lg:py-10'>
          {docsQuery.isLoading && (
            <div
              className='mx-auto grid max-w-4xl gap-4'
              aria-label={t('Loading...')}
            >
              <Skeleton className='h-9 w-2/5 bg-white/8' />
              <Skeleton className='h-4 w-full bg-white/8' />
              <Skeleton className='h-4 w-5/6 bg-white/8' />
              <Skeleton className='mt-5 h-64 w-full bg-white/8' />
            </div>
          )}

          {docsQuery.isError && (
            <div className='mx-auto max-w-4xl border-l-2 border-rose-300/50 py-2 pl-4 text-sm text-rose-100'>
              {t('Failed to load')}: {apiDocsPath}
            </div>
          )}

          {docsQuery.data && (
            <Markdown className='mx-auto max-w-4xl text-white/76 marker:text-cyan-200 [&_h1]:text-white [&_h2]:border-b [&_h2]:border-white/10 [&_h2]:pb-2 [&_h2]:text-white [&_h3]:text-white [&_pre]:border-white/10 [&_pre]:bg-black/45 [&_strong]:text-white [&_table]:border-white/10 [&_td]:border-white/10 [&_th]:border-white/10 [&_thead]:bg-white/[0.055]'>
              {docsQuery.data}
            </Markdown>
          )}
        </main>
      </div>
    </PublicLayout>
  )
}
