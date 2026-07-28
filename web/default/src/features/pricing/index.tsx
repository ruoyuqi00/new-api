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
import { useCallback, useMemo, useState } from 'react'

import { PublicLayout } from '@/components/layout'
import { PageTransition } from '@/components/page-transition'
import {
  YUCORE_BRAND_NAME,
  YucoreBrandMark,
  YucoreModelHubRadar,
  YucorePageShell,
  yucoreModelHubSignals,
} from '@/features/yucore-brand'
import { useYucoreTranslation } from '@/features/yucore-brand/i18n/use-yucore-translation'

import {
  LoadingSkeleton,
  EmptyState,
  SearchBar,
  PricingTable,
  PricingSidebar,
  PricingToolbar,
  ModelCardGrid,
  ModelDetailsDrawer,
} from './components'
import { EXCLUDED_GROUPS, VIEW_MODES } from './constants'
import { useFilters } from './hooks/use-filters'
import { usePricingData } from './hooks/use-pricing-data'

export function Pricing() {
  const { t } = useYucoreTranslation()
  const [selectedModelName, setSelectedModelName] = useState<string | null>(
    null
  )

  const {
    models,
    vendors,
    groupRatio,
    usableGroup,
    endpointMap,
    autoGroups,
    isLoading,
    priceRate,
    usdExchangeRate,
  } = usePricingData()

  const {
    searchInput,
    sortBy,
    vendorFilter,
    groupFilter,
    quotaTypeFilter,
    endpointTypeFilter,
    tagFilter,
    tokenUnit,
    viewMode,
    showRechargePrice,
    setSearchInput,
    setSortBy,
    setVendorFilter,
    setGroupFilter,
    setQuotaTypeFilter,
    setEndpointTypeFilter,
    setTagFilter,
    setTokenUnit,
    setViewMode,
    setShowRechargePrice,
    filteredModels,
    hasActiveFilters,
    activeFilterCount,
    availableTags,
    clearFilters,
    clearSearch,
  } = useFilters(models || [])

  const handleModelClick = useCallback((modelName: string) => {
    setSelectedModelName(modelName)
  }, [])

  const selectedModel = useMemo(
    () =>
      selectedModelName
        ? (models || []).find(
            (model) => model.model_name === selectedModelName
          ) || null
        : null,
    [models, selectedModelName]
  )

  const availableGroups = useMemo(
    () =>
      Object.keys(usableGroup || {}).filter(
        (g) => !EXCLUDED_GROUPS.includes(g)
      ),
    [usableGroup]
  )

  const handleClearAll = useCallback(() => {
    clearFilters()
    clearSearch()
  }, [clearFilters, clearSearch])

  const renderPricingContent = () => {
    if (filteredModels.length === 0) {
      return (
        <EmptyState
          searchQuery={searchInput}
          hasActiveFilters={hasActiveFilters}
          onClearFilters={handleClearAll}
        />
      )
    }

    if (viewMode === VIEW_MODES.CARD) {
      return (
        <ModelCardGrid
          models={filteredModels}
          onModelClick={handleModelClick}
          priceRate={priceRate}
          usdExchangeRate={usdExchangeRate}
          tokenUnit={tokenUnit}
          showRechargePrice={showRechargePrice}
        />
      )
    }

    return (
      <PricingTable
        models={filteredModels}
        priceRate={priceRate}
        usdExchangeRate={usdExchangeRate}
        tokenUnit={tokenUnit}
        showRechargePrice={showRechargePrice}
        onModelClick={handleModelClick}
      />
    )
  }

  if (isLoading) {
    return (
      <PublicLayout
        showMainContainer={false}
        logo={<YucoreBrandMark compact />}
        siteName={YUCORE_BRAND_NAME}
        headerProps={{
          className:
            'text-foreground [&_nav]:border [&_nav]:border-border/60 [&_nav]:bg-background/70 [&_nav]:backdrop-blur-2xl [&_nav_a]:text-muted-foreground [&_nav_a:hover]:text-foreground',
        }}
      >
        <YucorePageShell
          intensity='calm'
          persistentCoreClassName='yucore-persistent-core-public'
          showPersistentCore
        >
          <LoadingSkeleton viewMode={viewMode} />
        </YucorePageShell>
      </PublicLayout>
    )
  }

  return (
    <PublicLayout
      showMainContainer={false}
      logo={<YucoreBrandMark compact />}
      siteName={YUCORE_BRAND_NAME}
      headerProps={{
        className:
          'text-foreground [&_nav]:border [&_nav]:border-border/60 [&_nav]:bg-background/70 [&_nav]:backdrop-blur-2xl [&_nav_a]:text-muted-foreground [&_nav_a:hover]:text-foreground',
      }}
    >
      <YucorePageShell
        intensity='workbench'
        persistentCoreClassName='yucore-persistent-core-public'
        showPersistentCore
      >
        <PageTransition className='relative'>
          <header className='mx-auto mb-6 grid max-w-7xl gap-6 lg:grid-cols-[minmax(0,1fr)_25rem] lg:items-end'>
            <div className='min-w-0'>
              <div className='mb-6'>
                <YucoreBrandMark />
              </div>
              <div className='mb-3 inline-flex rounded-full border border-cyan-300/20 bg-cyan-300/10 px-3 py-1 text-xs font-medium text-cyan-100'>
                {t('YuCore Model Hub')}
              </div>
              <h1 className='max-w-4xl text-[clamp(2.4rem,6vw,5.8rem)] leading-[0.92] font-semibold tracking-tight text-white'>
                {t('Model command center')}
              </h1>
              <p className='mt-5 max-w-2xl text-sm leading-7 text-white/58 sm:text-base'>
                {t(
                  'Discover curated AI models, compare pricing and capabilities, and choose the right model for every scenario.'
                )}
              </p>

              <div className='mt-6 grid gap-2 sm:grid-cols-3'>
                {yucoreModelHubSignals.map((signal) => {
                  const Icon = signal.icon

                  return (
                    <div
                      key={signal.label}
                      className='yucore-panel yucore-sweep rounded-2xl px-4 py-3 text-left'
                    >
                      <div className='flex items-center gap-2 text-xs text-white/45'>
                        <Icon
                          className='size-3.5 text-cyan-100'
                          aria-hidden='true'
                        />
                        {t(signal.label)}
                      </div>
                      <div className='mt-1 text-sm font-semibold text-white'>
                        {t(signal.value)}
                      </div>
                    </div>
                  )
                })}
              </div>

              <div className='mt-5 flex flex-col gap-3 sm:flex-row sm:items-center'>
                <SearchBar
                  value={searchInput}
                  onChange={setSearchInput}
                  onClear={clearSearch}
                  placeholder={t(
                    'Search model name, provider, endpoint, or tag...'
                  )}
                  className='w-full max-w-2xl'
                />
                <p className='text-xs text-white/45'>
                  {t('{{count}} enabled models', {
                    count: models?.length || 0,
                  })}
                </p>
              </div>
            </div>

            <YucoreModelHubRadar
              className='lg:mb-1'
              modelCount={models?.length || 0}
              vendorCount={vendors?.length || 0}
              groupCount={availableGroups.length}
            />
          </header>

          <div className='grid gap-4 xl:grid-cols-[330px_minmax(0,1fr)]'>
            <PricingSidebar
              quotaTypeFilter={quotaTypeFilter}
              endpointTypeFilter={endpointTypeFilter}
              vendorFilter={vendorFilter}
              groupFilter={groupFilter}
              tagFilter={tagFilter}
              onQuotaTypeChange={setQuotaTypeFilter}
              onEndpointTypeChange={setEndpointTypeFilter}
              onVendorChange={setVendorFilter}
              onGroupChange={setGroupFilter}
              onTagChange={setTagFilter}
              vendors={vendors || []}
              groups={availableGroups}
              groupRatios={groupRatio}
              tags={availableTags}
              models={models || []}
              hasActiveFilters={hasActiveFilters}
              onClearFilters={clearFilters}
              className='hover-scrollbar sticky top-4 hidden max-h-[calc(100dvh-2rem)] self-start overflow-y-auto xl:block'
            />

            <main className='min-w-0 space-y-4'>
              <PricingToolbar
                filteredCount={filteredModels.length}
                totalCount={models?.length}
                sortBy={sortBy}
                onSortChange={setSortBy}
                tokenUnit={tokenUnit}
                onTokenUnitChange={setTokenUnit}
                showRechargePrice={showRechargePrice}
                onRechargePriceChange={setShowRechargePrice}
                viewMode={viewMode}
                onViewModeChange={setViewMode}
                quotaTypeFilter={quotaTypeFilter}
                endpointTypeFilter={endpointTypeFilter}
                vendorFilter={vendorFilter}
                groupFilter={groupFilter}
                tagFilter={tagFilter}
                onQuotaTypeChange={setQuotaTypeFilter}
                onEndpointTypeChange={setEndpointTypeFilter}
                onVendorChange={setVendorFilter}
                onGroupChange={setGroupFilter}
                onTagChange={setTagFilter}
                vendors={vendors || []}
                groups={availableGroups}
                groupRatios={groupRatio}
                tags={availableTags}
                models={models || []}
                hasActiveFilters={hasActiveFilters}
                activeFilterCount={activeFilterCount}
                onClearFilters={clearFilters}
              />

              {renderPricingContent()}
            </main>
          </div>

          {selectedModel && (
            <ModelDetailsDrawer
              open={Boolean(selectedModel)}
              onOpenChange={(open) => {
                if (!open) setSelectedModelName(null)
              }}
              model={selectedModel}
              groupRatio={groupRatio || {}}
              usableGroup={usableGroup || {}}
              endpointMap={
                (endpointMap as Record<
                  string,
                  { path?: string; method?: string }
                >) || {}
              }
              autoGroups={autoGroups || []}
              priceRate={priceRate ?? 1}
              usdExchangeRate={usdExchangeRate ?? 1}
              tokenUnit={tokenUnit}
              showRechargePrice={showRechargePrice}
            />
          )}
        </PageTransition>
      </YucorePageShell>
    </PublicLayout>
  )
}
