<template>
  <div class="border-t bg-card p-3">
    <AttachmentUploader v-model="attachmentIds" :disabled="disabled || sending" />
    <div class="mt-2 flex items-end gap-2">
      <Textarea
        v-model="body"
        :disabled="disabled"
        :placeholder="t('support.write_reply')"
        rows="3"
        class="min-h-[72px] flex-1"
        @keydown.enter.exact.prevent="handleSend"
      />
      <Button :disabled="disabled || sending || !body.trim()" @click="handleSend">
        <Loader2 v-if="sending" class="h-4 w-4 animate-spin" />
        {{ t('support.send') }}
      </Button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Loader2 } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Textarea } from '@/components/ui/textarea'
import AttachmentUploader from './AttachmentUploader.vue'

/**
 * 回复输入框：文本 + 附件，发送后清空。
 * disabled 时（工单已关闭）整框不可用。
 */
const props = defineProps<{
  disabled: boolean
  sending: boolean
}>()

const emit = defineEmits<{
  send: [body: string, attachmentIds: number[]]
}>()

const { t } = useI18n()

const body = ref('')
const attachmentIds = ref<number[]>([])

const handleSend = () => {
  const text = body.value.trim()
  if (!text || props.disabled || props.sending) return
  emit('send', text, [...attachmentIds.value])
  body.value = ''
  attachmentIds.value = []
}
</script>
