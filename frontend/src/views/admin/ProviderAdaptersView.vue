<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">
            {{ t('admin.accounts.providerAdaptersTitle') }}
          </h1>
          <p class="mt-1 max-w-3xl text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.accounts.providerAdaptersHint') }}
          </p>
        </div>
        <RouterLink to="/admin/accounts" class="btn btn-secondary">
          {{ t('nav.accounts') }}
        </RouterLink>
      </div>

      <ProviderAdaptersPanel :initial-active-key="initialActiveKey" />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import AppLayout from '@/components/layout/AppLayout.vue'
import ProviderAdaptersPanel from '@/components/admin/account/ProviderAdaptersPanel.vue'

type ProviderAdapterEndpointKey =
  | 'windsurfHealth'
  | 'windsurfAccounts'
  | 'kiroCredentials'
  | 'kiroRuntimeStatus'
  | 'kiroRuntimeAccounts'
  | 'kiroRuntimeModels'
  | 'kiroRuntimeRouting'

const route = useRoute()
const { t } = useI18n()

const initialActiveKey = computed<ProviderAdapterEndpointKey>(() => {
  return route.params.provider === 'kiro' ? 'kiroRuntimeStatus' : 'windsurfAccounts'
})
</script>
