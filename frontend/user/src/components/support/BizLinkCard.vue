<template>
  <div
    v-if="bizType && bizId"
    class="flex items-center gap-3 rounded-xl border border-border bg-muted/40 px-4 py-3 text-sm"
  >
    <Link2 class="h-4 w-4 flex-none text-muted-foreground" />
    <span class="text-muted-foreground">{{ t('support.biz_link') }}</span>
    <router-link :to="route" class="font-medium text-primary hover:underline">
      {{ typeLabel }} #{{ bizId }}
    </router-link>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Link2 } from 'lucide-vue-next'

/**
 * 工单关联业务卡片：展示 biz_type / biz_id 并跳转到对应业务页。
 * order → /orders/:id；recharge → /recharge-orders/:id；
 * withdraw → 提现记录；c2c → 订单列表（当前项目无 C2C 页面）。
 */
const props = defineProps<{
  bizType?: string | null
  bizId?: number | null
}>()

const { t } = useI18n()

const typeLabel = computed(() => {
  switch (props.bizType) {
    case 'order':
      return t('support.biz_type_order')
    case 'withdraw':
      return t('support.biz_type_withdraw')
    case 'c2c':
      return t('support.biz_type_c2c')
    case 'recharge':
      return t('support.biz_type_recharge')
    default:
      return props.bizType || ''
  }
})

const route = computed(() => {
  const id = props.bizId ?? 0
  switch (props.bizType) {
    case 'order':
      return `/orders/${id}`
    case 'recharge':
      return `/recharge-orders/${id}`
    case 'withdraw':
      return '/me/wallet/withdrawal-history'
    case 'c2c':
    default:
      return '/me/orders'
  }
})
</script>
