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

    <div class="grid grid-cols-1 gap-3 md:grid-cols-3">
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
          <span
            :class="[
              'rounded-full px-2 py-0.5 text-xs font-medium',
              endpoint.loading
                ? 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-dark-300'
                : endpoint.result?.ok
                  ? 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
                  : 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
            ]"
          >
            {{ endpoint.loading ? t('admin.accounts.providerAdaptersLoading') : endpoint.result?.upstream_status || '-' }}
          </span>
        </div>
        <div class="mt-3 text-sm text-gray-600 dark:text-dark-300">
          {{ endpointSummary(endpoint) }}
        </div>
        <div class="mt-2 truncate font-mono text-xs text-gray-400 dark:text-dark-500">
          {{ endpoint.result?.path || endpoint.path }}
        </div>
      </button>
    </div>

    <div class="rounded-xl border border-gray-200 dark:border-dark-700">
      <div class="flex flex-wrap items-center justify-between gap-3 border-b border-gray-200 px-4 py-3 dark:border-dark-700">
        <div>
          <div class="text-sm font-medium text-gray-900 dark:text-white">
            {{ activeEndpoint?.label }} / {{ t('admin.accounts.providerAdaptersRaw') }}
          </div>
          <div class="mt-1 text-xs text-gray-500 dark:text-dark-400">
            {{ activeEndpoint?.result?.fetched_at || '-' }}
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
import type { ProviderAdapterAdminResponse } from '@/types'

type ProviderAdapterEndpointKey = 'windsurfHealth' | 'windsurfAccounts' | 'kiroCredentials'

interface Props {
  initialActiveKey?: ProviderAdapterEndpointKey
}

interface EndpointState {
  key: ProviderAdapterEndpointKey
  label: string
  path: string
  loading: boolean
  error: string
  result: ProviderAdapterAdminResponse | null
  load: () => Promise<ProviderAdapterAdminResponse>
}

const props = withDefaults(defineProps<Props>(), {
  initialActiveKey: 'windsurfAccounts'
})

const { t } = useI18n()

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
  }
])

const activeKey = ref<ProviderAdapterEndpointKey>(props.initialActiveKey)

const activeEndpoint = computed(() => endpoints.value.find(endpoint => endpoint.key === activeKey.value) ?? endpoints.value[0])
const loadingAny = computed(() => endpoints.value.some(endpoint => endpoint.loading))

const activeRaw = computed(() => {
  const endpoint = activeEndpoint.value
  if (!endpoint) return t('admin.accounts.providerAdaptersEmpty')
  if (endpoint.loading) return t('admin.accounts.providerAdaptersLoading')
  if (endpoint.error) return endpoint.error
  const payload = endpoint.result?.data ?? endpoint.result
  if (!payload) return t('admin.accounts.providerAdaptersEmpty')
  try {
    return JSON.stringify(payload, null, 2)
  } catch {
    return String(payload)
  }
})

const asRecord = (value: unknown): Record<string, any> => {
  return value && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, any> : {}
}

const asArray = (value: unknown): any[] => Array.isArray(value) ? value : []

const endpointSummary = (endpoint: EndpointState) => {
  if (endpoint.loading) return t('admin.accounts.providerAdaptersLoading')
  if (endpoint.error) return endpoint.error
  if (!endpoint.result) return t('admin.accounts.providerAdaptersEmpty')
  const data = asRecord(endpoint.result.data)

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
