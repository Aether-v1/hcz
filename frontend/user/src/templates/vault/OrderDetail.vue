<template>
  <div class="mx-auto w-full max-w-[1180px] px-6 pb-8">
    <nav class="my-3.5 mt-[18px] flex flex-wrap items-center gap-2 text-[13px] text-muted-foreground">
      <RouterLink to="/" class="hover:text-primary">{{ t('nav.home') }}</RouterLink>
      <span class="text-hairline-strong">/</span>
      <RouterLink to="/me/orders" class="hover:text-primary">{{ t('orders.title') }}</RouterLink>
      <span class="text-hairline-strong">/</span>
      <span class="font-semibold text-foreground">{{ t('orderDetail.title') }}</span>
    </nav>

    <header class="mb-[18px]">
      <h1 class="mb-1.5 text-3xl font-extrabold">{{ t('orderDetail.title') }}</h1>
      <p class="text-muted-foreground">{{ t('orderDetail.subtitle') }}</p>
    </header>

    <!-- Loading -->
    <div v-if="loading" class="rounded-xl border bg-card p-[22px]">
      <div class="mb-4 h-5 w-[35%] rounded bg-secondary"></div>
      <div class="h-[200px] rounded-md bg-secondary"></div>
    </div>

    <!-- 不存在 -->
    <div v-else-if="!order" class="my-6 flex flex-col items-center gap-3 rounded-xl border border-dashed py-16 text-center text-muted-foreground">
      <AlertCircle class="h-10 w-10 opacity-60" />
      <p>{{ t('orderDetail.notFound') }}</p>
      <Button class="mt-2 rounded-full" size="sm" @click="debouncedLoadOrder()">{{ t('errorBoundary.retry') }}</Button>
    </div>

    <template v-else>
      <div class="mb-[18px] flex flex-wrap items-start justify-between gap-[18px] rounded-xl border bg-card p-[22px]">
        <div>
          <div class="text-[11px] uppercase tracking-[0.06em] text-muted-foreground">{{ t('orders.orderNo') }}</div>
          <div class="mt-1 font-bold">{{ order.order_no }}</div>
          <div class="mt-1.5 text-[13px] text-muted-foreground">{{ t('orderDetail.createdAtLabel') }}：{{ formatDate(order.created_at) }}</div>
        </div>
        <div>
          <div class="text-[11px] uppercase tracking-[0.06em] text-muted-foreground">{{ t('orderDetail.amountTotal') }}</div>
          <div class="mt-1 text-2xl font-extrabold tabular-nums">{{ formatMoney(order.total_amount, order.currency) }}</div>
        </div>
        <div class="flex flex-wrap items-center gap-2.5">
          <Badge :variant="statusVariant(order.status)" class="rounded-full">{{ statusLabel(order.status) }}</Badge>
          <Button v-if="order.status === 'pending_payment'" as-child size="sm" class="rounded-full">
            <RouterLink :to="`/pay?order_no=${order.order_no}`">{{ t('orderDetail.payNow') }}</RouterLink>
          </Button>
          <Button v-if="order.status === 'pending_payment'" type="button" variant="outline" size="sm" class="rounded-full border-destructive text-destructive hover:bg-destructive/10" @click="cancelOrder">{{ t('orderDetail.cancel') }}</Button>
          <Button v-if="afterSaleCanInitiate(order.status)" type="button" variant="outline" size="sm" class="rounded-full" @click="afterSaleOpenForm()">{{ t('afterSale.notReceived') }}</Button>
        </div>
      </div>

      <VaultOrderBody
        :order="order"
        variant="user"
        :fulfillment-downloading="fulfillmentDownloading"
        @download="handleDownloadFulfillment"
      />

      <!-- After-Sale -->
      <div v-if="afterSaleTicket || afterSaleCanInitiate(order.status)" class="mt-[18px] rounded-xl border bg-card p-[22px]">
        <h2 class="mb-3 text-lg font-bold">{{ t('afterSale.title') }}</h2>
        <div v-if="afterSaleTicket" class="space-y-2 text-sm">
          <div class="flex items-center gap-2.5">
            <Badge :variant="afterSaleTicket.status === 'pending' ? 'warning' : afterSaleTicket.status === 'resolved' ? 'success' : 'destructive'" class="rounded-full">
              {{ afterSaleStatusLabel(afterSaleTicket.status) }}
            </Badge>
            <span v-if="afterSaleTicket.refund_amount" class="font-semibold">{{ t('afterSale.refundAmount') }}：{{ afterSaleTicket.refund_amount }} {{ afterSaleTicket.refund_currency }}</span>
          </div>
          <div class="text-muted-foreground">
            <div>{{ t('afterSale.reason') }}：{{ afterSaleTicket.reason }}</div>
            <div v-if="afterSaleTicket.description">{{ t('afterSale.description') }}：{{ afterSaleTicket.description }}</div>
            <div v-if="afterSaleTicket.admin_note">{{ t('afterSale.adminNote') }}：{{ afterSaleTicket.admin_note }}</div>
            <div class="text-xs">{{ t('afterSale.createdAt') }}：{{ formatDate(afterSaleTicket.created_at) }}</div>
          </div>
        </div>
        <div v-else class="flex items-center justify-between">
          <span class="text-sm text-muted-foreground">{{ t('afterSale.notReceivedHint') }}</span>
          <Button variant="outline" size="sm" class="rounded-full" @click="afterSaleOpenForm()">{{ t('afterSale.notReceived') }}</Button>
        </div>
      </div>

      <!-- After-Sale Form Modal -->
      <div v-if="afterSaleShowForm" class="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4" @click.self="afterSaleCloseForm()">
        <div class="w-full max-w-md rounded-2xl border bg-card p-6 shadow-xl">
          <h3 class="text-lg font-bold mb-4">{{ t('afterSale.notReceived') }}</h3>
          <div class="space-y-4">
            <div>
              <label class="block text-sm font-medium mb-1">{{ t('afterSale.reason') }} *</label>
              <input v-model="afterSaleForm.reason" type="text" class="w-full rounded-lg border border-input bg-background px-3 py-2 text-sm" :placeholder="t('afterSale.reasonPlaceholder')" />
            </div>
            <div>
              <label class="block text-sm font-medium mb-1">{{ t('afterSale.description') }}</label>
              <textarea v-model="afterSaleForm.description" rows="3" class="w-full rounded-lg border border-input bg-background px-3 py-2 text-sm" :placeholder="t('afterSale.descriptionPlaceholder')"></textarea>
            </div>
          </div>
          <div class="mt-6 flex justify-end gap-3">
            <Button variant="outline" size="sm" @click="afterSaleCloseForm()" :disabled="afterSaleSubmitting">{{ t('common.cancel') }}</Button>
            <Button size="sm" @click="afterSaleSubmit()" :disabled="afterSaleSubmitting">
              {{ afterSaleSubmitting ? t('afterSaleSubmitting') : t('common.confirm') }}
            </Button>
          </div>
        </div>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { AlertCircle } from 'lucide-vue-next'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import VaultOrderBody from './components/VaultOrderBody.vue'
import { useOrderDetail } from '../../composables/useOrderDetail'
import { useAfterSale } from '../../composables/useAfterSale'

const { t } = useI18n()

const {
  loading, order, debouncedLoadOrder, cancelOrder, fulfillmentDownloading, handleDownloadFulfillment,
  statusLabel, statusVariant, formatDate, formatMoney,
} = useOrderDetail()

const afterSale = useAfterSale(() => order.value?.id ?? null)
const {
  ticket: afterSaleTicket,
  showForm: afterSaleShowForm,
  form: afterSaleForm,
  submitting: afterSaleSubmitting,
  canInitiate: afterSaleCanInitiate,
  openForm: afterSaleOpenForm,
  closeForm: afterSaleCloseForm,
  submit: afterSaleSubmit,
  statusLabel: afterSaleStatusLabel,
} = afterSale

watch(() => order.value?.id, (id) => {
  if (id) afterSale.loadTicket()
})
</script>

