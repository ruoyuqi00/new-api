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
import { type CSSProperties, useEffect, useState } from 'react'

import { Button } from '@/components/ui/button'

import { yucoreCapabilities } from '../data/content'
import { useYucoreTranslation } from '../i18n/use-yucore-translation'
import { YucoreBackground } from './yucore-background'
import { YucoreBrandMark } from './yucore-brand-mark'
import { YucoreCommandCenter } from './yucore-command-center'
import {
  YUCORE_BOOT_LOADER_DURATION_MS,
  YucoreEntranceLoader,
} from './yucore-entrance-loader'
import { YucoreMetricStrip } from './yucore-metric-strip'
import { YucorePersistentCore } from './yucore-persistent-core'
import { YucoreScrollReveal } from './yucore-scroll-reveal'
import { YucoreStudioEntry } from './yucore-studio-entry'
import { YucoreTerminalCard } from './yucore-terminal-card'

interface YucoreHomeProps {
  isAuthenticated?: boolean
}

const yucoreModelTags = [
  'ChatGPT',
  'gpt-5.5',
  'gpt-5.5-Fast',
  'gpt-5.4',
  'gpt-5.4-mini',
  'Codex',
  'gpt-image-2',
  'Video route',
  'Studio canvas',
]

const yucoreGatewayStats = [
  { value: '40+', label: 'provider routes' },
  { value: '24h', label: 'usage visibility' },
  { value: 'SSE', label: 'stream ready' },
  { value: 'API', label: 'OpenAI-compatible' },
]

const yucoreServiceAdvantages = [
  {
    eyebrow: '01',
    title: 'Latest model gateway',
    description:
      'Expose chat, code, image, video, and fast aliases from one model hub with route-aware pricing.',
    tags: ['ChatGPT', 'Codex', 'Fast', 'Image'],
  },
  {
    eyebrow: '02',
    title: 'Audit and privacy visibility',
    description:
      'Request logs, token usage, cache savings, wallet changes, and quota state stay visible for operators.',
    tags: ['Logs', 'Tokens', 'Cache', 'Wallet'],
  },
  {
    eyebrow: '03',
    title: 'Global access layer',
    description:
      'Pool routing, failover, endpoint mapping, and key scopes keep production calls stable across teams.',
    tags: ['Failover', 'Pools', 'Keys', 'Routes'],
  },
  {
    eyebrow: '04',
    title: 'Studio-ready workflows',
    description:
      'Move from API calls into image, video, canvas, prompt assets, and review states without leaving YuCore.',
    tags: ['Image', 'Video', 'Canvas', 'Assets'],
  },
]

const yucoreAccessFlow = [
  {
    step: '1',
    title: 'Choose model routes',
    detail:
      'Compare model pricing, tags, endpoint support, and routing hints before creating production traffic.',
  },
  {
    step: '2',
    title: 'Create keys and quota',
    detail:
      'Register once, connect wallet and API keys, then keep request limits and spend visible.',
  },
  {
    step: '3',
    title: 'Automate daily work',
    detail:
      'Use Codex, model calls, generated media, and studio assets to turn repeated work into workflows.',
  },
]

const yucoreEnterpriseSignals = [
  'Team billing controls',
  'Private model policy',
  'Onboarding support',
  'Invoice-ready usage',
]

const yucoreGatewayPreviewLines = [
  "curl -X POST '/v1/responses' \\",
  "  -H 'Authorization: Bearer sk-yucore' \\",
  "  -d '{",
  '    "model": "gpt-5.5-Fast",',
  '    "input": "Build a Codex-ready workflow"',
  "  }'",
  '',
  'response: 200 ok | stream:sse | usage tracked',
]

const HOME_HERO_REVEAL_MS = YUCORE_BOOT_LOADER_DURATION_MS - 80
const HOME_WEBGL_PHASE_OFFSET_SECONDS =
  (YUCORE_BOOT_LOADER_DURATION_MS / 1000) * 0.55

export function YucoreHome(props: YucoreHomeProps) {
  const { t } = useYucoreTranslation()
  const [revealHero, setRevealHero] = useState(false)

  useEffect(() => {
    const revealTimer = window.setTimeout(
      () => setRevealHero(true),
      HOME_HERO_REVEAL_MS
    )

    return () => {
      window.clearTimeout(revealTimer)
    }
  }, [])

  return (
    <main
      className='yucore-home relative min-h-svh overflow-hidden bg-[#05070c] text-white'
      data-yucore-home-phase={revealHero ? 'stable' : 'boot'}
      style={
        {
          '--yucore-loader-duration': `${YUCORE_BOOT_LOADER_DURATION_MS}ms`,
        } as CSSProperties
      }
    >
      <YucoreEntranceLoader />
      <YucoreBackground
        active
        coreMode='ambient'
        intensity='hero'
        showEarthCore={false}
        className='yucore-home-fixed-background fixed'
      />
      <YucorePersistentCore
        active
        className='yucore-persistent-core-home yucore-persistent-core-home-bridge'
        webglActive={revealHero}
        webglTimeOffsetSeconds={HOME_WEBGL_PHASE_OFFSET_SECONDS}
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
                ? 'yucore-intro-word yucore-home-title landing-animate-fade-up text-[clamp(2.9rem,8vw,6.8rem)] leading-[0.9] font-semibold tracking-tight text-white opacity-0'
                : 'yucore-intro-word yucore-home-title translate-y-6 text-[clamp(2.9rem,8vw,6.8rem)] leading-[0.9] font-semibold tracking-tight text-white opacity-0'
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
              'Quota sync intelligence',
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

        <div
          className={
            revealHero
              ? 'landing-animate-fade-left opacity-0'
              : 'translate-y-6 opacity-0'
          }
          style={{ animationDelay: '540ms' }}
        >
          <YucoreCommandCenter />
        </div>

        <div className='pointer-events-none absolute inset-x-5 bottom-5 hidden items-center justify-center text-white/38 min-[920px]:flex'>
          <span className='yucore-scroll-cue h-12 w-px bg-gradient-to-b from-transparent via-white/50 to-transparent' />
        </div>
      </section>

      <section className='relative z-10 mx-auto w-full max-w-7xl px-5 pb-16 sm:px-8 lg:px-10'>
        <YucoreScrollReveal>
          <YucoreMetricStrip />
        </YucoreScrollReveal>

        <YucoreScrollReveal delay={70}>
          <section className='mt-12 grid gap-6 border-y border-white/10 py-8 min-[960px]:grid-cols-[0.9fr_1.1fr] min-[960px]:items-start'>
            <div>
              <div className='mb-3 text-xs font-semibold tracking-[0.24em] text-cyan-100/58 uppercase'>
                {t('Supported model routes')}
              </div>
              <h2 className='max-w-3xl text-3xl leading-tight font-semibold tracking-tight text-white sm:text-4xl'>
                {t(
                  'From model access to working API calls, YuCore keeps the path visible.'
                )}
              </h2>
              <p className='mt-5 max-w-2xl text-sm leading-7 text-white/56 sm:text-base'>
                {t(
                  'The landing experience should explain what the core actually does: route models, expose copy-ready API examples, track usage, and open creative workflows.'
                )}
              </p>

              <div className='mt-7 flex flex-wrap gap-2'>
                {yucoreModelTags.map((tag) => (
                  <span
                    key={tag}
                    className='rounded-full border border-white/10 bg-white/[0.035] px-3 py-1.5 text-xs font-medium text-white/64'
                  >
                    {t(tag)}
                  </span>
                ))}
              </div>

              <div className='mt-7 grid grid-cols-2 gap-3 sm:grid-cols-4'>
                {yucoreGatewayStats.map((stat) => (
                  <div
                    key={stat.label}
                    className='border-l border-cyan-200/20 pl-3'
                  >
                    <div className='text-2xl font-semibold tracking-tight text-white'>
                      {t(stat.value)}
                    </div>
                    <div className='mt-1 text-xs text-white/45'>
                      {t(stat.label)}
                    </div>
                  </div>
                ))}
              </div>
            </div>

            <YucoreTerminalCard
              className='min-[960px]:mt-3'
              title={t('OpenAI-compatible request')}
              lines={yucoreGatewayPreviewLines}
            />
          </section>
        </YucoreScrollReveal>

        <YucoreScrollReveal delay={80}>
          <div className='mt-12 grid gap-5 border-y border-white/10 py-8 min-[860px]:grid-cols-[0.8fr_1.2fr] min-[860px]:items-end'>
            <div>
              <div className='mb-3 text-xs font-semibold tracking-[0.24em] text-amber-100/58 uppercase'>
                {t('YuCore operations')}
              </div>
              <h2 className='max-w-2xl text-2xl leading-tight font-semibold tracking-tight text-white sm:text-3xl'>
                {t(
                  'Model gateway, media studio, billing, and failover share one command layer.'
                )}
              </h2>
            </div>
            <p className='max-w-2xl text-sm leading-7 text-white/55 min-[860px]:justify-self-end'>
              {t(
                'The UI should feel like an operating room for AI work: fast routing for developers, clean quota visibility for admins, and a media workspace for creators.'
              )}
            </p>
          </div>
        </YucoreScrollReveal>

        <div className='mt-6 grid gap-4 md:grid-cols-2 xl:grid-cols-4'>
          {yucoreCapabilities.map((capability, index) => {
            const Icon = capability.icon

            return (
              <YucoreScrollReveal
                key={capability.title}
                className='h-full'
                delay={120 + index * 70}
              >
                <article className='yucore-panel yucore-sweep min-h-full rounded-2xl p-5'>
                  <div className='mb-5 flex items-center justify-between gap-3'>
                    <span className='flex size-11 items-center justify-center rounded-2xl border border-cyan-300/20 bg-cyan-300/10 text-cyan-100'>
                      <Icon className='size-5' aria-hidden='true' />
                    </span>
                    <span className='rounded-full border border-white/10 bg-white/[0.035] px-2.5 py-1 text-[11px] text-white/45'>
                      {t(capability.signal)}
                    </span>
                  </div>
                  <h2 className='text-base font-semibold text-white'>
                    {t(capability.title)}
                  </h2>
                  <p className='mt-3 text-sm leading-7 text-white/52'>
                    {t(capability.description)}
                  </p>
                </article>
              </YucoreScrollReveal>
            )
          })}
        </div>

        <section className='mt-14 grid gap-4 lg:grid-cols-4'>
          {yucoreServiceAdvantages.map((item, index) => (
            <YucoreScrollReveal
              key={item.title}
              className='h-full'
              delay={120 + index * 70}
            >
              <article className='yucore-panel yucore-sweep flex min-h-full flex-col rounded-2xl p-5'>
                <div className='mb-6 flex items-center justify-between gap-3'>
                  <span className='text-xs font-semibold tracking-[0.24em] text-cyan-100/45'>
                    {item.eyebrow}
                  </span>
                  <span className='h-px flex-1 bg-gradient-to-r from-cyan-200/30 to-transparent' />
                </div>
                <h3 className='text-lg font-semibold text-white'>
                  {t(item.title)}
                </h3>
                <p className='mt-3 flex-1 text-sm leading-7 text-white/52'>
                  {t(item.description)}
                </p>
                <div className='mt-5 flex flex-wrap gap-2'>
                  {item.tags.map((tag) => (
                    <span
                      key={tag}
                      className='rounded-full border border-white/10 bg-black/25 px-2.5 py-1 text-[11px] text-white/48'
                    >
                      {t(tag)}
                    </span>
                  ))}
                </div>
              </article>
            </YucoreScrollReveal>
          ))}
        </section>

        <YucoreScrollReveal delay={160}>
          <section className='mt-14 grid gap-8 border-t border-white/10 pt-10 min-[920px]:grid-cols-[0.8fr_1.2fr]'>
            <div>
              <div className='mb-3 text-xs font-semibold tracking-[0.24em] text-amber-100/58 uppercase'>
                {t('Access flow')}
              </div>
              <h2 className='max-w-xl text-3xl leading-tight font-semibold tracking-tight text-white sm:text-4xl'>
                {t('Three steps from account creation to useful AI workflows.')}
              </h2>
              <div className='mt-7 flex flex-wrap gap-3'>
                <Button
                  className='h-11 rounded-xl bg-white px-5 text-sm font-semibold text-black hover:bg-cyan-50'
                  render={
                    <Link
                      to={props.isAuthenticated ? '/dashboard' : '/sign-up'}
                    />
                  }
                >
                  {props.isAuthenticated
                    ? t('Enter console')
                    : t('Create access')}
                  <ArrowRight data-icon='inline-end' />
                </Button>
                <Button
                  variant='outline'
                  className='h-11 rounded-xl border-white/15 bg-white/[0.035] px-5 text-sm text-white hover:bg-white/10'
                  render={<Link to='/pricing' />}
                >
                  {t('Compare models')}
                </Button>
              </div>
            </div>

            <div className='grid gap-3'>
              {yucoreAccessFlow.map((item) => (
                <div
                  key={item.step}
                  className='grid gap-3 border-b border-white/10 pb-5 last:border-b-0 min-[640px]:grid-cols-[4rem_minmax(0,1fr)]'
                >
                  <div className='flex size-10 items-center justify-center rounded-full border border-cyan-200/20 bg-cyan-200/10 text-sm font-semibold text-cyan-100'>
                    {item.step}
                  </div>
                  <div>
                    <h3 className='text-base font-semibold text-white'>
                      {t(item.title)}
                    </h3>
                    <p className='mt-2 text-sm leading-7 text-white/52'>
                      {t(item.detail)}
                    </p>
                  </div>
                </div>
              ))}
            </div>
          </section>
        </YucoreScrollReveal>

        <YucoreScrollReveal delay={180}>
          <YucoreStudioEntry className='mt-6' />
        </YucoreScrollReveal>

        <YucoreScrollReveal delay={220}>
          <section className='mt-14 overflow-hidden rounded-[1.75rem] border border-white/10 bg-black/35 p-6 backdrop-blur-2xl sm:p-8'>
            <div className='grid gap-8 min-[920px]:grid-cols-[1fr_0.9fr] min-[920px]:items-end'>
              <div>
                <div className='mb-3 text-xs font-semibold tracking-[0.24em] text-cyan-100/58 uppercase'>
                  {t('Enterprise service')}
                </div>
                <h2 className='max-w-3xl text-3xl leading-tight font-semibold tracking-tight text-white sm:text-4xl'>
                  {t(
                    'Bring YuCore into a team without losing cost and access control.'
                  )}
                </h2>
                <p className='mt-5 max-w-2xl text-sm leading-7 text-white/54'>
                  {t(
                    'Use one branded access layer for developers, creators, operators, and finance reviewers. Model routing, usage records, and studio output stay connected.'
                  )}
                </p>
              </div>

              <div className='grid gap-2'>
                {yucoreEnterpriseSignals.map((signal) => (
                  <div
                    key={signal}
                    className='flex items-center justify-between gap-3 border-b border-white/10 py-3 last:border-b-0'
                  >
                    <span className='text-sm font-medium text-white/72'>
                      {t(signal)}
                    </span>
                    <CheckCircle2
                      className='size-4 shrink-0 text-emerald-300'
                      aria-hidden='true'
                    />
                  </div>
                ))}
              </div>
            </div>

            <div className='mt-8 flex flex-wrap gap-3'>
              <Button
                className='h-11 rounded-xl bg-white px-5 text-sm font-semibold text-black hover:bg-cyan-50'
                render={
                  <Link
                    to={props.isAuthenticated ? '/dashboard' : '/sign-up'}
                  />
                }
              >
                {props.isAuthenticated ? t('Enter console') : t('Start access')}
                <ArrowRight data-icon='inline-end' />
              </Button>
              <Button
                variant='outline'
                className='h-11 rounded-xl border-white/15 bg-white/[0.035] px-5 text-sm text-white hover:bg-white/10'
                render={<Link to='/pricing' />}
              >
                {t('View model pricing')}
              </Button>
            </div>
          </section>
        </YucoreScrollReveal>
      </section>
    </main>
  )
}
