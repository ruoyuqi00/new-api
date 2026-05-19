<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.windsurfImportTitle')"
    width="wide"
    close-on-click-outside
    @close="handleClose"
  >
    <form id="windsurf-import-form" class="space-y-4" @submit.prevent="handleImport">
      <div class="text-sm text-gray-600 dark:text-dark-300">
        {{ t('admin.accounts.windsurfImportHint') }}
      </div>
      <div
        class="rounded-lg border border-amber-200 bg-amber-50 p-3 text-xs text-amber-700 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-300"
      >
        {{ t('admin.accounts.windsurfImportWarning') }}
      </div>

      <div>
        <label class="input-label">{{ t('admin.accounts.windsurfImportMode') }}</label>
        <div class="grid grid-cols-2 gap-2 md:grid-cols-4">
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
        <label for="windsurf-import-input" class="input-label">
          {{ t('admin.accounts.windsurfImportInput') }}
        </label>
        <textarea
          id="windsurf-import-input"
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
            {{ t('admin.accounts.windsurfImportResult') }}
          </div>
          <span
            class="rounded-full bg-emerald-100 px-2.5 py-1 text-xs font-medium text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300"
          >
            HTTP {{ result.upstream_status }}
          </span>
        </div>
        <div class="text-sm text-gray-700 dark:text-dark-300">
          {{ t('admin.accounts.windsurfImportResultSummary', result) }}
        </div>
        <pre
          v-if="upstreamPreview"
          class="max-h-56 overflow-auto rounded-lg bg-gray-50 p-3 text-xs text-gray-700 dark:bg-dark-800 dark:text-dark-200"
        >{{ upstreamPreview }}</pre>
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
          form="windsurf-import-form"
          :disabled="importing"
        >
          {{ importing ? t('admin.accounts.windsurfImporting') : t('admin.accounts.windsurfImportButton') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import type { WindsurfImportResult } from '@/types'
import {
  WindsurfImportInputError,
  buildWindsurfImportPayload,
  createWindsurfImportIdempotencyKey,
  type WindsurfImportMode
} from './windsurfImport'

interface Props {
  show: boolean
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
const mode = ref<WindsurfImportMode>('token')
const credentialInput = ref('')
const result = ref<WindsurfImportResult | null>(null)

const modeOptions = computed(() => [
  {
    value: 'token' as const,
    label: t('admin.accounts.windsurfImportModeToken'),
    description: t('admin.accounts.windsurfImportModeTokenDesc')
  },
  {
    value: 'api_key' as const,
    label: t('admin.accounts.windsurfImportModeApiKey'),
    description: t('admin.accounts.windsurfImportModeApiKeyDesc')
  },
  {
    value: 'email_password' as const,
    label: t('admin.accounts.windsurfImportModeEmailPassword'),
    description: t('admin.accounts.windsurfImportModeEmailPasswordDesc')
  },
  {
    value: 'json' as const,
    label: t('admin.accounts.windsurfImportModeJson'),
    description: t('admin.accounts.windsurfImportModeJsonDesc')
  }
])

const inputPlaceholder = computed(() => {
  if (mode.value === 'api_key') return t('admin.accounts.windsurfImportApiKeyPlaceholder')
  if (mode.value === 'email_password') return t('admin.accounts.windsurfImportEmailPasswordPlaceholder')
  if (mode.value === 'json') return t('admin.accounts.windsurfImportJsonPlaceholder')
  return t('admin.accounts.windsurfImportTokenPlaceholder')
})

const modeHelp = computed(() => {
  if (mode.value === 'api_key') return t('admin.accounts.windsurfImportApiKeyHelp')
  if (mode.value === 'email_password') return t('admin.accounts.windsurfImportEmailPasswordHelp')
  if (mode.value === 'json') return t('admin.accounts.windsurfImportJsonHelp')
  return t('admin.accounts.windsurfImportTokenHelp')
})

const upstreamPreview = computed(() => {
  if (!result.value?.upstream) return ''
  try {
    return JSON.stringify(result.value.upstream, null, 2).slice(0, 6000)
  } catch {
    return String(result.value.upstream).slice(0, 6000)
  }
})

watch(
  () => props.show,
  (open) => {
    if (open) {
      mode.value = 'token'
      credentialInput.value = ''
      result.value = null
    }
  }
)

const handleClose = () => {
  if (importing.value) return
  emit('close')
}

const inputErrorMessage = (error: WindsurfImportInputError) => {
  if (error.code === 'invalid_json') return t('admin.accounts.windsurfImportInvalidJson')
  if (error.code === 'invalid_json_shape') return t('admin.accounts.windsurfImportInvalidJsonShape')
  if (error.code === 'invalid_email_password_line') return t('admin.accounts.windsurfImportInvalidEmailPassword')
  return t('admin.accounts.windsurfImportEmpty')
}

const handleImport = async () => {
  importing.value = true
  try {
    const payload = buildWindsurfImportPayload(credentialInput.value, mode.value)
    const res = await adminAPI.accounts.importWindsurf(payload, {
      idempotencyKey: createWindsurfImportIdempotencyKey()
    })

    result.value = res
    appStore.showSuccess(t('admin.accounts.windsurfImportSuccess', {
      forwarded: res.forwarded,
      duplicate_count: res.duplicate_count
    }))
    emit('imported')
  } catch (error: any) {
    if (error instanceof WindsurfImportInputError) {
      appStore.showError(inputErrorMessage(error))
      return
    }
    appStore.showError(error?.message || t('admin.accounts.windsurfImportFailed'))
  } finally {
    importing.value = false
  }
}
</script>
