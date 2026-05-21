<template>
  <div class="space-y-4">
    <div class="flex flex-wrap items-start justify-between gap-3 rounded-lg border border-blue-200 bg-blue-50 p-3 text-sm text-blue-700 dark:border-blue-800 dark:bg-blue-900/20 dark:text-blue-200">
      <div class="max-w-3xl">
        {{ t('admin.accounts.providerAdaptersHint') }}
      </div>
      <div class="flex flex-wrap gap-2">
        <a class="btn btn-secondary px-3 py-1.5 text-sm" href="/windsurf-dashboard" target="_blank" rel="noopener noreferrer">
          {{ t('admin.accounts.providerAdaptersOpenWindsurf') }}
        </a>
        <a class="btn btn-secondary px-3 py-1.5 text-sm" href="/kiro-admin" target="_blank" rel="noopener noreferrer">
          {{ t('admin.accounts.providerAdaptersOpenKiro') }}
        </a>
        <button class="btn btn-secondary px-3 py-1.5 text-sm" type="button" :disabled="loadingAny" @click="loadAll">
          {{ loadingAny ? t('admin.accounts.providerAdaptersLoading') : t('admin.accounts.providerAdaptersRefresh') }}
        </button>
      </div>
    </div>

    <div class="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-4">
      <button
        v-for="endpoint in endpoints"
        :key="endpoint.key"
        type="button"
        :class="[
          'rounded-lg border p-4 text-left transition-colors',
          activeKey === endpoint.key
            ? 'border-primary-500 bg-primary-50 dark:border-primary-400 dark:bg-primary-900/30'
            : 'border-gray-200 bg-white hover:bg-gray-50 dark:border-dark-700 dark:bg-dark-800 dark:hover:bg-dark-700'
        ]"
        @click="activeKey = endpoint.key"
      >
        <div class="flex items-center justify-between gap-3">
          <div class="font-medium text-gray-900 dark:text-white">{{ endpoint.label }}</div>
          <span :class="endpointBadgeClass(endpoint)">
            {{ endpointBadgeText(endpoint) }}
          </span>
        </div>
        <div class="mt-3 text-sm text-gray-600 dark:text-dark-300">
          {{ endpointSummary(endpoint) }}
        </div>
        <div class="mt-2 truncate font-mono text-xs text-gray-400 dark:text-dark-500">
          {{ endpointPath(endpoint) }}
        </div>
      </button>
    </div>

    <div v-if="showKiroRuntimePanel" class="space-y-4 rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <div class="text-base font-semibold text-gray-900 dark:text-white">Kiro Runtime</div>
          <div class="mt-1 text-sm text-gray-500 dark:text-dark-400">
            {{ kiroRuntimeStatus?.engine || 'runtime adapter' }} / {{ kiroRuntimeStatus?.public_entry_only ? 'Sub2API public entry only' : 'direct adapter exposure' }}
          </div>
        </div>
        <span :class="runtimeStatusClass(kiroRuntimeStatus?.status)">
          {{ kiroRuntimeStatus?.status || '-' }}
        </span>
      </div>

      <div class="grid grid-cols-2 gap-3 lg:grid-cols-4">
        <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
          <div class="text-xs text-gray-500 dark:text-dark-400">Accounts</div>
          <div class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">
            {{ kiroRuntimeSummary.accounts_available }} / {{ kiroRuntimeSummary.accounts_total }}
          </div>
        </div>
        <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
          <div class="text-xs text-gray-500 dark:text-dark-400">Cooldown / Quota</div>
          <div class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">
            {{ kiroRuntimeSummary.accounts_cooldown }} / {{ kiroRuntimeSummary.accounts_quota_exhausted }}
          </div>
        </div>
        <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
          <div class="text-xs text-gray-500 dark:text-dark-400">Models</div>
          <div class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">
            {{ kiroRuntimeSummary.models_discovered }}
          </div>
        </div>
        <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
          <div class="text-xs text-gray-500 dark:text-dark-400">Profile ARN</div>
          <div class="mt-1 text-xl font-semibold text-gray-900 dark:text-white">
            {{ kiroRuntimeSummary.accounts_with_profile_arn }}
          </div>
        </div>
      </div>

      <div class="grid grid-cols-1 gap-4 xl:grid-cols-2">
        <div class="overflow-hidden rounded-lg border border-gray-200 dark:border-dark-700">
          <div class="border-b border-gray-200 px-3 py-2 text-sm font-medium text-gray-900 dark:border-dark-700 dark:text-white">
            Kiro Accounts
          </div>
          <div class="overflow-auto">
            <table class="min-w-full divide-y divide-gray-200 text-sm dark:divide-dark-700">
              <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-900 dark:text-dark-400">
                <tr>
                  <th class="px-3 py-2 text-left font-medium">Account</th>
                  <th class="px-3 py-2 text-left font-medium">Plan</th>
                  <th class="px-3 py-2 text-left font-medium">Runtime</th>
                  <th class="px-3 py-2 text-left font-medium">Token</th>
                  <th class="px-3 py-2 text-left font-medium">Usage</th>
                </tr>
              </thead>
              <tbody class="divide-y divide-gray-200 dark:divide-dark-700">
                <tr v-if="kiroRuntimeAccounts.length === 0">
                  <td class="px-3 py-4 text-center text-gray-500 dark:text-dark-400" colspan="5">
                    {{ t('admin.accounts.providerAdaptersEmpty') }}
                  </td>
                </tr>
                <tr v-for="account in kiroRuntimeAccounts" :key="account.id">
                  <td class="px-3 py-2">
                    <div class="font-medium text-gray-900 dark:text-white">{{ account.label }}</div>
                    <div class="text-xs text-gray-500 dark:text-dark-400">{{ account.region || account.engine }}</div>
                  </td>
                  <td class="px-3 py-2 text-gray-600 dark:text-dark-300">{{ account.plan_name || account.plan_tier || '-' }}</td>
                  <td class="px-3 py-2">
                    <span :class="runtimeStatusClass(account.runtime_status)">{{ account.runtime_status || '-' }}</span>
                  </td>
                  <td class="px-3 py-2 text-gray-600 dark:text-dark-300">{{ account.token_status || '-' }}</td>
                  <td class="px-3 py-2 text-gray-600 dark:text-dark-300">{{ formatUsage(account.usage_current, account.usage_limit) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>

        <div class="space-y-4">
          <div class="overflow-hidden rounded-lg border border-gray-200 dark:border-dark-700">
            <div class="border-b border-gray-200 px-3 py-2 text-sm font-medium text-gray-900 dark:border-dark-700 dark:text-white">
              Kiro Models
            </div>
            <div class="max-h-72 overflow-auto">
              <table class="min-w-full divide-y divide-gray-200 text-sm dark:divide-dark-700">
                <thead class="bg-gray-50 text-xs uppercase text-gray-500 dark:bg-dark-900 dark:text-dark-400">
                  <tr>
                    <th class="px-3 py-2 text-left font-medium">Model</th>
                    <th class="px-3 py-2 text-left font-medium">Source</th>
                    <th class="px-3 py-2 text-left font-medium">Smoke</th>
                    <th class="px-3 py-2 text-left font-medium">Public</th>
                  </tr>
                </thead>
                <tbody class="divide-y divide-gray-200 dark:divide-dark-700">
                  <tr v-if="kiroRuntimeModels.length === 0">
                    <td class="px-3 py-4 text-center text-gray-500 dark:text-dark-400" colspan="4">
                      {{ t('admin.accounts.providerAdaptersEmpty') }}
                    </td>
                  </tr>
                  <tr v-for="model in kiroRuntimeModels" :key="model.id">
                    <td class="px-3 py-2">
                      <div class="font-medium text-gray-900 dark:text-white">{{ model.display_name || model.id }}</div>
                      <div class="font-mono text-xs text-gray-500 dark:text-dark-400">{{ model.id }}</div>
                    </td>
                    <td class="px-3 py-2 text-gray-600 dark:text-dark-300">{{ model.source || '-' }}</td>
                    <td class="px-3 py-2 text-gray-600 dark:text-dark-300">{{ model.last_smoke_status || '-' }}</td>
                    <td class="px-3 py-2 text-gray-600 dark:text-dark-300">{{ model.public_enabled ? 'yes' : 'no' }}</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </div>

          <div class="rounded-lg border border-gray-200 p-3 dark:border-dark-700">
            <div class="text-sm font-medium text-gray-900 dark:text-white">Routing</div>
            <div class="mt-2 grid grid-cols-2 gap-2 text-sm text-gray-600 dark:text-dark-300">
              <div>Strategy: {{ kiroRuntimeRouting?.default_strategy || '-' }}</div>
              <div>Sticky: {{ kiroRuntimeRouting?.session_sticky ? 'on' : 'off' }}</div>
              <div>Model aware: {{ kiroRuntimeRouting?.model_aware_routing ? 'on' : 'off' }}</div>
              <div>Quota switch: {{ kiroRuntimeRouting?.auto_switch_on_quota ? 'on' : 'off' }}</div>
            </div>
            <div class="mt-3 flex flex-wrap gap-2">
              <span
                v-for="capability in kiroRuntimeRouting?.capabilities || []"
                :key="capability"
                class="rounded-full bg-gray-100 px-2 py-0.5 text-xs text-gray-600 dark:bg-dark-700 dark:text-dark-300"
              >
                {{ capability }}
              </span>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div class="rounded-xl border border-gray-200 dark:border-dark-700">
      <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-200 px-4 py-3 dark:border-dark-700">
        <div>
          <div class="text-sm font-medium text-gray-900 dark:text-white">
            {{ activeEndpoint?.label }} / {{ t('admin.accounts.providerAdaptersRaw') }}
          </div>
          <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">
            {{ activeFetchedAt }}
          </div>
        </div>
        <button class="btn btn-secondary px-3 py-1.5 text-sm" :disabled="activeEndpoint?.loading" @click="reloadActive">
          {{ t('admin.accounts.providerAdaptersRefresh') }}
        </button>
      </div>
      <pre class="max-h-[60vh] overflow-auto rounded-b-xl bg-gray-50 p-4 text-xs text-gray-800 dark:bg-dark-900 dark:text-dark-100">{{ activeRaw }}</pre>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI } from '@/api/admin'
import type {
  ProviderAdapterAdminResponse,
  KiroRuntimeAccount,
  KiroRuntimeAccountsResponse,
  KiroRuntimeModel,
  KiroRuntimeModelsResponse,
  KiroRuntimeRouting,
  KiroRuntimeStatusResponse,
  KiroRuntimeSummary
} from '@/types'

type ProviderAdapterEndpointKey =
  | 'windsurfHealth'
  | 'windsurfAccounts'
  | 'kiroCredentials'
  | 'kiroRuntimeStatus'
  | 'kiroRuntimeAccounts'
  | 'kiroRuntimeModels'
  | 'kiroRuntimeRouting'

type EndpointResult =
  | ProviderAdapterAdminResponse
  | KiroRuntimeStatusResponse
  | KiroRuntimeAccountsResponse
  | KiroRuntimeModelsResponse
  | KiroRuntimeRouting

interface Props {
  initialActiveKey?: ProviderAdapterEndpointKey
}

interface EndpointState {
  key: ProviderAdapterEndpointKey
  label: string
  path: string
  loading: boolean
  error: string
  result: EndpointResult | null
  load: () => Promise<EndpointResult>
}

const props = withDefaults(defineProps<Props>(), {
  initialActiveKey: 'windsurfAccounts'
})

const { t } = useI18n()

const emptySummary: KiroRuntimeSummary = {
  accounts_total: 0,
  accounts_available: 0,
  accounts_cooldown: 0,
  accounts_quota_exhausted: 0,
  accounts_with_profile_arn: 0,
  models_discovered: 0,
  models_smoke_passed: 0,
  models_public_enabled: 0
}

const endpoints = ref<EndpointState[]>([
  {
    key: 'windsurfHealth',
    label: 'Windsurf Health',
    path: '/health',
    loading: false,
    error: '',
    result: null,
    load: () => adminAPI.accounts.getWindsurfAdapterHealth()
  },
  {
    key: 'windsurfAccounts',
    label: 'Windsurf Accounts',
    path: '/auth/accounts',
    loading: false,
    error: '',
    result: null,
    load: () => adminAPI.accounts.getWindsurfAdapterAccounts()
  },
  {
    key: 'kiroCredentials',
    label: 'Kiro Credentials',
    path: '/api/admin/credentials',
    loading: false,
    error: '',
    result: null,
    load: () => adminAPI.accounts.getKiroAdapterCredentials()
  },
  {
    key: 'kiroRuntimeStatus',
    label: 'Kiro Runtime',
    path: '/runtime/status',
    loading: false,
    error: '',
    result: null,
    load: () => adminAPI.accounts.getKiroRuntimeStatus()
  },
  {
    key: 'kiroRuntimeAccounts',
    label: 'Kiro Runtime Accounts',
    path: '/runtime/accounts',
    loading: false,
    error: '',
    result: null,
    load: () => adminAPI.accounts.getKiroRuntimeAccounts()
  },
  {
    key: 'kiroRuntimeModels',
    label: 'Kiro Runtime Models',
    path: '/runtime/models',
    loading: false,
    error: '',
    result: null,
    load: () => adminAPI.accounts.getKiroRuntimeModels()
  },
  {
    key: 'kiroRuntimeRouting',
    label: 'Kiro Routing',
    path: '/runtime/routing',
    loading: false,
    error: '',
    result: null,
    load: () => adminAPI.accounts.getKiroRuntimeRouting()
  }
])

const activeKey = ref<ProviderAdapterEndpointKey>(props.initialActiveKey)

const activeEndpoint = computed(() => endpoints.value.find(endpoint => endpoint.key === activeKey.value) ?? endpoints.value[0])
const loadingAny = computed(() => endpoints.value.some(endpoint => endpoint.loading))

const activeFetchedAt = computed(() => {
  const result = activeEndpoint.value?.result
  return result && 'fetched_at' in result ? result.fetched_at : '-'
})

const activeRaw = computed(() => {
  const endpoint = activeEndpoint.value
  if (!endpoint) return t('admin.accounts.providerAdaptersEmpty')
  if (endpoint.loading) return t('admin.accounts.providerAdaptersLoading')
  if (endpoint.error) return endpoint.error
  if (!endpoint.result) return t('admin.accounts.providerAdaptersEmpty')
  const payload = isProviderAdapterAdminResponse(endpoint.result)
    ? endpoint.result.data ?? endpoint.result
    : endpoint.result
  try {
    return JSON.stringify(payload, null, 2)
  } catch {
    return String(payload)
  }
})

const kiroRuntimeStatus = computed(() => endpointResult<KiroRuntimeStatusResponse>('kiroRuntimeStatus'))
const kiroRuntimeAccounts = computed<KiroRuntimeAccount[]>(() => endpointResult<KiroRuntimeAccountsResponse>('kiroRuntimeAccounts')?.accounts ?? [])
const kiroRuntimeModels = computed<KiroRuntimeModel[]>(() => endpointResult<KiroRuntimeModelsResponse>('kiroRuntimeModels')?.models ?? [])
const kiroRuntimeRouting = computed<KiroRuntimeRouting | null>(() => {
  return endpointResult<KiroRuntimeRouting>('kiroRuntimeRouting') ?? kiroRuntimeStatus.value?.routing ?? null
})
const kiroRuntimeSummary = computed<KiroRuntimeSummary>(() => kiroRuntimeStatus.value?.summary ?? {
  ...emptySummary,
  accounts_total: kiroRuntimeAccounts.value.length,
  accounts_available: kiroRuntimeAccounts.value.filter(account => account.runtime_status === 'available' || account.runtime_status === 'normal').length,
  models_discovered: kiroRuntimeModels.value.length
})
const showKiroRuntimePanel = computed(() => {
  return Boolean(kiroRuntimeStatus.value || kiroRuntimeAccounts.value.length > 0 || kiroRuntimeModels.value.length > 0 || kiroRuntimeRouting.value)
})

const asRecord = (value: unknown): Record<string, any> => {
  return value && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, any> : {}
}

const asArray = (value: unknown): any[] => Array.isArray(value) ? value : []

const endpointResult = <T extends EndpointResult>(key: ProviderAdapterEndpointKey): T | null => {
  return (endpoints.value.find(endpoint => endpoint.key === key)?.result as T | null) ?? null
}

const isProviderAdapterAdminResponse = (value: EndpointResult): value is ProviderAdapterAdminResponse => {
  return 'upstream_status' in value && 'endpoint' in value
}

const endpointPath = (endpoint: EndpointState) => {
  if (endpoint.result && 'path' in endpoint.result) return endpoint.result.path
  return endpoint.path
}

const endpointBadgeText = (endpoint: EndpointState) => {
  if (endpoint.loading) return t('admin.accounts.providerAdaptersLoading')
  if (endpoint.error) return 'ERR'
  if (!endpoint.result) return '-'
  if (endpoint.key === 'kiroRuntimeStatus') return endpointResult<KiroRuntimeStatusResponse>('kiroRuntimeStatus')?.status || '-'
  if (endpoint.key === 'kiroRuntimeAccounts') return String(endpointResult<KiroRuntimeAccountsResponse>('kiroRuntimeAccounts')?.total ?? '-')
  if (endpoint.key === 'kiroRuntimeModels') return String(endpointResult<KiroRuntimeModelsResponse>('kiroRuntimeModels')?.total ?? '-')
  if (endpoint.key === 'kiroRuntimeRouting') return 'config'
  if (isProviderAdapterAdminResponse(endpoint.result)) return String(endpoint.result.upstream_status || (endpoint.result.ok ? 'OK' : '-'))
  return 'OK'
}

const endpointBadgeClass = (endpoint: EndpointState) => {
  const base = 'rounded-full px-2 py-0.5 text-xs font-medium'
  if (endpoint.loading) return `${base} bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-dark-300`
  if (endpoint.error) return `${base} bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300`
  if (!endpoint.result) return `${base} bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300`
  if (endpoint.key === 'kiroRuntimeStatus') {
    const status = endpointResult<KiroRuntimeStatusResponse>('kiroRuntimeStatus')?.status
    return runtimeStatusClass(status)
  }
  if (isProviderAdapterAdminResponse(endpoint.result) && !endpoint.result.ok) {
    return `${base} bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300`
  }
  return `${base} bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300`
}

const runtimeStatusClass = (status?: string) => {
  const base = 'rounded-full px-2 py-0.5 text-xs font-medium'
  switch ((status || '').toLowerCase()) {
    case 'online':
    case 'available':
    case 'normal':
    case 'valid':
      return `${base} bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300`
    case 'degraded':
    case 'cooldown':
    case 'expiring':
      return `${base} bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300`
    case 'offline':
    case 'expired':
    case 'quota_exhausted':
    case 'disabled':
      return `${base} bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300`
    default:
      return `${base} bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-dark-300`
  }
}

const endpointSummary = (endpoint: EndpointState) => {
  if (endpoint.loading) return t('admin.accounts.providerAdaptersLoading')
  if (endpoint.error) return endpoint.error
  if (!endpoint.result) return t('admin.accounts.providerAdaptersEmpty')

  if (endpoint.key === 'kiroRuntimeStatus') {
    const result = endpointResult<KiroRuntimeStatusResponse>('kiroRuntimeStatus')
    const summary = result?.summary ?? emptySummary
    return `${result?.status || '-'} / accounts ${summary.accounts_available}/${summary.accounts_total} / models ${summary.models_discovered}`
  }

  if (endpoint.key === 'kiroRuntimeAccounts') {
    const result = endpointResult<KiroRuntimeAccountsResponse>('kiroRuntimeAccounts')
    const accounts = result?.accounts ?? []
    const available = accounts.filter(account => account.runtime_status === 'available' || account.runtime_status === 'normal').length
    return `total ${result?.total ?? accounts.length} / available ${available}`
  }

  if (endpoint.key === 'kiroRuntimeModels') {
    const result = endpointResult<KiroRuntimeModelsResponse>('kiroRuntimeModels')
    const models = result?.models ?? []
    const publicEnabled = models.filter(model => model.public_enabled).length
    return `total ${result?.total ?? models.length} / public ${publicEnabled}`
  }

  if (endpoint.key === 'kiroRuntimeRouting') {
    const routing = endpointResult<KiroRuntimeRouting>('kiroRuntimeRouting')
    return `${routing?.default_strategy || '-'} / sticky ${routing?.session_sticky ? 'on' : 'off'} / model-aware ${routing?.model_aware_routing ? 'on' : 'off'}`
  }

  const data = isProviderAdapterAdminResponse(endpoint.result) ? asRecord(endpoint.result.data) : {}

  if (endpoint.key === 'windsurfHealth') {
    const accounts = asRecord(data.accounts)
    const total = accounts.total ?? data.total ?? '-'
    const active = accounts.active ?? accounts.available ?? '-'
    const version = data.version ? ` / v${data.version}` : ''
    return `total ${total} / active ${active}${version}`
  }

  if (endpoint.key === 'windsurfAccounts') {
    const accounts = asArray(data.accounts)
    const active = accounts.filter(account => asRecord(account).status === 'active').length
    const first = asRecord(accounts[0])
    const credits = asRecord(first.credits)
    const plan = credits.planName || first.tier || '-'
    const remaining = asRecord(credits.prompt).remaining
    return `total ${accounts.length} / active ${active} / plan ${plan}${remaining != null ? ` / prompt ${remaining}` : ''}`
  }

  const credentials = asArray(data.credentials)
  return `total ${data.total ?? credentials.length} / available ${data.available ?? '-'} / ${t('admin.accounts.providerAdaptersUnavailableCredits')}`
}

const formatUsage = (current?: number, limit?: number) => {
  if (current == null && limit == null) return '-'
  if (limit == null || limit <= 0) return String(current ?? '-')
  return `${current ?? 0} / ${limit}`
}

const loadEndpoint = async (endpoint: EndpointState) => {
  endpoint.loading = true
  endpoint.error = ''
  try {
    endpoint.result = await endpoint.load()
  } catch (error: any) {
    endpoint.result = null
    endpoint.error = error?.message || t('admin.accounts.providerAdaptersLoadFailed')
  } finally {
    endpoint.loading = false
  }
}

const loadAll = async () => {
  await Promise.all(endpoints.value.map(endpoint => loadEndpoint(endpoint)))
}

const reloadActive = async () => {
  const endpoint = activeEndpoint.value
  if (!endpoint) return
  await loadEndpoint(endpoint)
}

onMounted(() => {
  loadAll()
})
</script>
