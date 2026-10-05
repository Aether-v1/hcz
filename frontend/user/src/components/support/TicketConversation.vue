<template>
  <div ref="scrollRef" class="min-h-0 flex-1 space-y-4 overflow-y-auto px-4 py-5">
    <template v-for="msg in messages" :key="msg.id">
      <!-- 系统消息：居中灰色文字 -->
      <div v-if="isSystem(msg)" class="text-center">
        <span class="inline-block rounded-full bg-muted px-3 py-1 text-xs text-muted-foreground">
          {{ msg.body }}
        </span>
      </div>

      <!-- 用户 / 客服消息气泡 -->
      <div v-else class="flex gap-2" :class="msg.sender_type === 'user' ? 'flex-row-reverse' : ''">
        <div
          class="flex h-8 w-8 flex-none items-center justify-center rounded-full"
          :class="msg.sender_type === 'user' ? 'bg-primary/15 text-primary' : 'bg-muted text-muted-foreground'"
        >
          <component :is="msg.sender_type === 'user' ? User : Headset" class="h-4 w-4" />
        </div>
        <div class="max-w-[78%]" :class="msg.sender_type === 'user' ? 'text-right' : 'text-left'">
          <div class="mb-1 text-[11px] text-muted-foreground">
            {{ msg.sender_type === 'user' ? t('support.you') : t('support.agent_name') }} ·
            {{ formatDateTime(msg.created_at, locale) }}
          </div>
          <div
            class="whitespace-pre-wrap break-words rounded-2xl px-3.5 py-2.5 text-left text-sm"
            :class="
              msg.sender_type === 'user'
                ? 'rounded-tr-sm bg-primary text-primary-foreground'
                : 'rounded-tl-sm border bg-card text-foreground'
            "
          >
            {{ msg.body }}
          </div>

          <!-- 该消息附带的附件卡片 -->
          <div
            v-if="(messageAttachments.get(msg.id) || []).length"
            class="mt-2 flex flex-col gap-1.5"
            :class="msg.sender_type === 'user' ? 'items-end' : 'items-start'"
          >
            <button
              v-for="att in messageAttachments.get(msg.id)"
              :key="att.id"
              type="button"
              class="flex items-center gap-2 rounded-lg border bg-card px-3 py-1.5 text-xs text-foreground hover:border-primary/50"
              @click="handleDownload(att)"
            >
              <FileText class="h-3.5 w-3.5 flex-none text-muted-foreground" />
              <span class="max-w-[180px] truncate">{{ att.file_name }}</span>
              <span class="flex-none text-muted-foreground">{{ formatSize(att.file_size) }}</span>
              <Download class="h-3.5 w-3.5 flex-none text-muted-foreground" />
            </button>
          </div>
        </div>
      </div>
    </template>

    <!-- 无法归属到具体消息的附件，统一显示在会话底部 -->
    <div v-if="leftoverAttachments.length" class="pt-2">
      <div class="mb-2 text-xs text-muted-foreground">{{ t('support.attachments') }}</div>
      <div class="flex flex-col gap-1.5">
        <button
          v-for="att in leftoverAttachments"
          :key="att.id"
          type="button"
          class="flex w-fit items-center gap-2 rounded-lg border bg-card px-3 py-1.5 text-xs text-foreground hover:border-primary/50"
          @click="handleDownload(att)"
        >
          <FileText class="h-3.5 w-3.5 text-muted-foreground" />
          <span class="max-w-[220px] truncate">{{ att.file_name }}</span>
          <span class="text-muted-foreground">{{ formatSize(att.file_size) }}</span>
          <Download class="h-3.5 w-3.5 text-muted-foreground" />
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Download, FileText, Headset, User } from 'lucide-vue-next'
import { supportAPI } from '../../api/support'
import type { SupportAttachment, SupportMessage } from '../../types/support'
import { formatDateTime } from '../../utils/datetime'
import { useAppStore } from '../../stores/app'
import { toast } from '../../composables/useToast'

/**
 * 会话时间线：用户消息右对齐、客服消息左对齐、系统消息居中；
 * 附件以文件卡片展示，点击下载；新消息自动滚动到底部。
 */
const props = defineProps<{
  messages: SupportMessage[]
  attachments: SupportAttachment[]
}>()

const { t } = useI18n()
const appStore = useAppStore()
const locale = appStore.locale

const scrollRef = ref<HTMLElement | null>(null)

const isSystem = (msg: SupportMessage) =>
  msg.sender_type === 'system' || msg.message_type === 'system'

const formatSize = (size: number) => {
  if (!size) return '-'
  if (size >= 1024 * 1024) return `${(size / 1024 / 1024).toFixed(1)} MB`
  return `${Math.max(1, Math.round(size / 1024))} KB`
}

/** 附件按 created_at 与消息时间就近匹配（±5s），其余归到底部统一展示 */
const messageAttachments = computed(() => {
  const map = new Map<number, SupportAttachment[]>()
  const used = new Set<number>()
  for (const msg of props.messages) {
    if (isSystem(msg)) continue
    const msgTime = new Date(msg.created_at).getTime()
    const related = props.attachments.filter(
      (a) => !used.has(a.id) && Math.abs(new Date(a.created_at).getTime() - msgTime) <= 5000,
    )
    if (related.length) {
      map.set(msg.id, related)
      related.forEach((a) => used.add(a.id))
    }
  }
  return map
})

const leftoverAttachments = computed(() => {
  const used = new Set<number>()
  messageAttachments.value.forEach((list) => list.forEach((a) => used.add(a.id)))
  return props.attachments.filter((a) => !used.has(a.id))
})

const scrollToBottom = (smooth = true) => {
  void nextTick(() => {
    const el = scrollRef.value
    if (el) el.scrollTo({ top: el.scrollHeight, behavior: smooth ? 'smooth' : 'auto' })
  })
}

const handleDownload = async (att: SupportAttachment) => {
  try {
    const res = await supportAPI.downloadAttachment(att.id)
    const blob = res.data as unknown as Blob
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = att.file_name
    document.body.appendChild(a)
    a.click()
    a.remove()
    URL.revokeObjectURL(url)
  } catch (err: any) {
    toast.error(err?.message || t('support.download_failed'))
  }
}

watch(
  () => props.messages.length,
  () => scrollToBottom(),
)

onMounted(() => scrollToBottom(false))
</script>
