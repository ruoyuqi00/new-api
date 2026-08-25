import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'

import type { GroupCatalogItem } from '../types'

type GroupAvailabilitySettingsProps = {
  value: string
  catalog: GroupCatalogItem[]
  onChange: (value: string) => void
}

function parseMonitoring(value: string): Record<string, boolean> {
  try {
    const parsed = JSON.parse(value || '{}')
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return {}
    return Object.fromEntries(
      Object.entries(parsed).filter((entry): entry is [string, boolean] =>
        typeof entry[1] === 'boolean'
      )
    )
  } catch {
    return {}
  }
}

export function GroupAvailabilitySettings(props: GroupAvailabilitySettingsProps) {
  const { t } = useTranslation()
  const values = useMemo(() => parseMonitoring(props.value), [props.value])
  const groups = useMemo(() => {
    const names = new Set(props.catalog.map((item) => item.name))
    Object.keys(values).forEach((name) => names.add(name))
    return [...names].sort()
  }, [props.catalog, values])

  return (
    <Card size='sm'>
      <CardHeader>
        <CardTitle>{t('Group availability monitoring')}</CardTitle>
        <CardDescription>
          {t('Show request success availability only; latency and upstream details are never exposed.')}
        </CardDescription>
      </CardHeader>
      <CardContent className='space-y-2'>
        {groups.length === 0 ? (
          <p className='text-muted-foreground text-sm'>{t('No groups configured.')}</p>
        ) : (
          groups.map((group) => (
            <div key={group} className='flex items-center justify-between gap-3 border-b py-2 last:border-b-0'>
              <Label htmlFor={`group-monitoring-${group}`} className='min-w-0 truncate'>
                {group}
              </Label>
              <Switch
                id={`group-monitoring-${group}`}
                checked={values[group] === true}
                onCheckedChange={(checked) =>
                  props.onChange(
                    JSON.stringify({ ...values, [group]: checked }, null, 2)
                  )
                }
              />
            </div>
          ))
        )}
      </CardContent>
    </Card>
  )
}
