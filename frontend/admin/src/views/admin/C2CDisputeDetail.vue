<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { c2cAPI, type C2CDispute, type C2CDisputeResult } from '@/api/c2c'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { Textarea } from '@/components/ui/textarea'
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog'
import { formatDate } from '@/utils/format'
import { notifySuccess, notifyError } from '@/utils/notify'

const route = useRoute()
const router = useRouter()
const id = computed(() => Number(route.params.id))

const loading = ref(true)
const dispute = ref<C2CDispute | null>(null)

// 仲裁对话框
const dialogOpen = ref(false)
const submitting = ref(false)
const arbitrateResult = ref<C2CDisputeResult>('release_to_buyer')
const reason = ref('')
const adminNote = ref('')
const authChallenge = ref('')
const idemKey = ref('')

const isResolved = computed(() => dispute.value?.status === 'resolved')

const safeJson = (raw?: string | Record<string, unknown> | Array<unknown>) => {
  if (raw === undefined || raw === null || raw === '') return ''
  if (typeof raw !== 'string') return JSON.stringify(raw, null, 2)
  try {
    return JSON.stringify(JSON.parse(raw), null, 2)
  } catch {
    return raw
  }
}

const tradeStatusLabel = (status?: string) => {
  const map: Record<string, string> = {
    pending_payment: '待付款',
    paid: '已付款',
    completed: '已完成',
    canceled: '已取消',
    expired: '已过期',
    disputed: '争议中',
  }
  return status ? (map[status] || status) : '-'
}

const resultLabel = (result?: string) => {
  if (result === 'release_to_buyer') return '放行给买家'
  if (result === 'return_to_seller') return '退回给卖家'
  return result || '-'
}

const fetchDetail = async () => {
  loading.value = true
  try {
    const res = await c2cAPI.getDispute(id.value)
    dispute.value = res.data.data
  } catch {
    dispute.value = null
  } finally {
    loading.value = false
  }
}

const openArbitrate = (result: C2CDisputeResult) => {
  arbitrateResult.value = result
  reason.value = ''
  adminNote.value = ''
  authChallenge.value = ''
  idemKey.value = crypto.randomUUID()
  dialogOpen.value = true
}

const doArbitrate = async () => {
  if (!reason.value.trim()) {
    notifyError('请填写仲裁理由')
    return
  }
  if (!authChallenge.value.trim()) {
    notifyError('请输入 Step-Up 2FA challenge token')
    return
  }
  submitting.value = true
  try {
    await c2cAPI.arbitrate(
      {
        trade_id: dispute.value?.trade_id as number,
        result: arbitrateResult.value,
        reason: reason.value.trim(),
        admin_note: adminNote.value.trim() || undefined,
      },
      {
        'Idempotency-Key': idemKey.value,
        'X-Auth-Challenge': authChallenge.value.trim(),
      },
    )
    notifySuccess('仲裁已提交')
    dialogOpen.value = false
    await fetchDetail()
  } catch {
    // 错误已统一提示
  } finally {
    submitting.value = false
  }
}

onMounted(() => fetchDetail())
</script>

<template>
  <div class="space-y-6">
    <div class="flex items-center gap-3">
      <Button variant="outline" size="sm" @click="router.push('/c2c/disputes')">← 返回申诉列表</Button>
      <h1 class="text-2xl font-semibold">申诉详情 / 仲裁</h1>
    </div>

    <div v-if="loading" class="rounded-xl border border-border bg-card p-8 text-center text-muted-foreground">加载中…</div>

    <template v-else-if="dispute">
      <!-- 申诉信息 -->
      <div class="rounded-xl border border-border bg-card p-6 shadow-sm space-y-4">
        <div class="flex items-center justify-between">
          <div>
            <div class="text-lg font-semibold">申诉 #{{ dispute.id }}</div>
            <div class="mt-1 text-xs text-muted-foreground">{{ formatDate(dispute.created_at) }}</div>
          </div>
          <span
            class="inline-flex rounded-full border px-3 py-1 text-sm"
            :class="isResolved ? 'border-success/30 bg-success/10 text-success' : 'border-destructive/30 bg-destructive/10 text-destructive'"
          >
            {{ isResolved ? '已处理' : '待处理' }}
          </span>
        </div>
        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div>
            <div class="text-xs text-muted-foreground">发起人</div>
            <div class="mt-1 text-sm text-foreground">
              #{{ dispute.initiator_user_id }}
              <span v-if="dispute.initiator?.email" class="ml-2 text-muted-foreground">{{ dispute.initiator.email }}</span>
            </div>
          </div>
          <div>
            <div class="text-xs text-muted-foreground">申诉原因</div>
            <div class="mt-1 text-sm text-foreground">{{ dispute.reason }}</div>
          </div>
          <div v-if="dispute.description" class="md:col-span-2">
            <div class="text-xs text-muted-foreground">问题描述</div>
            <div class="mt-1 whitespace-pre-wrap text-sm text-foreground">{{ dispute.description }}</div>
          </div>
          <div v-if="safeJson(dispute.evidence)" class="md:col-span-2">
            <div class="text-xs text-muted-foreground">证据材料</div>
            <pre class="mt-1 max-h-64 overflow-auto rounded-lg border border-border bg-muted/40 p-3 text-xs font-mono">{{ safeJson(dispute.evidence) }}</pre>
          </div>
        </div>
      </div>

      <!-- 交易信息 -->
      <div v-if="dispute.trade" class="rounded-xl border border-border bg-card p-6 shadow-sm space-y-4">
        <h2 class="text-base font-semibold">关联交易</h2>
        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div>
            <div class="text-xs text-muted-foreground">交易ID</div>
            <div class="mt-1 font-mono text-sm">#{{ dispute.trade.id }}</div>
          </div>
          <div>
            <div class="text-xs text-muted-foreground">交易状态</div>
            <div class="mt-1 text-sm">{{ tradeStatusLabel(dispute.trade.status) }}</div>
          </div>
          <div>
            <div class="text-xs text-muted-foreground">买家ID</div>
            <div class="mt-1 font-mono text-sm">#{{ dispute.trade.buyer_user_id }}</div>
          </div>
          <div>
            <div class="text-xs text-muted-foreground">卖家ID</div>
            <div class="mt-1 font-mono text-sm">#{{ dispute.trade.seller_user_id }}</div>
          </div>
          <div>
            <div class="text-xs text-muted-foreground">USDT 数量</div>
            <div class="mt-1 font-mono text-sm">{{ dispute.trade.amount_usdt }} USDT</div>
          </div>
          <div>
            <div class="text-xs text-muted-foreground">法币金额</div>
            <div class="mt-1 font-mono text-sm">{{ dispute.trade.fiat_amount }} {{ dispute.trade.fiat_currency }}</div>
          </div>
          <div v-if="safeJson(dispute.trade.payment_method_snapshot)" class="md:col-span-2">
            <div class="text-xs text-muted-foreground">支付方式快照</div>
            <pre class="mt-1 max-h-64 overflow-auto rounded-lg border border-border bg-muted/40 p-3 text-xs font-mono">{{ safeJson(dispute.trade.payment_method_snapshot) }}</pre>
          </div>
        </div>
      </div>

      <!-- 仲裁结果（已处理） -->
      <div v-if="isResolved" class="rounded-xl border border-success/40 bg-success/5 p-6 shadow-sm space-y-3">
        <h2 class="text-base font-semibold text-success">仲裁结果</h2>
        <div class="grid grid-cols-1 gap-4 md:grid-cols-2">
          <div>
            <div class="text-xs text-muted-foreground">裁决</div>
            <div class="mt-1 text-sm font-semibold text-foreground">{{ resultLabel(dispute.result) }}</div>
          </div>
          <div v-if="dispute.admin_note">
            <div class="text-xs text-muted-foreground">管理员备注</div>
            <div class="mt-1 whitespace-pre-wrap text-sm text-foreground">{{ dispute.admin_note }}</div>
          </div>
        </div>
      </div>

      <!-- 仲裁操作区 -->
      <div v-else class="rounded-xl border border-border bg-card p-6 shadow-sm space-y-4">
        <h2 class="text-base font-semibold">仲裁操作</h2>
        <p class="text-xs text-muted-foreground">
          「放行给买家」将解冻卖家冻结资金并结算给买家，交易标记为完成；「退回给卖家」将解冻资金退回卖家并恢复对应挂单，交易标记为已取消。操作需要 Step-Up 2FA 二次验证。
        </p>
        <div class="flex flex-wrap gap-3">
          <Button @click="openArbitrate('release_to_buyer')">放行给买家</Button>
          <Button variant="destructive" @click="openArbitrate('return_to_seller')">退回给卖家</Button>
        </div>
      </div>

      <!-- 仲裁对话框 -->
      <Dialog v-model:open="dialogOpen">
        <DialogContent class="max-w-lg">
          <DialogHeader>
            <DialogTitle>{{ arbitrateResult === 'release_to_buyer' ? '仲裁：放行给买家' : '仲裁：退回给卖家' }}</DialogTitle>
            <DialogDescription>提交后将执行资金结算/解冻，操作不可撤销。</DialogDescription>
          </DialogHeader>
          <div class="space-y-4 py-2">
            <div>
              <Label class="mb-1 block text-xs text-muted-foreground">仲裁理由（必填）</Label>
              <Textarea v-model="reason" rows="2" placeholder="请填写裁决依据" />
            </div>
            <div>
              <Label class="mb-1 block text-xs text-muted-foreground">管理员备注（可选）</Label>
              <Textarea v-model="adminNote" rows="2" placeholder="内部备注，用户不可见" />
            </div>
            <div>
              <Label class="mb-1 block text-xs text-muted-foreground">Step-Up 2FA Challenge Token（必填）</Label>
              <Input v-model="authChallenge" placeholder="通过 Admin 2FA 验证后获取的 challenge token" class="font-mono" />
            </div>
            <div>
              <Label class="mb-1 block text-xs text-muted-foreground">Idempotency-Key（自动生成）</Label>
              <Input :model-value="idemKey" readonly class="font-mono text-xs text-muted-foreground" />
            </div>
          </div>
          <DialogFooter>
            <Button variant="outline" :disabled="submitting" @click="dialogOpen = false">取消</Button>
            <Button
              :variant="arbitrateResult === 'return_to_seller' ? 'destructive' : 'default'"
              :disabled="submitting || !reason.trim() || !authChallenge.trim()"
              @click="doArbitrate"
            >
              {{ submitting ? '提交中…' : '确认仲裁' }}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </template>
  </div>
</template>
