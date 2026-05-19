import type { KiroImportAccount, KiroImportRequest } from '@/types'

export type KiroImportMode = 'refresh_token' | 'api_key' | 'json'

export class KiroImportInputError extends Error {
  constructor(public readonly code: 'empty' | 'invalid_json' | 'invalid_json_shape' | 'invalid_credential_line') {
    super(code)
  }
}

const splitCredentialItems = (input: string): string[] => {
  return input
    .split(/\r?\n|,|;/)
    .map(item => item.trim())
    .filter(Boolean)
}

const parseOptionalEmailCredential = (
  line: string,
  field: 'refresh_token' | 'kiro_api_key'
): KiroImportAccount => {
  const separatorIndex = line.indexOf('----')
  if (separatorIndex < 0) {
    return { [field]: line } as KiroImportAccount
  }
  if (separatorIndex === 0) {
    throw new KiroImportInputError('invalid_credential_line')
  }

  const email = line.slice(0, separatorIndex).trim()
  const credential = line.slice(separatorIndex + 4).trim()
  if (!email || !credential) {
    throw new KiroImportInputError('invalid_credential_line')
  }
  return { email, [field]: credential } as KiroImportAccount
}

const parseJSONImportInput = (input: string): KiroImportRequest => {
  let parsed: unknown
  try {
    parsed = JSON.parse(input)
  } catch {
    throw new KiroImportInputError('invalid_json')
  }

  if (Array.isArray(parsed)) {
    return { accounts: parsed as KiroImportAccount[] }
  }
  if (parsed && typeof parsed === 'object') {
    const record = parsed as Record<string, unknown>
    const accountMetadataKeys = [
      'access_token',
      'accessToken',
      'profile_arn',
      'profileArn',
      'expires_at',
      'expiresAt',
      'login_hint',
      'loginHint',
      'auth_method',
      'authMethod',
      'kiro_auth_token_raw'
    ]
    if (!Array.isArray(record.accounts) && accountMetadataKeys.some(key => key in record)) {
      return { accounts: [parsed as KiroImportAccount] }
    }
    return parsed as KiroImportRequest
  }
  throw new KiroImportInputError('invalid_json_shape')
}

export const buildKiroImportPayload = (
  input: string,
  mode: KiroImportMode
): KiroImportRequest => {
  const trimmed = input.trim()
  if (!trimmed) {
    throw new KiroImportInputError('empty')
  }

  if (mode === 'json') {
    return parseJSONImportInput(trimmed)
  }

  const items = splitCredentialItems(trimmed)
  if (!items.length) {
    throw new KiroImportInputError('empty')
  }

  if (mode === 'api_key') {
    return { accounts: items.map(item => parseOptionalEmailCredential(item, 'kiro_api_key')) }
  }
  return { accounts: items.map(item => parseOptionalEmailCredential(item, 'refresh_token')) }
}

export const createKiroImportIdempotencyKey = (): string => {
  const random = Math.random().toString(36).slice(2, 10)
  return `kiro-import-${Date.now()}-${random}`
}
