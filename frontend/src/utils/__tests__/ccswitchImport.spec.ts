import { describe, expect, it } from 'vitest'
import {
  OPENAI_CC_SWITCH_CODEX_MODEL,
  buildCcSwitchImportDeeplink,
  resolveClaudeClientModelConfig,
  resolveOpenAiCodexModel
} from '@/utils/ccswitchImport'
import type { GroupPlatform } from '@/types'

function paramsFromDeeplink(deeplink: string): URLSearchParams {
  const query = deeplink.split('?')[1] || ''
  return new URLSearchParams(query)
}

describe('ccswitchImport utils', () => {
  const baseInput = {
    baseUrl: 'https://api.example.com',
    providerName: 'Sub2API',
    apiKey: 'sk-test',
    usageScript: 'return true'
  }

  it('adds the Codex model parameter for OpenAI imports', () => {
    const params = paramsFromDeeplink(
      buildCcSwitchImportDeeplink({
        ...baseInput,
        platform: 'openai',
        clientType: 'claude'
      })
    )

    expect(params.get('resource')).toBe('provider')
    expect(params.get('app')).toBe('codex')
    expect(params.get('endpoint')).toBe(baseInput.baseUrl)
    expect(params.get('model')).toBe(OPENAI_CC_SWITCH_CODEX_MODEL)
    expect(atob(params.get('usageScript') || '')).toBe(baseInput.usageScript)
  })

  it('prefers the first usable OpenAI group model for Codex imports', () => {
    const params = paramsFromDeeplink(
      buildCcSwitchImportDeeplink({
        ...baseInput,
        platform: 'openai',
        clientType: 'claude',
        modelsListConfig: {
          enabled: false,
          models: ['gpt-image-1', 'gpt-5.5', 'gpt-5.4']
        }
      })
    )

    expect(params.get('model')).toBe('gpt-5.5')
  })

  it('falls back to the default Codex model when OpenAI group models are unavailable', () => {
    expect(resolveOpenAiCodexModel({ enabled: true, models: ['gpt-image-1'] })).toBe(
      OPENAI_CC_SWITCH_CODEX_MODEL
    )
  })

  it.each([
    { platform: 'anthropic' as GroupPlatform, clientType: 'claude' as const, app: 'claude' },
    { platform: 'gemini' as GroupPlatform, clientType: 'gemini' as const, app: 'gemini' }
  ])('does not add a generic model parameter for $platform imports', ({ platform, clientType, app }) => {
    const params = paramsFromDeeplink(
      buildCcSwitchImportDeeplink({
        ...baseInput,
        platform,
        clientType
      })
    )

    expect(params.get('app')).toBe(app)
    expect(params.get('endpoint')).toBe(baseInput.baseUrl)
    expect(params.has('model')).toBe(false)
  })

  it('adds Claude model fields when a group model list is available', () => {
    const modelConfig = resolveClaudeClientModelConfig({
      enabled: true,
      models: ['claude-sonnet-4.6', 'claude-opus-4.7', 'claude-haiku-4.5']
    })
    const params = paramsFromDeeplink(
      buildCcSwitchImportDeeplink({
        ...baseInput,
        platform: 'anthropic',
        clientType: 'claude',
        modelConfig
      })
    )

    expect(params.get('sonnetModel')).toBe('claude-sonnet-4.6')
    expect(params.get('opusModel')).toBe('claude-opus-4.7')
    expect(params.get('haikuModel')).toBe('claude-haiku-4.5')
  })

  it('falls back to the first custom model for opus-only Claude groups', () => {
    const modelConfig = resolveClaudeClientModelConfig({
      enabled: true,
      models: ['opus4.7', 'opus4.7-medium-thinking']
    })

    expect(modelConfig).toEqual({
      sonnetModel: 'opus4.7',
      opusModel: 'opus4.7',
      haikuModel: 'opus4.7'
    })
  })

  it('keeps Antigravity imports on the selected client endpoint without a model parameter', () => {
    const params = paramsFromDeeplink(
      buildCcSwitchImportDeeplink({
        ...baseInput,
        platform: 'antigravity',
        clientType: 'gemini'
      })
    )

    expect(params.get('app')).toBe('gemini')
    expect(params.get('endpoint')).toBe(`${baseInput.baseUrl}/antigravity`)
    expect(params.has('model')).toBe(false)
  })
})
