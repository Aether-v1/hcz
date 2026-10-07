export type TranslateFn = (...args: any[]) => string

/** 取 `points.*` 下的枚举文案；无对应 key 时返回空串，由调用方决定回退（如回退服务端 reason）。 */
const pointsEnumLabel = (t: TranslateFn, path: string, value?: string): string => {
  if (!value) return ''
  const key = `points.${path}.${value}`
  const translated = t(key)
  return translated === key ? '' : translated
}

export const pointsExchangeStatusLabel = (t: TranslateFn, status?: string) => {
  return pointsEnumLabel(t, 'exchangeStatus', status) || status || '-'
}

export const pointsExchangeStatusVariant = (status?: string): BadgeTone => {
  switch (status) {
    case 'COMPLETED':
      return 'success'
    case 'PROCESSING':
      return 'accent'
    case 'PENDING':
      return 'warning'
    case 'FAILED':
      return 'danger'
    case 'CANCELLED':
    default:
      return 'neutral'
  }
}

// 积分流水的行首标题：优先本地化 action_type，缺失则回退服务端 reason。
export const pointsLedgerLabel = (t: TranslateFn, actionType?: string, reason?: string) => {
  return pointsEnumLabel(t, 'actions', actionType) || reason || actionType || '-'
}

// 商品不可兑换原因（后端稳定业务码，非文案）。
export const pointsReasonCodeLabel = (t: TranslateFn, reasonCode?: string) => {
  return pointsEnumLabel(t, 'reasonCodes', reasonCode)
}

export const orderStatusLabel = (t: TranslateFn, status?: string) => {
  if (!status) return '-'
  const map: Record<string, string> = {
    pending_recharge: t('order.status.pending_recharge'),
    processing: t('order.status.processing'),
    failed: t('order.status.failed'),
    completed: t('order.status.completed'),
    canceled: t('order.status.canceled'),
    pending_payment: t('order.status.pending_payment'),
    paid: t('order.status.paid'),
    fulfilling: t('order.status.fulfilling'),
    partially_delivered: t('order.status.partially_delivered'),
    partially_refunded: t('order.status.partially_refunded'),
    delivered: t('order.status.delivered'),
    expired: t('order.status.expired'),
    refunded: t('order.status.refunded'),
  }
  return map[status] || status
}

export type BadgeTone = 'success' | 'warning' | 'info' | 'danger' | 'accent' | 'neutral'

export const orderStatusVariant = (status?: string): BadgeTone => {
  switch (status) {
    case 'pending_recharge':
      return 'warning'
    case 'processing':
      return 'accent'
    case 'completed':
      return 'success'
    case 'failed':
      return 'danger'
    case 'pending_payment':
    case 'partially_refunded':
      return 'warning'
    case 'paid':
    case 'delivered':
      return 'success'
    case 'partially_delivered':
    case 'refunded':
      return 'info'
    case 'fulfilling':
      return 'accent'
    case 'expired':
      return 'danger'
    case 'canceled':
    default:
      return 'neutral'
  }
}

