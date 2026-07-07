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
import { AnimatedOutlet } from '@/components/page-transition'
import { SkipToMain } from '@/components/skip-to-main'
import { SidebarInset, SidebarProvider } from '@/components/ui/sidebar'
import { LayoutProvider } from '@/context/layout-provider'
import {
  YUCORE_BRAND_NAME,
  YucoreBackground,
  YucorePersistentCore,
} from '@/features/yucore-brand'
import { SearchProvider } from '@/context/search-provider'
import { getCookie } from '@/lib/cookies'
import { cn } from '@/lib/utils'
import { useRouterState } from '@tanstack/react-router'
import { useEffect } from 'react'

import { AppHeader } from './app-header'
import { AppSidebar } from './app-sidebar'

type AuthenticatedLayoutProps = {
  children?: React.ReactNode
}

export function AuthenticatedLayout(props: AuthenticatedLayoutProps) {
  const defaultOpen = getCookie('sidebar_state') !== 'false'
  const pathname = useRouterState({
    select: (state) => state.location.pathname,
  })
  const isYucoreStudioSurface =
    pathname === '/playground/studio' || pathname === '/playground/canvas'

  useEffect(() => {
    document.title = YUCORE_BRAND_NAME
  }, [])

  if (isYucoreStudioSurface) {
    return (
      <LayoutProvider>
        <SearchProvider>
          <div className='dark relative h-svh overflow-hidden bg-[#05070c]'>
            <YucoreBackground
              coreMode='ambient'
              intensity='workbench'
              showEarthCore={false}
              className='fixed yucore-authenticated-background yucore-authenticated-background-studio opacity-100'
            />
            <YucorePersistentCore
              active
              className='yucore-persistent-core-workbench'
            />
            <div
              className='pointer-events-none fixed inset-0 z-0 bg-[radial-gradient(circle_at_18%_0%,rgba(103,232,249,0.08),transparent_28%),linear-gradient(180deg,rgba(0,0,0,0.04),rgba(0,0,0,0.2))]'
              aria-hidden='true'
            />
            <SkipToMain />
            <main id='main-content' className='relative z-10 h-full min-h-0'>
              {props.children ?? <AnimatedOutlet />}
            </main>
          </div>
        </SearchProvider>
      </LayoutProvider>
    )
  }

  return (
    <LayoutProvider>
      <SearchProvider>
        <SidebarProvider
          defaultOpen={defaultOpen}
          className='dark relative flex-col overflow-hidden bg-[#05070c]'
        >
          <YucoreBackground
            coreMode='ambient'
            intensity='workbench'
            showEarthCore={false}
            className='fixed yucore-authenticated-background yucore-authenticated-background-console opacity-100'
          />
          <YucorePersistentCore
            active
            className='yucore-persistent-core-console'
          />
          <div
            className='pointer-events-none fixed inset-0 z-0 bg-[radial-gradient(circle_at_18%_0%,rgba(103,232,249,0.08),transparent_28%),linear-gradient(180deg,rgba(0,0,0,0.04),rgba(0,0,0,0.24))]'
            aria-hidden='true'
          />
          <SkipToMain />
          <AppHeader />
          <div className='flex min-h-0 w-full flex-1'>
            <AppSidebar />
            <SidebarInset
              className={cn(
                '@container/content',
                'h-[calc(100svh-var(--app-header-height,0px))]',
                'min-h-0 overflow-hidden !bg-transparent',
                'peer-data-[variant=inset]:h-[calc(100svh-var(--app-header-height,0px)-(var(--spacing)*4))]'
              )}
            >
              {props.children ?? <AnimatedOutlet />}
            </SidebarInset>
          </div>
        </SidebarProvider>
      </SearchProvider>
    </LayoutProvider>
  )
}
