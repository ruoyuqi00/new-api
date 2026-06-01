import type { GroupPlatform, ModelsListConfig } from '@/types'

export const OPENAI_CC_SWITCH_CODEX_MODEL = 'gpt-5.5'

export type CcSwitchClientType = 'claude' | 'gemini'

export interface ClaudeClientModelConfig {
  sonnetModel?: string
  opusModel?: string
  haikuModel?: string
}

export interface CcSwitchImportConfig {
  app: string
  endpoint: string
  model?: string
}

export interface CcSwitchImportDeeplinkInput {
  baseUrl: string
  platform?: GroupPlatform | null
  clientType: CcSwitchClientType
  providerName: string
  apiKey: string
  usageScript: string
  modelsListConfig?: ModelsListConfig | null
  modelConfig?: ClaudeClientModelConfig
}

const normalizeModels = (
  modelsListConfig?: ModelsListConfig | null,
  options: { requireEnabled?: boolean } = { requireEnabled: true }
): string[] => {
  if (options.requireEnabled && !modelsListConfig?.enabled) {
    return []
  }
  if (!Array.isArray(modelsListConfig?.models)) {
    return []
  }
  return modelsListConfig.models
    .map((model) => model.trim())
    .filter(Boolean)
}

const preferNonThinking = (models: string[]): string | undefined =>
  models.find((model) => !/(?:^|[-_.])(thinking|fast)(?:$|[-_.])/i.test(model)) || models[0]

const findModel = (models: string[], pattern: RegExp): string | undefined =>
  preferNonThinking(models.filter((model) => pattern.test(model)))

export function resolveOpenAiCodexModel(modelsListConfig?: ModelsListConfig | null): string {
  const models = normalizeModels(modelsListConfig, { requireEnabled: false })
  return (
    models.find((model) => /^(?:gpt-|o\d|codex)/i.test(model) && !/image/i.test(model)) ||
    OPENAI_CC_SWITCH_CODEX_MODEL
  )
}

export function resolveClaudeClientModelConfig(
  modelsListConfig?: ModelsListConfig | null
): ClaudeClientModelConfig {
  const models = normalizeModels(modelsListConfig)
  if (!models.length) {
    return {}
  }

  const fallbackModel = preferNonThinking(models)
  const sonnetModel = findModel(models, /sonnet/i) || fallbackModel
  const opusModel = findModel(models, /opus/i) || sonnetModel || fallbackModel
  const haikuModel = findModel(models, /haiku/i) || sonnetModel || opusModel || fallbackModel

  return {
    ...(sonnetModel ? { sonnetModel } : {}),
    ...(opusModel ? { opusModel } : {}),
    ...(haikuModel ? { haikuModel } : {})
  }
}

export function resolveCcSwitchImportConfig(
  platform: GroupPlatform | undefined | null,
  clientType: CcSwitchClientType,
  baseUrl: string,
  modelsListConfig?: ModelsListConfig | null
): CcSwitchImportConfig {
  switch (platform || 'anthropic') {
    case 'antigravity':
      return {
        app: clientType === 'gemini' ? 'gemini' : 'claude',
        endpoint: `${baseUrl}/antigravity`
      }
    case 'openai':
      return {
        app: 'codex',
        endpoint: baseUrl,
        model: resolveOpenAiCodexModel(modelsListConfig)
      }
    case 'gemini':
      return {
        app: 'gemini',
        endpoint: baseUrl
      }
    default:
      return {
        app: 'claude',
        endpoint: baseUrl
      }
  }
}

export function buildCcSwitchImportDeeplink(input: CcSwitchImportDeeplinkInput): string {
  const config = resolveCcSwitchImportConfig(
    input.platform,
    input.clientType,
    input.baseUrl,
    input.modelsListConfig
  )
  const entries: [string, string][] = [
    ['resource', 'provider'],
    ['app', config.app],
    ['name', input.providerName],
    ['homepage', input.baseUrl],
    ['endpoint', config.endpoint],
    ['apiKey', input.apiKey],
    ['configFormat', 'json'],
    ['usageEnabled', 'true'],
    ['usageScript', btoa(input.usageScript)],
    ['usageAutoInterval', '30']
  ]

  if (config.model) {
    entries.splice(2, 0, ['model', config.model])
  }
  if (config.app === 'claude' && input.modelConfig) {
    const modelEntries: [keyof ClaudeClientModelConfig, string][] = [
      ['sonnetModel', input.modelConfig.sonnetModel || ''],
      ['opusModel', input.modelConfig.opusModel || ''],
      ['haikuModel', input.modelConfig.haikuModel || '']
    ]
    for (const [key, value] of modelEntries) {
      if (value) {
        entries.push([key, value])
      }
    }
  }

  return `ccswitch://v1/import?${new URLSearchParams(entries).toString()}`
}
