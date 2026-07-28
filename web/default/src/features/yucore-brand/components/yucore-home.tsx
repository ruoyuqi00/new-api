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
import { ArrowRight, CheckCircle2 } from 'lucide-react'
import { lazy, Suspense, useCallback, useEffect, useState } from 'react'

import { Button } from '@/components/ui/button'

import { useYucoreTranslation } from '../i18n/use-yucore-translation'
import {
  YucoreBackground,
  type YucoreBackgroundPreparation,
} from './yucore-background'
import { YucoreBrandMark } from './yucore-brand-mark'
import { YucoreCommandCenter } from './yucore-command-center'
import {
  YUCORE_BOOT_LOADER_DURATION_MS,
  YucoreEntranceLoader,
} from './yucore-entrance-loader'
import {
  scheduleYucoreHomeDetails,
  type YucoreHomeSchedulerHost,
} from './yucore-home-details-scheduler'
import { scheduleYucoreHomeRendererPrewarm } from './yucore-home-renderer-prewarm'
import { YucoreScrollReveal } from './yucore-scroll-reveal'

interface YucoreHomeProps {
  isAuthenticated?: boolean
}

const LazyYucoreHomeDetailsPrimary = lazy(() =>
  import('./yucore-home-details').then((module) => ({
    default: module.YucoreHomeDetailsPrimary,
  }))
)

const LazyYucoreHomeDetailsSecondary = lazy(() =>
  import('./yucore-home-details').then((module) => ({
    default: module.YucoreHomeDetailsSecondary,
  }))
)

export function YucoreHome(props: YucoreHomeProps) {
  const { t } = useYucoreTranslation()
  const [revealHero, setRevealHero] = useState(false)
  const [detailStage, setDetailStage] = useState<0 | 1 | 2>(0)
  const [backgroundPreparation, setBackgroundPreparation] =
    useState<YucoreBackgroundPreparation>('none')
  const showSecondaryDetails = useCallback(() => setDetailStage(2), [])

  useEffect(
    () =>
      scheduleYucoreHomeRendererPrewarm(
        window,
        YUCORE_BOOT_LOADER_DURATION_MS,
        () => setBackgroundPreparation('signal'),
        () => setBackgroundPreparation('all')
      ),
    []
  )

  useEffect(() => {
    if (!revealHero) return

    return scheduleYucoreHomeDetails(
      window as unknown as YucoreHomeSchedulerHost,
      () => setDetailStage(1)
    )
  }, [revealHero])

  return (
    <main
      className='yucore-app-shell yucore-home bg-background text-foreground relative min-h-svh overflow-hidden'
      data-yucore-home-phase={revealHero ? 'stable' : 'boot'}
    >
      <YucoreEntranceLoader onComplete={() => setRevealHero(true)} />
      <YucoreBackground
        active={revealHero}
        preparation={revealHero ? 'all' : backgroundPreparation}
        coreMode='ambient'
        corePlacement='hero'
        intensity='hero'
        className='yucore-home-fixed-background fixed'
      />
      <section className='yucore-home-gate relative z-10 flex min-h-svh items-center justify-center px-5 pt-24 pb-16 text-center'>
        <div className='yucore-home-gate-copy relative flex w-full max-w-3xl flex-col items-center'>
          <div
            className={
              revealHero
                ? 'landing-animate-fade-up mb-6 opacity-0'
                : 'mb-6 translate-y-6 opacity-0'
            }
          >
            <YucoreBrandMark />
          </div>
          <div
            className={
              revealHero
                ? 'landing-animate-fade-up mb-5 inline-flex items-center gap-2 rounded-full border border-amber-300/20 bg-black/35 px-3 py-1.5 text-xs font-medium text-amber-100 opacity-0 backdrop-blur-xl'
                : 'mb-5 inline-flex translate-y-6 items-center gap-2 rounded-full border border-amber-300/20 bg-black/35 px-3 py-1.5 text-xs font-medium text-amber-100 opacity-0 backdrop-blur-xl'
            }
            style={{ animationDelay: '80ms' }}
          >
            <span className='relative flex size-2'>
              <span className='absolute inline-flex h-full w-full animate-ping rounded-full bg-amber-300 opacity-70' />
              <span className='relative inline-flex size-2 rounded-full bg-amber-200' />
            </span>
            {t('YuCore AI Core is coming online')}
          </div>
          <h1
            className={
              revealHero
                ? 'yucore-intro-word yucore-home-title landing-animate-fade-up text-[2.25rem] leading-[0.9] font-semibold tracking-normal whitespace-nowrap text-white opacity-0 sm:text-[3.75rem] lg:text-[6.8rem]'
                : 'yucore-intro-word yucore-home-title translate-y-6 text-[2.25rem] leading-[0.9] font-semibold tracking-normal whitespace-nowrap text-white opacity-0 sm:text-[3.75rem] lg:text-[6.8rem]'
            }
            data-text={t('YuCore AI Core')}
            style={{ animationDelay: '160ms' }}
          >
            {t('YuCore AI Core')}
          </h1>
          <p
            className={
              revealHero
                ? 'landing-animate-fade-up mt-6 max-w-2xl text-sm leading-7 font-medium text-cyan-50/66 opacity-0 sm:text-base'
                : 'mt-6 max-w-2xl translate-y-6 text-sm leading-7 font-medium text-cyan-50/66 opacity-0 sm:text-base'
            }
            style={{ animationDelay: '260ms' }}
          >
            {t(
              'Gateway, media studio, billing, and quota intelligence are synchronizing.'
            )}
          </p>
          <div
            className={
              revealHero
                ? 'landing-animate-fade-up mt-6 flex items-center gap-4 opacity-0'
                : 'mt-6 flex translate-y-6 items-center gap-4 opacity-0'
            }
            style={{ animationDelay: '360ms' }}
          >
            <Link
              to={props.isAuthenticated ? '/dashboard' : '/sign-in'}
              className='text-sm font-semibold text-white/76 underline-offset-4 transition hover:text-white hover:underline'
            >
              {props.isAuthenticated ? t('Enter console') : t('Sign in')}
            </Link>
            <span className='h-1 w-1 rounded-full bg-white/28' />
            <Link
              to='/pricing'
              className='text-sm font-semibold text-white/76 underline-offset-4 transition hover:text-white hover:underline'
            >
              {t('Model hub')}
            </Link>
          </div>
          <a
            href='#yucore-command-system'
            className={
              revealHero
                ? 'landing-animate-fade-up absolute bottom-[-7rem] flex flex-col items-center gap-3 text-xs font-semibold tracking-[0.24em] text-white/40 uppercase opacity-0 transition hover:text-white/70'
                : 'absolute bottom-[-7rem] flex translate-y-6 flex-col items-center gap-3 text-xs font-semibold tracking-[0.24em] text-white/40 uppercase opacity-0'
            }
            style={{ animationDelay: '500ms' }}
          >
            <span>{t('Scroll to enter')}</span>
            <span className='yucore-scroll-cue h-14 w-px bg-gradient-to-b from-transparent via-white/60 to-transparent' />
          </a>
        </div>
      </section>

      <section
        id='yucore-command-system'
        className='relative z-10 mx-auto grid min-h-svh w-full max-w-7xl scroll-mt-10 items-center gap-8 px-5 pt-24 pb-12 min-[920px]:grid-cols-[0.86fr_1.14fr] min-[920px]:gap-5 sm:px-8 lg:px-10 xl:gap-10'
      >
        <div className='yucore-hero-spotlight pointer-events-none absolute top-[14%] left-[2%] h-72 w-72 rounded-full' />
        <div className='yucore-hero-spotlight yucore-hero-spotlight-right pointer-events-none absolute top-[16%] right-[7%] h-96 w-96 rounded-full' />
        <div className='max-w-3xl min-[920px]:max-w-none'>
          <div
            className={
              revealHero
                ? 'landing-animate-fade-up mb-5 opacity-0'
                : 'mb-5 translate-y-6 opacity-0'
            }
          >
            <YucoreBrandMark />
          </div>
          <div
            className={
              revealHero
                ? 'landing-animate-fade-up mb-5 inline-flex items-center gap-2 rounded-full border border-amber-300/20 bg-amber-300/10 px-3 py-1.5 text-xs font-medium text-amber-100 opacity-0'
                : 'mb-5 inline-flex translate-y-6 items-center gap-2 rounded-full border border-amber-300/20 bg-amber-300/10 px-3 py-1.5 text-xs font-medium text-amber-100 opacity-0'
            }
            style={{ animationDelay: '80ms' }}
          >
            <span className='relative flex size-2'>
              <span className='absolute inline-flex h-full w-full animate-ping rounded-full bg-amber-300 opacity-70' />
              <span className='relative inline-flex size-2 rounded-full bg-amber-200' />
            </span>
            {t('YuCore AI Core is coming online')}
          </div>
          <h2
            className={
              revealHero
                ? 'landing-animate-fade-up max-w-4xl text-[clamp(2.7rem,7vw,6.1rem)] leading-[0.92] font-semibold tracking-tight opacity-0'
                : 'max-w-4xl translate-y-6 text-[clamp(2.7rem,7vw,6.1rem)] leading-[0.92] font-semibold tracking-tight opacity-0'
            }
            style={{ animationDelay: '160ms' }}
          >
            {t('YuCore AI Core')}
            <span className='block bg-[linear-gradient(100deg,#67e8f9_0%,#facc15_48%,#fb7185_100%)] bg-clip-text text-transparent'>
              {t('command system')}
            </span>
          </h2>
          <p
            className={
              revealHero
                ? 'landing-animate-fade-up mt-6 max-w-2xl text-base leading-8 text-white/62 opacity-0 sm:text-lg'
                : 'mt-6 max-w-2xl translate-y-6 text-base leading-8 text-white/62 opacity-0 sm:text-lg'
            }
            style={{ animationDelay: '260ms' }}
          >
            {t(
              'One account, one API, one operational core for models, Codex, billing, failover, image generation, video generation, and infinite canvas workflows.'
            )}
          </p>

          <div
            className={
              revealHero
                ? 'landing-animate-fade-up mt-8 flex flex-wrap gap-3 opacity-0'
                : 'mt-8 flex translate-y-6 flex-wrap gap-3 opacity-0'
            }
            style={{ animationDelay: '360ms' }}
          >
            {props.isAuthenticated ? (
              <Button
                className='h-11 rounded-xl bg-white px-5 text-sm font-semibold text-black hover:bg-cyan-50'
                render={<Link to='/dashboard' />}
              >
                {t('Enter console')}
                <ArrowRight data-icon='inline-end' />
              </Button>
            ) : (
              <Button
                className='h-11 rounded-xl bg-white px-5 text-sm font-semibold text-black hover:bg-cyan-50'
                render={<Link to='/sign-up' />}
              >
                {t('Start with YuCore')}
                <ArrowRight data-icon='inline-end' />
              </Button>
            )}
            <Button
              variant='outline'
              className='h-11 rounded-xl border-white/15 bg-white/[0.035] px-5 text-sm text-white hover:bg-white/10'
              render={<Link to='/pricing' />}
            >
              {t('Explore model hub')}
            </Button>
          </div>

          <div
            className={
              revealHero
                ? 'landing-animate-fade-up mt-8 flex flex-wrap gap-2 opacity-0'
                : 'mt-8 flex translate-y-6 flex-wrap gap-2 opacity-0'
            }
            style={{ animationDelay: '440ms' }}
          >
            {[
              'OpenAI-compatible gateway',
              'Codex-ready routing',
              'Image and video studio',
              'Production billing brain',
            ].map((item) => (
              <span
                key={item}
                className='inline-flex items-center gap-1.5 rounded-full border border-white/10 bg-white/[0.035] px-3 py-1.5 text-xs text-white/62'
              >
                <CheckCircle2 className='size-3.5 text-emerald-300' />
                {t(item)}
              </span>
            ))}
          </div>
        </div>

        <YucoreScrollReveal delay={120}>
          <YucoreCommandCenter />
        </YucoreScrollReveal>

        <div className='pointer-events-none absolute inset-x-5 bottom-5 hidden items-center justify-center text-white/38 min-[920px]:flex'>
          <span className='yucore-scroll-cue h-12 w-px bg-gradient-to-b from-transparent via-white/50 to-transparent' />
        </div>
      </section>

      {detailStage >= 1 && (
        <Suspense fallback={null}>
          <LazyYucoreHomeDetailsPrimary onCommitted={showSecondaryDetails} />
        </Suspense>
      )}
      {detailStage >= 2 && (
        <Suspense fallback={null}>
          <LazyYucoreHomeDetailsSecondary
            isAuthenticated={props.isAuthenticated}
          />
        </Suspense>
      )}
    </main>
  )
}
