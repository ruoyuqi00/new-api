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
import { Link } from '@tanstack/react-router'
import { CheckCircle2 } from 'lucide-react'
import { useEffect } from 'react'

import {
  YUCORE_BRAND_NAME,
  YucoreBackground,
  YucoreBrandMark,
} from '@/features/yucore-brand'
import { useYucoreTranslation } from '@/features/yucore-brand/i18n/use-yucore-translation'

type AuthLayoutProps = {
  children: React.ReactNode
}

const authModelTags = [
  { label: 'ChatGPT', translate: false },
  { label: 'Codex', translate: false },
  { label: 'gpt-5.5-Fast', translate: false },
  { label: 'Image route', translate: true },
  { label: 'Video route', translate: true },
  { label: 'Studio', translate: true },
]

const authTrustItems = [
  'OpenAI-compatible API keys',
  'Wallet and quota ledger',
  'Request logs and token audit',
  'Model hub and media studio',
]

const authMotionNodes = [
  ['Identity', 'session protected', 'yucore-auth-signal-a'],
  ['Wallet', 'quota linked', 'yucore-auth-signal-b'],
  ['API Keys', 'scopes ready', 'yucore-auth-signal-c'],
  ['Studio', 'media unlocked', 'yucore-auth-signal-d'],
]

export function AuthLayout({ children }: AuthLayoutProps) {
  const { t } = useYucoreTranslation()

  useEffect(() => {
    const previousTitle = document.title
    document.title = YUCORE_BRAND_NAME

    return () => {
      document.title = previousTitle
    }
  }, [])

  return (
    <div className='yucore-app-shell yucore-auth-page bg-background text-foreground relative min-h-svh overflow-hidden'>
      <YucoreBackground
        active
        coreMode='ambient'
        corePlacement='auth'
        intensity='hero'
        className='yucore-auth-background yucore-background-motion-focused fixed'
      />
      <Link
        to='/'
        className='absolute top-4 left-4 z-10 transition-opacity hover:opacity-80 sm:top-8 sm:left-8'
        aria-label={t('Back to Home')}
      >
        <YucoreBrandMark />
      </Link>

      <div className='yucore-auth-stage relative z-10 mx-auto grid min-h-svh w-full max-w-7xl items-center gap-8 px-4 py-24 min-[920px]:grid-cols-[1.08fr_0.92fr] min-[920px]:py-16 sm:px-8 lg:px-10'>
        <section className='yucore-auth-visual relative hidden min-h-[min(740px,calc(100svh-8rem))] min-[920px]:flex min-[920px]:flex-col min-[920px]:justify-end'>
          <div className='yucore-auth-visual-copy relative z-10 max-w-2xl pb-10'>
            <div className='mb-4 inline-flex rounded-full border border-cyan-300/20 bg-black/28 px-3 py-1 text-xs font-medium text-cyan-100 backdrop-blur-xl'>
              {t('YuCore access station')}
            </div>
            <h1 className='max-w-2xl text-[clamp(2.8rem,4.8vw,4.85rem)] leading-[0.9] font-semibold tracking-tight text-white'>
              YuCore AI Core
            </h1>
            <div className='mt-2 max-w-2xl bg-[linear-gradient(100deg,#67e8f9_0%,#facc15_52%,#fb7185_100%)] bg-clip-text text-[clamp(2.3rem,4vw,4.2rem)] leading-[0.9] font-semibold tracking-tight text-transparent'>
              {t('access gateway')}
            </div>
            <p className='mt-5 max-w-xl text-sm leading-7 text-white/58'>
              {t(
                'Register once to connect API keys, model routing, wallet visibility, usage audit, Codex workflows, and creative studio production.'
              )}
            </p>

            <div className='mt-6 flex flex-wrap gap-2'>
              {authModelTags.map((tag) => (
                <span
                  key={tag.label}
                  className='rounded-full border border-white/10 bg-black/25 px-3 py-1.5 text-xs font-medium text-white/58 backdrop-blur-md'
                >
                  {tag.translate ? t(tag.label) : tag.label}
                </span>
              ))}
            </div>

            <div className='mt-6 grid max-w-xl gap-2 sm:grid-cols-2'>
              {authTrustItems.map((item) => (
                <div
                  key={item}
                  className='flex items-center gap-2 text-sm text-white/58'
                >
                  <CheckCircle2
                    className='size-4 shrink-0 text-emerald-300'
                    aria-hidden='true'
                  />
                  {t(item)}
                </div>
              ))}
            </div>
          </div>

          <div className='yucore-auth-signal-field pointer-events-none absolute inset-0'>
            {authMotionNodes.map(([title, detail, className]) => (
              <div
                key={title}
                className={`yucore-auth-signal ${className} absolute rounded-2xl border border-white/10 bg-black/60 px-3 py-2.5 text-left`}
              >
                <div className='truncate text-sm font-semibold text-white'>
                  {t(title)}
                </div>
                <div className='mt-0.5 truncate text-xs text-white/42'>
                  {t(detail)}
                </div>
              </div>
            ))}
          </div>
          <div className='yucore-auth-axis pointer-events-none absolute inset-x-[-10%] top-[47%] h-px' />
        </section>

        <div className='yucore-auth-form-column mx-auto flex w-full max-w-[480px] flex-col justify-center'>
          <div className='mb-5 min-[920px]:hidden'>
            <div className='mb-3'>
              <YucoreBrandMark />
            </div>
            <h1 className='text-3xl leading-tight font-semibold tracking-tight text-white'>
              {t('Create the YuCore access layer.')}
            </h1>
            <p className='mt-3 text-sm leading-6 text-white/52'>
              {t(
                'API keys, quota, logs, model routes, and studio workflows start from one account.'
              )}
            </p>
          </div>
          <div className='yucore-panel yucore-auth-form-panel text-foreground relative overflow-hidden rounded-3xl p-5 sm:p-8'>
            <div className='pointer-events-none absolute inset-x-0 top-0 h-px bg-[linear-gradient(90deg,transparent,rgba(255,255,255,0.42),transparent)]' />
            <div className='pointer-events-none absolute inset-0 bg-[radial-gradient(circle_at_50%_0%,rgba(103,232,249,0.1),transparent_36%)]' />
            <div className='relative'>{children}</div>
          </div>
        </div>
      </div>
    </div>
  )
}
