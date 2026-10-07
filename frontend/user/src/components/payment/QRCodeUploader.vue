<template>
  <div>
    <!-- 已上传：预览 + 删除/重传 -->
    <div v-if="modelValue?.url" class="flex items-center gap-3">
      <div class="relative h-20 w-20 overflow-hidden rounded-xl border bg-background">
        <img :src="resolvedPreviewUrl" alt="QR" class="h-full w-full object-contain" />
      </div>
      <div class="flex flex-col gap-1.5">
        <button
          type="button"
          class="text-sm font-medium text-primary hover:underline"
          @click="triggerPick"
        >
          {{ t('paymentMethods.qr.reupload') }}
        </button>
        <button
          type="button"
          class="text-sm font-medium text-destructive hover:underline"
          @click="onRemove"
        >
          {{ t('paymentMethods.qr.remove') }}
        </button>
      </div>
    </div>

    <!-- 上传区 -->
    <div
      v-else
      class="cursor-pointer rounded-xl border border-dashed p-5 text-center transition-colors"
      :class="dragOver ? 'border-primary bg-primary/5' : 'border-border hover:border-primary/50'"
      @click="triggerPick"
      @dragover.prevent="dragOver = true"
      @dragleave="dragOver = false"
      @drop.prevent="onDrop"
    >
      <input
        ref="inputRef"
        type="file"
        class="hidden"
        accept="image/*"
        @change="onPick"
      />
      <div v-if="uploading" class="flex flex-col items-center gap-2">
        <div class="h-6 w-6 animate-spin rounded-full border-2 border-primary border-t-transparent"></div>
        <p class="text-xs text-muted-foreground">{{ t('paymentMethods.qr.uploading') }}</p>
      </div>
      <template v-else>
        <QrCode class="mx-auto h-6 w-6 text-muted-foreground" :stroke-width="1.6" />
        <p class="mt-2 text-sm font-medium text-foreground">{{ t('paymentMethods.qr.upload') }}</p>
        <p class="mt-1 text-xs text-muted-foreground">{{ t('paymentMethods.qr.limit') }}</p>
      </template>
    </div>

    <p v-if="error" class="mt-1.5 text-xs text-destructive">{{ error }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { QrCode } from 'lucide-vue-next'
import { paymentMethodsAPI } from '@/api/paymentMethods'
import { toast } from '@/composables/useToast'

export interface QRCodeValue {
  file_id: string
  url: string
}

const props = defineProps<{
  modelValue: QRCodeValue | null
  disabled?: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: QRCodeValue | null]
}>()

const { t } = useI18n()

const inputRef = ref<HTMLInputElement | null>(null)
const dragOver = ref(false)
const uploading = ref(false)
const error = ref('')

const ACCEPTED = ['image/jpeg', 'image/png', 'image/webp']
const MAX_SIZE = 5 * 1024 * 1024 // 5MB

const resolvedPreviewUrl = computed(() => {
  const url = props.modelValue?.url || ''
  if (!url) return ''
  if (/^https?:\/\//i.test(url) || url.startsWith('data:')) return url
  // 相对路径补 API 基址
  return `${import.meta.env.VITE_API_BASE_URL || ''}${url}`
})

const triggerPick = () => {
  if (props.disabled || uploading.value) return
  inputRef.value?.click()
}

const onRemove = () => {
  emit('update:modelValue', null)
}

const isAllowed = (file: File) => ACCEPTED.includes(file.type)

const uploadOne = async (file: File) => {
  error.value = ''
  if (!isAllowed(file)) {
    error.value = t('paymentMethods.qr.typeRejected')
    return
  }
  if (file.size > MAX_SIZE) {
    error.value = t('paymentMethods.qr.sizeRejected')
    return
  }
  uploading.value = true
  try {
    const res = await paymentMethodsAPI.uploadQRCode(file)
    const data = res?.data?.data || {}
    const fileId = String(data.file_id || data.id || '')
    const url = String(data.url || '')
    if (!fileId || !url) {
      error.value = t('paymentMethods.qr.uploadFailed')
      return
    }
    emit('update:modelValue', { file_id: fileId, url })
  } catch (err: any) {
    error.value = err?.message || t('paymentMethods.qr.uploadFailed')
    toast.error(error.value)
  } finally {
    uploading.value = false
  }
}

const onPick = () => {
  const file = inputRef.value?.files?.[0]
  if (file) void uploadOne(file)
  if (inputRef.value) inputRef.value.value = ''
}

const onDrop = (ev: DragEvent) => {
  dragOver.value = false
  if (props.disabled) return
  const file = ev.dataTransfer?.files?.[0]
  if (file) void uploadOne(file)
}
</script>
