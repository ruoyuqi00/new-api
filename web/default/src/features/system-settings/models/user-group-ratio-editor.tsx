import { Plus, Trash2 } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'

import type { GroupCatalogItem } from '../types'
import {
  flattenUserGroupRatios,
  serializeUserGroupRatios,
  type UserGroupRatioRow,
} from './user-group-ratio-utils'

type UserGroupRatioEditorProps = {
  value: string
  catalog: GroupCatalogItem[]
  onChange: (value: string) => void
}

export function UserGroupRatioEditor(props: UserGroupRatioEditorProps) {
  const { t } = useTranslation()
  const rows = useMemo(() => flattenUserGroupRatios(props.value), [props.value])
  const [userId, setUserId] = useState('')
  const [group, setGroup] = useState('')
  const [ratio, setRatio] = useState('')

  const updateRows = (nextRows: UserGroupRatioRow[]) => {
    props.onChange(serializeUserGroupRatios(nextRows))
  }

  const handleAdd = () => {
    const parsedRatio = Number.parseFloat(ratio)
    if (!/^\d+$/.test(userId.trim()) || !group || !Number.isFinite(parsedRatio) || parsedRatio < 0) {
      return
    }
    updateRows([
      ...rows.filter((row) => !(row.userId === userId.trim() && row.group === group)),
      { userId: userId.trim(), group, ratio: parsedRatio },
    ])
    setUserId('')
    setGroup('')
    setRatio('')
  }

  return (
    <Card size='sm'>
      <CardHeader>
        <CardTitle>{t('User-specific group ratios')}</CardTitle>
        <CardDescription>
          {t('Configure a ratio for one user without changing their group permissions.')}
        </CardDescription>
      </CardHeader>
      <CardContent className='space-y-4'>
        <div className='grid gap-3 sm:grid-cols-[1fr_1fr_0.7fr_auto] sm:items-end'>
          <div className='space-y-1.5'>
            <Label htmlFor='user-group-ratio-user-id'>{t('User ID')}</Label>
            <Input
              id='user-group-ratio-user-id'
              inputMode='numeric'
              value={userId}
              onChange={(event) => setUserId(event.target.value)}
              placeholder='81'
            />
          </div>
          <div className='space-y-1.5'>
            <Label>{t('Target group')}</Label>
            <Select value={group} onValueChange={(value) => setGroup(value ?? '')}>
              <SelectTrigger>
                <SelectValue placeholder={t('Select a group')} />
              </SelectTrigger>
              <SelectContent>
                {props.catalog.map((item) => (
                  <SelectItem key={item.name} value={item.name}>
                    {item.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
          <div className='space-y-1.5'>
            <Label htmlFor='user-group-ratio-value'>{t('Ratio')}</Label>
            <Input
              id='user-group-ratio-value'
              inputMode='decimal'
              value={ratio}
              onChange={(event) => setRatio(event.target.value)}
              placeholder='0.8'
            />
          </div>
          <Button type='button' variant='outline' onClick={handleAdd}>
            <Plus className='mr-2 size-4' />
            {t('Add')}
          </Button>
        </div>

        {rows.length > 0 && (
          <div className='divide-border overflow-hidden rounded-lg border'>
            {rows.map((row) => (
              <div
                key={`${row.userId}:${row.group}`}
                className='flex items-center justify-between gap-3 border-b px-3 py-2 last:border-b-0'
              >
                <div className='min-w-0 text-sm'>
                  <span className='font-medium'>#{row.userId}</span>
                  <span className='text-muted-foreground mx-2'>·</span>
                  <span>{row.group}</span>
                  <span className='text-muted-foreground mx-2'>·</span>
                  <span className='font-mono tabular-nums'>{row.ratio}x</span>
                </div>
                <Button
                  type='button'
                  variant='ghost'
                  size='icon'
                  aria-label={t('Remove')}
                  onClick={() =>
                    updateRows(
                      rows.filter(
                        (item) =>
                          item.userId !== row.userId || item.group !== row.group
                      )
                    )
                  }
                >
                  <Trash2 className='size-4' />
                </Button>
              </div>
            ))}
          </div>
        )}
      </CardContent>
    </Card>
  )
}
