<template>
  <div class="min-h-screen bg-background text-foreground pt-20 pb-16">
    <div class="container mx-auto px-4 max-w-3xl">
      <!-- Header -->
      <div class="mb-6 mt-8 flex flex-wrap items-end justify-between gap-4">
        <div>
          <h1 class="text-2xl md:text-3xl font-bold tracking-tight">{{ t('c2c.paymentMethod.title') }}</h1>
          <p class="mt-1 text-sm text-muted-foreground">{{ t('c2c.paymentMethod.subtitle') }}</p>
        </div>
        <Button @click="openCreate">
          <Plus class="w-4 h-4" />
          {{ t('c2c.paymentMethod.add') }}
        </Button>
      </div>

      <!-- Loading -->
      <div v-if="c2cStore.paymentMethodsLoading && list.length === 0" class="space-y-3">
        <div v-for="i in 3" :key="i" class="rounded-2xl border bg-muted/60 h-24 animate-pulse"></div>
      </div>

      <!-- Empty state -->
      <EmptyState
        v-else-if="list.length === 0"
        variant="soft"
        size="lg"
        :title="t('c2c.paymentMethod.empty')"
        :action-label="t('c2c.paymentMethod.addFirst')"
        @action="openCreate"
      />

      <!-- List -->
      <div v-else class="space-y-3">
        <div
          v-for="pm in list"
          :key="pm.id"
          class="rounded-2xl border bg-card p-4 shadow-sm"
          :class="pm.enabled ? '' : 'opacity-60'"
        >
          <div class="flex items-start justify-between gap-3">
            <div class="flex items-start gap-3 min-w-0 flex-1">
              <div class="flex h-10 w-10 flex-none items-center justify-center rounded-xl bg-primary/10 text-primary">
                <component :is="typeIcon(pm.type)" class="h-5 w-5" />
              </div>
              <div class="min-w-0 flex-1">
                <div class="flex items-center gap-2">
                  <span class="text-sm font-semibold">{{ PAYMENT_METHOD_TYPE_LABELS[pm.type] || pm.type }}</span>
                  <Badge size="xs" :variant="pm.enabled ? 'success' : 'neutral'">
                    {{ pm.enabled ? t('c2c.paymentMethod.enabled') : t('c2c.paymentMethod.disabled') }}
                  </Badge>
                </div>
                <div class="mt-1 text-sm text-foreground truncate">{{ pm.account_name || '—' }}</div>
                <div class="mt-0.5 text-xs text-muted-foreground font-mono truncate">{{ pm.account_identifier }}</div>
              </div>
            </div>
            <div class="flex flex-col items-end gap-2 flex-none">
              <Switch
                :model-value="pm.enabled"
                @update:model-value="(val: any) => onToggleEnabled(pm, Boolean(val))"
              />
              <div class="flex gap-1">
                <Button size="xs" variant="ghost" @click="openEdit(pm)">
                  {{ t('c2c.paymentMethod.edit') }}
                </Button>
                <Button
                  size="xs"
                  variant="ghost"
                  class="text-destructive hover:text-destructive"
                  @click="onDelete(pm)"
                >
                  {{ t('c2c.paymentMethod.delete') }}
                </Button>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Form Modal -->
      <Teleport to="body">
        <div v-if="formModal.visible" class="fixed inset-0 z-[120] flex items-center justify-center p-4">
          <div class="fixed inset-0 bg-black/40 backdrop-blur-sm" @click="closeForm"></div>
          <div class="relative z-10 w-full max-w-md rounded-2xl bg-card border shadow-2xl p-6 max-h-[90vh] overflow-y-auto">
            <h3 class="text-lg font-bold">{{ editingId ? t('c2c.paymentMethod.edit') : t('c2c.paymentMethod.add') }}</h3>
            <form class="mt-5 space-y-4" @submit.prevent="submitForm">
              <div>
                <Label class="mb-1.5 block">{{ t('c2c.paymentMethod.type') }}</Label>
                <Select v-model="form.type" :disabled="!!editingId">
                  <SelectTrigger class="h-10">
                    <SelectValue :placeholder="t('c2c.paymentMethod.selectType')" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem v-for="opt in typeOptions" :key="opt.value" :value="opt.value">
                      {{ opt.label }}
                    </SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div>
                <Label class="mb-1.5 block">{{ t('c2c.paymentMethod.accountName') }}</Label>
                <Input v-model="form.account_name" />
              </div>
              <div>
                <Label class="mb-1.5 block">{{ t('c2c.paymentMethod.accountIdentifier') }}</Label>
                <Input v-model="form.account_identifier" required />
              </div>
              <div>
                <Label class="mb-1.5 block">{{ t('c2c.paymentMethod.qrImage') }}</Label>
                <Input v-model="form.qr_image" placeholder="https://..." />
              </div>
              <div>
                <Label class="mb-1.5 block">{{ t('c2c.paymentMethod.instructions') }}</Label>
                <Textarea v-model="form.instructions" rows="3" />
              </div>
              <div class="flex justify-end gap-2 pt-2">
                <Button type="button" variant="secondary" @click="closeForm">{{ t('c2c.buyPanel.cancel') }}</Button>
                <Button type="submit" :disabled="submitting">
                  {{ submitting ? t('c2c.paymentMethod.saving') : t('c2c.paymentMethod.save') }}
                </Button>
              </div>
            </form>
          </div>
        </div>
      </Teleport>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import {
  Plus, CreditCard, QrCode, Wallet, Smartphone, Settings,
} from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'
import {
  Select, SelectContent, SelectItem, SelectTrigger, SelectValue,
} from '@/components/ui/select'
import EmptyState from '@/components/EmptyState.vue'
import { useC2CStore } from '@/stores/c2c'
import {
  c2cAPI,
  C2C_PAYMENT_METHOD_TYPES,
  type C2CPaymentMethod,
} from '@/api/c2c'
import { PAYMENT_METHOD_TYPE_LABELS } from '@/composables/useC2C'
import { useConfirmDialog } from '@/composables/useConfirmDialog'
import { toast } from '@/composables/useToast'

const { t } = useI18n()
const c2cStore = useC2CStore()
const { confirm } = useConfirmDialog()

const list = ref<C2CPaymentMethod[]>([])
const submitting = ref(false)
const editingId = ref<number | null>(null)

const formModal = reactive({ visible: false })
const form = reactive({
  type: 'bank',
  account_name: '',
  account_identifier: '',
  qr_image: '',
  instructions: '',
})

const typeOptions = C2C_PAYMENT_METHOD_TYPES.map((tp) => ({
  value: tp,
  label: PAYMENT_METHOD_TYPE_LABELS[tp] || tp,
}))

const typeIcon = (type: string) => {
  switch (type) {
    case 'bank': return CreditCard
    case 'alipay': return Wallet
    case 'wechat': return Smartphone
    case 'paynow': return QrCode
    default: return Settings
  }
}

const syncList = () => {
  list.value = c2cStore.paymentMethods
}

const openCreate = () => {
  editingId.value = null
  form.type = 'bank'
  form.account_name = ''
  form.account_identifier = ''
  form.qr_image = ''
  form.instructions = ''
  formModal.visible = true
}

const openEdit = (pm: C2CPaymentMethod) => {
  editingId.value = pm.id
  form.type = pm.type
  form.account_name = pm.account_name
  form.account_identifier = pm.account_identifier
  form.qr_image = pm.qr_image || ''
  form.instructions = pm.instructions || ''
  formModal.visible = true
}

const closeForm = () => {
  formModal.visible = false
  editingId.value = null
}

const submitForm = async () => {
  if (!form.account_identifier.trim()) {
    toast.error(t('c2c.paymentMethod.accountRequired'))
    return
  }
  submitting.value = true
  try {
    if (editingId.value) {
      await c2cAPI.updatePaymentMethod(editingId.value, {
        account_name: form.account_name,
        account_identifier: form.account_identifier,
        qr_image: form.qr_image,
        instructions: form.instructions,
      })
      toast.success(t('c2c.paymentMethod.updated'))
    } else {
      await c2cAPI.createPaymentMethod({
        type: form.type,
        account_name: form.account_name,
        account_identifier: form.account_identifier,
        qr_image: form.qr_image,
        instructions: form.instructions,
      })
      toast.success(t('c2c.paymentMethod.added'))
    }
    closeForm()
    await c2cStore.fetchPaymentMethods()
    syncList()
  } catch (err) {
    toast.error(err instanceof Error ? err.message : t('c2c.paymentMethod.saveFailed'))
  } finally {
    submitting.value = false
  }
}

const onToggleEnabled = async (pm: C2CPaymentMethod, enabled: boolean) => {
  // optimistic
  pm.enabled = enabled
  try {
    await c2cAPI.setPaymentMethodEnabled(pm.id, enabled)
    toast.success(enabled ? t('c2c.paymentMethod.enabledToast') : t('c2c.paymentMethod.disabledToast'))
  } catch (err) {
    pm.enabled = !enabled
    toast.error(err instanceof Error ? err.message : t('c2c.paymentMethod.operationFailed'))
  }
}

const onDelete = async (pm: C2CPaymentMethod) => {
  const ok = await confirm({
    title: t('c2c.paymentMethod.deleteTitle'),
    message: t('c2c.paymentMethod.deleteConfirm'),
    variant: 'danger',
  })
  if (!ok) return
  try {
    await c2cAPI.deletePaymentMethod(pm.id)
    toast.success(t('c2c.paymentMethod.deleted'))
    await c2cStore.fetchPaymentMethods()
    syncList()
  } catch (err) {
    toast.error(err instanceof Error ? err.message : t('c2c.paymentMethod.deleteFailed'))
  }
}

onMounted(async () => {
  await c2cStore.fetchPaymentMethods()
  syncList()
})
</script>
