<template>
  <Teleport to="body">
    <Transition name="stepup">
      <div v-if="open" class="fixed inset-0 z-[130] flex items-center justify-center p-4">
        <div class="absolute inset-0 bg-black/50" @click="onCancel"></div>
        <div class="relative z-10 w-full max-w-sm rounded-2xl border bg-card p-6 shadow-2xl">
          <h3 class="text-base font-semibold text-foreground">{{ t('paymentMethods.verify.title') }}</h3>
          <p class="mt-1.5 text-xs leading-relaxed text-muted-foreground">
            {{ t('paymentMethods.verify.subtitle') }}
          </p>

          <div v-if="statusLoading" class="mt-6 flex justify-center py-6">
            <div class="h-6 w-6 animate-spin rounded-full border-2 border-primary border-t-transparent"></div>
          </div>

          <form v-else class="mt-5 space-y-4" @submit.prevent="onSubmit">
            <!-- 2FA 开启：TOTP 6 位 -->
            <div v-if="twoFAEnabled">
              <Label class="mb-1.5 block">{{ t('paymentMethods.verify.totpLabel') }}</Label>
              <Input
                v-model="totpCode"
                type="text"
                inputmode="numeric"
                autocomplete="one-time-code"
                maxlength="6"
                class="h-11 text-center text-lg tracking-[0.5em] font-mono"
                :placeholder="t('paymentMethods.verify.totpPlaceholder')"
                autofocus
              />
            </div>
            <!-- 2FA 关闭：登录密码 -->
            <div v-else>
              <Label class="mb-1.5 block">{{ t('paymentMethods.verify.passwordLabel') }}</Label>
              <Input
                v-model="password"
                type="password"
                autocomplete="current-password"
                class="h-11"
                :placeholder="t('paymentMethods.verify.passwordPlaceholder')"
                autofocus
              />
            </div>

            <p v-if="error" class="text-xs text-destructive">{{ error }}</p>

            <div class="flex gap-3 pt-1">
              <Button type="button" variant="outline" class="flex-1" :disabled="submitting" @click="onCancel">
                {{ t('paymentMethods.verify.cancel') }}
              </Button>
              <Button type="submit" class="flex-1" :disabled="submitting || !canSubmit">
                {{ submitting ? t('paymentMethods.verify.submitting') : t('paymentMethods.verify.confirm') }}
              </Button>
            </div>
          </form>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { userTotpAPI } from '@/api/auth'
import type { StepUpSecurity } from '@/types/paymentMethod'

const props = withDefaults(
  defineProps<{
    open: boolean
    /** 外部已知的 2FA 状态；不传则自动调用接口获取 */
    twoFAEnabled?: boolean | null
    submitting?: boolean
  }>(),
  { twoFAEnabled: null, submitting: false },
)

const emit = defineEmits<{
  'update:open': [value: boolean]
  verified: [value: StepUpSecurity]
  cancel: []
}>()

const { t } = useI18n()

const totpCode = ref('')
const password = ref('')
const statusLoading = ref(false)
const resolvedTwoFA = ref<boolean | null>(null)
const error = ref('')

const twoFAEnabled = computed(() => {
  if (props.twoFAEnabled !== null) return props.twoFAEnabled
  return resolvedTwoFA.value
})

const canSubmit = computed(() => {
  if (twoFAEnabled.value) return /^\d{6}$/.test(totpCode.value.trim())
  return password.value.length > 0
})

const reset = () => {
  totpCode.value = ''
  password.value = ''
  error.value = ''
}

const loadTwoFAStatus = async () => {
  if (props.twoFAEnabled !== null) {
    resolvedTwoFA.value = props.twoFAEnabled
    return
  }
  statusLoading.value = true
  try {
    const res = await userTotpAPI.status()
    resolvedTwoFA.value = Boolean(res?.data?.data?.enabled)
  } catch {
    // 获取失败时保守降级为密码验证
    resolvedTwoFA.value = false
  } finally {
    statusLoading.value = false
  }
}

watch(
  () => props.open,
  (visible) => {
    if (visible) {
      reset()
      void loadTwoFAStatus()
    }
  },
  { immediate: true },
)

const onSubmit = () => {
  error.value = ''
  if (twoFAEnabled.value) {
    const code = totpCode.value.trim()
    if (!/^\d{6}$/.test(code)) {
      error.value = t('paymentMethods.verify.totpInvalid')
      return
    }
    emit('verified', { totp_code: code, password: '' })
  } else {
    if (!password.value) {
      error.value = t('paymentMethods.verify.passwordRequired')
      return
    }
    emit('verified', { totp_code: '', password: password.value })
  }
}

const onCancel = () => {
  emit('update:open', false)
  emit('cancel')
}
</script>

<style>
.stepup-enter-active,
.stepup-leave-active {
  transition: opacity 0.2s ease;
}
.stepup-enter-from,
.stepup-leave-to {
  opacity: 0;
}
</style>
