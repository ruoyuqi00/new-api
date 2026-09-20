export type ApiKeyGroupProtocol =
  | 'openai'
  | 'claude'
  | 'gemini'
  | 'image'
  | 'video'

export type ApiKeyGroupProtocolFilter = 'all' | ApiKeyGroupProtocol

export type ProtocolFilterableGroup = {
  value: string
  label: string
  desc?: string
  ratio?: number | string
  protocols?: readonly ApiKeyGroupProtocol[]
  endpointPaths?: readonly string[]
}

const protocolRouteLabels: Record<ApiKeyGroupProtocolFilter, string> = {
  all: 'All protocol routes',
  openai: 'OpenAI compatible routes',
  claude: 'Claude messages routes',
  gemini: 'Gemini native routes',
  image: 'Image generation routes',
  video: 'Video generation routes',
}

export function getProtocolRouteLabel(
  protocol: ApiKeyGroupProtocolFilter
): string {
  return protocolRouteLabels[protocol]
}

export function getApiKeyGroupProtocolNode(
  option: ProtocolFilterableGroup
): string {
  const identity = `${option.value} ${option.label}`
  if (/国模|国产/.test(identity)) {
    return 'CN'
  }
  if (/claude/i.test(identity)) {
    return 'CL'
  }
  if (/gemini/i.test(identity)) {
    return 'GM'
  }
  if (/图片|image/i.test(identity)) {
    return 'IM'
  }
  if (/视频|video/i.test(identity)) {
    return 'VD'
  }

  const nonOpenAIProtocol = option.protocols?.find(
    (protocol) => protocol !== 'openai'
  )
  switch (nonOpenAIProtocol ?? option.protocols?.[0]) {
    case 'openai':
      return 'OA'
    case 'claude':
      return 'CL'
    case 'gemini':
      return 'GM'
    case 'image':
      return 'IM'
    case 'video':
      return 'VD'
    default:
      return 'RT'
  }
}

export function filterApiKeyGroupOptions<T extends ProtocolFilterableGroup>(
  options: readonly T[],
  protocol: ApiKeyGroupProtocolFilter,
  searchValue: string
): T[] {
  const search = searchValue.trim().toLowerCase()

  return options.filter((option) => {
    if (protocol !== 'all' && !option.protocols?.includes(protocol)) {
      return false
    }
    if (!search) return true

    return [
      option.value,
      option.label,
      option.desc ?? '',
      String(option.ratio ?? ''),
      ...(option.endpointPaths ?? []),
    ].some((value) => value.toLowerCase().includes(search))
  })
}
