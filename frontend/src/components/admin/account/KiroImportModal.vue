<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.kiroImportTitle')"
    width="wide"
    close-on-click-outside
    @close="handleClose"
  >
    <form id="kiro-import-form" class="space-y-4" @submit.prevent="handleImport">
      <div class="text-sm text-gray-600 dark:text-dark-300">
        {{ t('admin.accounts.kiroImportHint') }}
      </div>
      <div
        class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-xs text-amber-700 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-300"
      >
        {{ t('admin.accounts.kiroImportWarning') }}
      </div>

      <GroupSelector
        v-model="selectedGroupIds"
        :groups="groups"
        platform="anthropic"
        searchable="auto"
      />
      <div class="text-xs text-gray-500 dark:text-dark-400">
        {{ t('admin.accounts.kiroImportGroupHelp') }}
      </div>

      <div>
        <label class="input-label">{{ t('admin.accounts.kiroImportMode') }}</label>
        <div class="grid grid-cols-1 gap-2 md:grid-cols-3">
          <button
            v-for="option in modeOptions"
            :key="option.value"
            type="button"
            :class="[
              'rounded-lg border px-3 py-2 text-left text-sm transition-colors',
              mode === option.value
                ? 'border-primary-500 bg-primary-50 text-primary-700 dark:border-primary-400 dark:bg-primary-900/30 dark:text-primary-200'
                : 'border-gray-200 bg-white text-gray-700 hover:bg-gray-50 dark:border-dark-600 dark:bg-dark-800 dark:text-dark-200 dark:hover:bg-dark-700'
            ]"
            @click="mode = option.value"
          >
            <span class="block font-medium">{{ option.label }}</span>
            <span class="mt-1 block text-xs opacity-75">{{ option.description }}</span>
          </button>
        </div>
      </div>

      <div>
        <label for="kiro-import-input" class="input-label">
          {{ t('admin.accounts.kiroImportInput') }}
        </label>
        <textarea
          id="kiro-import-input"
          v-model="credentialInput"
          class="input min-h-[220px] font-mono text-sm"
          :placeholder="inputPlaceholder"
          spellcheck="false"
        />
        <div class="mt-2 text-xs text-gray-500 dark:text-dark-400">
          {{ modeHelp }}
        </div>
      </div>

      <div
        v-if="result"
        class="space-y-3 rounded-xl border border-gray-200 p-4 dark:border-dark-700"
      >
        <div class="flex flex-wrap items-center justify-between gap-3">
          <div class="text-sm font-medium text-gray-900 dark:text-white">
            {{ t('admin.accounts.kiroImportResult') }}
          </div>
          <span
            :class="[
              'rounded-full px-2.5 py-1 text-xs font-medium',
              result.failed > 0
                ? 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
                : 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300'
            ]"
          >
            {{ t('admin.accounts.kiroImportResultBadge', { succeeded: result.succeeded, failed: result.failed }) }}
          </span>
        </div>
        <div class="text-sm text-gray-700 dark:text-dark-300">
          {{ t('admin.accounts.kiroImportResultSummary', result) }}
        </div>
        <pre
          v-if="itemsPreview"
          class="max-h-56 overflow-auto rounded-lg bg-gray-50 p-3 text-xs text-gray-700 dark:bg-dark-800 dark:text-dark-200"
        >{{ itemsPreview }}</pre>
      </div>
    </form>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button class="btn btn-secondary" type="button" :disabled="importing" @click="handleClose">
          {{ t('common.cancel') }}
        </button>
        <button
          class="btn btn-primary"
          type="submit"
          form="kiro-import-form"
          :disabled="importing"
        >
          {{ importing ? t('admin.accounts.kiroImporting') : t('admin.accounts.kiroImportButton') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import GroupSelector from '@/components/common/GroupSelector.vue'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import type { AdminGroup, KiroImportResult } from '@/types'
import {
  KiroImportInputError,
  buildKiroImportPayload,
  createKiroImportIdempotencyKey,
  type KiroImportMode
} from './kiroImport'

interface Props {
  show: boolean
  groups: AdminGroup[]
}

interface Emits {
  (e: 'close'): void
  (e: 'imported'): void
}

const props = defineProps<Props>()
const emit = defineEmits<Emits>()

const { t } = useI18n()
const appStore = useAppStore()

const importing = ref(false)
const mode = ref<KiroImportMode>('refresh_token')
const credentialInput = ref('')
const selectedGroupIds = ref<number[]>([])
const result = ref<KiroImportResult | null>(null)

const modeOptions = computed(() => [
  {
    value: 'refresh_token' as const,
    label: t('admin.accounts.kiroImportModeRefreshToken'),
    description: t('admin.accounts.kiroImportModeRefreshTokenDesc')
  },
  {
    value: 'api_key' as const,
    label: t('admin.accounts.kiroImportModeApiKey'),
    description: t('admin.accounts.kiroImportModeApiKeyDesc')
  },
  {
    value: 'json' as const,
    label: t('admin.accounts.kiroImportModeJson'),
    description: t('admin.accounts.kiroImportModeJsonDesc')
  }
])

const inputPlaceholder = computed(() => {
  if (mode.value === 'api_key') return t('admin.accounts.kiroImportApiKeyPlaceholder')
  if (mode.value === 'json') return t('admin.accounts.kiroImportJsonPlaceholder')
  return t('admin.accounts.kiroImportRefreshTokenPlaceholder')
})

const modeHelp = computed(() => {
  if (mode.value === 'api_key') return t('admin.accounts.kiroImportApiKeyHelp')
  if (mode.value === 'json') return t('admin.accounts.kiroImportJsonHelp')
  return t('admin.accounts.kiroImportRefreshTokenHelp')
})

const itemsPreview = computed(() => {
  if (!result.value?.items?.length) return ''
  try {
    return JSON.stringify(result.value.items, null, 2).slice(0, 6000)
  } catch {
    return String(result.value.items).slice(0, 6000)
  }
})

watch(
  () => props.show,
  (open) => {
    if (open) {
      mode.value = 'refresh_token'
      credentialInput.value = ''
      selectedGroupIds.value = []
      result.value = null
    }
  }
)

const handleClose = () => {
  if (importing.value) return
  emit('close')
}

const inputErrorMessage = (error: KiroImportInputError) => {
  if (error.code === 'invalid_json') return t('admin.accounts.kiroImportInvalidJson')
  if (error.code === 'invalid_json_shape') return t('admin.accounts.kiroImportInvalidJsonShape')
  if (error.code === 'invalid_credential_line') return t('admin.accounts.kiroImportInvalidCredentialLine')
  return t('admin.accounts.kiroImportEmpty')
}

const handleImport = async () => {
  importing.value = true
  try {
    const payload = buildKiroImportPayload(credentialInput.value, mode.value, selectedGroupIds.value)
    const res = await adminAPI.accounts.importKiro(payload, {
      idempotencyKey: createKiroImportIdempotencyKey()
    })

    result.value = res
    const successKey = res.failed > 0 ? 'kiroImportCompletedWithErrors' : 'kiroImportSuccess'
    appStore.showSuccess(t(`admin.accounts.${successKey}`, {
      succeeded: res.succeeded,
      failed: res.failed,
      duplicate_count: res.duplicate_count
    }))
    emit('imported')
  } catch (error: any) {
    if (error instanceof KiroImportInputError) {
      appStore.showError(inputErrorMessage(error))
      return
    }
    appStore.showError(error?.message || t('admin.accounts.kiroImportFailed'))
  } finally {
    importing.value = false
  }
}
</script>
