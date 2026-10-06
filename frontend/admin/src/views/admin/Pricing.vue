<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { adminAPI, type ExchangeRateState, type PricingPreviewResult, type ProfitGuardSetting } from '@/api/admin'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Switch } from '@/components/ui/switch'
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert'
import { Badge } from '@/components/ui/badge'
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { notifyError, notifySuccess } from '@/utils/notify'

const loading = ref(false)
const savingPg = ref(false)
const savingFx = ref(false)
const refreshing = ref(false)

const pg = reactive<ProfitGuardSetting>({
  enabled: false,
  require_cost_price: false,
  rate_safety_buffer_percent: 1,
  minimum_profit_amount_cny: 0,
  minimum_profit_rate_percent: 0,
})

const fx = reactive<ExchangeRateState>({
  site_currency: '',
  provider: '',
  auto_enabled: false,
  refresh_interval: 0,
  auto_rate: '',
  manual_fallback_rate: '',
  fetched_at: '',
  last_success_at: '',
  last_error: '',
  api_key_masked: '',
  status: '',
  effective_rate: '',
  effective_source: '',
  rate_safety_buffer_percent: 1,
  max_auto_rate_age_minutes: 1440,
  manual_rate_updated_at: '',
})

// FX 可编辑表单（与展示分离）
const fxForm = reactive({
  manual_fallback_rate: '',
  rate_safety_buffer_percent: 1,
  max_auto_rate_age_minutes: 1440,
})

const applyFx = (d: ExchangeRateState) => {
  Object.assign(fx, d)
  fxForm.manual_fallback_rate = d.manual_fallback_rate || ''
  fxForm.rate_safety_buffer_percent = Number(d.rate_safety_buffer_percent || 1)
  fxForm.max_auto_rate_age_minutes = Number(d.max_auto_rate_age_minutes || 1440)
}

const loadAll = async () => {
  loading.value = true
  try {
    const [pgRes, fxRes] = await Promise.all([
      adminAPI.getProfitGuardSettings(),
      adminAPI.getExchangeRateSettings(),
    ])
    if (pgRes.data?.data) Object.assign(pg, pgRes.data.data as ProfitGuardSetting)
    if (fxRes.data?.data) applyFx(fxRes.data.data as ExchangeRateState)
  } catch (err: any) {
    notifyError(err?.message || 'load failed')
  } finally {
    loading.value = false
  }
}

const savePg = async () => {
  savingPg.value = true
  try {
    const res = await adminAPI.updateProfitGuardSettings({ ...pg })
    if (res.data?.data) Object.assign(pg, res.data.data as ProfitGuardSetting)
    notifySuccess('Profit Guard 已保存')
  } catch (err: any) {
    notifyError(err?.message || 'save failed')
  } finally {
    savingPg.value = false
  }
}

const saveFx = async () => {
  savingFx.value = true
  try {
    await adminAPI.updateExchangeRateSettings({
      manual_fallback_rate: fxForm.manual_fallback_rate,
      rate_safety_buffer_percent: Number(fxForm.rate_safety_buffer_percent),
      max_auto_rate_age_minutes: Number(fxForm.max_auto_rate_age_minutes),
    })
    notifySuccess('汇率设置已保存')
    await reloadFx()
  } catch (err: any) {
    notifyError(err?.message || 'save failed')
  } finally {
    savingFx.value = false
  }
}

const reloadFx = async () => {
  try {
    const res = await adminAPI.getExchangeRateSettings()
    if (res.data?.data) applyFx(res.data.data as ExchangeRateState)
  } catch {
    /* ignore */
  }
}

const refreshFx = async () => {
  refreshing.value = true
  try {
    await adminAPI.refreshExchangeRate()
    notifySuccess('已触发刷新')
    await reloadFx()
  } catch (err: any) {
    notifyError(err?.message || 'refresh failed')
  } finally {
    refreshing.value = false
  }
}

// ---- 定价预览 ----
const previewProductId = ref<number | ''>('')
const previewQuantity = ref<number>(1)
const previewResult = ref<PricingPreviewResult | null>(null)
const previewLoading = ref(false)

const runPreview = async () => {
  if (!previewProductId.value) {
    notifyError('请输入商品 ID')
    return
  }
  previewLoading.value = true
  previewResult.value = null
  try {
    const res = await adminAPI.previewPricing({
      product_id: Number(previewProductId.value),
      quantity: Number(previewQuantity.value) || 1,
    })
    previewResult.value = (res.data?.data ?? null) as PricingPreviewResult | null
  } catch (err: any) {
    notifyError(err?.message || 'preview failed')
  } finally {
    previewLoading.value = false
  }
}

const warningText = (code: string): string => {
  switch (code) {
    case 'COST_MISSING': return '成本未配置且未豁免（COST_MISSING）'
    case 'COST_ENFORCED_MISSING': return 'Profit Guard 已开启，但该商品成本缺失'
    case 'PROFIT_BELOW_REQUIRED': return '预计利润低于最低门槛（BLOCKED）'
    case 'UNPROFITABLE_PRICE': return '成本 ≥ 售价，价格倒挂'
    case 'FX_UNAVAILABLE': return '汇率不可用（FX_UNAVAILABLE）'
    case 'RATE_STALE': return 'AUTO 汇率陈旧且无 MANUAL 兜底（RATE_STALE）'
    default: return code
  }
}

onMounted(loadAll)
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <h1 class="text-2xl font-semibold">定价 / Profit Guard</h1>
    </div>

    <!-- 启用顺序提示 -->
    <Alert>
      <AlertTitle>启用顺序（务必按序操作，避免误拒单）</AlertTitle>
      <AlertDescription>
        <ol class="mt-1 list-decimal space-y-1 pl-5">
          <li>录入真实成本（商品编辑 → Cost Price）</li>
          <li>确认 Pricing Preview 核算结果</li>
          <li>设置 FX Buffer（汇率安全系数）</li>
          <li>设置 Minimum Profit（最低利润门槛）</li>
          <li>开启 Require Cost Price</li>
          <li>最后开启 Profit Guard 总开关</li>
        </ol>
      </AlertDescription>
    </Alert>

    <Tabs default-value="guard">
      <TabsList>
        <TabsTrigger value="guard">Profit Guard 设置</TabsTrigger>
        <TabsTrigger value="fx">汇率设置</TabsTrigger>
        <TabsTrigger value="preview">定价预览</TabsTrigger>
      </TabsList>

      <!-- ============ Profit Guard ============ -->
      <TabsContent value="guard" class="space-y-4">
        <div class="rounded-xl border border-border bg-card">
          <div class="border-b border-border bg-muted/40 px-6 py-4">
            <h2 class="text-lg font-semibold">Profit Guard 资金安全闸门</h2>
            <p class="mt-1 text-xs text-muted-foreground">
              下单前的成本门 + 最低利润门。安全默认全部关闭，录入成本后再逐步开启。
            </p>
          </div>
          <div class="space-y-6 p-6">
            <div class="flex items-center gap-3 rounded-lg border border-border bg-muted/20 px-4 py-3">
              <Switch id="pg-enabled" v-model="pg.enabled" />
              <Label for="pg-enabled">启用 Profit Guard（拒单总开关）</Label>
            </div>
            <div class="flex items-center gap-3 rounded-lg border border-border bg-muted/20 px-4 py-3">
              <Switch id="pg-require-cost" v-model="pg.require_cost_price" />
              <Label for="pg-require-cost">Require Cost Price（成本未配置且未豁免 → 拒单）</Label>
            </div>

            <div class="grid grid-cols-1 gap-6 md:grid-cols-3">
              <div class="space-y-2">
                <label class="text-xs font-medium text-muted-foreground">Rate Safety Buffer（%，0-5）</label>
                <Input v-model.number="pg.rate_safety_buffer_percent" type="number" min="0" max="5" step="0.1" />
                <p class="text-xs text-muted-foreground">有效汇率 = 市场汇率 × (1 - buffer/100)</p>
              </div>
              <div class="space-y-2">
                <label class="text-xs font-medium text-muted-foreground">Minimum Profit Amount（CNY）</label>
                <Input v-model.number="pg.minimum_profit_amount_cny" type="number" min="0" step="0.01" />
                <p class="text-xs text-muted-foreground">固定最低利润额</p>
              </div>
              <div class="space-y-2">
                <label class="text-xs font-medium text-muted-foreground">Minimum Profit Rate（%）</label>
                <Input v-model.number="pg.minimum_profit_rate_percent" type="number" min="0" step="0.1" />
                <p class="text-xs text-muted-foreground">按销售额比例计算，取与固定额的较大值</p>
              </div>
            </div>

            <div class="flex justify-end">
              <Button :disabled="savingPg || loading" @click="savePg">保存</Button>
            </div>
          </div>
        </div>
      </TabsContent>

      <!-- ============ 汇率设置 ============ -->
      <TabsContent value="fx" class="space-y-4">
        <div class="rounded-xl border border-border bg-card">
          <div class="border-b border-border bg-muted/40 px-6 py-4">
            <h2 class="text-lg font-semibold">USDT 结算汇率</h2>
            <p class="mt-1 text-xs text-muted-foreground">
              三个概念务必区分：市场汇率（auto）、安全系数（buffer）、有效结算汇率（effective）。
            </p>
          </div>
          <div class="space-y-6 p-6">
            <!-- 三概念区分展示 -->
            <div class="grid grid-cols-1 gap-4 md:grid-cols-3">
              <div class="rounded-lg border border-border bg-blue-50 p-4 text-sm dark:bg-blue-950/20">
                <div class="text-xs font-medium text-blue-700 dark:text-blue-300">市场汇率（auto_rate）</div>
                <div class="mt-1 text-lg font-semibold">1 USDT = {{ fx.auto_rate || '—' }} {{ fx.site_currency }}</div>
                <div class="mt-1 text-xs text-muted-foreground">来源 {{ fx.provider || 'coingecko' }} · 更新于 {{ fx.fetched_at || '—' }}</div>
              </div>
              <div class="rounded-lg border border-border bg-amber-50 p-4 text-sm dark:bg-amber-950/20">
                <div class="text-xs font-medium text-amber-700 dark:text-amber-300">安全系数（buffer）</div>
                <div class="mt-1 text-lg font-semibold">{{ fx.rate_safety_buffer_percent }}%</div>
                <div class="mt-1 text-xs text-muted-foreground">有效汇率 = 市场 × (1 - buffer/100)</div>
              </div>
              <div class="rounded-lg border border-border bg-green-50 p-4 text-sm dark:bg-green-950/20">
                <div class="text-xs font-medium text-green-700 dark:text-green-300">有效结算汇率（effective_rate）</div>
                <div class="mt-1 text-lg font-semibold">1 USDT = {{ fx.effective_rate || '—' }} {{ fx.site_currency }}</div>
                <div class="mt-1 text-xs text-muted-foreground">
                  来源 {{ fx.effective_source || fx.status || '—' }}
                  <Badge v-if="fx.status" class="ml-2" :variant="fx.status === 'unavailable' ? 'destructive' : 'secondary'">{{ fx.status }}</Badge>
                </div>
              </div>
            </div>

            <div class="grid grid-cols-1 gap-6 md:grid-cols-2 text-sm">
              <div class="rounded-lg border border-border bg-muted/20 p-3">
                <span class="text-xs text-muted-foreground">手动备用汇率（MANUAL）：</span>
                1 USDT = {{ fx.manual_fallback_rate || '—' }} {{ fx.site_currency }}
                <div class="text-xs text-muted-foreground">更新于 {{ fx.manual_rate_updated_at || '—' }}</div>
              </div>
              <div class="rounded-lg border border-border bg-muted/20 p-3">
                <span class="text-xs text-muted-foreground">AUTO 最大陈旧度：</span>
                {{ fx.max_auto_rate_age_minutes }} 分钟
                <div v-if="fx.last_error" class="text-xs text-red-500">最近错误：{{ fx.last_error }}</div>
              </div>
            </div>

            <div class="grid grid-cols-1 gap-6 md:grid-cols-3">
              <div class="space-y-2">
                <label class="text-xs font-medium text-muted-foreground">手动备用汇率</label>
                <Input v-model="fxForm.manual_fallback_rate" placeholder="留空保留；0 清除兜底" />
              </div>
              <div class="space-y-2">
                <label class="text-xs font-medium text-muted-foreground">Rate Safety Buffer（%，0-5）</label>
                <Input v-model.number="fxForm.rate_safety_buffer_percent" type="number" min="0" max="5" step="0.1" />
              </div>
              <div class="space-y-2">
                <label class="text-xs font-medium text-muted-foreground">Max Auto Rate Age（分钟）</label>
                <Input v-model.number="fxForm.max_auto_rate_age_minutes" type="number" min="1" step="10" />
              </div>
            </div>

            <div class="flex gap-3">
              <Button :disabled="savingFx" @click="saveFx">保存</Button>
              <Button variant="secondary" :disabled="refreshing" @click="refreshFx">
                {{ refreshing ? '刷新中…' : '立即刷新' }}
              </Button>
            </div>
          </div>
        </div>
      </TabsContent>

      <!-- ============ 定价预览 ============ -->
      <TabsContent value="preview" class="space-y-4">
        <div class="rounded-xl border border-border bg-card">
          <div class="border-b border-border bg-muted/40 px-6 py-4">
            <h2 class="text-lg font-semibold">Pricing Preview（仅管理员可见）</h2>
            <p class="mt-1 text-xs text-muted-foreground">
              模拟该商品在当前 Profit Guard + 汇率配置下的核算明细，不实际下单。
            </p>
          </div>
          <div class="space-y-6 p-6">
            <div class="flex flex-wrap items-end gap-3">
              <div class="space-y-2">
                <label class="text-xs font-medium text-muted-foreground">商品 ID</label>
                <Input v-model.number="previewProductId" type="number" min="1" placeholder="例如 12" class="w-40" />
              </div>
              <div class="space-y-2">
                <label class="text-xs font-medium text-muted-foreground">数量</label>
                <Input v-model.number="previewQuantity" type="number" min="1" step="1" class="w-28" />
              </div>
              <Button :disabled="previewLoading" @click="runPreview">
                {{ previewLoading ? '核算中…' : '开始核算' }}
              </Button>
            </div>

            <!-- 红色风险 banner -->
            <template v-if="previewResult && previewResult.warnings?.length">
              <Alert v-for="w in previewResult.warnings" :key="w" variant="destructive">
                <AlertTitle>风险</AlertTitle>
                <AlertDescription>{{ warningText(w) }}</AlertDescription>
              </Alert>
            </template>

            <div v-if="previewResult" class="space-y-4">
              <div class="flex items-center gap-2">
                <span class="text-sm font-medium">Guard Result：</span>
                <Badge :variant="previewResult.guard_result === 'PASS' ? 'secondary' : 'destructive'">
                  {{ previewResult.guard_result }}
                </Badge>
                <span v-if="previewResult.guard_reason" class="text-xs text-red-500">{{ previewResult.guard_reason }}</span>
              </div>

              <div class="grid grid-cols-1 gap-3 md:grid-cols-2 lg:grid-cols-3 text-sm">
                <div class="rounded-lg border border-border bg-muted/20 p-3"><span class="text-xs text-muted-foreground">销售价 CNY：</span>{{ previewResult.sale_price_cny }}</div>
                <div class="rounded-lg border border-border bg-muted/20 p-3"><span class="text-xs text-muted-foreground">成本价 CNY：</span>{{ previewResult.cost_price_cny }}<span v-if="previewResult.is_cost_exempt" class="ml-1 text-xs text-emerald-600">(豁免)</span></div>
                <div class="rounded-lg border border-border bg-muted/20 p-3"><span class="text-xs text-muted-foreground">最大 Affiliate 成本 CNY：</span>{{ previewResult.max_affiliate_cost_cny }}</div>
                <div class="rounded-lg border border-border bg-muted/20 p-3"><span class="text-xs text-muted-foreground">已纳入 Fee Cost CNY：</span>{{ previewResult.fee_cost_cny }}</div>
                <div class="rounded-lg border border-border bg-muted/20 p-3"><span class="text-xs text-muted-foreground">市场汇率：</span>{{ previewResult.market_rate || '—' }}</div>
                <div class="rounded-lg border border-border bg-muted/20 p-3"><span class="text-xs text-muted-foreground">安全 Buffer：</span>{{ previewResult.rate_safety_buffer_percent }}%</div>
                <div class="rounded-lg border border-border bg-muted/20 p-3"><span class="text-xs text-muted-foreground">有效汇率：</span>{{ previewResult.effective_rate || '—' }} <span class="text-xs text-muted-foreground">({{ previewResult.effective_source || '' }})</span></div>
                <div class="rounded-lg border border-border bg-muted/20 p-3"><span class="text-xs text-muted-foreground">理论 USDT：</span>{{ previewResult.theoretical_usdt || '—' }}</div>
                <div class="rounded-lg border border-border bg-muted/20 p-3"><span class="text-xs text-muted-foreground">最终应收 USDT：</span>{{ previewResult.final_usdt || '—' }}</div>
                <div class="rounded-lg border border-border bg-muted/20 p-3"><span class="text-xs text-muted-foreground">预计利润 CNY：</span>{{ previewResult.expected_profit_cny || '—' }}</div>
                <div class="rounded-lg border border-border bg-muted/20 p-3"><span class="text-xs text-muted-foreground">最低利润门槛 CNY：</span>{{ previewResult.required_profit_cny }}</div>
              </div>
            </div>
          </div>
        </div>
      </TabsContent>
    </Tabs>
  </div>
</template>
