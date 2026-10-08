<template>
  <div class="space-y-4 pb-8">
    <!-- 顶部：标题（返回由全局 Navbar 提供） -->
    <div>
      <h1 class="text-xl font-bold tracking-tight md:text-2xl">{{ t('paymentMethods.pageTitle') }}</h1>
      <p class="mt-1 text-sm text-muted-foreground">{{ t('paymentMethods.pageSubtitle') }}</p>
    </div>

    <!-- 加载骨架 -->
    <div v-if="loading && list.length === 0" class="grid gap-4 lg:grid-cols-2">
      <div v-for="i in 2" :key="i" class="h-48 animate-pulse rounded-2xl border bg-muted/50"></div>
    </div>

    <div v-else class="grid gap-4 lg:grid-cols-2">
      <!-- 区域一：数字资产 -->
      <section class="overflow-hidden rounded-2xl border bg-card shadow-sm">
        <div class="flex items-center justify-between px-5 py-4">
          <h2 class="text-sm font-semibold">{{ t('paymentMethods.digitalAssets') }}</h2>
          <span class="text-xs text-muted-foreground">USDT · TRC20</span>
        </div>

        <template v-if="usdtMethods.length">
          <div
            v-for="pm in usdtMethods"
            :key="pm.id"
            class="flex min-h-[52px] items-center gap-3 border-b px-5 py-3"
          >
            <div class="grid h-9 w-9 flex-none place-items-center rounded-xl bg-accent text-muted-foreground">
              <Coins :size="18" :stroke-width="1.8" />
            </div>
            <div class="min-w-0 flex-1">
              <div class="flex items-center gap-2">
                <span class="truncate font-mono text-sm font-medium">{{ pm.address_masked || pm.address }}</span>
                <Badge v-if="pm.is_default" size="xs" variant="success">{{ t('paymentMethods.default') }}</Badge>
              </div>
              <div v-if="pm.label" class="mt-0.5 truncate text-xs text-muted-foreground">{{ pm.label }}</div>
            </div>
            <div class="flex flex-none items-center gap-1">
              <Button
                v-if="!pm.is_default"
                size="xs"
                variant="ghost"
                @click="onSetDefault(pm)"
              >{{ t('paymentMethods.setDefault') }}</Button>
              <Button size="xs" variant="ghost" @click="onEdit(pm)">{{ t('paymentMethods.edit') }}</Button>
              <Button size="xs" variant="ghost" class="text-destructive hover:text-destructive" @click="onDelete(pm)">
                {{ t('paymentMethods.delete') }}
              </Button>
            </div>
          </div>
        </template>

        <!-- 空状态 -->
        <div v-else class="px-5 pb-6 pt-2 text-center">
          <p class="text-sm text-muted-foreground">{{ t('paymentMethods.usdtEmpty') }}</p>
          <Button size="sm" class="mt-4" @click="openCreate('USDT_TRC20')">
            <Plus :size="15" :stroke-width="1.8" />
            {{ t('paymentMethods.bindAddress') }}
          </Button>
        </div>
      </section>

      <!-- 区域二：C2C 收款方式 -->
      <section class="overflow-hidden rounded-2xl border bg-card shadow-sm">
        <div class="px-5 py-4">
          <h2 class="text-sm font-semibold">{{ t('paymentMethods.c2cMethods') }}</h2>
        </div>

        <template v-if="fiatMethods.length">
          <div
            v-for="pm in fiatMethods"
            :key="pm.id"
            class="flex min-h-[52px] items-center gap-3 border-b px-5 py-3"
          >
            <div class="grid h-9 w-9 flex-none place-items-center rounded-xl bg-accent text-muted-foreground">
              <component :is="typeIcon(pm.type)" :size="18" :stroke-width="1.8" />
            </div>
            <div class="min-w-0 flex-1">
              <div class="flex items-center gap-2">
                <span class="truncate text-sm font-medium">{{ typeLabel(pm.type) }}</span>
                <Badge v-if="pm.is_default" size="xs" variant="success">{{ t('paymentMethods.default') }}</Badge>
                <Badge v-if="pm.qr_code_url" size="xs" variant="neutral">{{ t('paymentMethods.qrBadge') }}</Badge>
              </div>
              <div class="mt-0.5 truncate text-xs text-muted-foreground">
                {{ fiatSummary(pm) }}
              </div>
            </div>
            <div class="flex flex-none items-center gap-1">
              <Button
                v-if="!pm.is_default && (pm.type === 'BANK_CARD' || pm.type === 'ALIPAY' || pm.type === 'WECHAT')"
                size="xs"
                variant="ghost"
                @click="onSetDefault(pm)"
              >{{ t('paymentMethods.setDefault') }}</Button>
              <Button size="xs" variant="ghost" @click="onEdit(pm)">{{ t('paymentMethods.edit') }}</Button>
              <Button size="xs" variant="ghost" class="text-destructive hover:text-destructive" @click="onDelete(pm)">
                {{ t('paymentMethods.delete') }}
              </Button>
            </div>
          </div>
        </template>

        <div v-else class="px-5 pb-2 pt-1 text-center">
          <p class="text-sm text-muted-foreground">{{ t('paymentMethods.fiatEmpty') }}</p>
        </div>

        <!-- 底部：添加收款方式（仅银行卡 / 支付宝 / 微信） -->
        <div class="px-5 py-4 text-center">
          <Button size="sm" @click="openCreateFiat">
            <Plus :size="15" :stroke-width="1.8" />
            {{ t('paymentMethods.addMethod') }}
          </Button>
        </div>
      </section>
    </div>

    <!-- 添加 / 编辑弹窗 -->
    <PaymentMethodForm
      v-model:open="formOpen"
      :editing-id="editingId"
      :two-f-a-enabled="twoFAEnabled"
      :preset-type="formPresetType"
      :allowed-types="formAllowedTypes"
      @saved="reload"
    />

    <!-- 列表操作的二次验证 -->
    <StepUpVerifyDialog v-model:open="verifyOpen" :two-f-a-enabled="twoFAEnabled" @verified="onVerified" />
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Coins, CreditCard, MessageCircle, Plus, Wallet } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { paymentMethodsAPI, parsePaymentMethodList } from '@/api/paymentMethods'
import PaymentMethodForm from '@/components/payment/PaymentMethodForm.vue'
import StepUpVerifyDialog from '@/components/payment/StepUpVerifyDialog.vue'
import { useConfirmDialog } from '@/composables/useConfirmDialog'
import { toast } from '@/composables/useToast'
import type { PaymentMethod, PaymentMethodType, StepUpSecurity } from '@/types/paymentMethod'

const { t } = useI18n()
const { confirm } = useConfirmDialog()

const list = ref<PaymentMethod[]>([])
const loading = ref(false)
const twoFAEnabled = ref<boolean | null>(null)

const formOpen = ref(false)
const editingId = ref<number | null>(null)
const formPresetType = ref<PaymentMethodType | ''>('')
const formAllowedTypes = ref<PaymentMethodType[] | undefined>(undefined)

const verifyOpen = ref(false)
const pendingAction = ref<{ kind: 'setDefault' | 'delete'; id: number } | null>(null)

const usdtMethods = computed(() => list.value.filter((m) => m.type === 'USDT_TRC20'))
const fiatMethods = computed(() =>
  list.value.filter((m) => m.type === 'BANK_CARD' || m.type === 'ALIPAY' || m.type === 'WECHAT'),
)

const typeIcon = (type: string) => {
  switch (type) {
    case 'BANK_CARD': return CreditCard
    case 'ALIPAY': return Wallet
    case 'WECHAT': return MessageCircle
    default: return Wallet
  }
}

const typeLabel = (type: string) => t(`paymentMethods.types.${type}` as any) || type

const fiatSummary = (pm: PaymentMethod): string => {
  if (pm.type === 'BANK_CARD') {
    const parts = [pm.bank_name, pm.bank_account_masked, pm.account_name_masked]
    return parts.filter(Boolean).join(' · ')
  }
  const parts = [pm.account_identifier_masked, pm.account_name_masked]
  return parts.filter(Boolean).join(' · ')
}

const reload = async () => {
  loading.value = true
  try {
    const res = await paymentMethodsAPI.list()
    list.value = parsePaymentMethodList(res)
  } catch (err: any) {
    toast.error(err?.message || t('paymentMethods.errors.loadFailed'))
  } finally {
    loading.value = false
  }
}

const FIAT_TYPES: PaymentMethodType[] = ['BANK_CARD', 'ALIPAY', 'WECHAT']

const openCreate = (presetType?: PaymentMethodType) => {
  editingId.value = null
  formPresetType.value = presetType ?? ''
  formAllowedTypes.value = undefined
  formOpen.value = true
}

const openCreateFiat = () => {
  editingId.value = null
  formPresetType.value = ''
  formAllowedTypes.value = FIAT_TYPES
  formOpen.value = true
}

const onEdit = (pm: PaymentMethod) => {
  editingId.value = pm.id
  formPresetType.value = ''
  formAllowedTypes.value = undefined
  formOpen.value = true
}

const onSetDefault = (pm: PaymentMethod) => {
  pendingAction.value = { kind: 'setDefault', id: pm.id }
  verifyOpen.value = true
}

const onDelete = async (pm: PaymentMethod) => {
  const ok = await confirm({
    title: t('paymentMethods.deleteTitle'),
    message: t('paymentMethods.deleteConfirm'),
    variant: 'danger',
    confirmText: t('paymentMethods.delete'),
  })
  if (!ok) return
  pendingAction.value = { kind: 'delete', id: pm.id }
  verifyOpen.value = true
}

const onVerified = async (security: StepUpSecurity) => {
  const action = pendingAction.value
  verifyOpen.value = false
  pendingAction.value = null
  if (!action) return
  try {
    if (action.kind === 'setDefault') {
      await paymentMethodsAPI.setDefault(action.id, security)
      toast.success(t('paymentMethods.setDefaultSuccess'))
    } else {
      await paymentMethodsAPI.remove(action.id, security)
      toast.success(t('paymentMethods.deleted'))
    }
    await reload()
  } catch (err: any) {
    toast.error(err?.message || t('paymentMethods.errors.operationFailed'))
  }
}

onMounted(() => {
  void reload()
})
</script>
