<template>
  <div>
    <!-- 拖拽 / 点击上传区 -->
    <div
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
        multiple
        :accept="ACCEPT_ATTR"
        @change="onPick"
      />
      <Upload class="mx-auto h-6 w-6 text-muted-foreground" />
      <p class="mt-2 text-sm font-medium text-foreground">{{ t('support.upload') }}</p>
      <p class="mt-1 text-xs text-muted-foreground">{{ t('support.attachment_limit') }}</p>
    </div>

    <!-- 已选文件列表 -->
    <ul v-if="items.length" class="mt-3 space-y-2">
      <li
        v-for="item in items"
        :key="item.localId"
        class="flex items-center gap-2 rounded-lg border bg-card px-3 py-2"
      >
        <FileText class="h-4 w-4 flex-none text-muted-foreground" />
        <div class="min-w-0 flex-1">
          <div class="truncate text-xs text-foreground">{{ item.fileName }}</div>
          <div v-if="item.status === 'uploading'" class="mt-1.5 h-1 w-full overflow-hidden rounded bg-muted">
            <div class="h-full rounded bg-primary transition-all duration-300" :style="{ width: item.progress + '%' }" />
          </div>
          <div v-else class="mt-0.5 text-[11px] text-muted-foreground">{{ formatSize(item.fileSize) }}</div>
        </div>
        <span v-if="item.status === 'error'" class="flex-none text-[11px] text-destructive">
          {{ t('support.upload_failed') }}
        </span>
        <button
          type="button"
          class="flex-none text-muted-foreground transition-colors hover:text-destructive"
          @click="remove(item)"
        >
          <X class="h-4 w-4" />
        </button>
      </li>
    </ul>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { FileText, Upload, X } from 'lucide-vue-next'
import { supportAPI } from '../../api/support'
import type { UploadAttachmentResult } from '../../types/support'
import { toast } from '../../composables/useToast'

/**
 * 附件上传组件（可复用于创建工单 / 回复工单）。
 * - 多选、拖拽或点击上传；
 * - 上传成功立即把附件 id 写入 v-model（attachment_ids）；
 * - 允许类型：jpg/jpeg/png/webp/pdf/txt/log，单文件 ≤ 10MB。
 */

const ACCEPTED_EXT = ['.jpg', '.jpeg', '.png', '.webp', '.pdf', '.txt', '.log']
const ACCEPT_ATTR = ACCEPTED_EXT.join(',')
const MAX_SIZE = 10 * 1024 * 1024

interface UploadItem {
  localId: number
  id: number | null
  fileName: string
  fileSize: number
  progress: number
  status: 'uploading' | 'done' | 'error'
}

const props = defineProps<{
  modelValue: number[]
  disabled?: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [ids: number[]]
}>()

const { t } = useI18n()

const inputRef = ref<HTMLInputElement | null>(null)
const dragOver = ref(false)
const items = ref<UploadItem[]>([])
let localSeq = 0

const formatSize = (size: number) => {
  if (!size) return '-'
  if (size >= 1024 * 1024) return `${(size / 1024 / 1024).toFixed(1)} MB`
  return `${Math.max(1, Math.round(size / 1024))} KB`
}

const isAllowed = (file: File) => {
  const name = file.name.toLowerCase()
  return ACCEPTED_EXT.some((ext) => name.endsWith(ext))
}

const triggerPick = () => {
  if (props.disabled) return
  inputRef.value?.click()
}

const remove = (item: UploadItem) => {
  items.value = items.value.filter((i) => i.localId !== item.localId)
  if (item.id !== null) {
    emit('update:modelValue', props.modelValue.filter((id) => id !== item.id))
  }
}

const uploadOne = async (file: File) => {
  const item: UploadItem = {
    localId: ++localSeq,
    id: null,
    fileName: file.name,
    fileSize: file.size,
    progress: 0,
    status: 'uploading',
  }
  items.value.push(item)
  // fetch 无法拿到上传进度事件，这里用确定性动画模拟上传进度（完成时跳到 100%）
  const timer = window.setInterval(() => {
    if (item.progress < 90) item.progress = Math.min(90, item.progress + 15 + Math.random() * 15)
  }, 200)
  try {
    const res = await supportAPI.uploadAttachment(file)
    const data = res.data.data as UploadAttachmentResult
    item.id = data.id
    item.fileSize = data.file_size
    item.progress = 100
    item.status = 'done'
    emit('update:modelValue', [...props.modelValue, data.id])
  } catch (err: any) {
    item.status = 'error'
    toast.error(err?.message || t('support.upload_failed'))
  } finally {
    window.clearInterval(timer)
  }
}

const handleFiles = (files: FileList | File[]) => {
  if (props.disabled) return
  const list = Array.from(files || [])
  for (const file of list) {
    if (!isAllowed(file)) {
      toast.error(t('support.attachment_type_rejected'))
      continue
    }
    if (file.size > MAX_SIZE) {
      toast.error(t('support.attachment_size_rejected'))
      continue
    }
    void uploadOne(file)
  }
}

const onPick = () => {
  if (inputRef.value?.files) handleFiles(inputRef.value.files)
  if (inputRef.value) inputRef.value.value = ''
}

const onDrop = (ev: DragEvent) => {
  dragOver.value = false
  if (ev.dataTransfer?.files) handleFiles(ev.dataTransfer.files)
}
</script>
