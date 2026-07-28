import Papa from 'papaparse'

export type ImportedAccount = {
  name: string
  type: string
  credential: string
  base_url: string
  model_mapping: string
  priority: number
  weight: number
  concurrency_limit: number
  cooldown_seconds: number
  status?: number
  expires_at?: number
  metadata?: string
  adapter_type?: number
  provider?: string
}

export type ImportPreviewRow = {
  source: string
  line: number
  account?: ImportedAccount
  error?: string
}

type ImportDefaults = {
  type: string
  namePrefix: string
  startIndex: number
}

type UnknownRecord = Record<string, unknown>

export function parseAccountImport(
  content: string,
  source: string,
  defaults: ImportDefaults
): ImportPreviewRow[] {
  const trimmed = content.trim()
  if (!trimmed) return []

  if (
    source.toLowerCase().endsWith('.json') ||
    /^[[{]/.test(trimmed) ||
    defaults.type === 'oauth_json'
  ) {
    return parseJsonImport(trimmed, source, defaults)
  }
  if (source.toLowerCase().endsWith('.csv')) {
    const result = Papa.parse<UnknownRecord>(trimmed, {
      header: true,
      skipEmptyLines: 'greedy',
      transformHeader: (header) => header.trim().toLowerCase(),
    })
    const rows = result.data.map((record, index) =>
      normalizeRecord(record, source, index + 2, defaults, index)
    )
    return [
      ...rows,
      ...result.errors.map((error) => ({
        source,
        line: (error.row ?? 0) + 2,
        error: error.message,
      })),
    ]
  }

  return trimmed
    .split(/\r?\n/)
    .map((credential, index) => ({ credential: credential.trim(), index }))
    .filter((entry) => entry.credential !== '')
    .map((entry, offset) =>
      normalizeRecord(
        { credential: entry.credential },
        source,
        entry.index + 1,
        defaults,
        offset
      )
    )
}

function parseJsonImport(
  content: string,
  source: string,
  defaults: ImportDefaults
): ImportPreviewRow[] {
  let parsed: unknown
  try {
    parsed = JSON.parse(content)
  } catch (error) {
    const stream = parseJsonStream(content)
    if (stream) {
      return normalizeJsonValues(stream, source, defaults)
    }
    return parseMixedImportLines(content, source, defaults, error)
  }
  if (isSub2ApiEnvelope(parsed)) {
    return parsed.accounts.map((item, index) =>
      normalizeSub2ApiRecord(item, parsed, source, index + 1, defaults, index)
    )
  }
  return normalizeJsonValues(
    flattenJsonValues([parsed]).map((value, index) => ({
      value,
      line: index + 1,
    })),
    source,
    defaults
  )
}

type JsonImportValue = {
  value: unknown
  line: number
}

function normalizeJsonValues(
  values: JsonImportValue[],
  source: string,
  defaults: ImportDefaults
): ImportPreviewRow[] {
  return values.map((entry, index) => {
    if (typeof entry.value === 'string') {
      return normalizeRecord(
        { credential: entry.value },
        source,
        entry.line,
        defaults,
        index
      )
    }
    if (!isRecord(entry.value)) {
      return { source, line: entry.line, error: 'Credential is required' }
    }
    const record = entry.value
    if (isRecord(record.credentials)) {
      return normalizeSub2ApiRecord(
        record,
        {},
        source,
        entry.line,
        defaults,
        index
      )
    }
    const codex = codexFields(record)
    if (codex.accessToken && codex.accountId) {
      return normalizeDirectCodexRecord(
        record,
        source,
        entry.line,
        defaults,
        index
      )
    }
    return normalizeRecord(record, source, entry.line, defaults, index)
  })
}

function parseJsonStream(content: string): JsonImportValue[] | undefined {
  const values: JsonImportValue[] = []
  let cursor = 0
  let line = 1
  while (cursor < content.length) {
    while (/\s/.test(content[cursor] ?? '')) {
      if (content[cursor] === '\n') line++
      cursor++
    }
    if (cursor >= content.length) break
    if (content[cursor] !== '{' && content[cursor] !== '[') return undefined

    const start = cursor
    const startLine = line
    const stack: string[] = []
    let inString = false
    let escaped = false
    for (; cursor < content.length; cursor++) {
      const char = content[cursor]
      if (char === '\n') line++
      if (inString) {
        if (escaped) escaped = false
        else if (char === '\\') escaped = true
        else if (char === '"') inString = false
        continue
      }
      if (char === '"') {
        inString = true
        continue
      }
      if (char === '{' || char === '[') stack.push(char)
      if (char === '}' || char === ']') {
        const expected = char === '}' ? '{' : '['
        if (stack.pop() !== expected) return undefined
        if (stack.length === 0) {
          cursor++
          break
        }
      }
    }
    if (stack.length > 0 || inString) return undefined
    try {
      const parsed: unknown = JSON.parse(content.slice(start, cursor))
      for (const value of flattenJsonValues([parsed])) {
        values.push({ value, line: startLine })
      }
    } catch {
      return undefined
    }
  }
  return values.length > 0 ? values : undefined
}

function parseMixedImportLines(
  content: string,
  source: string,
  defaults: ImportDefaults,
  jsonError: unknown
): ImportPreviewRow[] {
  const lines = content.split(/\r?\n/)
  if (lines.length === 1 && /^[{[]/.test(content.trim())) {
    return [
      {
        source,
        line: 1,
        error: jsonError instanceof Error ? jsonError.message : 'Invalid JSON',
      },
    ]
  }

  const values: JsonImportValue[] = []
  const errors: ImportPreviewRow[] = []
  for (let index = 0; index < lines.length; index++) {
    const line = lines[index].trim()
    if (!line) continue
    if (!/^[{[]/.test(line)) {
      values.push({ value: line, line: index + 1 })
      continue
    }
    try {
      const parsed: unknown = JSON.parse(line)
      for (const value of flattenJsonValues([parsed])) {
        values.push({ value, line: index + 1 })
      }
    } catch (error) {
      errors.push({
        source,
        line: index + 1,
        error: error instanceof Error ? error.message : 'Invalid JSON',
      })
    }
  }
  return [...normalizeJsonValues(values, source, defaults), ...errors]
}

function flattenJsonValues(values: unknown[]): unknown[] {
  const flattened: unknown[] = []
  for (const value of values) {
    if (Array.isArray(value)) flattened.push(...flattenJsonValues(value))
    else flattened.push(value)
  }
  return flattened
}

function isSub2ApiEnvelope(value: unknown): value is UnknownRecord & {
  accounts: unknown[]
} {
  if (!isRecord(value) || !Array.isArray(value.accounts)) return false
  if (value.format === 'sub2api') return true
  if (value.type === 'sub2api-data' || value.type === 'sub2api-bundle') {
    return true
  }
  return typeof value.exported_at === 'string' && Array.isArray(value.proxies)
}

function normalizeSub2ApiRecord(
  value: unknown,
  envelope: UnknownRecord,
  source: string,
  line: number,
  defaults: ImportDefaults,
  offset: number
): ImportPreviewRow {
  if (!isRecord(value) || !isRecord(value.credentials)) {
    return { source, line, error: 'Credential is required' }
  }

  const credentials = value.credentials
  const extra = isRecord(value.extra) ? value.extra : undefined
  const platform = stringValue(value.platform).trim().toLowerCase()
  const accountType = stringValue(value.type).trim().toLowerCase()
  const credentialSource: UnknownRecord = {
    ...value,
    ...credentials,
    tokens: credentials.tokens ?? value.tokens,
    account: credentials.account ?? value.account,
    user: credentials.user ?? value.user,
  }
  const codex = codexFields(credentialSource)
  const isCodexOAuth =
    codex.accessToken !== '' &&
    codex.accountId !== '' &&
    (platform === 'openai' ||
      platform === 'codex' ||
      (platform === '' &&
        (accountType.includes('oauth') ||
          accountType.includes('codex') ||
          defaults.type === 'oauth_json')))
  let type = defaults.type
  let credential = stringValue(
    credentials.api_key ??
      credentials.key ??
      credentials.token ??
      credentials.cookie ??
      credentials.session_key ??
      credentials.session_token ??
      credentials.setup_token
  ).trim()
  let adapterType = providerAdapterType(platform)

  if (isCodexOAuth) {
    type = 'oauth_json'
    adapterType = 57
    if (
      credentialSource.last_refresh === undefined &&
      extra?.last_refresh !== undefined
    ) {
      credentialSource.last_refresh = extra.last_refresh
    }
    if (
      credentialSource.expired === undefined &&
      extra?.expired !== undefined
    ) {
      credentialSource.expired = extra.expired
    }
    if (
      credentialSource.expired === undefined &&
      credentialSource.expires_at === undefined &&
      credentialSource.expiresAt === undefined &&
      value.expires_at !== undefined
    ) {
      credentialSource.expired = value.expires_at
    }
    credential = JSON.stringify(
      normalizeCodexCredential(
        credentialSource,
        codex.accessToken,
        codex.accountId
      )
    )
  } else if (!credential) {
    credential = stringValue(credentials.access_token).trim()
    type = credential ? 'custom' : defaults.type
  } else if (
    accountType.includes('cookie') ||
    accountType.includes('session')
  ) {
    type = 'cookie'
  }

  let expiresAt = value.expires_at ?? credentials.expires_at
  if (isCodexOAuth) {
    expiresAt = codex.refreshToken
      ? undefined
      : codexExpiryUnixSeconds(
          codex.expiresAt ?? extra?.expired ?? value.expires_at
        )
  }

  const normalized = normalizeRecord(
    {
      name: value.name,
      type,
      credential,
      base_url: credentials.base_url ?? credentials.baseUrl,
      model_mapping: credentials.model_mapping ?? credentials.modelMapping,
      priority: value.priority,
      concurrency: value.concurrency,
      expires_at: expiresAt,
      metadata: JSON.stringify({
        source_format: 'sub2api',
        source_type: envelope.type,
        source_version: envelope.version,
        exported_at: envelope.exported_at,
        workspace_id: envelope.workspace_id,
        token_kind: envelope.token_kind,
        platform,
        account_type: accountType,
        plan_type: value.plan_type ?? extra?.plan_type ?? credentials.plan_type,
        notes: value.notes,
        proxy_key: value.proxy_key,
        rate_multiplier: value.rate_multiplier,
        auto_pause_on_expired: value.auto_pause_on_expired,
        extra,
      }),
      adapter_type: adapterType,
      provider: platform,
    },
    source,
    line,
    defaults,
    offset
  )
  return normalized
}

function normalizeDirectCodexRecord(
  record: UnknownRecord,
  source: string,
  line: number,
  defaults: ImportDefaults,
  offset: number
): ImportPreviewRow {
  const codex = codexFields(record)
  if (!codex.accessToken || !codex.accountId) {
    return { source, line, error: 'Credential is required' }
  }
  return normalizeRecord(
    {
      ...record,
      name:
        stringValue(record.name).trim() ||
        codex.name ||
        codex.email ||
        codex.accountId ||
        codex.userId,
      type: 'oauth_json',
      credential: JSON.stringify(
        normalizeCodexCredential(record, codex.accessToken, codex.accountId)
      ),
      expires_at: codex.refreshToken
        ? undefined
        : codexExpiryUnixSeconds(codex.expiresAt),
      adapter_type: 57,
      provider: 'openai',
    },
    source,
    line,
    defaults,
    offset
  )
}

function normalizeCodexCredential(
  record: UnknownRecord,
  accessToken: string,
  accountId: string
): UnknownRecord {
  const credential = { ...record }
  for (const field of [
    'name',
    'credential',
    'base_url',
    'baseUrl',
    'model_mapping',
    'modelMapping',
    'priority',
    'weight',
    'concurrency_limit',
    'concurrency',
    'cooldown_seconds',
    'cooldown',
    'status',
    'expires_at',
    'expiresAt',
    'metadata',
    'adapter_type',
    'provider',
    'platform',
    'credentials',
    'extra',
    'notes',
    'proxy_key',
    'rate_multiplier',
    'auto_pause_on_expired',
    'tokens',
    'account',
    'user',
    'session_token',
    'sessionToken',
    'expires',
  ]) {
    delete credential[field]
  }

  const codex = codexFields(record)
  credential.access_token = accessToken
  credential.account_id = accountId
  credential.type = 'codex'
  delete credential.token
  delete credential.accessToken
  delete credential.accountId
  delete credential.chatgpt_account_id
  delete credential.chatgptAccountId
  delete credential.chatgptUserId
  delete credential.user_id
  delete credential.userId
  delete credential.planType
  delete credential.organizationId
  delete credential.org_id
  delete credential.orgId

  if (
    hasCodexField(
      record,
      ['id_token'],
      ['idToken'],
      ['tokens', 'id_token'],
      ['tokens', 'idToken']
    )
  ) {
    credential.id_token = codex.idToken
  }
  delete credential.idToken

  if (
    hasCodexField(
      record,
      ['refresh_token'],
      ['refreshToken'],
      ['tokens', 'refresh_token'],
      ['tokens', 'refreshToken']
    )
  ) {
    credential.refresh_token = codex.refreshToken
  }
  delete credential.refreshToken

  if (codex.email) credential.email = codex.email
  if (codex.userId) credential.chatgpt_user_id = codex.userId
  if (codex.planType) credential.plan_type = codex.planType
  if (codex.organizationId) credential.organization_id = codex.organizationId

  const lastRefresh = normalizeCodexTimestamp(
    firstDefined(record.last_refresh, record.lastRefresh)
  )
  if (lastRefresh) credential.last_refresh = lastRefresh
  else delete credential.last_refresh
  delete credential.lastRefresh

  const expired = normalizeCodexTimestamp(codex.expiresAt)
  if (expired) credential.expired = expired
  else delete credential.expired
  delete credential.expiresAt

  return credential
}

type CodexFields = {
  accessToken: string
  refreshToken: string
  idToken: string
  accountId: string
  userId: string
  email: string
  name: string
  planType: string
  organizationId: string
  expiresAt: unknown
}

function codexFields(record: UnknownRecord): CodexFields {
  const accessToken = firstCodexString(
    record,
    ['tokens', 'access_token'],
    ['tokens', 'accessToken'],
    ['access_token'],
    ['accessToken'],
    ['token']
  )
  const claims = decodeJwtClaims(accessToken)
  const auth = claims?.['https://api.openai.com/auth']
  const authRecord = isRecord(auth) ? auth : undefined
  return {
    accessToken,
    refreshToken: firstCodexString(
      record,
      ['tokens', 'refresh_token'],
      ['tokens', 'refreshToken'],
      ['refresh_token'],
      ['refreshToken']
    ),
    idToken: firstCodexString(
      record,
      ['tokens', 'id_token'],
      ['tokens', 'idToken'],
      ['id_token'],
      ['idToken']
    ),
    accountId:
      firstCodexString(
        record,
        ['chatgpt_account_id'],
        ['chatgptAccountId'],
        ['account_id'],
        ['accountId'],
        ['account', 'id'],
        ['account', 'account_id'],
        ['account', 'chatgpt_account_id']
      ) || stringValue(authRecord?.chatgpt_account_id).trim(),
    userId:
      firstCodexString(
        record,
        ['chatgpt_user_id'],
        ['chatgptUserId'],
        ['user_id'],
        ['userId'],
        ['user', 'id']
      ) ||
      stringValue(
        authRecord?.chatgpt_user_id ?? authRecord?.user_id ?? claims?.sub
      ).trim(),
    email:
      firstCodexString(record, ['email'], ['user', 'email']) ||
      stringValue(claims?.email).trim(),
    name: firstCodexString(record, ['name'], ['user', 'name']),
    planType:
      firstCodexString(
        record,
        ['plan_type'],
        ['planType'],
        ['account', 'plan_type'],
        ['account', 'planType']
      ) || stringValue(authRecord?.chatgpt_plan_type).trim(),
    organizationId:
      firstCodexString(
        record,
        ['organization_id'],
        ['organizationId'],
        ['org_id'],
        ['orgId']
      ) || stringValue(authRecord?.poid).trim(),
    expiresAt: firstDefined(
      nestedValue(record, ['tokens', 'expires_at']),
      nestedValue(record, ['tokens', 'expiresAt']),
      record.expired,
      record.expires_at,
      record.expiresAt,
      claims?.exp
    ),
  }
}

function firstCodexString(record: UnknownRecord, ...paths: string[][]): string {
  for (const path of paths) {
    const value = stringValue(nestedValue(record, path)).trim()
    if (value) return value
  }
  return ''
}

function hasCodexField(record: UnknownRecord, ...paths: string[][]): boolean {
  return paths.some((path) => nestedValue(record, path) !== undefined)
}

function nestedValue(record: UnknownRecord, path: string[]): unknown {
  let value: unknown = record
  for (const key of path) {
    if (!isRecord(value)) return undefined
    value = value[key]
  }
  return value
}

function firstDefined(...values: unknown[]): unknown {
  return values.find((value) => value !== undefined && value !== null)
}

function normalizeCodexTimestamp(value: unknown): string | undefined {
  const raw = stringValue(value).trim()
  if (raw && !/^\d+(?:\.\d+)?$/.test(raw)) return raw

  const numeric = typeof value === 'number' ? value : Number(raw)
  if (!Number.isFinite(numeric) || numeric <= 0) return undefined
  const milliseconds = numeric >= 1_000_000_000_000 ? numeric : numeric * 1000
  const date = new Date(milliseconds)
  return Number.isNaN(date.getTime()) ? undefined : date.toISOString()
}

function codexExpiryUnixSeconds(value: unknown): number | undefined {
  const normalized = normalizeCodexTimestamp(value)
  if (!normalized) return undefined
  const milliseconds = Date.parse(normalized)
  return Number.isNaN(milliseconds)
    ? undefined
    : Math.floor(milliseconds / 1000)
}

function normalizeRecord(
  record: UnknownRecord,
  source: string,
  line: number,
  defaults: ImportDefaults,
  offset: number
): ImportPreviewRow {
  const type = stringValue(record.type).trim() || defaults.type
  let credential = stringValue(
    record.credential ??
      record.api_key ??
      record.key ??
      record.token ??
      record.cookie ??
      record.session_key ??
      record.session_token ??
      record.setup_token
  ).trim()
  if (type === 'oauth_json' && credential && !credential.startsWith('{')) {
    const accountId = extractCodexAccountId(credential)
    if (accountId) {
      return normalizeDirectCodexRecord(
        { ...record, access_token: credential, account_id: accountId },
        source,
        line,
        defaults,
        offset
      )
    }
  }
  if (!credential && stringValue(record.access_token).trim()) {
    credential =
      type === 'oauth_json'
        ? JSON.stringify(record)
        : stringValue(record.access_token).trim()
  }
  const baseUrl = stringValue(record.base_url ?? record.baseUrl).trim()
  const modelMapping = normalizeModelMapping(
    record.model_mapping ?? record.modelMapping
  )
  const error = validateImportedAccount(type, credential, baseUrl, modelMapping)
  if (error) return { source, line, error }

  return {
    source,
    line,
    account: {
      name:
        stringValue(record.name).trim() ||
        `${defaults.namePrefix}-${defaults.startIndex + offset + 1}`,
      type,
      credential,
      base_url: baseUrl.replace(/\/+$/, ''),
      model_mapping: modelMapping,
      priority: numberValue(record.priority, 0),
      weight: numberValue(record.weight, 100),
      concurrency_limit: numberValue(
        record.concurrency_limit ?? record.concurrency,
        0
      ),
      cooldown_seconds: numberValue(
        record.cooldown_seconds ?? record.cooldown,
        20
      ),
      ...(record.status !== undefined
        ? { status: numberValue(record.status, 1) }
        : {}),
      ...(record.expires_at !== undefined
        ? { expires_at: numberValue(record.expires_at, 0) }
        : {}),
      ...(record.metadata !== undefined
        ? { metadata: normalizeMetadata(record.metadata) }
        : {}),
      ...(numberValue(record.adapter_type, 0) > 0
        ? { adapter_type: numberValue(record.adapter_type, 0) }
        : {}),
      ...(stringValue(record.provider).trim()
        ? { provider: stringValue(record.provider).trim() }
        : {}),
    },
  }
}

function providerAdapterType(platform: string): number {
  if (platform === 'openai') return 1
  if (platform === 'anthropic' || platform === 'claude') return 14
  if (platform === 'gemini' || platform === 'google') return 24
  return 0
}

function extractCodexAccountId(token: string): string {
  const claims = decodeJwtClaims(token)
  const auth = claims?.['https://api.openai.com/auth']
  if (!isRecord(auth)) return ''
  return stringValue(auth.chatgpt_account_id).trim()
}

function decodeJwtClaims(token: string): UnknownRecord | undefined {
  const parts = token.split('.')
  if (parts.length !== 3) return undefined
  try {
    const payload = parts[1].replaceAll('-', '+').replaceAll('_', '/')
    const padded = payload.padEnd(Math.ceil(payload.length / 4) * 4, '=')
    const parsed: unknown = JSON.parse(atob(padded))
    return isRecord(parsed) ? parsed : undefined
  } catch {
    return undefined
  }
}

function validateImportedAccount(
  type: string,
  credential: string,
  baseUrl: string,
  modelMapping: string
): string | undefined {
  if (!credential) return 'Credential is required'
  if (type === 'oauth_json') {
    try {
      const parsed: unknown = JSON.parse(credential)
      if (!isRecord(parsed)) return 'OAuth credential must be a JSON object'
    } catch {
      return 'OAuth credential must be valid JSON'
    }
  }
  if (baseUrl && !/^https?:\/\/[^\s]+$/i.test(baseUrl)) {
    return 'Base URL must use HTTP or HTTPS'
  }
  if (modelMapping) {
    try {
      const parsed: unknown = JSON.parse(modelMapping)
      if (
        !isRecord(parsed) ||
        !Object.values(parsed).every((value) => typeof value === 'string')
      ) {
        return 'Model mapping must contain string values'
      }
    } catch {
      return 'Model mapping must be valid JSON'
    }
  }
  return undefined
}

function normalizeModelMapping(value: unknown): string {
  if (isRecord(value)) return JSON.stringify(value)
  return stringValue(value).trim()
}

function normalizeMetadata(value: unknown): string {
  if (isRecord(value) || Array.isArray(value)) return JSON.stringify(value)
  return stringValue(value).trim()
}

function stringValue(value: unknown): string {
  return typeof value === 'string' ? value : ''
}

function numberValue(value: unknown, fallback: number): number {
  const number = typeof value === 'number' ? value : Number(value)
  return Number.isFinite(number) && number >= 0 ? Math.floor(number) : fallback
}

function isRecord(value: unknown): value is UnknownRecord {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}
