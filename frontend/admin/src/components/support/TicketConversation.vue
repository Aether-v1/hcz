<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { SupportAttachment, SupportMessage } from '@/types/support'
import { supportAPI } from '@/api/support'
import { formatDate } from '@/utils/format'
import { notifyError } from '@/utils/notify'

const props = defineProps<{
  messages: SupportMessage[]
  attachments: SupportAttachment[]
}>()

const { t } = useI18n()

const attachmentMap = computed(() => {
  const map = new Map<number, SupportAttachment>()
  props.attachments.forEach((a) => map.set(a.id, a))
  return map
})

const sideOf = (m: SupportMessage): 'user' | 'admin' | 'system' => {
  if (m.sender_type === 'system') return 'system'
  if (m.sender_type === 'user') return 'user'
  return 'admin'
}

const download = async (att: SupportAttachment) => {
  try {
    const res = await supportAPI.downloadAttachment(att.id)
    const blob = res.data as Blob
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = att.file_name || `attachment-${att.id}`
    document.body.appendChild(a)
    a.click()
    document.body.removeChild(a)
    URL.revokeObjectURL(url)
  } catch {
    notifyError(t('admin.support.downloadFailed'))
  }
}
</script>

<template>
  <div class="space-y-4">
    <div v-if="messages.length === 0" class="py-10 text-center text-sm text-muted-foreground">
      {{ t('admin.support.no_tickets') }}
    </div>

    <template v-for="m in messages" :key="m.id">
      <!-- System message: centered -->
      <div v-if="sideOf(m) === 'system'" class="flex justify-center">
        <span class="rounded-full bg-muted px-3 py-1 text-xs text-muted-foreground">{{ m.body }}</span>
      </div>

      <!-- User message: left -->
      <div v-else-if="sideOf(m) === 'user'" class="flex justify-start">
        <div class="max-w-[75%]">
          <div class="mb-1 text-xs text-muted-foreground">{{ m.sender_name || t('admin.support.user') }}</div>
          <div class="rounded-2xl rounded-bl-sm border border-border bg-card px-4 py-2 text-sm whitespace-pre-wrap break-words">
            {{ m.body }}
          </div>
          <div v-if="m.attachment_ids?.length" class="mt-1 flex flex-wrap gap-1">
            <button
              v-for="aid in m.attachment_ids"
              :key="aid"
              type="button"
              class="rounded-md border border-border bg-muted/40 px-2 py-1 text-xs hover:bg-muted"
              @click="download(attachmentMap.get(aid)!)"
            >
              📎 {{ attachmentMap.get(aid)?.file_name || `#${aid}` }}
            </button>
          </div>
          <div class="mt-1 text-[11px] text-muted-foreground">{{ formatDate(m.created_at) }}</div>
        </div>
      </div>

      <!-- Admin message: right -->
      <div v-else class="flex justify-end">
        <div class="max-w-[75%]">
          <div class="mb-1 text-right text-xs text-muted-foreground">{{ m.sender_name || t('admin.support.reply') }}</div>
          <div class="rounded-2xl rounded-br-sm border border-primary/30 bg-primary/5 px-4 py-2 text-sm whitespace-pre-wrap break-words">
            {{ m.body }}
          </div>
          <div v-if="m.attachment_ids?.length" class="mt-1 flex flex-wrap justify-end gap-1">
            <button
              v-for="aid in m.attachment_ids"
              :key="aid"
              type="button"
              class="rounded-md border border-border bg-muted/40 px-2 py-1 text-xs hover:bg-muted"
              @click="download(attachmentMap.get(aid)!)"
            >
              📎 {{ attachmentMap.get(aid)?.file_name || `#${aid}` }}
            </button>
          </div>
          <div class="mt-1 text-right text-[11px] text-muted-foreground">{{ formatDate(m.created_at) }}</div>
        </div>
      </div>
    </template>
  </div>
</template>
