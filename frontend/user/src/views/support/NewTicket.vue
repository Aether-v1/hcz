<template>
  <div class="min-h-screen bg-background pb-16 pt-20 text-foreground">
    <div class="container mx-auto max-w-2xl px-4">
      <router-link to="/support/tickets" class="mb-5 inline-flex text-sm text-muted-foreground hover:text-foreground">
        ← {{ t('support.my_tickets') }}
      </router-link>

      <h1 class="text-2xl font-bold tracking-tight">{{ t('support.create_ticket') }}</h1>
      <p class="mt-1 text-sm text-muted-foreground">{{ t('support.create_subtitle') }}</p>

      <!-- 充值类目提示：引导走订单售后 -->
      <div
        v-if="showRechargeWarning"
        class="mt-4 flex items-start gap-2 rounded-xl border border-warning/30 bg-warning/5 p-3 text-sm"
      >
        <AlertTriangle class="mt-0.5 h-4 w-4 flex-none text-warning" />
        <span>{{ t('support.recharge_aftersale_warning') }}</span>
      </div>

      <form class="mt-6 space-y-5 rounded-2xl border bg-card p-6 shadow-sm" @submit.prevent="submit">
        <!-- 分类 -->
        <div>
          <Label class="mb-2 block">{{ t('support.category') }} <span class="text-destructive">*</span></Label>
          <Select v-model="categoryId">
            <SelectTrigger class="h-11 w-full">
              <SelectValue :placeholder="t('support.category_placeholder')" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem v-for="c in categories" :key="c.id" :value="String(c.id)">
                {{ c.name }}
              </SelectItem>
            </SelectContent>
          </Select>
        </div>

        <!-- 主题 -->
        <div>
          <div class="mb-2 flex items-center justify-between">
            <Label>{{ t('support.subject') }} <span class="text-destructive">*</span></Label>
            <span class="text-xs text-muted-foreground">{{ subject.length }}/255</span>
          </div>
          <Input v-model="subject" type="text" maxlength="255" :placeholder="t('support.subject_placeholder')" class="h-11" />
        </div>

        <!-- 描述 -->
        <div>
          <div class="mb-2 flex items-center justify-between">
            <Label>{{ t('support.description') }} <span class="text-destructive">*</span></Label>
            <span class="text-xs text-muted-foreground">{{ body.length }}/10000</span>
          </div>
          <Textarea v-model="body" rows="6" maxlength="10000" :placeholder="t('support.description_placeholder')" />
        </div>

        <!-- 关联业务（可选） -->
        <div class="rounded-xl border border-border bg-muted/30 p-4">
          <div class="mb-3 text-sm font-medium">{{ t('support.biz_link') }}</div>
          <div class="grid gap-3 sm:grid-cols-2">
            <Select v-model="bizType">
              <SelectTrigger class="h-11 w-full">
                <SelectValue :placeholder="t('support.biz_type')" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="">{{ t('support.biz_type_none') }}</SelectItem>
                <SelectItem value="order">{{ t('support.biz_type_order') }}</SelectItem>
                <SelectItem value="withdraw">{{ t('support.biz_type_withdraw') }}</SelectItem>
                <SelectItem value="c2c">{{ t('support.biz_type_c2c') }}</SelectItem>
                <SelectItem value="recharge">{{ t('support.biz_type_recharge') }}</SelectItem>
              </SelectContent>
            </Select>
            <Input v-model="bizId" type="text" inputmode="numeric" :placeholder="t('support.biz_id_placeholder')" class="h-11" />
          </div>
          <p class="mt-2 text-xs text-muted-foreground">{{ t('support.biz_link_hint') }}</p>
        </div>

        <!-- 附件 -->
        <div>
          <Label class="mb-2 block">{{ t('support.attachments') }}</Label>
          <AttachmentUploader v-model="attachmentIds" />
        </div>

        <!-- 提交 -->
        <div class="flex justify-end gap-2 pt-2">
          <Button type="button" variant="outline" @click="goBack">{{ t('support.cancel') }}</Button>
          <Button type="submit" :disabled="submitting">
            <Loader2 v-if="submitting" class="h-4 w-4 animate-spin" />
            {{ t('support.submit') }}
          </Button>
        </div>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { AlertTriangle, Loader2 } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import AttachmentUploader from '../../components/support/AttachmentUploader.vue'
import { supportAPI } from '../../api/support'
import type { CreateTicketPayload, CreateTicketResult, SupportCategory } from '../../types/support'
import { toast } from '../../composables/useToast'

const { t } = useI18n()
const router = useRouter()

const categories = ref<SupportCategory[]>([])
const categoryId = ref('')
const subject = ref('')
const body = ref('')
const bizType = ref('')
const bizId = ref('')
const attachmentIds = ref<number[]>([])
const submitting = ref(false)

const showRechargeWarning = computed(() => {
  const cat = categories.value.find((c) => String(c.id) === categoryId.value)
  return cat?.code === 'recharge'
})

onMounted(async () => {
  try {
    const res = await supportAPI.categories()
    categories.value = (res.data.data || []) as SupportCategory[]
  } catch (err: any) {
    toast.error(err?.message || t('support.load_failed'))
  }
})

const goBack = () => {
  void router.push('/support/tickets')
}

const submit = async () => {
  if (submitting.value) return
  if (!categoryId.value) {
    toast.error(t('support.category_required'))
    return
  }
  if (!subject.value.trim()) {
    toast.error(t('support.subject_required'))
    return
  }
  if (!body.value.trim()) {
    toast.error(t('support.body_required'))
    return
  }
  submitting.value = true
  try {
    const payload: CreateTicketPayload = {
      category_id: Number(categoryId.value),
      subject: subject.value.trim(),
      body: body.value.trim(),
    }
    if (bizType.value && bizId.value.trim()) {
      payload.biz_type = bizType.value
      payload.biz_id = Number(bizId.value.trim())
    }
    if (attachmentIds.value.length) {
      payload.attachment_ids = attachmentIds.value
    }
    const res = await supportAPI.createTicket(payload)
    const data = res.data.data as CreateTicketResult
    toast.success(t('support.create_success'))
    void router.push(`/support/tickets/${data.id}`)
  } catch (err: any) {
    toast.error(err?.message || t('support.create_failed'))
  } finally {
    submitting.value = false
  }
}
</script>
