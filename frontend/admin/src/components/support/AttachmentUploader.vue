<script setup lang="ts">
import { ref } from 'vue'
import { Button } from '@/components/ui/button'
import { supportAPI } from '@/api/support'
import type { SupportAttachment } from '@/types/support'
import { notifyError } from '@/utils/notify'
import { useI18n } from 'vue-i18n'

const props = defineProps<{
  modelValue: SupportAttachment[]
  disabled?: boolean
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: SupportAttachment[]): void
}>()

const { t } = useI18n()
const inputRef = ref<HTMLInputElement | null>(null)
const uploading = ref(false)

const trigger = () => inputRef.value?.click()

const onPick = async (e: Event) => {
  const files = (e.target as HTMLInputElement).files
  if (!files || files.length === 0) return
  uploading.value = true
  try {
    for (const file of Array.from(files)) {
      const res = await supportAPI.uploadAttachment(file)
      const att = res.data?.data as SupportAttachment
      if (att) {
        emit('update:modelValue', [...props.modelValue, att])
      }
    }
  } catch {
    notifyError(t('admin.support.uploadFailed'))
  } finally {
    uploading.value = false
    if (inputRef.value) inputRef.value.value = ''
  }
}

const remove = (id: number) => {
  emit('update:modelValue', props.modelValue.filter((a) => a.id !== id))
}
</script>

<template>
  <div class="space-y-2">
    <input ref="inputRef" type="file" multiple class="hidden" :disabled="disabled || uploading" @change="onPick" />
    <Button type="button" size="sm" variant="outline" :disabled="disabled || uploading" @click="trigger">
      {{ uploading ? t('admin.support.uploading') : t('admin.support.addAttachment') }}
    </Button>
    <ul v-if="modelValue.length" class="space-y-1">
      <li
        v-for="att in modelValue"
        :key="att.id"
        class="flex items-center justify-between rounded-md border border-border bg-muted/40 px-2 py-1 text-xs"
      >
        <span class="truncate">{{ att.file_name }}</span>
        <button
          type="button"
          class="ml-2 shrink-0 text-muted-foreground hover:text-destructive"
          :disabled="disabled"
          @click="remove(att.id)"
        >
          ×
        </button>
      </li>
    </ul>
  </div>
</template>
