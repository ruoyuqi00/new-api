import { AlertCircle, CheckCircle2, FileUp, Upload, X } from 'lucide-react'
import { useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Textarea } from '@/components/ui/textarea'

import {
  type ImportedAccount,
  type ImportPreviewRow,
  parseAccountImport,
} from '../import-parser'

type AccountImportPanelProps = {
  provider: string
  existingCount: number
  defaultCredentialType?: string
  onImport: (accounts: ImportedAccount[]) => void
}

export function AccountImportPanel(props: AccountImportPanelProps) {
  const { t } = useTranslation()
  const fileInputRef = useRef<HTMLInputElement>(null)
  const [credentialType, setCredentialType] = useState(
    props.defaultCredentialType ?? 'api_key'
  )
  const [rawInput, setRawInput] = useState('')
  const [preview, setPreview] = useState<ImportPreviewRow[]>([])
  const validAccounts = useMemo(
    () => preview.flatMap((row) => (row.account ? [row.account] : [])),
    [preview]
  )
  const invalidCount = preview.length - validAccounts.length

  const defaults = {
    type: credentialType,
    namePrefix: props.provider.trim() || 'account',
    startIndex: props.existingCount,
  }

  const parseText = () => {
    setPreview(
      markDuplicateCredentials(
        parseAccountImport(rawInput, 'pasted.txt', defaults)
      )
    )
  }

  const parseFiles = async (files: FileList | File[]) => {
    const selected = [...files]
    const contents = await Promise.all(
      selected.map(async (file) => ({
        file,
        content: file.size <= 2 * 1024 * 1024 ? await file.text() : null,
      }))
    )
    const next: ImportPreviewRow[] = []
    let nextIndex = defaults.startIndex
    for (const { file, content } of contents) {
      if (content === null) {
        next.push({
          source: file.name,
          line: 1,
          error: 'File exceeds 2 MB limit',
        })
        continue
      }
      const rows = parseAccountImport(content, file.name, {
        ...defaults,
        startIndex: nextIndex,
      })
      next.push(...rows)
      nextIndex += rows.filter((row) => row.account).length
    }
    setPreview(markDuplicateCredentials(next))
  }

  return (
    <div
      className='border-primary/25 bg-primary/[0.025] min-w-0 space-y-3 overflow-hidden border-l-2 p-4'
      onDragOver={(event) => event.preventDefault()}
      onDrop={(event) => {
        event.preventDefault()
        void parseFiles(event.dataTransfer.files)
      }}
    >
      <div className='flex flex-wrap items-start justify-between gap-3'>
        <div>
          <h4 className='font-medium'>{t('Import Provider Accounts')}</h4>
          <p className='text-muted-foreground mt-1 text-sm'>
            {t('Paste credentials or import TXT, CSV, and JSON files.')}
          </p>
        </div>
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={() => fileInputRef.current?.click()}
        >
          <FileUp />
          {t('Choose Files')}
        </Button>
        <Input
          ref={fileInputRef}
          type='file'
          multiple
          accept='.txt,.csv,.json,text/plain,text/csv,application/json'
          className='hidden'
          onChange={(event) => {
            if (event.target.files) void parseFiles(event.target.files)
            event.target.value = ''
          }}
        />
      </div>

      <div className='grid min-w-0 gap-3 md:grid-cols-[11rem_minmax(0,1fr)_auto] md:items-end'>
        <div className='min-w-0'>
          <Label className='mb-1.5'>{t('Credential Type')}</Label>
          <NativeSelect
            className='w-full'
            value={credentialType}
            onChange={(event) => setCredentialType(event.target.value)}
          >
            <NativeSelectOption value='api_key'>API Key</NativeSelectOption>
            <NativeSelectOption value='oauth_json'>
              OAuth JSON
            </NativeSelectOption>
            <NativeSelectOption value='cookie'>Cookie</NativeSelectOption>
            <NativeSelectOption value='custom'>
              {t('Custom')}
            </NativeSelectOption>
          </NativeSelect>
        </div>
        <div>
          <Label className='mb-1.5'>
            {t('Credentials or structured data')}
          </Label>
          <Textarea
            rows={3}
            value={rawInput}
            onChange={(event) => setRawInput(event.target.value)}
            placeholder={t('One credential per line, or paste a JSON array')}
          />
        </div>
        <Button
          type='button'
          variant='outline'
          disabled={!rawInput.trim()}
          onClick={parseText}
        >
          {t('Preview')}
        </Button>
      </div>

      {preview.length > 0 ? (
        <div className='space-y-3 border-t pt-3'>
          <div className='flex flex-wrap items-center gap-3 text-sm'>
            <span className='text-emerald-600 dark:text-emerald-400'>
              <CheckCircle2 className='mr-1 inline size-4' />
              {t('{{count}} valid accounts', { count: validAccounts.length })}
            </span>
            {invalidCount > 0 ? (
              <span className='text-destructive'>
                <AlertCircle className='mr-1 inline size-4' />
                {t('{{count}} invalid rows', { count: invalidCount })}
              </span>
            ) : null}
            <Button
              type='button'
              variant='ghost'
              size='icon-sm'
              className='ml-auto'
              aria-label={t('Clear preview')}
              onClick={() => setPreview([])}
            >
              <X />
            </Button>
          </div>
          <div className='max-h-64 min-w-0 touch-pan-y overflow-y-auto overscroll-contain border'>
            {preview.map((row) => (
              <div
                key={`${row.source}-${row.line}-${row.error ?? row.account?.name}`}
                className='grid min-w-0 grid-cols-[minmax(0,1fr)_auto] gap-2 border-b px-3 py-2 text-xs last:border-b-0 sm:grid-cols-[minmax(8rem,1fr)_5rem_minmax(10rem,2fr)] sm:gap-3'
              >
                <span className='truncate'>{row.source}</span>
                <span className='text-muted-foreground'>
                  {t('Line {{line}}', { line: row.line })}
                </span>
                <span
                  className={`col-span-2 break-words sm:col-span-1 ${row.error ? 'text-destructive' : ''}`}
                >
                  {row.error ? t(row.error) : row.account?.name}
                </span>
              </div>
            ))}
          </div>
          <div className='flex justify-end'>
            <Button
              type='button'
              disabled={validAccounts.length === 0 || invalidCount > 0}
              onClick={() => {
                props.onImport(validAccounts)
                setPreview([])
                setRawInput('')
              }}
            >
              <Upload />
              {t('Add Validated Accounts')}
            </Button>
          </div>
        </div>
      ) : null}
    </div>
  )
}

function markDuplicateCredentials(
  rows: ImportPreviewRow[]
): ImportPreviewRow[] {
  const credentials = new Set<string>()
  return rows.map((row) => {
    if (!row.account || !credentials.has(row.account.credential)) {
      if (row.account) credentials.add(row.account.credential)
      return row
    }
    return {
      source: row.source,
      line: row.line,
      error: 'Duplicate credential in import',
    }
  })
}
