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
import { useEffect } from 'react'
import { useTranslation } from 'react-i18next'

import { PublicLayout } from '@/components/layout'
import { RichContent } from '@/components/rich-content'
import {
  YUCORE_BRAND_NAME,
  YucoreBackground,
  YucoreBrandMark,
  YucoreHome,
  YucorePersistentCore,
} from '@/features/yucore-brand'
import { isLikelyHtml } from '@/lib/content-format'
import { useAuthStore } from '@/stores/auth-store'

import { useHomePageContent } from './hooks'

function CustomHomeSurface(props: { children: React.ReactNode }) {
  return (
    <div className='yucore-app-shell bg-background text-foreground relative min-h-svh overflow-hidden'>
      <YucoreBackground
        coreMode='ambient'
        corePlacement='hero'
        intensity='hero'
        showEarthCore={false}
        className='yucore-home-custom-background fixed'
      />
      <YucorePersistentCore active className='yucore-persistent-core-home' />
      <div className='relative z-10 min-h-svh'>{props.children}</div>
    </div>
  )
}

export function Home() {
  const { t } = useTranslation()
  const { auth } = useAuthStore()
  const isAuthenticated = !!auth.user
  const { content, isLoaded, isUrl } = useHomePageContent()

  useEffect(() => {
    const previousTitle = document.title
    document.title = YUCORE_BRAND_NAME

    return () => {
      document.title = previousTitle
    }
  }, [])

  if (!isLoaded && !content) {
    return (
      <PublicLayout
        showMainContainer={false}
        logo={<YucoreBrandMark compact />}
        siteName={YUCORE_BRAND_NAME}
        showNotifications={false}
        headerProps={{
          className:
            'yucore-public-header-home text-foreground opacity-0 transition-opacity duration-700 data-[scrolled=true]:opacity-100 data-[scrolled=false]:pointer-events-none [&_nav]:border [&_nav]:border-border/60 [&_nav]:bg-background/70 [&_nav]:backdrop-blur-2xl [&_nav_a]:text-muted-foreground [&_nav_a:hover]:text-foreground',
        }}
      >
        <YucoreHome isAuthenticated={isAuthenticated} />
      </PublicLayout>
    )
  }

  if (content) {
    if (isUrl) {
      return (
        <PublicLayout showMainContainer={false}>
          <CustomHomeSurface>
            <iframe
              src={content}
              className='relative z-10 h-screen w-full border-none bg-transparent'
              title={t('Custom Home Page')}
              sandbox='allow-forms allow-popups allow-popups-to-escape-sandbox allow-scripts'
            />
          </CustomHomeSurface>
        </PublicLayout>
      )
    }

    const contentIsHtml = isLikelyHtml(content)

    if (contentIsHtml) {
      return (
        <PublicLayout showMainContainer={false}>
          <CustomHomeSurface>
            <RichContent
              mode='html'
              htmlVariant='isolated'
              content={content}
              className='custom-home-content'
            />
          </CustomHomeSurface>
        </PublicLayout>
      )
    }

    return (
      <PublicLayout showMainContainer={false}>
        <CustomHomeSurface>
          <div className='mx-auto max-w-6xl px-4 py-24 sm:px-6'>
            <RichContent
              mode='markdown'
              content={content}
              className='custom-home-content'
            />
          </div>
        </CustomHomeSurface>
      </PublicLayout>
    )
  }

  return (
    <PublicLayout
      showMainContainer={false}
      logo={<YucoreBrandMark compact />}
      siteName={YUCORE_BRAND_NAME}
      showNotifications={false}
      headerProps={{
        className:
          'yucore-public-header-home text-foreground opacity-0 transition-opacity duration-700 data-[scrolled=true]:opacity-100 data-[scrolled=false]:pointer-events-none [&_nav]:border [&_nav]:border-border/60 [&_nav]:bg-background/70 [&_nav]:backdrop-blur-2xl [&_nav_a]:text-muted-foreground [&_nav_a:hover]:text-foreground',
      }}
    >
      <YucoreHome isAuthenticated={isAuthenticated} />
    </PublicLayout>
  )
}
