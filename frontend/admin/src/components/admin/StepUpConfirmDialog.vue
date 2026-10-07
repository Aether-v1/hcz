<script setup lang="ts">
import { ref, watch } from 'vue'
import { adminAPI } from '@/api/admin'
import { Button } from '@/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'

const props = withDefaults(
  defineProps<{
    open: boolean
    title?: string
    description?: string
    confirmText?: string
    requireReason?: boolean
    danger?: boolean
    // 本次高风险动作的绑定范围，形如 wallet.adjust:user:123。challenge 将绑定该 scope，
    // 后端强制校验与本次动作一致，防止跨动作复用。
    scope?: string
  }>(),
  {
    title: '高风险操作确认',
    description: '',
    confirmText: '确认执行',
    requireReason: false,
    danger: false,
    scope: '',
  },
)

const emit = defineEmits<{
  (e: 'update:open', value: boolean): void
  (e: 'confirm', payload: { reason?: string; idempotencyKey: string; challengeToken: string }): void
}>()

const code = ref('')
const reason = ref('')
const challengeToken = ref('')
const challengeLoading = ref(false)
const challengeError = ref('')
const idempotencyKey = ref('')

const reset = () => {
  code.value = ''
  reason.value = ''
  challengeToken.value = ''
  challengeError.value = ''
  challengeLoading.value = false
  // 每次打开都生成新的幂等键；同一次操作的重试复用此键。
  idempotencyKey.value = (typeof crypto !== 'undefined' && 'randomUUID' in crypto)
    ? crypto.randomUUID()
    : `${Date.now()}-${Math.random().toString(36).slice(2)}`
}

watch(
  () => props.open,
  (v) => {
    if (v) reset()
  },
  { immediate: true },
)

const fetchChallenge = async () => {
  const c = code.value.trim()
  if (!/^\d{6}$/.test(c)) {
    challengeError.value = '请输入 6 位 TOTP 验证码'
    return
  }
  if (!props.scope) {
    challengeError.value = '缺少操作范围，请关闭后重新发起该操作'
    return
  }
  challengeLoading.value = true
  challengeError.value = ''
  try {
    const res = await adminAPI.stepUp(c, props.scope)
    challengeToken.value = res.data?.data?.challenge_token || ''
    if (!challengeToken.value) {
      challengeError.value = '未获取到挑战令牌，请重试'
    }
  } catch (err: any) {
    challengeToken.value = ''
    challengeError.value = err?.response?.data?.msg || err?.message || 'TOTP 校验失败'
  } finally {
    challengeLoading.value = false
  }
}

const canConfirm = () => {
  if (!challengeToken.value) return false
  if (props.requireReason && !reason.value.trim()) return false
  return true
}

const confirm = () => {
  if (!canConfirm()) return
  emit('confirm', {
    reason: props.requireReason ? reason.value.trim() : undefined,
    idempotencyKey: idempotencyKey.value,
    challengeToken: challengeToken.value,
  })
}

const close = () => emit('update:open', false)
</script>

<template>
  <Dialog :open="open" @update:open="(v: boolean) => emit('update:open', v)">
    <DialogContent class="sm:max-w-md">
      <DialogHeader>
        <DialogTitle>{{ title }}</DialogTitle>
        <DialogDescription v-if="description">{{ description }}</DialogDescription>
      </DialogHeader>

      <div class="space-y-4 py-2">
        <div>
          <Label class="mb-1 block text-sm">TOTP 验证码（身份验证器 App）</Label>
          <div class="flex gap-2">
            <Input
              v-model="code"
              inputmode="numeric"
              maxlength="6"
              placeholder="6 位数字"
              class="font-mono tracking-widest"
              @keyup.enter="fetchChallenge"
            />
            <Button variant="outline" size="sm" :disabled="challengeLoading" @click="fetchChallenge">
              {{ challengeLoading ? '验证中…' : '获取挑战令牌' }}
            </Button>
          </div>
          <p v-if="challengeToken" class="mt-2 break-all rounded-md border border-success/40 bg-success/10 p-2 text-xs text-success">
            挑战令牌已获取（5 分钟内有效）
          </p>
          <p v-else-if="challengeError" class="mt-2 text-xs text-destructive">{{ challengeError }}</p>
        </div>

        <div v-if="requireReason">
          <Label class="mb-1 block text-sm">操作原因 <span class="text-destructive">*</span></Label>
          <Textarea v-model="reason" rows="2" placeholder="请填写本次高风险操作的原因/备注" />
        </div>

        <div class="rounded-md border border-border bg-muted/50 p-2">
          <div class="text-xs text-muted-foreground">幂等键（本次操作自动生成，重试复用）</div>
          <div class="mt-1 break-all font-mono text-xs">{{ idempotencyKey }}</div>
        </div>
      </div>

      <DialogFooter class="gap-2">
        <Button variant="outline" @click="close">取消</Button>
        <Button :variant="danger ? 'destructive' : 'default'" :disabled="!canConfirm()" @click="confirm">
          {{ confirmText }}
        </Button>
      </DialogFooter>
    </DialogContent>
  </Dialog>
</template>
