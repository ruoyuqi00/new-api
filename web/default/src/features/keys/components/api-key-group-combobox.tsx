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
import {
  ChevronsUpDown,
  Image as ImageIcon,
  Network,
  Video,
} from 'lucide-react'
import { useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from '@/components/ui/command'
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from '@/components/ui/popover'
import { cn } from '@/lib/utils'

import {
  filterApiKeyGroupOptions,
  getApiKeyGroupProtocolNode,
  getProtocolRouteLabel,
  type ApiKeyGroupProtocol,
  type ApiKeyGroupProtocolFilter,
} from '../lib/api-key-group-protocols'

export type ApiKeyGroupOption = {
  value: string
  label: string
  desc?: string
  ratio?: number | string
  protocols?: ApiKeyGroupProtocol[]
  endpointPaths?: string[]
}

const protocolFilters: Array<{
  value: ApiKeyGroupProtocolFilter
  label: string
  icon?: ReactNode
}> = [
  { value: 'all', label: 'All', icon: <Network aria-hidden='true' /> },
  { value: 'openai', label: 'OpenAI' },
  { value: 'claude', label: 'Claude' },
  { value: 'gemini', label: 'Gemini' },
  { value: 'image', label: 'Image', icon: <ImageIcon aria-hidden='true' /> },
  { value: 'video', label: 'Video', icon: <Video aria-hidden='true' /> },
]

type ApiKeyGroupComboboxProps = {
  options: ApiKeyGroupOption[]
  value?: string
  onValueChange: (value: string) => void
  placeholder?: string
  disabled?: boolean
}

function formatGroupRatio(
  ratio: ApiKeyGroupOption['ratio'],
  ratioLabel: string
) {
  if (ratio === undefined || ratio === null || ratio === '') return null
  return `${ratio}x ${ratioLabel}`
}

function GroupRatioBadge(props: {
  ratio: ApiKeyGroupOption['ratio']
  className?: string
}) {
  const { t } = useTranslation()
  const label = formatGroupRatio(props.ratio, t('Ratio'))

  if (!label) return null

  return (
    <Badge
      variant='outline'
      className={cn(
        'h-5 max-w-28 rounded-md border-emerald-600/35 bg-emerald-500/10 px-2 font-mono text-[10px] font-bold text-emerald-700 dark:border-emerald-400/35 dark:text-emerald-300',
        props.className
      )}
    >
      <span className='truncate'>{label}</span>
    </Badge>
  )
}

function ProtocolNode(props: {
  option?: ApiKeyGroupOption
  selected?: boolean
}) {
  const node = props.option ? getApiKeyGroupProtocolNode(props.option) : 'RT'
  const hasCapabilities = (props.option?.protocols?.length ?? 0) > 0

  return (
    <span
      data-protocol-node
      className={cn(
        'relative grid size-8 shrink-0 place-items-center rounded-md border border-cyan-700/25 bg-cyan-500/8 font-mono text-[10px] font-bold text-cyan-700 dark:border-cyan-300/25 dark:bg-cyan-400/8 dark:text-cyan-200',
        props.selected &&
          'border-cyan-600/50 bg-cyan-500/12 dark:border-cyan-300/50 dark:bg-cyan-400/12'
      )}
    >
      {node}
      <span
        className={cn(
          'border-popover bg-muted-foreground absolute -right-1 -bottom-1 size-2 rounded-full border-2',
          hasCapabilities && 'bg-emerald-400',
          hasCapabilities && props.selected && 'shadow-[0_0_7px_currentColor]'
        )}
        aria-hidden='true'
      />
    </span>
  )
}

function MultiProtocolBadge() {
  const { t } = useTranslation()

  return (
    <Badge
      variant='outline'
      className='h-5 shrink-0 gap-1 rounded-sm border-violet-600/35 bg-violet-500/10 px-1.5 text-[9px] font-bold text-violet-700 dark:border-violet-400/35 dark:text-violet-300'
    >
      <span
        className='border-primary w-2 border-y border-t-violet-300'
        aria-hidden='true'
      />
      {t('Multi-protocol')}
    </Badge>
  )
}

export function ApiKeyGroupCombobox(props: ApiKeyGroupComboboxProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [searchValue, setSearchValue] = useState('')
  const [protocolFilter, setProtocolFilter] =
    useState<ApiKeyGroupProtocolFilter>('all')
  const selectedOption = props.options.find(
    (option) => option.value === props.value
  )
  const filteredOptions = filterApiKeyGroupOptions(
    props.options,
    protocolFilter,
    searchValue
  )

  const handleSelect = (selectedValue: string) => {
    props.onValueChange(selectedValue)
    setOpen(false)
    setSearchValue('')
  }

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger
        render={
          <Button
            type='button'
            variant='outline'
            role='combobox'
            aria-expanded={open}
            disabled={props.disabled}
            className='bg-muted/30 hover:bg-muted/45 active:bg-background data-popup-open:bg-background h-auto min-h-14 w-full justify-between gap-2 rounded-lg border-cyan-300/20 px-3 py-2 text-start shadow-[0_0_0_3px_rgb(103_232_249_/_5%)] transition-[background-color,border-color,box-shadow] duration-150 hover:border-cyan-300/35 data-popup-open:border-cyan-300/40 data-popup-open:ring-[3px] data-popup-open:ring-cyan-300/10 sm:min-h-16'
          />
        }
      >
        <span className='flex min-w-0 flex-1 items-center gap-2.5'>
          <ProtocolNode option={selectedOption} selected={open} />
          <span className='min-w-0 flex-1'>
            <span className='block truncate font-semibold'>
              {selectedOption?.label ||
                props.placeholder ||
                t('Select a group')}
            </span>
            <span className='text-muted-foreground block truncate text-[11px]'>
              {selectedOption?.desc || t('Filter available routes by protocol')}
            </span>
          </span>
          <span className='hidden sm:block'>
            <GroupRatioBadge ratio={selectedOption?.ratio} />
          </span>
        </span>
        <ChevronsUpDown className='size-4 shrink-0 opacity-50' />
      </PopoverTrigger>

      <PopoverContent
        data-route-picker
        className='bg-popover data-closed:zoom-out-100 data-open:zoom-in-100 data-[side=bottom]:slide-in-from-top-0 data-[side=left]:slide-in-from-right-0 data-[side=right]:slide-in-from-left-0 data-[side=top]:slide-in-from-bottom-0 w-[var(--anchor-width)] min-w-[min(36rem,calc(100vw-1.5rem))] overflow-hidden rounded-lg border-cyan-700/25 p-0 shadow-2xl data-closed:duration-75 data-open:duration-100 dark:border-cyan-300/25 dark:bg-[#0e141a]'
        onWheel={(event) => event.stopPropagation()}
        onTouchMove={(event) => event.stopPropagation()}
        onPointerDown={(event) => event.stopPropagation()}
      >
        <div
          className='bg-background grid h-0.5 grid-cols-[2.3fr_0.65fr_0.16fr_0.8fr] gap-1'
          aria-hidden='true'
        >
          <span className='bg-cyan-300' />
          <span className='bg-amber-400/80' />
          <span className='bg-violet-400/80' />
          <span className='bg-emerald-400/60' />
        </div>

        <Command
          className='rounded-none bg-transparent p-0'
          shouldFilter={false}
        >
          <div className='border-border/70 bg-muted/35 border-b p-2.5 dark:bg-[#11171d]'>
            <div className='text-muted-foreground mb-2 flex items-center justify-between font-mono text-[9px] font-bold uppercase'>
              <span>PROTOCOL ROUTE MATRIX</span>
              <span className='flex items-center gap-1.5 text-emerald-700 dark:text-emerald-400'>
                <span className='size-1.5 rounded-full bg-current shadow-[0_0_7px_currentColor]' />
                CATALOG ONLINE
              </span>
            </div>
            <CommandInput
              className='font-normal'
              placeholder={t('Search groups, descriptions, or API endpoints')}
              value={searchValue}
              onValueChange={setSearchValue}
            />
            <div
              className='mt-2 flex [scrollbar-width:none] gap-1 overflow-x-auto [&::-webkit-scrollbar]:hidden'
              role='group'
              aria-label={t('Protocol routing')}
            >
              {protocolFilters.map((filter) => {
                const active = protocolFilter === filter.value
                return (
                  <button
                    key={filter.value}
                    type='button'
                    aria-pressed={active}
                    onClick={() => setProtocolFilter(filter.value)}
                    className={cn(
                      'border-border/70 bg-muted/25 text-muted-foreground inline-flex h-7 shrink-0 items-center gap-1.5 rounded-md border px-2.5 text-[11px] font-medium transition-colors hover:border-cyan-700/25 hover:text-foreground dark:hover:border-cyan-300/25 [&_svg]:size-3',
                      active &&
                        'border-cyan-700/45 bg-cyan-500/10 text-cyan-800 shadow-[inset_0_-1px_0_rgb(14_116_144_/_65%)] dark:border-cyan-300/45 dark:bg-cyan-400/10 dark:text-cyan-200 dark:shadow-[inset_0_-1px_0_rgb(103_232_249_/_65%)]'
                    )}
                  >
                    {filter.icon ?? (
                      <span
                        className={cn(
                          'size-1.5 rounded-full bg-current opacity-45',
                          active && 'opacity-100 shadow-[0_0_7px_currentColor]'
                        )}
                        aria-hidden='true'
                      />
                    )}
                    {t(filter.label)}
                  </button>
                )
              })}
            </div>
          </div>

          <div className='text-muted-foreground flex items-center justify-between px-3 py-2 font-mono text-[9px] uppercase'>
            <span className='text-foreground/70'>
              {t(getProtocolRouteLabel(protocolFilter))}
            </span>
            <span>
              {t('{{count}} available groups', {
                count: filteredOptions.length,
              })}
            </span>
          </div>

          <CommandList className='max-h-[min(18.5rem,34vh)] scroll-py-1 [scrollbar-width:thin] [scrollbar-color:color-mix(in_srgb,var(--muted-foreground)_45%,transparent)_transparent] overflow-y-auto px-1.5 pb-2 dark:[scrollbar-color:rgb(55_83_91)_rgb(16_23_28)]'>
            <CommandEmpty>{t('No group found.')}</CommandEmpty>
            <CommandGroup className='p-0'>
              {filteredOptions.map((option) => {
                const selected = props.value === option.value
                const endpoints = option.endpointPaths?.slice(0, 2) ?? []
                const remainingEndpointCount = Math.max(
                  0,
                  (option.endpointPaths?.length ?? 0) - endpoints.length
                )

                return (
                  <CommandItem
                    key={option.value}
                    value={option.value}
                    data-checked={selected}
                    onSelect={() => handleSelect(option.value)}
                    className={cn(
                      'group/item relative min-h-18 items-start gap-2.5 overflow-hidden rounded-md border border-transparent px-2.5 py-2.5 transition-colors before:absolute before:inset-y-2 before:left-0 before:w-0.5 before:rounded-full before:bg-transparent',
                      'data-[selected=true]:border-border/80 data-[selected=true]:bg-muted/45',
                      selected &&
                        'border-cyan-700/35 bg-cyan-500/8 before:bg-cyan-600 shadow-[inset_0_0_24px_rgb(14_116_144_/_5%)] dark:border-cyan-300/35 dark:bg-cyan-400/8 dark:before:bg-cyan-300 dark:shadow-[inset_0_0_24px_rgb(103_232_249_/_5%)]'
                    )}
                  >
                    <ProtocolNode option={option} selected={selected} />
                    <span className='min-w-0 flex-1'>
                      <span className='flex min-w-0 items-center gap-1.5'>
                        <span className='truncate text-sm font-semibold'>
                          {option.label}
                        </span>
                        {(option.protocols?.length ?? 0) > 1 && (
                          <MultiProtocolBadge />
                        )}
                      </span>
                      {option.desc && (
                        <span className='text-muted-foreground mt-0.5 block truncate text-[11px]'>
                          {option.desc}
                        </span>
                      )}
                      {endpoints.length > 0 && (
                        <span className='mt-1.5 flex min-w-0 flex-wrap gap-1'>
                          {endpoints.map((endpoint) => (
                            <span
                              key={endpoint}
                              data-route-endpoint
                              className='border-border/50 bg-background/55 text-muted-foreground max-w-52 truncate rounded-sm border px-1.5 py-0.5 font-mono text-[9px]'
                            >
                              <span className='text-cyan-700/70 dark:text-cyan-300/70'>
                                &gt;
                              </span>{' '}
                              {endpoint}
                            </span>
                          ))}
                          {remainingEndpointCount > 0 && (
                            <span className='text-muted-foreground px-1 py-0.5 font-mono text-[9px]'>
                              +{remainingEndpointCount}
                            </span>
                          )}
                        </span>
                      )}
                    </span>
                    <GroupRatioBadge
                      ratio={option.ratio}
                      className='self-center'
                    />
                  </CommandItem>
                )
              })}
            </CommandGroup>
          </CommandList>
        </Command>
      </PopoverContent>
    </Popover>
  )
}
