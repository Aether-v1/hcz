<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { adminAPI, type AdminAffiliateSetting, type AffiliateLevelRate } from '@/api/admin'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { Switch } from '@/components/ui/switch'
import { Label } from '@/components/ui/label'
import { notifyError, notifySuccess } from '@/utils/notify'

const { t } = useI18n()
const loading = ref(false)

const MAX_LEVELS = 10

const form = reactive({
  enabled: false,
  commission_rate: 0,
  confirm_days: 0,
  min_withdraw_amount: 0,
  withdraw_channels_text: '',
  max_level: 1,
})

const levelRates = ref<AffiliateLevelRate[]>(
  Array.from({ length: MAX_LEVELS }, (_, i) => ({ level: i + 1, enabled: false, rate: 0 })),
)

const normalizeNumber = (value: unknown, fallback: number) => {
  if (value === null || value === undefined || value === '') return fallback
  const parsed = Number(value)
  if (Number.isNaN(parsed)) return fallback
  return parsed
}

const clampNumber = (value: unknown, min: number, max: number, fallback: number) => {
  const parsed = normalizeNumber(value, fallback)
  if (parsed < min) return min
  if (parsed > max) return max
  return parsed
}

const splitChannels = (raw: string) => {
  return raw
    .split(/\r?\n|,/)
    .map((item) => item.trim())
    .filter((item) => item !== '')
}

const joinChannels = (items: unknown) => {
  if (!Array.isArray(items)) return ''
  return items
    .map((item) => String(item || '').trim())
    .filter((item) => item !== '')
    .join('\n')
}

const buildLevelRates = (raw: unknown): AffiliateLevelRate[] => {
  const base = Array.from({ length: MAX_LEVELS }, (_, i) => ({
    level: i + 1,
    enabled: false,
    rate: 0,
  }))
  if (Array.isArray(raw)) {
    for (const item of raw) {
      const lv = Number((item as AffiliateLevelRate)?.level)
      if (Number.isInteger(lv) && lv >= 1 && lv <= MAX_LEVELS) {
        base[lv - 1] = {
          level: lv,
          enabled: Boolean((item as AffiliateLevelRate)?.enabled),
          rate: clampNumber((item as AffiliateLevelRate)?.rate, 0, 100, 0),
        }
      }
    }
  }
  return base
}

// 总比例：仅统计已启用且层级不超过 max_level 的行
const totalRate = computed(() => {
  let sum = 0
  for (const row of levelRates.value) {
    if (row.enabled && row.level <= form.max_level) {
      sum += Number(row.rate) || 0
    }
  }
  return Math.round(sum * 100) / 100
})

const totalRateExceeded = computed(() => totalRate.value > 100)

const applyData = (data: AdminAffiliateSetting) => {
  form.enabled = Boolean(data.enabled)
  form.commission_rate = clampNumber(data.commission_rate, 0, 100, 0)
  form.confirm_days = clampNumber(data.confirm_days, 0, 3650, 0)
  form.min_withdraw_amount = Math.max(normalizeNumber(data.min_withdraw_amount, 0), 0)
  form.withdraw_channels_text = joinChannels(data.withdraw_channels)
  form.max_level = clampNumber(data.max_level, 1, MAX_LEVELS, 1)
  levelRates.value = buildLevelRates(data.level_rates)
}

const fetchSettings = async () => {
  loading.value = true
  try {
    const res = await adminAPI.getAffiliateSettings()
    if (res.data && res.data.data) {
      applyData(res.data.data as AdminAffiliateSetting)
    }
  } catch (err: any) {
    notifyError(err?.response?.data?.message || t('admin.settings.alerts.loadFailed'))
  } finally {
    loading.value = false
  }
}

const saveSettings = async () => {
  loading.value = true
  try {
    // 固定长度 10 的数组；超出 max_level 的行强制不启用
    const levelRatesPayload: AffiliateLevelRate[] = levelRates.value.map((row) => ({
      level: row.level,
      enabled: row.level <= form.max_level ? Boolean(row.enabled) : false,
      rate: clampNumber(row.rate, 0, 100, 0),
    }))
    const payload = {
      enabled: form.enabled,
      commission_rate: clampNumber(form.commission_rate, 0, 100, 0),
      confirm_days: clampNumber(form.confirm_days, 0, 3650, 0),
      min_withdraw_amount: Math.max(normalizeNumber(form.min_withdraw_amount, 0), 0),
      withdraw_channels: splitChannels(form.withdraw_channels_text),
      max_level: clampNumber(form.max_level, 1, MAX_LEVELS, 1),
      level_rates: levelRatesPayload,
    }
    const response = await adminAPI.updateAffiliateSettings(payload)
    const data = response.data?.data as AdminAffiliateSetting | undefined
    if (data) {
      applyData(data)
    }
    notifySuccess(t('admin.settings.alerts.saveSuccess'))
  } catch (err: any) {
    notifyError(err?.response?.data?.message || t('admin.settings.alerts.saveFailed'))
  } finally {
    loading.value = false
  }
}

onMounted(fetchSettings)
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <h1 class="text-2xl font-semibold">{{ t('admin.settings.affiliate.title') }}</h1>
    </div>

    <div class="rounded-xl border border-border bg-card">
      <div class="border-b border-border bg-muted/40 px-6 py-4">
        <p class="text-sm text-muted-foreground">{{ t('admin.settings.affiliate.subtitle') }}</p>
      </div>
      <div class="space-y-6 p-6">
        <div class="flex flex-col gap-3 rounded-lg border border-border bg-muted/20 px-4 py-3 sm:flex-row sm:items-center">
          <Switch id="affiliate-enabled" v-model="form.enabled" />
          <Label for="affiliate-enabled" class="text-sm font-medium">{{ t('admin.settings.affiliate.enabled') }}</Label>
        </div>

        <div class="grid grid-cols-1 gap-6 md:grid-cols-3">
          <div class="space-y-2">
            <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.affiliate.commissionRate') }}</label>
            <Input v-model.number="form.commission_rate" type="number" min="0" max="100" step="0.01" />
            <p class="text-xs text-muted-foreground">{{ t('admin.settings.affiliate.commissionRateHint') }}</p>
          </div>
          <div class="space-y-2">
            <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.affiliate.confirmDays') }}</label>
            <Input v-model.number="form.confirm_days" type="number" min="0" max="3650" step="1" />
            <p class="text-xs text-muted-foreground">{{ t('admin.settings.affiliate.confirmDaysHint') }}</p>
          </div>
          <div class="space-y-2">
            <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.affiliate.minWithdrawAmount') }}</label>
            <Input v-model.number="form.min_withdraw_amount" type="number" min="0" step="0.01" />
            <p class="text-xs text-muted-foreground">{{ t('admin.settings.affiliate.minWithdrawAmountHint') }}</p>
          </div>
        </div>

        <div class="space-y-3">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.affiliate.levelRates') }}</label>
            <div class="text-right">
              <div class="text-sm" :class="totalRateExceeded ? 'font-semibold text-red-600' : 'text-muted-foreground'">
                {{ t('admin.settings.affiliate.totalRate') }}: {{ totalRate.toFixed(2) }}%
              </div>
              <p v-if="totalRateExceeded" class="text-xs text-red-600">{{ t('admin.settings.affiliate.totalRateWarning') }}</p>
            </div>
          </div>

          <div class="grid grid-cols-1 gap-6 md:grid-cols-3">
            <div class="space-y-2">
              <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.affiliate.maxLevel') }}</label>
              <Input v-model.number="form.max_level" type="number" min="1" max="10" step="1" />
            </div>
          </div>

          <div class="space-y-1 rounded-lg border border-border bg-muted/20 p-4">
            <div class="mb-2 flex items-center gap-3 px-1 text-xs text-muted-foreground">
              <span class="w-10">{{ t('admin.settings.affiliate.level') }}</span>
              <span class="w-16">{{ t('admin.settings.affiliate.levelEnabled') }}</span>
              <span class="max-w-[200px] flex-1">{{ t('admin.settings.affiliate.levelRate') }}</span>
            </div>
            <div
              v-for="row in levelRates"
              :key="row.level"
              class="flex items-center gap-3 px-1 py-1"
              :class="row.level > form.max_level ? 'opacity-40' : ''"
            >
              <span class="w-10 font-mono text-sm font-medium text-foreground">L{{ row.level }}</span>
              <span class="w-16">
                <Switch :disabled="row.level > form.max_level" v-model="row.enabled" />
              </span>
              <div class="relative max-w-[200px] flex-1">
                <Input
                  :disabled="row.level > form.max_level"
                  v-model.number="row.rate"
                  type="number"
                  min="0"
                  max="100"
                  step="0.01"
                  class="pr-8"
                />
                <span class="pointer-events-none absolute right-2 top-1/2 -translate-y-1/2 text-xs text-muted-foreground">%</span>
              </div>
            </div>
          </div>
        </div>

        <div class="space-y-2">
          <label class="text-xs font-medium text-muted-foreground">{{ t('admin.settings.affiliate.withdrawChannels') }}</label>
          <Textarea
            v-model="form.withdraw_channels_text"
            rows="5"
            :placeholder="t('admin.settings.affiliate.withdrawChannelsPlaceholder')"
          />
          <p class="text-xs text-muted-foreground">{{ t('admin.settings.affiliate.withdrawChannelsHint') }}</p>
        </div>

        <div class="flex justify-end pt-2">
          <Button class="w-full sm:w-auto" @click="saveSettings" :disabled="loading">
            {{ t('admin.settings.actions.save') }}
          </Button>
        </div>
      </div>
    </div>
  </div>
</template>
