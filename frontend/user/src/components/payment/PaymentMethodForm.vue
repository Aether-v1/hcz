<template>
  <Teleport to="body">
    <Transition name="pmf">
      <div v-if="open" class="fixed inset-0 z-[120] flex items-end justify-center sm:items-center sm:p-4">
        <div class="absolute inset-0 bg-black/50" @click="onCancel"></div>
        <div class="relative z-10 flex max-h-[92vh] w-full flex-col overflow-hidden rounded-t-3xl bg-background shadow-2xl sm:max-w-md sm:rounded-3xl">
          <!-- Header -->
          <div class="flex items-center justify-between border-b px-5 py-4">
            <div>
              <h3 class="text-base font-semibold text-foreground">{{ titleText }}</h3>
              <p v-if="!isEditing && !form.type" class="mt-0.5 text-xs text-muted-foreground">
                {{ t('paymentMethods.form.stepChooseType') }}
              </p>
            </div>
            <button type="button" class="text-muted-foreground hover:text-foreground" @click="onCancel">
              <X :size="20" :stroke-width="1.8" />
            </button>
          </div>

          <!-- Body -->
          <div class="overflow-y-auto px-5 py-5">
            <!-- 加载详情 -->
            <div v-if="loadingDetail" class="flex justify-center py-10">
              <div class="h-6 w-6 animate-spin rounded-full border-2 border-primary border-t-transparent"></div>
            </div>

            <!-- 第一步：选择类型 -->
            <div v-else-if="!form.type" class="grid grid-cols-2 gap-3">
              <button
                v-for="opt in typeOptions"
                :key="opt.value"
                type="button"
                class="flex flex-col items-start gap-3 rounded-2xl border bg-card p-4 text-left transition-colors hover:border-primary/50"
                @click="selectType(opt.value)"
              >
                <div class="grid h-10 w-10 place-items-center rounded-xl bg-accent text-muted-foreground">
                  <component :is="opt.icon" :size="20" :stroke-width="1.8" />
                </div>
                <div>
                  <div class="text-sm font-medium text-foreground">{{ opt.label }}</div>
                  <div class="mt-0.5 text-xs text-muted-foreground">{{ opt.desc }}</div>
                </div>
              </button>
            </div>

            <!-- 第二步：字段表单 -->
            <form v-else class="space-y-4" @submit.prevent="onRequestSubmit">
              <!-- 非编辑态可返回换类型 -->
              <button
                v-if="!isEditing"
                type="button"
                class="flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground"
                @click="form.type = ''"
              >
                <ArrowLeft :size="14" :stroke-width="1.8" />
                {{ t('paymentMethods.form.changeType') }}
              </button>

              <!-- USDT TRC20 -->
              <template v-if="form.type === 'USDT_TRC20'">
                <div>
                  <Label class="mb-1.5 block">{{ t('paymentMethods.fields.address') }}</Label>
                  <Input v-model.trim="form.address" class="h-11 font-mono" :placeholder="t('paymentMethods.fields.addressPlaceholder')" />
                  <p v-if="form.address" class="mt-1 text-xs" :class="addressValid ? 'text-emerald-600' : 'text-destructive'">
                    {{ addressValid ? t('paymentMethods.fields.addressValid') : t('paymentMethods.fields.addressInvalid') }}
                  </p>
                </div>
                <div>
                  <Label class="mb-1.5 block">{{ t('paymentMethods.fields.labelOptional') }}</Label>
                  <Input v-model.trim="form.label" class="h-11" :placeholder="t('paymentMethods.fields.labelPlaceholder')" />
                </div>
              </template>

              <!-- 银行卡 -->
              <template v-else-if="form.type === 'BANK_CARD'">
                <div>
                  <Label class="mb-1.5 block">{{ t('paymentMethods.fields.accountName') }}</Label>
                  <Input v-model.trim="form.account_name" class="h-11" :placeholder="t('paymentMethods.fields.accountNamePlaceholder')" />
                </div>
                <div>
                  <Label class="mb-1.5 block">{{ t('paymentMethods.fields.bankName') }}</Label>
                  <Input v-model.trim="form.bank_name" class="h-11" :placeholder="t('paymentMethods.fields.bankNamePlaceholder')" />
                </div>
                <div>
                  <Label class="mb-1.5 block">{{ t('paymentMethods.fields.bankAccount') }}</Label>
                  <Input v-model.trim="form.bank_account" inputmode="numeric" class="h-11 font-mono" :placeholder="t('paymentMethods.fields.bankAccountPlaceholder')" />
                </div>
                <div>
                  <Label class="mb-1.5 block">{{ t('paymentMethods.fields.branchOptional') }}</Label>
                  <Input v-model.trim="form.branch_name" class="h-11" :placeholder="t('paymentMethods.fields.branchPlaceholder')" />
                </div>
              </template>

              <!-- 支付宝 / 微信 -->
              <template v-else>
                <div>
                  <Label class="mb-1.5 block">{{ t('paymentMethods.fields.accountName') }}</Label>
                  <Input v-model.trim="form.account_name" class="h-11" :placeholder="t('paymentMethods.fields.accountNamePlaceholder')" />
                </div>
                <div>
                  <Label class="mb-1.5 block">
                    {{ form.type === 'ALIPAY' ? t('paymentMethods.fields.alipayAccount') : t('paymentMethods.fields.wechatAccount') }}
                  </Label>
                  <Input v-model.trim="form.account_identifier" class="h-11" :placeholder="t('paymentMethods.fields.accountIdentifierPlaceholder')" />
                </div>
                <div>
                  <Label class="mb-1.5 block">{{ t('paymentMethods.fields.qrCodeOptional') }}</Label>
                  <QRCodeUploader v-model="qr" />
                </div>
              </template>

              <p v-if="formError" class="text-xs text-destructive">{{ formError }}</p>
            </form>
          </div>

          <!-- Footer -->
          <div v-if="form.type" class="flex gap-3 border-t px-5 py-4">
            <Button type="button" variant="outline" class="flex-1" :disabled="submitting" @click="onCancel">
              {{ t('paymentMethods.form.cancel') }}
            </Button>
            <Button type="button" class="flex-1" :disabled="submitting" @click="onRequestSubmit">
              {{ submitting ? t('paymentMethods.form.submitting') : t('paymentMethods.form.submit') }}
            </Button>
          </div>
        </div>
      </div>
    </Transition>

    <!-- 二次验证 -->
    <StepUpVerifyDialog
      v-model:open="verifyOpen"
      :two-f-a-enabled="twoFAEnabled"
      :submitting="submitting"
      @verified="onVerified"
    />
  </Teleport>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ArrowLeft, Coins, CreditCard, MessageCircle, Wallet, X } from 'lucide-vue-next'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { paymentMethodsAPI } from '@/api/paymentMethods'
import StepUpVerifyDialog from './StepUpVerifyDialog.vue'
import QRCodeUploader, { type QRCodeValue } from './QRCodeUploader.vue'
import type {
    CreatePaymentMethodRequest,
    PaymentMethod,
    PaymentMethodType,
    StepUpSecurity,
} from '@/types/paymentMethod'
import { toast } from '@/composables/useToast'

const props = defineProps<{
  open: boolean
  editingId: number | null
  twoFAEnabled?: boolean | null
}>()

const emit = defineEmits<{
  'update:open': [value: boolean]
  saved: []
}>()

const { t } = useI18n()

const isEditing = computed(() => props.editingId !== null)

const form = reactive({
  type: '' as '' | PaymentMethodType,
  label: '',
  address: '',
  account_name: '',
  bank_name: '',
  bank_account: '',
  branch_name: '',
  account_identifier: '',
})
const qr = ref<QRCodeValue | null>(null)
const loadingDetail = ref(false)
const submitting = ref(false)
const verifyOpen = ref(false)
const formError = ref('')

const typeOptions = [
  { value: 'USDT_TRC20' as PaymentMethodType, icon: Coins, label: t('paymentMethods.types.USDT_TRC20'), desc: t('paymentMethods.typesDesc.USDT_TRC20') },
  { value: 'BANK_CARD' as PaymentMethodType, icon: CreditCard, label: t('paymentMethods.types.BANK_CARD'), desc: t('paymentMethods.typesDesc.BANK_CARD') },
  { value: 'ALIPAY' as PaymentMethodType, icon: Wallet, label: t('paymentMethods.types.ALIPAY'), desc: t('paymentMethods.typesDesc.ALIPAY') },
  { value: 'WECHAT' as PaymentMethodType, icon: MessageCircle, label: t('paymentMethods.types.WECHAT'), desc: t('paymentMethods.typesDesc.WECHAT') },
]

const titleText = computed(() => {
  if (isEditing.value) return t('paymentMethods.form.editTitle')
  return t('paymentMethods.form.addTitle')
})

// TRON 地址：T 开头，base58，长度约 34
const addressValid = computed(() => /^T[A-HJ-NP-Za-km-z1-9]{25,44}$/.test(form.address.trim()))

const resetForm = () => {
  form.type = ''
  form.label = ''
  form.address = ''
  form.account_name = ''
  form.bank_name = ''
  form.bank_account = ''
  form.branch_name = ''
  form.account_identifier = ''
  qr.value = null
  formError.value = ''
}

const selectType = (type: PaymentMethodType) => {
  form.type = type
  formError.value = ''
}

const loadDetail = async (id: number) => {
  loadingDetail.value = true
  try {
    const res = await paymentMethodsAPI.get(id)
    const pm = (res?.data?.data || {}) as PaymentMethod
    form.type = (pm.type as PaymentMethodType) || ''
    form.label = pm.label || ''
    form.address = pm.address || ''
    form.account_name = pm.account_name || ''
    form.bank_name = pm.bank_name || ''
    form.bank_account = pm.bank_account || ''
    form.branch_name = pm.branch_name || ''
    form.account_identifier = pm.account_identifier || ''
    if (pm.qr_code_url) {
      qr.value = { file_id: '', url: pm.qr_code_url }
    }
  } catch (err: any) {
    toast.error(err?.message || t('paymentMethods.form.loadFailed'))
    onCancel()
  } finally {
    loadingDetail.value = false
  }
}

watch(
  () => props.open,
  (visible) => {
    if (visible) {
      resetForm()
      if (props.editingId) {
        void loadDetail(props.editingId)
      }
    }
  },
  { immediate: true },
)

const validate = (): boolean => {
  formError.value = ''
  switch (form.type) {
    case 'USDT_TRC20':
      if (!addressValid.value) {
        formError.value = t('paymentMethods.errors.addressInvalid')
        return false
      }
      return true
    case 'BANK_CARD':
      if (!form.account_name) { formError.value = t('paymentMethods.errors.accountNameRequired'); return false }
      if (!form.bank_name) { formError.value = t('paymentMethods.errors.bankNameRequired'); return false }
      if (!/^\d{8,}$/.test(form.bank_account.replace(/\s/g, ''))) { formError.value = t('paymentMethods.errors.bankAccountInvalid'); return false }
      return true
    case 'ALIPAY':
    case 'WECHAT':
      if (!form.account_name) { formError.value = t('paymentMethods.errors.accountNameRequired'); return false }
      if (!form.account_identifier) { formError.value = t('paymentMethods.errors.accountIdentifierRequired'); return false }
      return true
    default:
      return false
  }
}

const buildPayload = (security: StepUpSecurity): CreatePaymentMethodRequest => {
  const payload: CreatePaymentMethodRequest = {
    type: form.type as PaymentMethodType,
    totp_code: security.totp_code,
    password: security.password,
  }
  if (form.type === 'USDT_TRC20') {
    payload.currency = 'USDT'
    payload.network = 'TRC20'
    payload.address = form.address.trim()
    payload.label = form.label.trim()
  } else if (form.type === 'BANK_CARD') {
    payload.account_name = form.account_name.trim()
    payload.bank_name = form.bank_name.trim()
    payload.bank_account = form.bank_account.replace(/\s/g, '')
    payload.branch_name = form.branch_name.trim() || undefined
  } else {
    payload.account_name = form.account_name.trim()
    payload.account_identifier = form.account_identifier.trim()
    if (qr.value?.file_id) payload.qr_code_file_id = qr.value.file_id
    if (qr.value?.url) payload.qr_code_url = qr.value.url
  }
  return payload
}

const onRequestSubmit = () => {
  if (!validate()) return
  verifyOpen.value = true
}

const onVerified = async (security: StepUpSecurity) => {
  verifyOpen.value = false
  submitting.value = true
  try {
    const payload = buildPayload(security)
    if (isEditing.value && props.editingId) {
      const { type: _type, ...rest } = payload
      await paymentMethodsAPI.update(props.editingId, rest)
    } else {
      await paymentMethodsAPI.create(payload)
    }
    toast.success(t('paymentMethods.form.saved'))
    emit('saved')
    emit('update:open', false)
  } catch (err: any) {
    toast.error(err?.message || t('paymentMethods.form.saveFailed'))
  } finally {
    submitting.value = false
  }
}

const onCancel = () => {
  emit('update:open', false)
}
</script>

<style>
.pmf-enter-active,
.pmf-leave-active {
  transition: opacity 0.25s ease;
}
.pmf-enter-from,
.pmf-leave-to {
  opacity: 0;
}
</style>
