<script setup lang="ts">
import { ref } from 'vue'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import AttachmentUploader from './AttachmentUploader.vue'
import type { SupportAttachment } from '@/types/support'
import { useI18n } from 'vue-i18n'

const props = defineProps<{
  disabled?: boolean
  sending?: boolean
}>()

const emit = defineEmits<{
  (e: 'send', payload: { body: string; attachment_ids: number[] }): void
}>()

const { t } = useI18n()
const body = ref('')
const attachments = ref<SupportAttachment[]>([])

const send = () => {
  const text = body.value.trim()
  if (!text || props.disabled || props.sending) return
  emit('send', {
    body: text,
    attachment_ids: attachments.value.map((a) => a.id),
  })
  body.value = ''
  attachments.value = []
}
</script>

<template>
  <div class="space-y-2 border-t border-border pt-3">
    <Textarea
      v-model="body"
      :rows="3"
      :placeholder="t('admin.support.write_reply')"
      :disabled="disabled || sending"
      @keydown.enter.exact.prevent="send"
    />
    <div class="flex items-end justify-between gap-3">
      <AttachmentUploader v-model="attachments" :disabled="disabled || sending" />
      <Button size="sm" :disabled="disabled || sending || !body.trim()" @click="send">
        {{ sending ? t('admin.support.sending') : t('admin.support.reply') }}
      </Button>
    </div>
  </div>
</template>
