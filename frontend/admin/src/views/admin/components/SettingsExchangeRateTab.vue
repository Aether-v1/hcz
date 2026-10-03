<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { adminAPI } from '@/api/admin'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { notifyError, notifySuccess } from '@/utils/notify'

const loading = ref(false)
const saving = ref(false)
const refreshing = ref(false)

const state = reactive({
  site_currency: '',
  provider: 'coingecko',
  auto_enabled: false,
  refresh_interval: 5,
  auto_rate: '',
  manual_fallback_rate: '',
  effective_rate: '',
  effective_source: '',
  fetched_at: '',
  last_success_at: '',
  last_error: '',
  status: '',
  api_key_masked: '',
})

const form = reactive({
  api_key: '',
  auto_enabled: false,
  refresh_interval: 5,
  manual_fallback_rate: '',
})

const load = async () => {
  loading.value = true
  try {
    const res = await adminAPI.getExchangeRateSettings()
    const d = (res.data?.data ?? {}) as Record<string, unknown>
    state.site_currency = String(d.site_currency ?? '')
    state.provider = String(d.provider ?? 'coingecko')
    state.auto_enabled = !!d.auto_enabled
    state.refresh_interval = Number(d.refresh_interval ?? 5)
    state.auto_rate = String(d.auto_rate ?? '')
    state.manual_fallback_rate = String(d.manual_fallback_rate ?? '')
    state.effective_rate = String(d.effective_rate ?? '')
    state.effective_source = String(d.effective_source ?? '')
    state.fetched_at = String(d.fetched_at ?? '')
    state.last_success_at = String(d.last_success_at ?? '')
    state.last_error = String(d.last_error ?? '')
    state.status = String(d.status ?? '')
    state.api_key_masked = String(d.api_key_masked ?? '')
    form.auto_enabled = state.auto_enabled
    form.refresh_interval = state.refresh_interval || 5
    form.manual_fallback_rate = state.manual_fallback_rate
    form.api_key = ''
  } catch (err) {
    notifyError((err as Error)?.message || 'load failed')
  } finally {
    loading.value = false
  }
}

const save = async () => {
  saving.value = true
  try {
    await adminAPI.updateExchangeRateSettings({
      api_key: form.api_key,
      auto_enabled: form.auto_enabled,
      refresh_interval_min: Number(form.refresh_interval),
      manual_fallback_rate: form.manual_fallback_rate,
    })
    notifySuccess('saved')
    await load()
  } catch (err) {
    notifyError((err as Error)?.message || 'save failed')
  } finally {
    saving.value = false
  }
}

const refresh = async () => {
  refreshing.value = true
  try {
    await adminAPI.refreshExchangeRate()
    notifySuccess('refreshed')
    await load()
  } catch (err) {
    notifyError((err as Error)?.message || 'refresh failed')
  } finally {
    refreshing.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="space-y-6">
    <div class="rounded-xl border border-border bg-card">
      <div class="border-b border-border bg-muted/40 px-6 py-4">
        <h2 class="text-lg font-semibold">USDT 结算汇率</h2>
        <p class="mt-1 text-xs text-muted-foreground">
          商品按全站币种计价，实际从 USDT 钱包扣款。此汇率仅用于商品订单，与钱包充值的 Gateway 汇率相互独立。
        </p>
      </div>
      <div class="space-y-6 p-6">
        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div class="rounded-lg border border-border bg-muted/20 p-4 text-sm">
            <div class="text-xs text-muted-foreground">全站币种</div>
            <div class="mt-1 font-medium">{{ state.site_currency || '-' }}</div>
          </div>
          <div class="rounded-lg border border-border bg-muted/20 p-4 text-sm">
            <div class="text-xs text-muted-foreground">当前生效汇率 / 来源</div>
            <div class="mt-1 font-medium">
              1 USDT = {{ state.effective_rate || '—' }} {{ state.site_currency }}
              <span class="ml-2 text-muted-foreground">({{ state.effective_source || state.status || '-' }})</span>
            </div>
          </div>
          <div class="rounded-lg border border-border bg-muted/20 p-4 text-sm">
            <div class="text-xs text-muted-foreground">自动汇率</div>
            <div class="mt-1 font-medium">1 USDT = {{ state.auto_rate || '—' }} {{ state.site_currency }}</div>
            <div class="mt-1 text-xs text-muted-foreground">更新于 {{ state.fetched_at || '-' }}</div>
          </div>
          <div class="rounded-lg border border-border bg-muted/20 p-4 text-sm">
            <div class="text-xs text-muted-foreground">手动备用汇率</div>
            <div class="mt-1 font-medium">1 USDT = {{ state.manual_fallback_rate || '—' }} {{ state.site_currency }}</div>
            <div v-if="state.last_error" class="mt-1 text-xs text-red-500">错误：{{ state.last_error }}</div>
          </div>
        </div>

        <div class="flex items-center gap-3 rounded-lg border border-border bg-muted/20 px-4 py-3">
          <Switch id="er-auto" v-model="form.auto_enabled" />
          <Label for="er-auto">自动汇率（CoinGecko）</Label>
        </div>

        <div class="grid grid-cols-1 gap-6 md:grid-cols-2">
          <div class="space-y-2">
            <label class="text-xs font-medium text-muted-foreground">CoinGecko API Key{{ state.api_key_masked ? `（已设置：${state.api_key_masked}）` : '' }}</label>
            <Input v-model="form.api_key" type="password" placeholder="留空保留现有 Key" />
          </div>
          <div class="space-y-2">
            <label class="text-xs font-medium text-muted-foreground">刷新间隔（分钟）</label>
            <Input v-model.number="form.refresh_interval" type="number" min="1" />
          </div>
          <div class="space-y-2">
            <label class="text-xs font-medium text-muted-foreground">手动备用汇率（1 USDT = ? {{ state.site_currency }}）</label>
            <Input v-model="form.manual_fallback_rate" placeholder="例如 7.20" />
          </div>
        </div>

        <div class="flex gap-3">
          <Button :disabled="saving || loading" @click="save">保存</Button>
          <Button variant="secondary" :disabled="refreshing" @click="refresh">
            {{ refreshing ? '刷新中…' : '立即刷新' }}
          </Button>
        </div>
      </div>
    </div>
  </div>
</template>
