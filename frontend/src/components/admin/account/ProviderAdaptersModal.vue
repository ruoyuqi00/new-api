<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.providerAdaptersTitle')"
    width="extra-wide"
    close-on-click-outside
    @close="emit('close')"
  >
    <ProviderAdaptersPanel />

    <template #footer>
      <div class="flex justify-end gap-3">
        <button class="btn btn-secondary" type="button" @click="emit('close')">
          {{ t('common.close') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ProviderAdaptersPanel from './ProviderAdaptersPanel.vue'

interface Props {
  show: boolean
}

interface Emits {
  (e: 'close'): void
}

defineProps<Props>()
const emit = defineEmits<Emits>()
const { t } = useI18n()
</script>
