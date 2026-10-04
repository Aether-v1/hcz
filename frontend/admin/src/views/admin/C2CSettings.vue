<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { c2cAPI, type C2CSettings } from '@/api/c2c'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { notifySuccess, notifyError } from '@/utils/notify'

const loading = ref(true)
const saving = ref(false)

const form = reactive({
  enabled: false,
  trade_timeout_minutes: 0,
  new_user_cooldown_hours: 0,
  min_trade_usdt: 0,
  max_trade_usdt: 0,
  daily_trade_limit_usdt: 0,
  max_cancel_count: 0,
  fee_rate: 0,
})

const fetchSettings = async () => {
  loading.value = true
  try {
    const res = await c2cAPI.getSettings()
    const data = (res.data.data || {}) as C2CSettings
    Object.assign(form, {
      enabled: !!data.enabled,
      trade_timeout_minutes: Number(data.trade_timeout_minutes) || 0,
      new_user_cooldown_hours: Number(data.new_user_cooldown_hours) || 0,
      min_trade_usdt: Number(data.min_trade_usdt) || 0,
      max_trade_usdt: Number(data.max_trade_usdt) || 0,
      daily_trade_limit_usdt: Number(data.daily_trade_limit_usdt) || 0,
      max_cancel_count: Number(data.max_cancel_count) || 0,
      fee_rate: Number(data.fee_rate) || 0,
    })
  } catch {
    // 错误已统一提示
  } finally {
    loading.value = false
  }
}

const save = async () => {
  saving.value = true
  try {
    await c2cAPI.updateSettings({
      enabled: form.enabled,
      trade_timeout_minutes: Number(form.trade_timeout_minutes),
      new_user_cooldown_hours: Number(form.new_user_cooldown_hours),
      min_trade_usdt: Number(form.min_trade_usdt),
      max_trade_usdt: Number(form.max_trade_usdt),
      daily_trade_limit_usdt: Number(form.daily_trade_limit_usdt),
      max_cancel_count: Number(form.max_cancel_count),
      fee_rate: Number(form.fee_rate),
    })
    notifySuccess('C2C 设置已保存')
  } catch {
    notifyError('保存失败')
  } finally {
    saving.value = false
  }
}

onMounted(() => fetchSettings())
</script>

<template>
  <div class="space-y-6">
    <h1 class="text-2xl font-semibold">C2C 设置</h1>

    <Card v-if="!loading">
      <CardHeader>
        <CardTitle>全局参数</CardTitle>
        <CardDescription>调整 C2C 交易的全局开关与限额参数，保存后即时生效。</CardDescription>
      </CardHeader>
      <CardContent class="space-y-6">
        <div class="flex items-center justify-between">
          <div>
            <Label class="text-sm">启用 C2C 功能</Label>
            <p class="text-xs text-muted-foreground mt-1">关闭后用户前台将不可见 C2C 交易入口。</p>
          </div>
          <Switch v-model="form.enabled" />
        </div>

        <div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <div>
            <Label class="mb-1 block text-xs text-muted-foreground">交易超时时间（分钟）</Label>
            <Input v-model.number="form.trade_timeout_minutes" type="number" min="0" />
          </div>
          <div>
            <Label class="mb-1 block text-xs text-muted-foreground">新用户冷却时间（小时）</Label>
            <Input v-model.number="form.new_user_cooldown_hours" type="number" min="0" />
          </div>
          <div>
            <Label class="mb-1 block text-xs text-muted-foreground">最大取消次数</Label>
            <Input v-model.number="form.max_cancel_count" type="number" min="0" />
          </div>
          <div>
            <Label class="mb-1 block text-xs text-muted-foreground">最小交易 USDT</Label>
            <Input v-model.number="form.min_trade_usdt" type="number" min="0" />
          </div>
          <div>
            <Label class="mb-1 block text-xs text-muted-foreground">最大交易 USDT</Label>
            <Input v-model.number="form.max_trade_usdt" type="number" min="0" />
          </div>
          <div>
            <Label class="mb-1 block text-xs text-muted-foreground">每日交易限额 USDT</Label>
            <Input v-model.number="form.daily_trade_limit_usdt" type="number" min="0" />
          </div>
          <div>
            <Label class="mb-1 block text-xs text-muted-foreground">手续费率（第一版固定为 0）</Label>
            <Input v-model.number="form.fee_rate" type="number" min="0" step="0.0001" disabled />
            <p class="mt-1 text-xs text-muted-foreground">当前版本暂不开放手续费率调整。</p>
          </div>
        </div>

        <div class="flex justify-end gap-2 border-t border-border pt-4">
          <Button variant="outline" :disabled="saving" @click="fetchSettings">重置</Button>
          <Button :disabled="saving" @click="save">{{ saving ? '保存中…' : '保存设置' }}</Button>
        </div>
      </CardContent>
    </Card>

    <div v-else class="rounded-xl border border-border bg-card p-8 text-center text-muted-foreground">加载中…</div>
  </div>
</template>
