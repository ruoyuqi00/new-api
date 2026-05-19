import type { WindsurfImportAccount, WindsurfImportRequest } from '@/types'

export type WindsurfImportMode = 'token' | 'api_key' | 'email_password' | 'json'

export class WindsurfImportInputError extends Error {
  constructor(public readonly code: 'empty' | 'invalid_json' | 'invalid_json_shape' | 'invalid_email_password_line') {
    super(code)
  }
}

const splitCredentialItems = (input: string): string[] => {
  return input
    .split(/\r?\n|,|;/)
    .map(item => item.trim())
    .filter(Boolean)
}

const splitCredentialLines = (input: string): string[] => {
  return input
    .split(/\r?\n/)
    .map(item => item.trim())
    .filter(Boolean)
}

const parseEmailPasswordAccount = (line: string): WindsurfImportAccount => {
  const separatorIndex = line.indexOf('----')
  if (separatorIndex <= 0) {
    throw new WindsurfImportInputError('invalid_email_password_line')
  }

  const email = line.slice(0, separatorIndex).trim()
  const password = line.slice(separatorIndex + 4).trim()
  if (!email || !password) {
    throw new WindsurfImportInputError('invalid_email_password_line')
  }
  return { email, password }
}

const parseJSONImportInput = (input: string): WindsurfImportRequest => {
  let parsed: unknown
  try {
    parsed = JSON.parse(input)
  } catch {
    throw new WindsurfImportInputError('invalid_json')
  }

  if (Array.isArray(parsed)) {
    return { accounts: parsed as WindsurfImportAccount[] }
  }
  if (parsed && typeof parsed === 'object') {
    return parsed as WindsurfImportRequest
  }
  throw new WindsurfImportInputError('invalid_json_shape')
}

export const buildWindsurfImportPayload = (
  input: string,
  mode: WindsurfImportMode
): WindsurfImportRequest => {
  const trimmed = input.trim()
  if (!trimmed) {
    throw new WindsurfImportInputError('empty')
  }

  if (mode === 'json') {
    return parseJSONImportInput(trimmed)
  }

  if (mode === 'token') {
    const tokens = splitCredentialItems(trimmed)
    if (!tokens.length) {
      throw new WindsurfImportInputError('empty')
    }
    return { tokens }
  }

  if (mode === 'api_key') {
    const accounts = splitCredentialItems(trimmed).map(apiKey => ({ api_key: apiKey }))
    if (!accounts.length) {
      throw new WindsurfImportInputError('empty')
    }
    return { accounts }
  }

  const accounts = splitCredentialLines(trimmed).map(parseEmailPasswordAccount)
  if (!accounts.length) {
    throw new WindsurfImportInputError('empty')
  }
  return { accounts }
}

export const createWindsurfImportIdempotencyKey = (): string => {
  const random = Math.random().toString(36).slice(2, 10)
  return `windsurf-import-${Date.now()}-${random}`
}
